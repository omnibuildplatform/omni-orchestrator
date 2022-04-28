package plugins

import (
	"go.uber.org/zap"
)

type (
	JobPlugin interface {
		CreateJobHandler(dataFolder string, logger *zap.Logger) (JobHandler, error)
	}

	JobHandler interface {
		Reload()
		Serialize(namespace, name string, parameters map[string]interface{}) (map[string][]byte, string, error)
	}
)
