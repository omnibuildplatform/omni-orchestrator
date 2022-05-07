package plugins

import (
	"github.com/omnibuildplatform/omni-orchestrator/common"
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
		Serialize(namespace, name string, job common.Job) (map[KubernetesResource][]byte, string, error)
	}
)
