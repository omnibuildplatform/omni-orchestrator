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
	JobStopped JobState = "JobStopped"
)

const (
	LogCompleteFlag = "0F80573E-61DA-4C1B-9CDE-396F139D63DD"
	JobUnrecognized = "unrecognized"
)

type StepState string

const (
	StepCreated StepState = "StepCreated"
	StepRunning StepState = "StepRunning"
	StepSucceed StepState = "StepSucceed"
	StepFailed  StepState = "StepFailed"
	StepStopped StepState = "StepStopped"
)

type JobKind string

type (
	JobImageBuildFromReleasePara struct {
		Version      string   `json:"version"`
		Packages     []string `json:"packages"`
		Format       string   `json:"format"`
		Architecture string   `json:"architecture"`
	}

	JobIdentity struct {
		Service         string            `json:"service"`
		Task            string            `json:"task"`
		Domain          string            `json:"domain"`
		ID              string            `json:"id"`
		ExtraIdentities map[string]string `json:"extraIdentities,omitempty"`
	}

	Job struct {
		JobIdentity
		UserID    string                 `json:"userID"`
		Spec      map[string]interface{} `json:"spec"`
		Engine    string                 `json:"engine"`
		StartTime time.Time              `json:"startTime"`
		EndTime   time.Time              `json:"endTime"`
		State     JobState               `json:"state"`
		Steps     []Step                 `json:"steps"`
		Detail    string                 `json:"detail"`
		Version   int32                  `json:"version"`
	}

	Step struct {
		ID        int       `json:"id"`
		Name      string    `json:"name"`
		State     StepState `json:"state"`
		StartTime time.Time `json:"startTime"`
		EndTime   time.Time `json:"endTime"`
		Message   string    `json:"message"`
	}

	JobStepLog struct {
		JobIdentity
		StepID  string
		LogTime time.Time
		Data    []byte
	}

	JobLogPart struct {
		Data           []byte
		MaxJobTimeUUID string
		Finished       bool
	}

	Closeable interface {
		Close()
	}
	Reloadable interface {
		Reload()
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
		Reloadable
		GetName() string
		CreateJob(ctx context.Context, j *Job, kind string) error
		AcceptableJob(ctx context.Context, j Job) string
		DeleteJob(ctx context.Context, jobID JobIdentity) error
		GetJob(ctx context.Context, jobID JobIdentity) (Job, error)
		BatchGetJobs(ctx context.Context, jobID JobIdentity, IDs []string) ([]Job, error)
		StartLoop() error
		RegisterJobChangeNotifyChannel(ch chan<- Job)
	}

	LogManager interface {
		Closeable
		Reloadable
		GetName() string
		StartLoop() error
		DeleteJob(ctx context.Context, jobID JobIdentity) error
		GetJobChangeChannel() chan<- Job
		GetJobStepLogs(ctx context.Context, jobID JobIdentity, stepID string, startTime string, maxRecord int) (*JobLogPart, error)
	}

	JobEngine interface {
		Closeable
		Reloadable
		Initialize() error
		GetName() string
		GetSupportedJobs() []string
		CreateJob(ctx context.Context, job *Job) error
		GetJobStatus(ctx context.Context, job Job) (*Job, error)
		DeleteJob(ctx context.Context, jobID JobIdentity) error
		StartLoop() error
		GetJobEventChannel() <-chan JobIdentity
		FetchJobStepLog(ctx context.Context, jobID JobIdentity, stepName string) (io.ReadCloser, error)
	}

	JobStore interface {
		Closeable
		Reloadable
		Initialize() error
		GetName() string
		CreateJob(ctx context.Context, job *Job, ttl int64) error
		UpdateJobStatus(ctx context.Context, job *Job, version int32) error
		GetJob(ctx context.Context, jobID JobIdentity) (Job, error)
		BatchGetJobs(ctx context.Context, jobID JobIdentity, IDs []string) ([]Job, error)
		DeleteJob(ctx context.Context, jobID JobIdentity) error
		DeleteJobLog(ctx context.Context, jobID JobIdentity) error
		InsertJobStepLog(ctx context.Context, log *JobStepLog, ttl int64) error
		GetJobStepLogs(ctx context.Context, jobID JobIdentity, stepID, startTime string, maxRecord int) (*JobLogPart, error)
		DeleteJobStepLog(ctx context.Context, log *JobStepLog) error
		JobStepLogFinished(ctx context.Context, jobID JobIdentity, stepID string) bool
	}
)
