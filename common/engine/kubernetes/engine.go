package kubernetes

import (
	"context"
	syserror "errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	//Plugins Register
	_ "github.com/omnibuildplatform/omni-orchestrator/common/engine/kubernetes/extended_jobs/buildimagefromiso"
	_ "github.com/omnibuildplatform/omni-orchestrator/common/engine/kubernetes/extended_jobs/buildimagefromrelease"
	"github.com/omnibuildplatform/omni-orchestrator/common/engine/kubernetes/extended_jobs/plugins"
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
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	ArchitectureKey     = "architecture"
	AppLabelKey         = "omni-orchestrator"
	AppLabelValue       = "true"
	AnnotationService   = "omni-tag/service"
	AnnotationDomain    = "omni-tag/domain"
	AnnotationTask      = "omni-tag/task"
	AnnotationArch      = "omni-tag/architecture"
	DefaultSyncInterval = 3
	DefaultEventChannel = 200
)

type BuildImagePackages struct {
	Packages []string `json:"packages"`
}

type Engine struct {
	sync.Mutex
	logger           *zap.Logger
	config           appconfig.Engine
	x86clientSet     *kubernetes.Clientset
	aarch64clientSet *kubernetes.Clientset
	x86Factory       *informers.SharedInformerFactory
	aarch64Factory   *informers.SharedInformerFactory
	closeCh          chan struct{}
	eventChannel     chan common.JobIdentity
	JobWatchMap      sync.Map
	JobPluginMap     map[string]plugins.JobHandler
}

func NewEngine(config appconfig.Engine, logger *zap.Logger) (common.JobEngine, error) {
	var x86clientSet, aarch64ClientSet *kubernetes.Clientset
	var x86Factory, aarch64Factory *informers.SharedInformerFactory
	//x86 client will try to use in cluster mode
	x86k8sConfig, err := clientcmd.BuildConfigFromFlags("", config.X86ConfigFile)
	if err != nil {
		return nil, err
	}
	x86clientSet, err = kubernetes.NewForConfig(x86k8sConfig)
	if err != nil {
		return nil, err
	}
	factory1 := informers.NewSharedInformerFactoryWithOptions(x86clientSet, DefaultSyncInterval*time.Second,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = fmt.Sprintf("%s=%s", AppLabelKey, AppLabelValue)
		}))
	x86Factory = &factory1
	if config.Aarch64ConfigFile != "" {
		aarch64Config, err := clientcmd.BuildConfigFromFlags("", config.Aarch64ConfigFile)
		if err != nil {
			return nil, err
		}
		aarch64ClientSet, err = kubernetes.NewForConfig(aarch64Config)
		if err != nil {
			return nil, err
		}
		factory2 := informers.NewSharedInformerFactoryWithOptions(aarch64ClientSet, DefaultSyncInterval*time.Second,
			informers.WithTweakListOptions(func(options *metav1.ListOptions) {
				options.LabelSelector = fmt.Sprintf("%s=%s", AppLabelKey, AppLabelValue)
			}))
		aarch64Factory = &factory2
		if err != nil {
			return nil, err
		}
	} else {
		logger.Info("aarch64 kubernetes engine not configured")
	}
	return &Engine{
		logger:           logger,
		config:           config,
		x86clientSet:     x86clientSet,
		aarch64clientSet: aarch64ClientSet,
		x86Factory:       x86Factory,
		aarch64Factory:   aarch64Factory,
		closeCh:          make(chan struct{}),
		eventChannel:     make(chan common.JobIdentity, DefaultEventChannel),
		JobWatchMap:      sync.Map{},
		JobPluginMap:     make(map[string]plugins.JobHandler),
	}, nil
}

func (e *Engine) Initialize() error {
	if e.x86clientSet != nil {
		v, err := e.x86clientSet.ServerVersion()
		if err != nil {
			return err
		}
		e.logger.Info(fmt.Sprintf("x86 kubernetes connected %s", v.String()))
	}
	if e.aarch64clientSet != nil {
		v, err := e.aarch64clientSet.ServerVersion()
		if err != nil {
			return err
		}
		e.logger.Info(fmt.Sprintf("aarch64 kubernetes connected %s", v.String()))
	}
	return nil
}

