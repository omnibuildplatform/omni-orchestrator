package application

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/omnibuildplatform/omni-orchestrator/app"
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"github.com/omnibuildplatform/omni-orchestrator/common/engine"
	"github.com/omnibuildplatform/omni-orchestrator/common/store"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

const (
	LogTimeUUIDHeader  = "logTimeUUID"
	LogCompletedHeader = "logCompleted"
)

type Orchestrator struct {
	jobManager    common.JobManager
	logManager    common.LogManager
	appConfig     appconfig.Config
	routerGroup   *gin.RouterGroup
	logger        *zap.Logger
	paraValidator *validator.Validate
}

type CreateJobRequest struct {
	Service string                 `form:"service" json:"service" validate:"required"`
	Task    string                 `form:"task" json:"task" validate:"required"`
	Domain  string                 `form:"domain" json:"domain" validate:"required"`
	UserID  string                 `json:"userID" json:"userID"  validate:"required"`
	Spec    map[string]interface{} `json:"spec" json:"spec" validate:"required"`
	Engine  string                 `json:"engine" json:"engine" validate:"required"`
}

func (q CreateJobRequest) GetJobResource() common.Job {
	return common.Job{
		JobIdentity: common.JobIdentity{
			Service: q.Service,
			Task:    q.Task,
			Domain:  q.Domain,
		},
		UserID: q.UserID,
		Spec:   q.Spec,
		Engine: q.Engine,
	}
}

type QueryJobRequest struct {
	Service string `form:"service" json:"service" validate:"required"`
	Task    string `form:"task" json:"task" validate:"required"`
	Domain  string `form:"domain" json:"domain" validate:"required"`
	ID      string `form:"ID" json:"ID" validate:"required,uuid"`
}

func (q QueryJobRequest) GetJobIdentity() common.JobIdentity {
	return common.JobIdentity{
		Service: q.Service,
		Task:    q.Task,
		Domain:  q.Domain,
		ID:      q.ID,
	}
}

type QueryJobStepLogRequest struct {
	QueryJobRequest
	StepID        string `form:"stepID" json:"stepID" validate:"required,number"`
	StartTimeUUID string `form:"startTimeUUID" json:"startTime"`
	MaxRecord     int    `form:"maxRecord" json:"maxRecord"`
}

func (q QueryJobStepLogRequest) GetJobIdentity() common.JobIdentity {
	return common.JobIdentity{
		Service: q.Service,
		Task:    q.Task,
		Domain:  q.Domain,
		ID:      q.ID,
	}
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
		jobManager:    jobManager,
		logManager:    logManager,
		appConfig:     config,
		routerGroup:   group,
		logger:        logger,
		paraValidator: validator.New(),
	}, nil

}

func (r *Orchestrator) Initialize() error {
	r.routerGroup.POST("/", r.createJob)
	r.routerGroup.GET("/", r.queryJob)
	r.routerGroup.DELETE("/", r.deleteJob)
	r.routerGroup.GET("/logs", r.logs)
	return nil

}

// @BasePath /v1/

// CreateJob godoc
// @Summary Create Job
// @Param	body	body 	CreateJobRequest	true		"body for create a job"
// @Description Create a job with specified SPEC
// @Tags Job
// @Accept json
// @Produce json
// @Success 201 object common.Job
// @Router /jobs [post]
func (r *Orchestrator) createJob(c *gin.Context) {
	//parameter validation
	var jobCreate CreateJobRequest
	var err error
	if err = c.ShouldBindJSON(&jobCreate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = r.paraValidator.Struct(jobCreate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	job := jobCreate.GetJobResource()

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
	var jobQuery QueryJobRequest
	var err error
	if err = c.ShouldBindQuery(&jobQuery); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err = r.paraValidator.Struct(jobQuery); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	job, err := r.jobManager.GetJob(context.TODO(), jobQuery.GetJobIdentity())
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
	var jobStep QueryJobStepLogRequest
	var err error
	if err = c.ShouldBindQuery(&jobStep); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err = r.paraValidator.Struct(jobStep); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	logPart, err := r.logManager.GetJobStepLogs(context.TODO(), jobStep.GetJobIdentity(), jobStep.StepID, jobStep.StartTimeUUID, jobStep.MaxRecord)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to collect step logs %s", err)})
		return
	}
	//additional headers
	if len(logPart.Data) != 0 {
		c.Header(LogTimeUUIDHeader, logPart.MaxJobTimeUUID)
		c.Header(LogCompletedHeader, strconv.FormatBool(logPart.Finished))
		c.Data(http.StatusOK, "text/plain", logPart.Data)
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "job log not found"})
	}

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
