package extended_jobs

import (
	"github.com/omnibuildplatform/omni-orchestrator/common/engine/kubernetes/extended_jobs/plugins"
	"strings"
)

func GetResourceType(name string) plugins.KubernetesResource {
	availableResources := []plugins.KubernetesResource{plugins.ResJob, plugins.ResDeployment, plugins.ResConfigmap}
	for _, v := range availableResources {
		if string(v) == strings.ToLower(name) {
			return v
		}
	}
	return plugins.ResUnSupported
}
