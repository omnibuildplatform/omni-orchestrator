package cadence

import (
	"context"
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
func (e *Engine) BuildOSImage(ctx context.Context, job common.Job, spec common.JobImageBuildPara) error {
	return nil
}

func (e *Engine) GetJob(ctx context.Context, domain, jobID string) (*common.Job, error) {
	return nil, nil
}
