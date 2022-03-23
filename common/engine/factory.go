package engine

import (
	"errors"
	"fmt"
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	//Register plugins
	_ "github.com/omnibuildplatform/omni-orchestrator/common/engine/cadence"
	_ "github.com/omnibuildplatform/omni-orchestrator/common/engine/kubernetes"
	pluginPkg "github.com/omnibuildplatform/omni-orchestrator/common/engine/plugin"
	"go.uber.org/zap"
)

type engineFactory struct {
	logger *zap.Logger
}

func NewEngineFactory(logger *zap.Logger) common.EngineFactory {
	return &engineFactory{
		logger: logger,
	}
}

func (e *engineFactory) CreateJobEngine(config appconfig.Engine, logger *zap.Logger) (common.JobEngine, error) {
	plugin, ok := pluginPkg.SupportedPlugins[config.PluginName]
	if !ok {
		return nil, errors.New(fmt.Sprintf("unsupported engine plugin %s", config.PluginName))
	}
	return plugin.CreateEngine(config, logger)
}
