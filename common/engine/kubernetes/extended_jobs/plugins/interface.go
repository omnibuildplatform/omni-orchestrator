package plugins

import (
	"go.uber.org/zap"
)

type KubernetesResource string

const (
	ResDeployment  KubernetesResource = "deployment"
	ResJob         KubernetesResource = "job"
	ResConfigmap   KubernetesResource = "configmap"
	ResUnSupported KubernetesResource = "unsupported"
)

type (
	JobPlugin interface {
		CreateJobHandler(dataFolder string, logger *zap.Logger) (JobHandler, error)
	}

	JobHandler interface {
		Reload()
		Serialize(namespace, name string, parameters map[string]interface{}) (map[KubernetesResource][]byte, string, error)
	}
)
