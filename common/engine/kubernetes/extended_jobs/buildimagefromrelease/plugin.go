package buildimagefromrelease

import (
	"github.com/omnibuildplatform/omni-orchestrator/common/engine/kubernetes/extended_jobs/plugins"
	"go.uber.org/zap"
)

const (
	JobName = "buildimagefromrelease"
)

type plugin struct{}

var _ plugins.JobPlugin = (*plugin)(nil)

func init() {
	plugins.RegisterPlugin(JobName, &plugin{})
}

func (p *plugin) CreateJobHandler(dataFolder string, logger *zap.Logger) (plugins.JobHandler, error) {
	return NewHandler(dataFolder, logger)
}
