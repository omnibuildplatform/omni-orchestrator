package buildimagefromrelease

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/omnibuildplatform/omni-orchestrator/common"
	"github.com/omnibuildplatform/omni-orchestrator/common/engine/kubernetes/extended_jobs"
	"github.com/omnibuildplatform/omni-orchestrator/common/engine/kubernetes/extended_jobs/plugins"
	"go.uber.org/zap"
	"os"
	"strings"
	"sync"
	"text/template"
)

type JobImageBuildFromReleasePara struct {
	Version      string   `json:"version"`
	Packages     []string `json:"packages"`
	Format       string   `json:"format"`
	Architecture string   `json:"architecture"`
}

type BuildImagePackages struct {
	Packages []string `json:"packages"`
}

type Handler struct {
	logger     *zap.Logger
	dataFolder string
	namespace  string
	name       string
	templates  map[plugins.KubernetesResource][]byte
	sync.Mutex
}

func NewHandler(dataFolder string, logger *zap.Logger) (*Handler, error) {
	f, err := os.Stat(dataFolder)
	if err != nil {
		return nil, err
	}
	if !f.IsDir() {
		return nil, errors.New(fmt.Sprintf("path %s is not a directory", dataFolder))
	}
	//check necessary yaml files
	handler := &Handler{
		logger:     logger,
		dataFolder: dataFolder,
	}
	handler.templates, err = extended_jobs.LoadTemplates(dataFolder)
	if err != nil {
		return nil, err
	}
	return handler, nil
}

func (h *Handler) Reload() {
	h.Lock()
	defer h.Unlock()
	templates, err := extended_jobs.LoadTemplates(h.dataFolder)
	if err != nil {
		h.logger.Warn(fmt.Sprintf("unable to reload template files %s", err))
	} else {
		h.templates = templates
	}
	h.logger.Info("extend job:buildimagefromrelease reloaded job templates")
}
func (h *Handler) Serialize(namespace, name string, job common.Job) (map[plugins.KubernetesResource][]byte, string, error) {
	renderedTemplates := make(map[plugins.KubernetesResource][]byte)
	var paras JobImageBuildFromReleasePara
	h.name = name
	h.namespace = namespace
	err := mapstructure.Decode(job.Spec, &paras)
	if err != nil {
		return map[plugins.KubernetesResource][]byte{}, "", errors.New(fmt.Sprintf("unable to decode job specification %s for BuildImageFromRelease job", err))
	}
	if len(paras.Version) == 0 || len(paras.Format) == 0 || len(paras.Packages) == 0 || len(paras.Architecture) == 0 {
		return map[plugins.KubernetesResource][]byte{}, "", errors.New("format packages, architecture or version empty")
	}
	// parse templates
	wrapPackages := BuildImagePackages{
		Packages: paras.Packages,
	}
	packages, err := json.Marshal(wrapPackages)
	if err != nil {
		return map[plugins.KubernetesResource][]byte{}, "", err
	}
	variables := map[string]string{
		"name":      h.name,
		"namespace": h.namespace,
		"version":   paras.Version,
		"packages":  string(packages),
		"format":    paras.Format,
		"userID":    job.UserID,
		"type":      "buildimagefromrelease",
	}
	h.Lock()
	defer h.Unlock()
	for k, value := range h.templates {
		resourceTemplate, err := template.New(string(k)).Parse(string(value))
		if err != nil {
			return map[plugins.KubernetesResource][]byte{}, "", err
		}
		buffer := new(bytes.Buffer)
		err = resourceTemplate.ExecuteTemplate(buffer, string(k), variables)
		if err != nil {
			return map[plugins.KubernetesResource][]byte{}, "", err
		}
		renderedTemplates[k] = buffer.Bytes()
	}
	return renderedTemplates, strings.ToLower(paras.Architecture), nil
}
