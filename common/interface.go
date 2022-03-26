package common

import (
	"context"
	"github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
	"io"
	"time"
)

type JobState string

const (
	JobCreated JobState = "JobCreated"
	JobRunning JobState = "JobRunning"
	JobSucceed JobState = "JobSucceed"
	JobFailed  JobState = "JobFailed"
)

type StepState string

const (
	StepCreated StepState = "StepCreated"
	StepRunning StepState = "StepRunning"
	StepSucceed StepState = "StepSucceed"
	StepFailed  StepState = "StepFailed"
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

	JobEvent struct {
		Service string
		Domain  string
		ID      string
		UserID  string
		Task    string
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
		Duration      int                    `json:"duration"`
		Steps         []Step                 `json:"steps"`
	}

	Step struct {
		Index     int       `json:"index"`
		Name      string    `json:"name"`
		State     StepState `json:"state"`
		StartTime time.Time `json:"startTime"`
		EndTime   time.Time `json:"endTime"`
		Message   string    `json:"message"`
		LogUrl    string    `json:"logUrl"`
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
		NewLogManager(engine JobEngine, store JobStore, config config.LogManager, logger *zap.Logger) (LogManager, error)
	}

	JobManager interface {
		Closeable
		GetName() string
		CreateJob(ctx context.Context, j Job, kind JobKind) error
		AcceptableJob(ctx context.Context, j Job) JobKind
		DeleteJob(ctx context.Context, jobID string) error
		GetJob(ctx context.Context, jobID string) Job
		StartLoop() error
		RegisterJobChangeNotifyChannel(ch chan<- Job)
	}

	LogManager interface {
		Closeable
		GetName() string
		StartLoop() error
		GetJobChangeChannel() chan<- Job
	}

	JobEngine interface {
		Closeable
		Initialize() error
		GetName() string
		GetSupportedJobs() []JobKind
		BuildOSImage(ctx context.Context, job Job, spec JobImageBuildPara) error
		GetJob(ctx context.Context, domain, jobID string) (*Job, error)
		StartLoop() error
		GetJobEventChannel() <-chan JobEvent
		FetchJobStepLog(ctx context.Context, domain, jobID, stepName string) (io.ReadCloser, error)
	}

	JobStore interface {
		Closeable
		Initialize() error
		GetName() string
		CreateJob(ctx context.Context, job *Job) (string, error)
	}
)
