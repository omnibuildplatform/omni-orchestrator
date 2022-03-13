package cadence

import (
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"github.com/omnibuildplatform/omni-orchestrator/common/engine"
	"go.uber.org/zap"
)

const (
	PluginName = "cadence"
)

type plugin struct{}

var _ engine.Plugin = (*plugin)(nil)

func init() {
	engine.RegisterPlugin(PluginName, &plugin{})
}

func (p *plugin) CreateEngine(cfg appconfig.Engine, logger *zap.Logger) (common.JobEngine, error) {
	e, err := NewEngine()
	if err != nil {
		panic(err)
	}
	return e, nil
}
