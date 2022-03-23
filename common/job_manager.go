package common

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
)

type jobManagerImpl struct {
	engine JobEngine
	logger *zap.Logger
	store  JobStore
	config appconfig.JobManager
}

func NewJobManagerImpl(engine JobEngine, store JobStore, config appconfig.JobManager, logger *zap.Logger) (JobManager, error) {
	return &jobManagerImpl{
		engine: engine,
		logger: logger,
		config: config,
		store:  store,
	}, nil
}

func (m *jobManagerImpl) GetName() string {
	return ""
}

func (m *jobManagerImpl) CreateJob(ctx context.Context, job Job, kind JobKind) error {
	switch kind {
	case JobImageBuild:
		return m.createBuildISOJob(ctx, job)
	}
	return nil
}

func (m jobManagerImpl) createBuildISOJob(ctx context.Context, job Job) error {
	var err error
	var para = JobImageBuildPara{}
	err = mapstructure.Decode(job.Spec, &para)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to decode job specification %s", err))
	}
	//create record
	job.ID = uuid.New().String()
	//start up job
	//update job information
	return m.engine.BuildOSImage(ctx, job, para)
}

func (m *jobManagerImpl) jobSupported(kind JobKind) bool {
	for _, j := range m.engine.GetSupportedJobs() {
		if kind == j {
			return true
		}
	}
	return false
}
func (m *jobManagerImpl) AcceptableJob(ctx context.Context, job Job) JobKind {
	if job.Engine != m.engine.GetName() {
		m.logger.Info(fmt.Sprintf("configured engine %s while job asked for engine %s", m.engine.GetName(), job.Engine))
		return JobUnrecognized
	}
	for _, j := range AvailableJobs {
		if string(j) == job.Task && m.jobSupported(j) {
			return j
		}
	}
	m.logger.Info(fmt.Sprintf("configured engine doesn't support job task %s", job.Task))
	return JobUnrecognized
}
func (m *jobManagerImpl) DeleteJob(ctx context.Context, jobID string) error {
	return nil
}
func (m *jobManagerImpl) GetJob(ctx context.Context, jobID string) Job {
	return Job{}
}
func (m *jobManagerImpl) Close() {

}
