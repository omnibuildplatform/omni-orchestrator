package common

import (
	"errors"
	"fmt"
	"github.com/omnibuildplatform/omni-orchestrator/common/config"
	"github.com/omnibuildplatform/omni-orchestrator/common/engine"
	"github.com/omnibuildplatform/omni-orchestrator/common/manager"
	"go.uber.org/zap"
)

type factoryImpl struct {
}

func NewFactory() (Factory, error) {
	return nil, nil
}

func (f *factoryImpl) Close() {

}

func (f *factoryImpl) NewJobManager(config config.JobManager, logger *zap.Logger) (JobManager, error) {
	engFactory := engine.NewEngineFactory(logger)
	engine, err := engFactory.NewJobEngine(config.Engine, logger)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to setup engine for job manager %v", err))
	}
	return manager.NewJobManagerImpl(engine, config, logger)
}
func (f *factoryImpl) NewLogManager(config config.LobManager, logger *zap.Logger) (LogManager, error) {
	return nil, nil
}
