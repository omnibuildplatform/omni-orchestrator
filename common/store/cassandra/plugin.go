package cadence

import (
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	pluginPkg "github.com/omnibuildplatform/omni-orchestrator/common/store/plugin"
	"go.uber.org/zap"
)

const (
	PluginName = "cassandra"
)

type plugin struct{}

var _ pluginPkg.Plugin = (*plugin)(nil)

func init() {
	pluginPkg.RegisterPlugin(PluginName, &plugin{})
}

func (p *plugin) CreateJobStore(cfg appconfig.PersistentStore, logger *zap.Logger) (common.JobStore, error) {
	//e, err := NewJobStore(cfg, logger)
	//if err != nil {
	//	panic(err)
	//}
	//return e, nil
	return &Store{}, nil
}
