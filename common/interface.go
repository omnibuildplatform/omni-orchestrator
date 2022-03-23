package common

import (
	"context"
	"github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
	"time"
)

type JobState int

const (
	JobCreated JobState = iota
	JobRunning
	JobSucceed
	JobFailed
	JobTimeout
)

type JobKind string

const (
	JobImageBuild   JobKind = "buildImage"
	JobRPMBuild     JobKind = "buildRPM"
	JobUnrecognized JobKind = "unrecognized"
)

var AvailableJobs = []JobKind{JobImageBuild, JobRPMBuild}

type (
	JobImageBuildPara struct {
		Version      string   `json:"version"`
		Packages     []string `json:"packages"`
		Format       string   `json:"format"`
		Architecture string   `json:"architecture"`
	}

	Job struct {
		Service       string                 `json:"service" binding:"required"`
		Domain        string                 `json:"domain" binding:"required"`
		ID            string                 `json:"id"`
		UserID        string                 `json:"userID" binding:"required"`
		Task          string                 `json:"task" binding:"required"`
		Spec          map[string]interface{} `json:"spec"`
		Engine        string                 `json:"engine" binding:"required"`
		StartTime     time.Time              `json:"startTime"`
		EndTime       time.Time              `json:"endTime"`
		State         JobState               `json:"state"`
		JobResult     string                 `json:"jobResult"`
		FailureDetail string                 `json:"failureDetail"`
		LogUrl        string                 `json:"logUrl"`
		Duration      int                    `json:"duration"`
		TotalStep     int                    `json:"totalStep"`
		CurrentStep   int                    `json:"currentStep"`
	}

	Closeable interface {
		Close()
	}

	EngineFactory interface {
		CreateJobEngine(config config.Engine, logger *zap.Logger) (JobEngine, error)
	}
	StoreFactory interface {
		CreateJobStore(config config.PersistentStore, logger *zap.Logger) (JobStore, error)
	}

	ManagerFactory interface {
		NewJobManager(engine JobEngine, store JobStore, config config.JobManager, logger *zap.Logger) (JobManager, error)
		NewLogManager(config config.LobManager, logger *zap.Logger) (LogManager, error)
	}

	JobManager interface {
		Closeable
		GetName() string
		CreateJob(ctx context.Context, j Job, kind JobKind) error
		AcceptableJob(ctx context.Context, j Job) JobKind
		DeleteJob(ctx context.Context, jobID string) error
		GetJob(ctx context.Context, jobID string) Job
	}

	LogManager interface {
		Closeable
		GetName() string
		StartLoop() error
	}

	JobEngine interface {
		Closeable
		Initialize() error
		GetName() string
		GetSupportedJobs() []JobKind
		BuildOSImage(ctx context.Context, job Job, spec JobImageBuildPara) error
		GetJob(ctx context.Context, domain, jobID string) (*Job, error)
	}

	JobStore interface {
		Closeable
		Initialize() error
		GetName() string
		CreateJob(ctx context.Context, job *Job) (string, error)
	}
)
