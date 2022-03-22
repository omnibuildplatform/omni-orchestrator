package store

import (
	"go.uber.org/zap"

	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
)

type (
	Plugin interface {
		CreateJobStore(cfg appconfig.PersistentStore, logger *zap.Logger) (common.JobStore, error)
	}
)
