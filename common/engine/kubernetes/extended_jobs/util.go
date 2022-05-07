package extended_jobs

import (
	"github.com/omnibuildplatform/omni-orchestrator/common/engine/kubernetes/extended_jobs/plugins"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func getResourceType(name string) plugins.KubernetesResource {
	availableResources := []plugins.KubernetesResource{plugins.ResJob, plugins.ResDeployment, plugins.ResConfigmap}
	for _, v := range availableResources {
		if strings.HasPrefix(strings.ToLower(name), string(v)) {
			return v
		}
	}
	return plugins.ResUnSupported
}

func LoadTemplates(folder string) (map[plugins.KubernetesResource][]byte, error) {
	templates := make(map[plugins.KubernetesResource][]byte)
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		//read all yaml file
		if strings.HasSuffix(path, ".yaml") {
			templateFile, err := os.Open(path)
			if err != nil {
				return err
			}
			defer templateFile.Close()
			bytes, err := ioutil.ReadAll(templateFile)
			if err != nil {
				return err
			}
			templates[getResourceType(info.Name())] = bytes
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return templates, nil
}
