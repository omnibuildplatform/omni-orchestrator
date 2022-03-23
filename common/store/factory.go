package store

import (
	"errors"
	"fmt"
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	_ "github.com/omnibuildplatform/omni-orchestrator/common/store/cassandra"
	pluginPkg "github.com/omnibuildplatform/omni-orchestrator/common/store/plugin"
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
	plugin, ok := pluginPkg.SupportedPlugins[config.PluginName]
	if !ok {
		return nil, errors.New(fmt.Sprintf("unsupported store plugin %s", config.PluginName))
	}
	return plugin.CreateJobStore(config, logger)
}