func (e *Engine) CreateNamespaceIfNeeded(id *common.JobIdentity) error {
	ns := e.ConvertToNamespace(id.Domain)
	_, err := e.GetClientSet(id.ExtraIdentities).CoreV1().Namespaces().Get(context.TODO(), ns, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			namespace := v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
					Annotations: map[string]string{
						"controlledBy": "omni-orchestrator",
					},
					Labels: e.generateSystemLabels(map[string]string{}),
				},
			}
			_, err := e.GetClientSet(id.ExtraIdentities).CoreV1().Namespaces().Create(context.TODO(), &namespace, metav1.CreateOptions{})
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

func (e *Engine) Reload() {
	e.Lock()
	defer e.Unlock()
	for _, p := range e.JobPluginMap {
		p.Reload()
	}
	e.logger.Info("job engine configuration reloaded")
}

func (e *Engine) GetReloadDirs() []string {
	var dirs []string
	err := filepath.Walk(e.config.KubernetesTemplateFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.TrimRight(e.config.KubernetesTemplateFolder, "/") != path {
				dirs = append(dirs, path)
			}
		}
		return nil
	})
	if err != nil {
		e.logger.Error(fmt.Sprintf("unable to collect extend job templates folders %v", err))
		return []string{}
	}
	return dirs
}

func (e *Engine) GetName() string {
	return "kubernetes"
}
func (e *Engine) GetSupportedJobs() []string {
	return plugins.GetRegisteredJobPluginNames()
}

func (e *Engine) generateSystemAnnotations(existing map[string]string, job *common.JobIdentity) map[string]string {
	annotations := make(map[string]string)
	for k, v := range existing {
		annotations[k] = v
	}
	annotations[AnnotationService] = job.Service
	annotations[AnnotationTask] = job.Task
	annotations[AnnotationDomain] = job.Domain
	annotations[AnnotationArch] = e.getArchitectureKey(job.ExtraIdentities)
	return annotations
}

func (e *Engine) hasSystemAnnotations(annotations map[string]string) bool {
	if _, ok := annotations[AnnotationService]; ok {
		if _, ok := annotations[AnnotationTask]; ok {
			if _, ok := annotations[AnnotationDomain]; ok {
				if _, ok := annotations[AnnotationArch]; ok {
					return true
				}
			}
		}
	}
	return false
}

func (e *Engine) generateSystemLabels(existing map[string]string) map[string]string {
	labels := make(map[string]string)
	for k, v := range existing {
		labels[k] = v
	}
	labels[AppLabelKey] = AppLabelValue
	return labels
}

func (e *Engine) ConvertToNamespace(domain string) string {
	return strings.Replace(strings.ToLower(domain), ".", "-", -1)
}

func (e *Engine) getArchitectureKey(attrs map[string]string) string {
	if len(attrs) == 0 {
		return "x86"
	}
	if v, ok := attrs[ArchitectureKey]; !ok {
		return "x86"
	} else {
		return v
	}
}

func (e *Engine) GetClientSet(attrs map[string]string) *kubernetes.Clientset {
	key := e.getArchitectureKey(attrs)
	switch key {
	case "x86":
		return e.x86clientSet
	case "x86_64":
		return e.x86clientSet
	case "aarch64":
		return e.aarch64clientSet
	default:
		e.logger.Error(fmt.Sprintf("unable to find related kubernetes clientset %s, will use x86 as default", key))
		return e.x86clientSet
	}
}

func (e *Engine) GetKubernetesJobIdentity(job *common.Job, architecture string) common.JobIdentity {
	return common.JobIdentity{
		Service: job.Service,
		Task:    job.Task,
		Domain:  job.Domain,
		ID:      job.ID,
		ExtraIdentities: map[string]string{
			ArchitectureKey: strings.ToLower(architecture),
		},
	}
}

