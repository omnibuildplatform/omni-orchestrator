package store

import (
	"errors"
	"fmt"
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
)

type storeFactory struct {
	logger *zap.Logger
}

func NewStoreFactory(logger *zap.Logger) common.StoreFactory {
	return &storeFactory{
		logger: logger,
	}
}

func (e *storeFactory) CreateJobStore(config appconfig.PersistentStore, logger *zap.Logger) (common.JobStore, error) {
	plugin, ok := supportedPlugins[config.PluginName]
	if !ok {
		return nil, errors.New(fmt.Sprintf("unsupported store plugin %s", config.PluginName))
	}
	return plugin.CreateJobStore(config, logger)
}
