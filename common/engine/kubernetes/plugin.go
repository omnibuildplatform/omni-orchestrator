package kubernetes

import (
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"github.com/omnibuildplatform/omni-orchestrator/common/engine"
	"go.uber.org/zap"
)

const (
	PluginName = "kubernetes"
)

type plugin struct{}

var _ engine.Plugin = (*plugin)(nil)

func init() {
	engine.RegisterPlugin(PluginName, &plugin{})
}

func (p *plugin) CreateEngine(cfg appconfig.Engine, logger *zap.Logger) (common.JobEngine, error) {
	e, err := NewEngine(cfg, logger)
	if err != nil {
		return nil, err
	}
	err = e.Initialize()
	if err != nil {
		return nil, err
	}
	return e, nil
}
