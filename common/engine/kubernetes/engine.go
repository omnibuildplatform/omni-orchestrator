package kubernetes

import (
	"context"
	"encoding/json"
	syserror "errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
	"io"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"strings"
	"sync"
	"time"
)

const (
	AppLabelKey               = "omni-orchestrator"
	AppLabelValue             = "true"
	AnnotationService         = "omni-tag/service"
	AnnotationDomain          = "omni-tag/domain"
	AnnotationTask            = "omni-tag/task"
	DefaultSyncInterval       = 3
	DefaultJobTTL             = 1800
	DefaultJobRetry           = 0
	DefaultEventChannel       = 200
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
repo_file: /etc/omni-imager/repos/%s.repo
use_cached_rootfs: True
cached_rootfs_gz: /data/rootfs_cache/rootfs.tar.gz`
)

type BuildImagePackages struct {
	Packages []string `json:"packages"`
}

type Engine struct {
	logger       *zap.Logger
	clientSet    *kubernetes.Clientset
	config       appconfig.Engine
	factory      *informers.SharedInformerFactory
	closeCh      chan struct{}
	eventChannel chan common.JobIdentity
	JobWatchMap  sync.Map
}

func NewEngine(config appconfig.Engine, logger *zap.Logger) (common.JobEngine, error) {
	k8sConfig, err := clientcmd.BuildConfigFromFlags("", config.ConfigFile)
	if err != nil {
		return nil, err
	}
	clientSet, err := kubernetes.NewForConfig(k8sConfig)
	factory := informers.NewSharedInformerFactoryWithOptions(clientSet, DefaultSyncInterval*time.Second,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = fmt.Sprintf("%s=%s", AppLabelKey, AppLabelValue)
		}))
	if err != nil {
		return nil, err
	}
	return &Engine{
		logger:       logger,
		config:       config,
		clientSet:    clientSet,
		factory:      &factory,
		closeCh:      make(chan struct{}),
		eventChannel: make(chan common.JobIdentity, DefaultEventChannel),
		JobWatchMap:  sync.Map{},
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
	_, err := e.clientSet.CoreV1().Namespaces().Get(context.TODO(), ns, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			namespace := v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
					Annotations: map[string]string{
						"controlledBy": "omni-orchestrator",
					},
					Labels: e.generateSystemLabels(),
				},
			}
			_, err := e.clientSet.CoreV1().Namespaces().Create(context.TODO(), &namespace, metav1.CreateOptions{})
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
	runtime.HandleCrash()
	close(e.closeCh)
	close(e.eventChannel)
}

func (e *Engine) GetName() string {
	return "kubernetes"
}
func (e *Engine) GetSupportedJobs() []common.JobKind {
	return []common.JobKind{common.JobImageBuild}
}

func (e *Engine) prepareJobConfigmap(job *common.Job, spec common.JobImageBuildPara) (string, error) {
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
			Name:        job.ID,
			Namespace:   e.ConvertToNamespace(job.Domain),
			Annotations: e.generateSystemAnnotations(job),
			Labels:      e.generateSystemLabels(),
		},
		Data: map[string]string{
			DefaultPackagesConfigFile: string(packages),
			DefaultImageConfigFile:    fmt.Sprintf(DefaultImageConfig, spec.Version),
		},
	}
	_, err = e.clientSet.CoreV1().ConfigMaps(e.ConvertToNamespace(job.Domain)).Get(context.TODO(), job.ID, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		newConfigmap, err := e.clientSet.CoreV1().ConfigMaps(e.ConvertToNamespace(job.Domain)).Create(context.TODO(), &configMap, metav1.CreateOptions{})
		if err != nil {
			return "", nil
		}
		return newConfigmap.Name, nil
	} else if err == nil {
		newConfigmap, err := e.clientSet.CoreV1().ConfigMaps(e.ConvertToNamespace(job.Domain)).Update(context.TODO(), &configMap, metav1.UpdateOptions{})
		if err != nil {
			return "", nil
		}
		return newConfigmap.Name, nil
	}
	return "", err
}

func (e *Engine) generateSystemAnnotations(job *common.Job) map[string]string {
	return map[string]string{
		AnnotationService: job.Service,
		AnnotationTask:    job.Task,
		AnnotationDomain:  job.Domain,
	}
}

func (e *Engine) hasSystemAnnotations(annotations map[string]string) bool {
	if _, ok := annotations[AnnotationService]; ok {
		if _, ok := annotations[AnnotationTask]; ok {
			if _, ok := annotations[AnnotationDomain]; ok {
				return true
			}
		}
	}
	return false
}

func (e *Engine) generateSystemLabels() map[string]string {
	return map[string]string{
		AppLabelKey: AppLabelValue,
	}
}
func (e *Engine) generateBuildOSImageJob(job *common.Job, spec common.JobImageBuildPara, configmapName string) *batchv1.Job {
	jobTTLSecondsAfterFinished := int32(DefaultJobTTL)
	jobRetry := int32(DefaultJobRetry)
	privileged := true
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        job.ID,
			Namespace:   e.ConvertToNamespace(job.Domain),
			Annotations: e.generateSystemAnnotations(job),
			Labels:      e.generateSystemLabels(),
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &jobTTLSecondsAfterFinished,
			BackoffLimit:            &jobRetry,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      e.generateSystemLabels(),
					Annotations: e.generateSystemAnnotations(job),
				},
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
							Name:  "rootfs-download",
							Image: "alpine/curl",
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      fmt.Sprintf("%s-data", job.ID),
									MountPath: "/data/",
								},
							},
							Command: []string{
								"sh", "-c", fmt.Sprintf(
									"mkdir -p /data/rootfs_cache; curl -vvv %s/data/browse/util/imager/rootfs.tar.gz -o /data/rootfs_cache/rootfs.tar.gz",
									e.config.OmniRepoAddress),
							},
						},
						{
							Name:  "image-build",
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
							Name:  "image-upload",
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
								fmt.Sprintf("%s/data/upload?token=%s", strings.TrimRight(e.config.OmniRepoAddress,
									"/"), e.config.OmniRepoToken),
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
	return strings.Replace(strings.ToLower(domain), ".", "-", -1)
}

func (e *Engine) BuildOSImage(ctx context.Context, job *common.Job, spec common.JobImageBuildPara) error {
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
	_, err = e.clientSet.BatchV1().Jobs(e.ConvertToNamespace(job.Domain)).Get(context.TODO(), job.ID, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err := e.clientSet.BatchV1().Jobs(e.ConvertToNamespace(job.Domain)).Create(context.TODO(), jobResource, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		return nil
	} else if err == nil {
		_, err = e.clientSet.BatchV1().Jobs(e.ConvertToNamespace(job.Domain)).Update(context.TODO(), jobResource, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	}
	return err
}

func (e *Engine) collectJobAnnotations(namespace, name string, annotations map[string]string) (string, string, string) {
	var service, task, domain string
	if value, ok := annotations[AnnotationService]; !ok {
		e.logger.Warn(fmt.Sprintf("job %s/%s event has been dropped due to '%s' not found",
			namespace, name, AnnotationService))
		service = ""
	} else {
		service = value
	}
	if value, ok := annotations[AnnotationTask]; !ok {
		e.logger.Warn(fmt.Sprintf("job %s/%s event has been dropped due to '%s' not found",
			namespace, name, AnnotationTask))
		task = ""
	} else {
		task = value
	}
	if value, ok := annotations[AnnotationDomain]; !ok {
		e.logger.Warn(fmt.Sprintf("job %s/%s event has been dropped due to '%s' not found",
			namespace, name, AnnotationDomain))
		domain = ""
	} else {
		domain = value
	}
	return service, task, domain
}

func (e *Engine) GetJob(ctx context.Context, jobID common.JobIdentity) (*common.Job, error) {
	jobResource := common.Job{
		JobIdentity: jobID,
	}
	existing, err := e.clientSet.BatchV1().Jobs(e.ConvertToNamespace(jobID.Domain)).Get(context.TODO(), jobID.ID, metav1.GetOptions{})
	if err == nil {
		completionRequired := existing.Spec.Completions
		backoffLimitRequired := existing.Spec.BackoffLimit
		if existing.Status.Succeeded >= *completionRequired {
			jobResource.State = common.JobSucceed

		} else if existing.Status.Failed > *backoffLimitRequired {
			jobResource.State = common.JobFailed
		} else {
			jobResource.State = common.JobRunning
		}
		if existing.Status.StartTime != nil {
			jobResource.StartTime = existing.Status.StartTime.Time
		}
		// Find out job end time via CompletionTime or condition
		if jobResource.State == common.JobSucceed {
			if existing.Status.CompletionTime != nil {
				jobResource.EndTime = existing.Status.CompletionTime.Time
			}
		}
		if jobResource.State == common.JobFailed {
			for _, c := range existing.Status.Conditions {
				if c.Type == batchv1.JobFailed {
					jobResource.EndTime = c.LastTransitionTime.Time
					break
				}
			}
		}
		jobResource.Service, jobResource.Task, jobResource.Domain = e.collectJobAnnotations(
			jobID.Domain, jobID.ID, existing.Annotations)
		//append steps
		jobResource.Steps = e.CollectSteps(existing)
		return &jobResource, nil
	}
	return nil, err
}

func (e *Engine) DeleteJob(ctx context.Context, jobID common.JobIdentity) error {
	err := e.clientSet.BatchV1().Jobs(e.ConvertToNamespace(jobID.Domain)).Delete(ctx, jobID.ID, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			e.logger.Warn(fmt.Sprintf("unable to delete job %s/%s in kubernetes, it may be deleted before",
				e.ConvertToNamespace(jobID.Domain), jobID.ID))
			return nil
		}
		return err
	}
	e.DeleteJobRelatedResource(e.ConvertToNamespace(jobID.Domain), jobID.ID)
	return nil
}

func (e *Engine) DeleteJobRelatedResource(namespace, name string) {
	//delete configmap
	err := e.clientSet.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			e.logger.Info(fmt.Sprintf("unable to delete job configmap %s/%s in kubernetes, it may be deleted before",
				namespace, name))
		} else {
			e.logger.Warn(fmt.Sprintf("unable to delete job configmap %s/%s in kubernetes, error: %s",
				namespace, name, err))
		}
	} else {
		e.logger.Info(fmt.Sprintf("Job %s/%s related configmap has been deleted", namespace, name))
	}
	//delete pod
	pods, err := e.clientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", name),
	})
	if len(pods.Items) != 0 {
		graceful := int64(10)
		delOption := metav1.DeleteOptions{
			GracePeriodSeconds: &graceful,
		}
		for _, pd := range pods.Items {
			err := e.clientSet.CoreV1().Pods(pd.Namespace).Delete(context.TODO(), pd.Name, delOption)
			if err == nil {
				e.logger.Info(fmt.Sprintf("job pod %s/%s has been deleted", namespace, name))
			} else {
				if errors.IsNotFound(err) {
					e.logger.Info(fmt.Sprintf("unable to delete job pod %s/%s in kubernetes, it may be deleted before",
						namespace, name))
				} else {
					e.logger.Warn(fmt.Sprintf("unable to delete job pod %s/%s in kubernetes, error: %s",
						namespace, name, err))
				}
			}
		}
	}
}

func (e *Engine) CollectSteps(job *batchv1.Job) []common.Step {
	var steps []common.Step
	pods, err := e.clientSet.CoreV1().Pods(job.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", job.Name),
	})
	if err != nil {
		e.logger.Warn(fmt.Sprintf("failed to list pods for job %s/%s, error: %s", job.Namespace, job.Name, err))
		return steps
	}
	if len(pods.Items) == 0 {
		return steps
	}
	// Job will only execute once
	for i, container := range pods.Items[0].Status.InitContainerStatuses {
		step := common.Step{
			ID:   i + 1,
			Name: container.Name,
		}
		if container.State.Terminated != nil {
			step.StartTime = container.State.Terminated.StartedAt.Time
			step.EndTime = container.State.Terminated.FinishedAt.Time
			if container.State.Terminated.ExitCode == 0 {
				step.State = common.StepSucceed
			} else {
				step.State = common.StepFailed
				step.Message = container.State.Terminated.Message
			}
		} else if container.State.Running != nil {
			step.StartTime = container.State.Running.StartedAt.Time
			step.State = common.StepRunning
		} else {
			step.State = common.StepCreated
		}
		steps = append(steps, step)
	}
	return steps
}

func (e *Engine) TriggerJobEvent(namespace, name string, annotations map[string]string) {
	event := common.JobIdentity{}
	event.ID = name
	event.Service, event.Task, event.Domain = e.collectJobAnnotations(namespace, name, annotations)
	e.eventChannel <- event
}

func (e *Engine) JobAddEvent(obj interface{}) {
	job := obj.(*batchv1.Job)
	e.TriggerJobEventOnce(job)
}

func (e *Engine) TriggerJobEventOnce(job *batchv1.Job) {
	mark := uuid.New().String()
	if _, ok := e.JobWatchMap.LoadOrStore(job.Name, mark); !ok {
		e.TriggerJobEvent(job.Namespace, job.Name, job.Annotations)
		e.logger.Info(fmt.Sprintf("job %s/%s triggered once",
			job.Namespace, job.Name))
	}
}

func (e *Engine) JobUpdateEvent(old interface{}, new interface{}) {
	oldJob := old.(*batchv1.Job)
	newJob := new.(*batchv1.Job)
	//NOTE: re-list leads to all job with/without change will be notified
	if !equality.Semantic.DeepEqual(oldJob.Status, newJob.Status) {
		e.logger.Info(fmt.Sprintf("job %s/%s has been updated", newJob.Namespace, newJob.Name))
		e.TriggerJobEvent(newJob.Namespace, newJob.Name, newJob.Annotations)
		return
	}
	//trigger once for unchanged jobs
	e.TriggerJobEventOnce(newJob)
}

func (e *Engine) PodUpdateEvent(old interface{}, new interface{}) {
	oldPod := old.(*v1.Pod)
	newPod := new.(*v1.Pod)
	//NOTE: only status change will lead to event
	if jobName, ok := oldPod.Labels["job-name"]; ok {
		if !equality.Semantic.DeepEqual(oldPod.Status, newPod.Status) {
			e.logger.Info(fmt.Sprintf("job's pod %s/%s has been updated", newPod.Namespace, jobName))
			e.TriggerJobEvent(newPod.Namespace, jobName, newPod.Annotations)
		}
	}
}

func (e *Engine) ConfigmapAddEvent(obj interface{}) {
	configmap := obj.(*v1.ConfigMap)
	//delete orphan configmap
	//all configmap created one hour before will take into account
	if configmap.CreationTimestamp.Time.Before(time.Now().Add(-1 * time.Minute * 60)) {
		if _, ok := e.JobWatchMap.Load(configmap.Name); !ok {
			_, err := e.clientSet.BatchV1().Jobs(configmap.Namespace).Get(context.TODO(), configmap.Name, metav1.GetOptions{})
			if err != nil && errors.IsNotFound(err) {
				err = e.clientSet.CoreV1().ConfigMaps(configmap.Namespace).Delete(context.TODO(),
					configmap.Name, metav1.DeleteOptions{})
				if err != nil {
					e.logger.Warn(fmt.Sprintf("Unable to delete orphan configmap %s/%s, error %s.",
						configmap.Namespace, configmap.Name, err))
				} else {
					e.logger.Info(fmt.Sprintf("orphan configmap found, %s/%s delete it.", configmap.Namespace, configmap.Name))
				}
			}
		}
	}
}

func (e *Engine) JobDeleteEvent(obj interface{}) {
	job := obj.(*batchv1.Job)
	e.logger.Info(fmt.Sprintf("job %s/%s has been deleted", job.Namespace, job.Name))
	//remove job related resource
	e.DeleteJobRelatedResource(job.Namespace, job.Name)
	e.JobWatchMap.Delete(job.Name)
}

func (e *Engine) engineControlledFilter(obj interface{}) bool {
	job, ok := obj.(*batchv1.Job)
	if ok {
		return e.hasSystemAnnotations(job.Annotations)
	}
	pod, ok := obj.(*v1.Pod)
	if ok {
		return e.hasSystemAnnotations(pod.Annotations)
	}
	configmap, ok := obj.(*v1.ConfigMap)
	if ok {
		return e.hasSystemAnnotations(configmap.Annotations)
	}
	return false
}

func (e *Engine) StartLoop() error {
	jobInformers := (*e.factory).Batch().V1().Jobs().Informer()
	podInformers := (*e.factory).Core().V1().Pods().Informer()
	configmapInformers := (*e.factory).Core().V1().ConfigMaps().Informer()
	jobInformers.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: e.engineControlledFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    e.JobAddEvent,
			UpdateFunc: e.JobUpdateEvent,
			DeleteFunc: e.JobDeleteEvent,
		},
	})
	//Watch pod used for init container changes
	podInformers.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: e.engineControlledFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			UpdateFunc: e.PodUpdateEvent,
		},
	})
	//Watch configmap used for cleanup
	configmapInformers.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: e.engineControlledFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: e.ConfigmapAddEvent,
		},
	})

	go jobInformers.Run(e.closeCh)
	go podInformers.Run(e.closeCh)
	go configmapInformers.Run(e.closeCh)

	if !cache.WaitForCacheSync(e.closeCh, jobInformers.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for job caches to sync"))
		close(e.closeCh)
		return fmt.Errorf("timed out waiting for job caches to sync")
	}
	if !cache.WaitForCacheSync(e.closeCh, podInformers.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for pod caches to sync"))
		close(e.closeCh)
		return fmt.Errorf("timed out waiting for pod caches to sync")
	}
	if !cache.WaitForCacheSync(e.closeCh, configmapInformers.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for configmap caches to sync"))
		close(e.closeCh)
		return fmt.Errorf("timed out waiting for configmap caches to sync")
	}
	e.logger.Info("kubernetes engine loop started")
	return nil
}

func (e *Engine) GetJobEventChannel() <-chan common.JobIdentity {
	return e.eventChannel
}

func (e *Engine) FetchJobStepLog(ctx context.Context, domain, jobID, stepName string) (io.ReadCloser, error) {
	pods, err := e.clientSet.CoreV1().Pods(e.ConvertToNamespace(domain)).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobID),
	})
	if err != nil {
		e.logger.Warn(fmt.Sprintf("failed to list pods for job %s/%s, error: %s", domain, jobID, err))
		return nil, err
	}
	if len(pods.Items) == 0 {
		e.logger.Warn(fmt.Sprintf("no pod found for job %s/%s.", domain, jobID))
		return nil, syserror.New(fmt.Sprintf("no pod found for job %s/%s.", domain, jobID))
	}

	runningPod := pods.Items[0]
	req := e.clientSet.CoreV1().Pods(runningPod.Namespace).GetLogs(runningPod.Name, &v1.PodLogOptions{
		Container: stepName,
		Follow:    true,
	})
	podLogReader, err := req.Stream(ctx)
	if err != nil {
		return nil, err
	}
	return podLogReader, nil
}
