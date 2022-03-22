package common

import (
	"context"
	"fmt"
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
	var err error
	switch kind {
	case JobImageBuild:
		var para JobImageBuildPara
		err = mapstructure.Decode(job.Spec, para)
		if err != nil {
			return err
		}
	}
	return nil
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
		m.logger.Info(fmt.Sprintf("configured engine %s while job asked %s", job.Engine, m.engine.GetName()))
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