func (e *Engine) deleteSingleResource(job *common.Job, resource interface{}) error {
	switch resource.(type) {
	case *batchv1.Job:
		return e.GetClientSet(job.ExtraIdentities).BatchV1().Jobs(e.ConvertToNamespace(job.Domain)).Delete(context.TODO(), job.ID, metav1.DeleteOptions{})
	case *v1.ConfigMap:
		return e.GetClientSet(job.ExtraIdentities).CoreV1().ConfigMaps(e.ConvertToNamespace(job.Domain)).Delete(context.TODO(), job.ID, metav1.DeleteOptions{})
	default:
		e.logger.Warn(fmt.Sprintf("unsupported kubernetes resource %s", resource))
	}
	return nil
}

func (e *Engine) createSingleResource(job *common.Job, resource interface{}) error {
	var err error
	switch resource.(type) {
	case *batchv1.Job:
		//additional labels and annotations
		k8sJob := resource.(*batchv1.Job)
		k8sJob.Annotations = e.generateSystemAnnotations(k8sJob.Annotations, &job.JobIdentity)
		k8sJob.Labels = e.generateSystemLabels(k8sJob.Labels)
		k8sJob.Spec.Template.Annotations = e.generateSystemAnnotations(k8sJob.Spec.Template.Annotations, &job.JobIdentity)
		k8sJob.Spec.Template.Labels = e.generateSystemLabels(k8sJob.Spec.Template.Labels)
		_, err = e.GetClientSet(job.ExtraIdentities).BatchV1().Jobs(e.ConvertToNamespace(job.Domain)).Get(context.TODO(), job.ID, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err := e.GetClientSet(job.ExtraIdentities).BatchV1().Jobs(e.ConvertToNamespace(job.Domain)).Create(context.TODO(), k8sJob, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			return nil
		} else if err == nil {
			_, err = e.GetClientSet(job.ExtraIdentities).BatchV1().Jobs(e.ConvertToNamespace(job.Domain)).Update(context.TODO(), k8sJob, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			return nil
		}
		return nil
	case *v1.ConfigMap:
		//additional labels and annotations
		k8sConfigmap := resource.(*v1.ConfigMap)
		k8sConfigmap.Annotations = e.generateSystemAnnotations(k8sConfigmap.Annotations, &job.JobIdentity)
		k8sConfigmap.Labels = e.generateSystemLabels(k8sConfigmap.Labels)
		_, err = e.GetClientSet(job.ExtraIdentities).CoreV1().ConfigMaps(e.ConvertToNamespace(job.Domain)).Get(context.TODO(), job.ID, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			_, err := e.GetClientSet(job.ExtraIdentities).CoreV1().ConfigMaps(e.ConvertToNamespace(job.Domain)).Create(context.TODO(), k8sConfigmap, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			return nil
		} else if err == nil {
			_, err := e.GetClientSet(job.ExtraIdentities).CoreV1().ConfigMaps(e.ConvertToNamespace(job.Domain)).Update(context.TODO(), k8sConfigmap, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			return nil
		}
	default:
		e.logger.Warn(fmt.Sprintf("unsupported kubernetes resource %s", resource))
	}
	return nil
}

func (e *Engine) GetJobHandler(task string) (plugins.JobHandler, error) {
	e.Lock()
	defer e.Unlock()
	if v, ok := e.JobPluginMap[task]; ok {
		return v, nil
	} else {
		jobHandler, err := plugins.SupportedJobPlugins[task].CreateJobHandler(path.Join(e.config.KubernetesTemplateFolder, strings.ToLower(task)), e.logger)
		if err != nil {
			return nil, err
		}
		e.JobPluginMap[task] = jobHandler
		return jobHandler, nil
	}
}

func (e *Engine) CreateJob(ctx context.Context, job *common.Job) error {
	var err error
	var deleteResource = true
	var createdResource = make(map[plugins.KubernetesResource]interface{})
	//1. initialize job handler
	jobHandler, err := e.GetJobHandler(job.Task)
	if err != nil {
		return err
	}
	templates, architecture, err := jobHandler.Serialize(e.ConvertToNamespace(job.Domain), job.ID, *job)
	for k, v := range templates {
		if k == plugins.ResUnSupported {
			return syserror.New(fmt.Sprintf("%s resource is unsupported, please update the available resource type", v))
		}
	}
	if err != nil {
		return err
	}
	jobId := e.GetKubernetesJobIdentity(job, architecture)
	//2. prepare namespace
	err = e.CreateNamespaceIfNeeded(&jobId)
	if err != nil {
		return err
	}
	//3. delete resource if failed creating
	defer func(delete *bool) {
		if *delete {
			for k, r := range createdResource {
				e.logger.Info(fmt.Sprintf("task creation revert, delete resource %s", k))
				err := e.deleteSingleResource(job, r)
				if err != nil {
					e.logger.Warn(fmt.Sprintf("failed to delete kubernetes resource %s for job %s/%s", k, job.Domain, job.ID))
				} else {
					e.logger.Info(fmt.Sprintf("kubernetes resource %s deleted for job %s/%s", k, job.Domain, job.ID))
				}
			}
		}
	}(&deleteResource)
	//4. iterate yaml resource and create
	for k, bytes := range templates {
		decode := scheme.Codecs.UniversalDeserializer().Decode
		rs, _, err := decode(bytes, nil, nil)
		if err != nil {
			return syserror.New(fmt.Sprintf("unable to decode kubernetes resource %s for job %s", k, job.Task))
		}
		err = e.createSingleResource(job, rs)
		if err != nil {
			return err
		}
		e.logger.Info(fmt.Sprintf("resource %s created for job %s/%s and type %s", k, job.Domain, job.ID, job.Task))
		createdResource[k] = rs
	}
	deleteResource = false
	return nil
}

func (e *Engine) collectJobAnnotations(namespace, name string, annotations map[string]string) (string, string, string, map[string]string) {
	var service, task, domain string
	extraIDs := make(map[string]string)
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
	if value, ok := annotations[AnnotationArch]; !ok {
		e.logger.Warn(fmt.Sprintf("job %s/%s event has been dropped due to '%s' not found",
			namespace, name, AnnotationArch))
	} else {
		extraIDs[ArchitectureKey] = value
	}
	return service, task, domain, extraIDs
}

func (e *Engine) GetJobStatus(ctx context.Context, job common.Job) (*common.Job, error) {
	existing, err := e.GetClientSet(job.ExtraIdentities).BatchV1().Jobs(e.ConvertToNamespace(job.Domain)).Get(context.TODO(), job.ID, metav1.GetOptions{})
	if err == nil {
		completionRequired := existing.Spec.Completions
		backoffLimitRequired := existing.Spec.BackoffLimit
		if existing.Status.Succeeded >= *completionRequired {
			job.State = common.JobSucceed

		} else if existing.Status.Failed > *backoffLimitRequired {
			job.State = common.JobFailed
		} else {
			job.State = common.JobRunning
		}
		if existing.Status.StartTime != nil {
			job.StartTime = existing.Status.StartTime.Time
		}
		// Find out job end time via CompletionTime or condition
		if job.State == common.JobSucceed {
			if existing.Status.CompletionTime != nil {
				job.EndTime = existing.Status.CompletionTime.Time
			}
		}
		if job.State == common.JobFailed {
			for _, c := range existing.Status.Conditions {
				if c.Type == batchv1.JobFailed {
					job.EndTime = c.LastTransitionTime.Time
					break
				}
			}
		}
		job.Service, job.Task, job.Domain, job.ExtraIdentities = e.collectJobAnnotations(
			job.Domain, job.ID, existing.Annotations)
		//append steps
		newSteps := e.CollectSteps(job.JobIdentity, existing)
		if len(newSteps) != 0 {
			job.Steps = newSteps
		}
		// mark step failed if necessary
		if job.State == common.JobFailed {
			for index, _ := range job.Steps {
				if job.Steps[index].State == common.StepRunning || job.Steps[index].State == common.StepCreated {
					job.Steps[index].State = common.StepFailed
					job.Steps[index].EndTime = time.Now()
				}
			}
		}
		return &job, nil
	}
	return nil, err
}

func (e *Engine) DeleteJob(ctx context.Context, jobID common.JobIdentity) error {
	err := e.GetClientSet(jobID.ExtraIdentities).BatchV1().Jobs(e.ConvertToNamespace(jobID.Domain)).Delete(ctx, jobID.ID, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			e.logger.Warn(fmt.Sprintf("unable to delete job %s/%s in kubernetes, it may be deleted before",
				e.ConvertToNamespace(jobID.Domain), jobID.ID))
			return nil
		}
		return err
	}
	e.DeleteJobRelatedResource(&jobID)
	return nil
}

func (e *Engine) DeleteJobRelatedResource(jobID *common.JobIdentity) {
	//delete configmap
	ns := e.ConvertToNamespace(jobID.Domain)
	err := e.GetClientSet(jobID.ExtraIdentities).CoreV1().ConfigMaps(ns).Delete(context.TODO(), jobID.ID, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			e.logger.Info(fmt.Sprintf("unable to delete job configmap %s/%s in kubernetes, it may be deleted before",
				ns, jobID.ID))
		} else {
			e.logger.Warn(fmt.Sprintf("unable to delete job configmap %s/%s in kubernetes, error: %s",
				ns, jobID.ID, err))
		}
	} else {
		e.logger.Info(fmt.Sprintf("Job %s/%s related configmap has been deleted", ns, jobID.ID))
	}
	//delete pod
	pods, err := e.GetClientSet(jobID.ExtraIdentities).CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobID.ID),
	})
	if len(pods.Items) != 0 {
		graceful := int64(10)
		delOption := metav1.DeleteOptions{
			GracePeriodSeconds: &graceful,
		}
		for _, pd := range pods.Items {
			err := e.GetClientSet(jobID.ExtraIdentities).CoreV1().Pods(pd.Namespace).Delete(context.TODO(), pd.Name, delOption)
			if err == nil {
				e.logger.Info(fmt.Sprintf("job pod %s/%s has been deleted", ns, jobID.ID))
			} else {
				if errors.IsNotFound(err) {
					e.logger.Info(fmt.Sprintf("unable to delete job pod %s/%s in kubernetes, it may be deleted before",
						ns, jobID.ID))
				} else {
					e.logger.Warn(fmt.Sprintf("unable to delete job pod %s/%s in kubernetes, error: %s",
						ns, jobID.ID, err))
				}
			}
		}
	}
}

