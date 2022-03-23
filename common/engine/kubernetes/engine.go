package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"strings"
)

const (
	DefaultJobTTL             = 1800
	DefaultJobRetry           = 2
	DefaultPackagesConfigFile = "openEuler-customized.json"
	DefaultImageConfigFile    = "conf.yaml"
	DefaultImageConfig        = `working_dir: /data/omni-workspace
debug: True
user_name: root
user_passwd: openEuler
installer_configs: /etc/omni-imager/installer_assets/calamares-configs
systemd_configs: /etc/omni-imager/installer_assets/systemd-configs
init_script: /etc/omni-imager/init
installer_script: /etc/omni-imager/runinstaller
repo_file: /etc/omni-imager/repos/%s.repo`
)

type BuildImagePackages struct {
	Packages []string `json:"packages"`
}

type Engine struct {
	logger    *zap.Logger
	clientSet *kubernetes.Clientset
	config    appconfig.Engine
}

func NewEngine(config appconfig.Engine, logger *zap.Logger) (common.JobEngine, error) {
	k8sConfig, err := clientcmd.BuildConfigFromFlags("", config.ConfigFile)
	if err != nil {
		return nil, err
	}
	clientSet, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, err
	}
	return &Engine{
		logger:    logger,
		config:    config,
		clientSet: clientSet,
	}, nil
}

func (e *Engine) Initialize() error {
	v, err := e.clientSet.ServerVersion()
	if err != nil {
		return err
	}
	e.logger.Info(fmt.Sprintf("kubernetes connected %s", v.String()))
	return nil
}

func (e *Engine) CreateNamespaceIfNeeded(ns string) error {
	_, err := e.clientSet.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			namespace := v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
					Annotations: map[string]string{
						"controlledBy": "omni-orchestrator",
					},
				},
			}
			_, err := e.clientSet.CoreV1().Namespaces().Create(&namespace)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	e.logger.Info(fmt.Sprintf("namespace %s already exists in kubernetes", ns))
	return nil
}

func (e *Engine) Close() {

}

func (e *Engine) GetName() string {
	return "kubernetes"
}
func (e *Engine) GetSupportedJobs() []common.JobKind {
	return []common.JobKind{common.JobImageBuild}
}

func (e *Engine) prepareJobConfigmap(job common.Job, spec common.JobImageBuildPara) (string, error) {
	pkg := BuildImagePackages{
		Packages: spec.Packages,
	}
	packages, err := json.MarshalIndent(pkg, "", "\t")
	if err != nil {
		return "", err
	}
	//prepare configmap
	configMap := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      job.ID,
			Namespace: e.ConvertToNamespace(job.Domain),
			Annotations: map[string]string{
				"controlled-by":    "omni-orchestrator",
				"omni-tag/service": job.Service,
				"omni-tag/user":    job.UserID,
			},
			OwnerReferences: []metav1.OwnerReference{
				//TODO: Add job reference
			},
		},
		Data: map[string]string{
			DefaultPackagesConfigFile: string(packages),
			DefaultImageConfigFile:    fmt.Sprintf(DefaultImageConfig, spec.Version),
		},
	}
	_, err = e.clientSet.CoreV1().ConfigMaps(e.ConvertToNamespace(job.Domain)).Get(job.ID, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		newConfigmap, err := e.clientSet.CoreV1().ConfigMaps(e.ConvertToNamespace(job.Domain)).Create(&configMap)
		if err != nil {
			return "", nil
		}
		return newConfigmap.Name, nil
	} else if err == nil {
		newConfigmap, err := e.clientSet.CoreV1().ConfigMaps(e.ConvertToNamespace(job.Domain)).Update(&configMap)
		if err != nil {
			return "", nil
		}
		return newConfigmap.Name, nil
	}
	return "", err
}

