package engine

import (
	"errors"
	"fmt"
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
)

type engineFactory struct {
	logger *zap.Logger
}

func NewEngineFactory(logger *zap.Logger) *engineFactory {
	return &engineFactory{
		logger: logger,
	}
}

func (e *engineFactory)NewJobEngine(config appconfig.Engine, logger *zap.Logger) (common.JobEngine, error) {
	plugin, ok := supportedPlugins[config.Name]
	if !ok {
		return nil, errors.New(fmt.Sprintf("unsupported engine plugin %s", config.Name))
	}
	return plugin.CreateEngine(config, logger)
}