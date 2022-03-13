package manager

import (
	"fmt"
	"github.com/lxc/lxd/shared/logger"
	"github.com/mitchellh/mapstructure"
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
)

type jobManagerImpl struct {
	engine common.JobEngine
	logger *zap.Logger
	config appconfig.JobManager
}

func NewJobManagerImpl(engine common.JobEngine, config appconfig.JobManager, logger *zap.Logger) (common.JobManager, error) {
	return &jobManagerImpl{
		engine: engine,
		logger: logger,
		config: config,
	}, nil
}

func (m *jobManagerImpl) GetName() string {
	return ""
}

func (m *jobManagerImpl) CreateJob(job common.Job, kind common.JobKind) error {
	var err error
	switch kind {
	case common.JobImageBuild:
		var para common.JobImageBuildPara
		err = mapstructure.Decode(job.Spec, para)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *jobManagerImpl) jobSupported(kind common.JobKind) bool {
	for _, j := range m.engine.GetSupportedJobs() {
		if kind == j {
			return true
		}
	}
	return false
}
func (m *jobManagerImpl) AcceptableJob(job common.Job) common.JobKind {
	if job.Engine != m.engine.GetName() {
		logger.Info(fmt.Sprintf("configured engine %s while job asked %s", job.Engine, m.engine.GetName()))
		return common.JobUnrecognized
	}
	for _, j := range common.AvailableJobs {
		if string(j) == job.Task && m.jobSupported(j) {
			return j
		}
	}
	logger.Info(fmt.Sprintf("configured engine doesn't support job task %s", job.Task))
	return common.JobUnrecognized
}
func (m *jobManagerImpl) DeleteJob(jobID string) error {
	return nil
}
func (m *jobManagerImpl) GetJob(jobID string) common.Job {
	return common.Job{}
}
func (m *jobManagerImpl) Close() {

}
