package application

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/omnibuildplatform/omni-orchestrator/app"
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"github.com/omnibuildplatform/omni-orchestrator/common/engine"
	"github.com/omnibuildplatform/omni-orchestrator/common/store"
	"go.uber.org/zap"
	"net/http"
)

type Orchestrator struct {
	jobManager  common.JobManager
	logManager  common.LogManager
	appConfig   appconfig.Config
	routerGroup *gin.RouterGroup
	logger      *zap.Logger
}

func NewOrchestrator(config appconfig.Config, group *gin.RouterGroup, logger *zap.Logger) (*Orchestrator, error) {
	factory, err := common.NewFactory()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to initialize manager factory: %v\n", err))
	}

	engineFactory := engine.NewEngineFactory(logger)
	jobEngine, err := engineFactory.CreateJobEngine(config.Engine, logger)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to initialize job engine: %v\n", err))
	}

	storeFactory := store.NewStoreFactory(logger)
	jobStore, err := storeFactory.CreateJobStore(config.PersistentStore, logger)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to initialize job store: %v\n", err))
	}

	logManager, err := factory.NewLogManager(jobEngine, jobStore, *app.AppConfig.LogManager, app.Logger)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to initialize log manager: %v\n", err))
	}

	jobManager, err := factory.NewJobManager(jobEngine, jobStore, *app.AppConfig.JobManager, app.Logger)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to initialize job manager: %v\n", err))
	}

	return &Orchestrator{
		jobManager:  jobManager,
		logManager:  logManager,
		appConfig:   config,
		routerGroup: group,
		logger:      logger,
	}, nil

}

func (r *Orchestrator) Initialize() error {
	r.routerGroup.POST("/", r.createJob)
	r.routerGroup.GET("/", r.queryJob)
	r.routerGroup.DELETE("/", r.deleteJob)
	r.routerGroup.POST("/logs", r.logs)
	return nil

}

func (r *Orchestrator) createJob(c *gin.Context) {
	//parameter validation
	var job common.Job
	var err error
	if err = c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// valid job type
	jobKind := r.jobManager.AcceptableJob(context.TODO(), job)
	if jobKind == common.JobUnrecognized {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unrecognized job kind"})
		return
	}
	if len(job.Spec) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job spec empty"})
		return
	}
	err = r.jobManager.CreateJob(context.TODO(), &job, jobKind)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, job)
}

func (r *Orchestrator) queryJob(c *gin.Context) {
	service := c.Query("service")
	task := c.Query("task")
	domain := c.Query("domain")
	jobID := c.Query("jobID")
	if domain == "" || service == "" || task == "" || jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required parameter"})
		return
	}
	job, err := r.jobManager.GetJob(context.TODO(), service, task, domain, jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (r *Orchestrator) getJob(c *gin.Context) {
	jobID := c.Param("jobID")
	_ = jobID
}

func (r *Orchestrator) deleteJob(c *gin.Context) {
	jobID := c.Param("jobID")
	_ = jobID
}

func (r *Orchestrator) logs(c *gin.Context) {
	jobID := c.Param("jobID")
	_ = jobID
}

func (r *Orchestrator) StartLoop() error {
	if r.logManager != nil {
		err := r.logManager.StartLoop()
		if err != nil {
			return err
		}
	}

	if r.jobManager != nil {
		//register log manger handler
		r.jobManager.RegisterJobChangeNotifyChannel(r.logManager.GetJobChangeChannel())
		//start up loop
		err := r.jobManager.StartLoop()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Orchestrator) Close() {
	//close job first
	if r.jobManager != nil {
		r.jobManager.Close()
	}
	r.logger.Info("job manager closed.")
	if r.logManager != nil {
		r.logManager.Close()
	}
	r.logger.Info("log manager closed.")
}
