package buildimagefromrelease

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"io/ioutil"
	"os"
	"path/filepath"
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

func loadTemplates(folder string) (map[string][]byte, error) {
	templates := make(map[string][]byte)
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
			templates[info.Name()] = bytes
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return templates, nil
}

type Handler struct {
	logger     *zap.Logger
	dataFolder string
	namespace  string
	name       string
	paras      JobImageBuildFromReleasePara
	templates  map[string][]byte
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
		paras:      JobImageBuildFromReleasePara{},
	}
	handler.templates, err = loadTemplates(dataFolder)
	if err != nil {
		return nil, err
	}
	return handler, nil
}

func (h *Handler) GetJobArchitecture() string {
	return strings.ToLower(h.paras.Architecture)
}

func (h *Handler) Reload() {
	h.Lock()
	defer h.Unlock()
	templates, err := loadTemplates(h.dataFolder)
	if err != nil {
		h.logger.Warn(fmt.Sprintf("unable to reload template files %s", err))
	} else {
		h.templates = templates
	}
	h.logger.Info("extend job:buildimagefromrelease reloaded job templates")
}
func (h *Handler) Initialize(namespace, name string, parameters map[string]interface{}) error {
	h.name = name
	h.namespace = namespace
	err := mapstructure.Decode(parameters, &h.paras)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to decode job specification %s for BuildImageFromRelease job", err))
	}
	// parse templates
	wrapPackages := BuildImagePackages{
		Packages: h.paras.Packages,
	}
	packages, err := json.Marshal(wrapPackages)
	if err != nil {
		return err
	}
	variables := map[string]string{
		"name":      h.name,
		"namespace": h.namespace,
		"version":   h.paras.Version,
		"packages":  string(packages),
		"format":    h.paras.Format,
	}
	for k, value := range h.templates {
		resourceTemplate, err := template.New(k).Parse(string(value))
		if err != nil {
			return err
		}
		buffer := new(bytes.Buffer)
		err = resourceTemplate.ExecuteTemplate(buffer, k, variables)
		if err != nil {
			return err
		}
		h.templates[k] = buffer.Bytes()
	}
	return nil
}
func (h *Handler) GetAllSerializedObjects() map[string][]byte {
	h.Lock()
	defer h.Unlock()
	return h.templates
}
