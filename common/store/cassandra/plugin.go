package cadence

import (
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"github.com/omnibuildplatform/omni-orchestrator/common/store"
	"go.uber.org/zap"
)

const (
	PluginName = "cassandra"
)

type plugin struct{}

var _ store.Plugin = (*plugin)(nil)

func init() {
	store.RegisterPlugin(PluginName, &plugin{})
}

func (p *plugin) CreateJobStore(cfg appconfig.PersistentStore, logger *zap.Logger) (common.JobStore, error) {
	e, err := NewJobStore(cfg, logger)
	if err != nil {
		panic(err)
	}
	return e, nil
}
