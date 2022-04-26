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
		GetJobArchitecture() string
		Initialize(namespace, name string, parameters map[string]interface{}) error
		GetAllSerializedObjects() map[string][]byte
	}
)
