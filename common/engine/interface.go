package engine

import (
	"go.uber.org/zap"

	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
)

type (
	Plugin interface {
		CreateEngine(cfg appconfig.Engine, logger *zap.Logger) (common.JobEngine, error)
	}
)
