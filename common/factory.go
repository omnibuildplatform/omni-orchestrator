package common

import (
	"github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
)

type factoryImpl struct {
}

func NewFactory() (Factory, error) {
	return nil, nil
}

func (f *factoryImpl) Close() {

}

func (f *factoryImpl) NewJobManager(config config.JobManager, logger *zap.Logger, engine JobEngine, store JobStore) (JobManager, error) {
	return NewJobManagerImpl(engine, store, config, logger)
}
func (f *factoryImpl) NewLogManager(config config.LobManager, logger *zap.Logger) (LogManager, error) {
	return nil, nil
}