func (e *Engine) CollectSteps(jobID common.JobIdentity, job *batchv1.Job) []common.Step {
	var steps []common.Step
	pods, err := e.GetClientSet(jobID.ExtraIdentities).CoreV1().Pods(job.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", job.Name),
	})
	if err != nil {
		e.logger.Warn(fmt.Sprintf("failed to list pods for job %s/%s, error: %s", job.Namespace, job.Name, err))
		return steps
	}
	if len(pods.Items) == 0 {
		e.logger.Warn(fmt.Sprintf("failed to list pods for job %s/%s", job.Namespace, job.Name))
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
	event.Service, event.Task, event.Domain, event.ExtraIdentities = e.collectJobAnnotations(namespace, name, annotations)
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
			_, err := e.GetClientSet(configmap.Annotations).BatchV1().Jobs(configmap.Namespace).Get(context.TODO(), configmap.Name, metav1.GetOptions{})
			if err != nil && errors.IsNotFound(err) {
				err = e.GetClientSet(configmap.Annotations).CoreV1().ConfigMaps(configmap.Namespace).Delete(context.TODO(),
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
	jobID := common.JobIdentity{}
	jobID.ID = job.Name
	jobID.Service, jobID.Task, jobID.Domain, jobID.ExtraIdentities = e.collectJobAnnotations(job.Namespace, job.Name, job.Annotations)
	e.DeleteJobRelatedResource(&jobID)
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
	if e.x86Factory != nil {
		err := e.startX86Loop()
		if err != nil {
			return err
		}
	}
	if e.aarch64Factory != nil {
		err := e.startAarch64Loop()
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) startX86Loop() error {
	jobInformers := (*e.x86Factory).Batch().V1().Jobs().Informer()
	podInformers := (*e.x86Factory).Core().V1().Pods().Informer()
	configmapInformers := (*e.x86Factory).Core().V1().ConfigMaps().Informer()
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
		runtime.HandleError(fmt.Errorf("timed out waiting for x86 job caches to sync"))
		close(e.closeCh)
		return fmt.Errorf("timed out waiting for x86 job caches to sync")
	}
	if !cache.WaitForCacheSync(e.closeCh, podInformers.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for x86 pod caches to sync"))
		close(e.closeCh)
		return fmt.Errorf("timed out waiting for x86 pod caches to sync")
	}
	if !cache.WaitForCacheSync(e.closeCh, configmapInformers.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for x86 configmap caches to sync"))
		close(e.closeCh)
		return fmt.Errorf("timed out waiting for x86 configmap caches to sync")
	}
	e.logger.Info("x86 kubernetes engine loop started")
	return nil
}

func (e *Engine) startAarch64Loop() error {
	jobInformers := (*e.aarch64Factory).Batch().V1().Jobs().Informer()
	podInformers := (*e.aarch64Factory).Core().V1().Pods().Informer()
	configmapInformers := (*e.aarch64Factory).Core().V1().ConfigMaps().Informer()
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
		runtime.HandleError(fmt.Errorf("timed out waiting for aarch64 job caches to sync"))
		close(e.closeCh)
		return fmt.Errorf("timed out waiting for aarch64 job caches to sync")
	}
	if !cache.WaitForCacheSync(e.closeCh, podInformers.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for aarch64 pod caches to sync"))
		close(e.closeCh)
		return fmt.Errorf("timed out waiting for aarch64 pod caches to sync")
	}
	if !cache.WaitForCacheSync(e.closeCh, configmapInformers.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for aarch64 configmap caches to sync"))
		close(e.closeCh)
		return fmt.Errorf("timed out waiting for aarch64 configmap caches to sync")
	}
	e.logger.Info("aarch64 kubernetes engine loop started")
	return nil
}

func (e *Engine) GetJobEventChannel() <-chan common.JobIdentity {
	return e.eventChannel
}

func (e *Engine) FetchJobStepLog(ctx context.Context, jobID common.JobIdentity, stepName string) (io.ReadCloser, error) {
	ns := e.ConvertToNamespace(jobID.Domain)
	pods, err := e.GetClientSet(jobID.ExtraIdentities).CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobID.ID),
	})
	if err != nil {
		e.logger.Warn(fmt.Sprintf("failed to list pods for job %s/%s, error: %s", ns, jobID.ID, err))
		return nil, err
	}
	if len(pods.Items) == 0 {
		e.logger.Warn(fmt.Sprintf("no pod found for job %s/%s.", ns, jobID.ID))
		return nil, syserror.New(fmt.Sprintf("no pod found for job %s/%s.", ns, jobID.ID))
	}

	runningPod := pods.Items[0]
	req := e.GetClientSet(jobID.ExtraIdentities).CoreV1().Pods(runningPod.Namespace).GetLogs(runningPod.Name, &v1.PodLogOptions{
		Container: stepName,
		Follow:    true,
	})
	podLogReader, err := req.Stream(ctx)
	if err != nil {
		return nil, err
	}
	return podLogReader, nil
}
