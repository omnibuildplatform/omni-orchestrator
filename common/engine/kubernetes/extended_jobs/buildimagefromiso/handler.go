package buildimagefromiso

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/omnibuildplatform/omni-orchestrator/common"
	"github.com/omnibuildplatform/omni-orchestrator/common/engine/kubernetes/extended_jobs"
	"github.com/omnibuildplatform/omni-orchestrator/common/engine/kubernetes/extended_jobs/plugins"
	"go.uber.org/zap"
	"net/url"
	"os"
	"strings"
	"sync"
	"text/template"
)

type JobImageBuildFromISOPara struct {
	KickStart *CommonFile `json:"kickStart"`
	Image     *ImageFile  `json:"image"`
}

type ImageFile struct {
	Url          string `json:"url"`
	CheckSum     string `json:"checksum"`
	Name         string `json:"name"`
	Architecture string `json:"architecture"`
}
type CommonFile struct {
	Content string `json:"content"`
	Name    string `json:"name"`
}

func imageValid(f ImageFile) error {
	if len(f.Url) == 0 || len(f.CheckSum) == 0 || len(f.Name) == 0 || len(f.Architecture) == 0 {
		return errors.New("url, checksum, filename or architecture empty")
	}
	u, err := url.ParseRequestURI(f.Url)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(strings.ToLower(u.Scheme), "http") {
		return errors.New("acceptable schema for url are http and https")
	}
	return nil
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
	h.logger.Info("extend job:buildimagefromiso reloaded job templates")
}
func (h *Handler) Serialize(namespace, name string, job common.Job) (map[plugins.KubernetesResource][]byte, string, error) {
	renderedTemplates := make(map[plugins.KubernetesResource][]byte)
	var paras JobImageBuildFromISOPara
	h.name = name
	h.namespace = namespace
	err := mapstructure.Decode(job.Spec, &paras)
	if err != nil {
		return map[plugins.KubernetesResource][]byte{}, "", errors.New(fmt.Sprintf("unable to decode job specification %s for JobImageBuildFromISOPara job", err))
	}
	if paras.KickStart == nil || paras.Image == nil {
		return map[plugins.KubernetesResource][]byte{}, "", errors.New("kickstart image, name empty")
	}
	err = imageValid(*paras.Image)
	if err != nil {
		return map[plugins.KubernetesResource][]byte{}, "", err
	}
	if len(paras.KickStart.Name) == 0 || len(paras.KickStart.Content) == 0 {
		return map[plugins.KubernetesResource][]byte{}, "", errors.New("kickstart content or name empty")
	}
	variables := map[string]string{
		"name":             h.name,
		"namespace":        h.namespace,
		"imageUrl":         paras.Image.Url,
		"imageName":        paras.Image.Name,
		"kickstartContent": paras.KickStart.Content,
		"kickstartName":    paras.KickStart.Name,
		"userID":           job.UserID,
		"type":             "buildimagefromiso",
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
	return renderedTemplates, strings.ToLower(paras.Image.Architecture), nil
}
