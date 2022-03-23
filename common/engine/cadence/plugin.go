package cadence

import (
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	pluginPkg "github.com/omnibuildplatform/omni-orchestrator/common/engine/plugin"
	"go.uber.org/zap"
)

const (
	PluginName = "cadence"
)

type plugin struct{}

var _ pluginPkg.Plugin = (*plugin)(nil)

func init() {
	pluginPkg.RegisterPlugin(PluginName, &plugin{})
}

func (p *plugin) CreateEngine(cfg appconfig.Engine, logger *zap.Logger) (common.JobEngine, error) {
	e, err := NewEngine()
	if err != nil {
		panic(err)
	}
	return e, nil
}
