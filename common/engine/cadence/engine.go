package cadence

import (
	"github.com/omnibuildplatform/omni-orchestrator/common"
)

type Engine struct {
}

func NewEngine() (common.JobEngine, error) {
	return nil, nil
}

func (e *Engine) GetName() string {
	return "cadence"
}
func (e *Engine) GetSupportedJobs() []common.JobKind {
	return []common.JobKind{}
}
func (e *Engine) BuildImage(job common.Job, spec common.JobImageBuildPara) error {
	return nil
}
