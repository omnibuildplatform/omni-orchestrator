package common

import (
	"github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
)

type factoryImpl struct {
}

func NewFactory() (ManagerFactory, error) {
	return &factoryImpl{}, nil
}

func (f *factoryImpl) NewJobManager(engine JobEngine, store JobStore, config config.JobManager, logger *zap.Logger) (JobManager, error) {
	return NewJobManagerImpl(engine, store, config, logger)
}
func (f *factoryImpl) NewLogManager(config config.LobManager, logger *zap.Logger) (LogManager, error) {
	return nil, nil
}