func (e *Engine) generateBuildOSImageJob(job common.Job, spec common.JobImageBuildPara, configmapName string) *batchv1.Job {
	jobTTLSecondsAfterFinished := int32(DefaultJobTTL)
	jobRetry := int32(DefaultJobRetry)
	privileged := true
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      job.ID,
			Namespace: e.ConvertToNamespace(job.Domain),
			Annotations: map[string]string{
				"controlled-by":    "omni-orchestrator",
				"omni-tag/service": job.Service,
				"omni-tag/user":    job.UserID,
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &jobTTLSecondsAfterFinished,
			BackoffLimit:            &jobRetry,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyNever,
					Containers: []v1.Container{
						{
							Name:  "job-completed",
							Image: "alpine/curl",
							Command: []string{
								"echo", "job succeed",
							},
						},
					},
					InitContainers: []v1.Container{
						{
							Name:  "01-osimage-build",
							Image: e.config.ImageTagForOSImageBuild,
							SecurityContext: &v1.SecurityContext{
								Privileged: &privileged,
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      fmt.Sprintf("%s-config", job.ID),
									MountPath: fmt.Sprintf("/etc/omni-imager/%s", DefaultPackagesConfigFile),
									SubPath:   DefaultPackagesConfigFile,
								},
								{
									Name:      fmt.Sprintf("%s-config", job.ID),
									MountPath: fmt.Sprintf("/etc/omni-imager/%s", DefaultImageConfigFile),
									SubPath:   DefaultImageConfigFile,
								},
								{
									Name:      fmt.Sprintf("%s-data", job.ID),
									MountPath: "/data/",
								},
							},
							Command: []string{
								"omni-imager", "--package-list",
								fmt.Sprintf("/etc/omni-imager/%s", DefaultPackagesConfigFile),
								"--config-file", fmt.Sprintf("/etc/omni-imager/%s", DefaultImageConfigFile),
								"--build-type", spec.Format, "--output-file", fmt.Sprintf("openEuler-%s.iso",
									job.ID),
							},
						},
						{
							Name:  "02-image-upload",
							Image: "alpine/curl",
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      fmt.Sprintf("%s-data", job.ID),
									MountPath: "/data/",
								},
							},
							Command: []string{
								"curl", "-vvv", fmt.Sprintf("-Ffile=@/data/omni-workspace/openEuler-%s.iso", job.ID),
								fmt.Sprintf("-Fproject=%s", spec.Version), "-FfileType=image",
								fmt.Sprintf("%s?token=%s", e.config.OmniRepoAddress, e.config.OmniRepoToken),
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: fmt.Sprintf("%s-config", job.ID),
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: configmapName,
									},
								},
							},
						},
						{
							Name: fmt.Sprintf("%s-data", job.ID),
							VolumeSource: v1.VolumeSource{
								EmptyDir: &v1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

}

func (e *Engine) ConvertToNamespace(domain string) string {
	return strings.ToLower(domain)
}

func (e *Engine) BuildOSImage(ctx context.Context, job common.Job, spec common.JobImageBuildPara) error {
	var err error
	//prepare namespace
	err = e.CreateNamespaceIfNeeded(e.ConvertToNamespace(job.Domain))
	if err != nil {
		return err
	}
	//prepare configmap
	configMapName, err := e.prepareJobConfigmap(job, spec)
	if err != nil {
		return err
	}
	//prepare job resource
	jobResource := e.generateBuildOSImageJob(job, spec, configMapName)
	_, err = e.clientSet.BatchV1().Jobs(e.ConvertToNamespace(job.Domain)).Get(job.ID, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err := e.clientSet.BatchV1().Jobs(e.ConvertToNamespace(job.Domain)).Create(jobResource)
		if err != nil {
			return err
		}
		return nil
	} else if err == nil {
		_, err = e.clientSet.BatchV1().Jobs(e.ConvertToNamespace(job.Domain)).Update(jobResource)
		if err != nil {
			return err
		}
		return nil
	}
	return err
}

func (e *Engine) GetJob(ctx context.Context, domain, jobID string) (*common.Job, error) {
	jobResource := common.Job{
		ID:     jobID,
		Domain: domain,
	}
	existing, err := e.clientSet.BatchV1().Jobs(domain).Get(jobID, metav1.GetOptions{})
	if err == nil {
		completionRequired := existing.Spec.Completions
		backoffLimitRequired := existing.Spec.BackoffLimit
		if existing.Status.Succeeded >= *completionRequired {
			jobResource.State = common.JobSucceed

		} else if existing.Status.Failed >= *backoffLimitRequired {
			jobResource.State = common.JobFailed
		} else {
			jobResource.State = common.JobRunning
		}
		jobResource.StartTime = existing.Status.StartTime.Time
		jobResource.EndTime = existing.Status.CompletionTime.Time
		jobResource.TotalStep = len(existing.Spec.Template.Spec.InitContainers)
	}
	return nil, err
}
