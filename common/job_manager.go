package common

import (
	"context"
	"errors"
	"fmt"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"strings"
	"sync"
	"time"
)

const (
	DefaultJobTTL    = 60 * 60 * 24 * 7
	FlushChannelSize = 200
	EventMapSize     = 10000
)

type jobChangeListener struct {
	sync.Mutex
	channels []chan<- Job
}

func (c *jobChangeListener) Push(ch chan<- Job) {
	c.Lock()
	defer c.Unlock()
	c.channels = append(c.channels, ch)
}

func (c *jobChangeListener) Notify(j Job) {
	c.Lock()
	defer c.Unlock()
	for _, ch := range c.channels {
		ch <- j
	}
}

type jobManagerImpl struct {
	engine            JobEngine
	logger            *zap.Logger
	store             JobStore
	config            appconfig.JobManager
	closeCh           chan struct{}
	closed            bool
	jobChangeListener *jobChangeListener
	jobTTL            int64
}

func NewJobManagerImpl(engine JobEngine, store JobStore, config appconfig.JobManager, logger *zap.Logger) (JobManager, error) {
	var jobTTL int64
	if config.TTL == 0 {
		jobTTL = DefaultJobTTL
	} else {
		jobTTL = config.TTL
	}
	return &jobManagerImpl{
		engine:  engine,
		logger:  logger,
		config:  config,
		store:   store,
		closeCh: make(chan struct{}, 1),
		closed:  false,
		jobChangeListener: &jobChangeListener{
			channels: []chan<- Job{},
		},
		jobTTL: jobTTL,
	}, nil
}

func (m *jobManagerImpl) GetName() string {
	return ""
}

func (m *jobManagerImpl) CreateJob(ctx context.Context, job *Job, kind string) error {
	var err error
	err = m.store.CreateJob(ctx, job, m.jobTTL)
	if err != nil {
		m.logger.Error(fmt.Sprintf("unable to save job info %s", err))
		m.store.DeleteJob(ctx, job.JobIdentity)
		return err
	}
	err = m.engine.CreateJob(ctx, job)
	if err != nil {
		m.store.DeleteJob(ctx, job.JobIdentity)
		return err
	}
	oldJob, err := m.store.GetJob(ctx, job.JobIdentity)
	if err != nil {
		m.logger.Error(fmt.Sprintf("unable to get job info for update %s", err))
		return err
	}
	oldJob.Version += 1
	if err != nil {
		oldJob.State = JobFailed
		oldJob.Detail = err.Error()
		updateErr := m.store.UpdateJobStatus(ctx, &oldJob, oldJob.Version-1)
		if updateErr != nil {
			m.logger.Error(fmt.Sprintf("failed to update job info into database %s", updateErr))
		}
		return err
	} else {
		oldJob.State = JobCreated
		updateErr := m.store.UpdateJobStatus(ctx, &oldJob, oldJob.Version-1)
		if updateErr != nil {
			m.logger.Error(fmt.Sprintf("failed to update job info into database %s", updateErr))
			return updateErr
		}
	}
	return nil
}

func (m *jobManagerImpl) jobSupported(kind string) bool {
	for _, j := range m.engine.GetSupportedJobs() {
		if kind == j {
			return true
		}
	}
	return false
}
func (m *jobManagerImpl) AcceptableJob(ctx context.Context, job Job) string {
	if job.Engine != m.engine.GetName() {
		m.logger.Info(fmt.Sprintf("configured engine %s while job asked for engine %s", m.engine.GetName(), job.Engine))
		return JobUnrecognized
	}
	if m.jobSupported(job.Task) {
		return job.Task
	}
	m.logger.Info(fmt.Sprintf("configured engine doesn't support job task %s, available jobs are %s", job.Task, strings.Join(m.engine.GetSupportedJobs(), ",")))
	return JobUnrecognized
}
func (m *jobManagerImpl) DeleteJob(ctx context.Context, jobID JobIdentity) error {
	err := m.engine.DeleteJob(ctx, jobID)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to delete job %s/%s from engine %s", jobID.Domain, jobID.ID, err))
	}
	//when delete we mark all job and step state stopped
	job, err := m.store.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job.State == JobStopped || job.State == JobSucceed || job.State == JobFailed {
		return errors.New(fmt.Sprintf("unable to delete job %s/%s, it's already finished", jobID.Domain, jobID.ID))
	}
	job.State = JobStopped
	stopTime := time.Now()
	job.EndTime = stopTime
	job.Detail = "job stopped"
	for index, _ := range job.Steps {
		if job.Steps[index].State == StepCreated || job.Steps[index].State == StepRunning {
			job.Steps[index].State = StepStopped
			job.Steps[index].EndTime = stopTime
			job.Steps[index].Message = "step stopped"
		}
	}
	job.Version += 1
	err = m.store.UpdateJobStatus(ctx, &job, job.Version-1)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to delete job %s/%s from store %s", jobID.Domain, jobID.ID, err))
	}
	return nil
}
func (m *jobManagerImpl) GetJob(ctx context.Context, jobID JobIdentity) (Job, error) {
	return m.store.GetJob(ctx, jobID)
}

func (m *jobManagerImpl) BatchGetJobs(ctx context.Context, jobID JobIdentity, IDs []string) ([]Job, error) {
	return m.store.BatchGetJobs(ctx, jobID, IDs)
}
func (m *jobManagerImpl) Close() {
	m.closed = true
	close(m.closeCh)
	if m.engine != nil {
		m.engine.Close()
	}
	if m.store != nil {
		m.store.Close()
	}
}

func (m *jobManagerImpl) Reload() {
	if m.engine != nil {
		m.engine.Reload()
	}
	if m.store != nil {
		m.store.Reload()
	}
	m.logger.Info("job manager configuration reloaded")
}

func (m *jobManagerImpl) GetReloadDirs() []string {
	var dirs []string
	if m.engine != nil {
		dirs = append(dirs, m.engine.GetReloadDirs()...)
	}
	if m.store != nil {
		dirs = append(dirs, m.store.GetReloadDirs()...)
	}
	return dirs
}

func (m *jobManagerImpl) StartLoop() error {
	if m.engine == nil {
		return errors.New("task engine is empty")
	}
	eventChannel := m.engine.GetJobEventChannel()
	flushChannel := make(chan JobIdentity, FlushChannelSize)
	// SyncRequestReduce used to reduce job query count
	go m.SyncRequestReduce(flushChannel, eventChannel)
	m.logger.Info(fmt.Sprintf("starting to initialzie %d job manager worker(s) to sync job status.",
		m.config.Worker))
	for i := 1; i <= m.config.Worker; i++ {
		go m.syncJobStatus(i, flushChannel)
	}
	err := m.engine.StartLoop()
	if err != nil {
		return err
	}
	m.logger.Info("job manager fully starts")
	return nil
}

func (m *jobManagerImpl) SyncRequestReduce(flush chan<- JobIdentity, ch <-chan JobIdentity) {
	ticker := time.NewTicker(time.Duration(m.config.SyncInterval) * time.Second)
	jobEventMap := make(map[string]JobIdentity, EventMapSize)
	for {
		select {
		case job, ok := <-ch:
			if !ok {
				m.logger.Info("channel closed request reducer will quite")
				close(flush)
				return
			}
			//store job event
			if _, ok := jobEventMap[job.ID]; !ok {
				jobEventMap[job.ID] = job
			}
		case <-ticker.C:
			//flush and clear map
			for _, e := range jobEventMap {
				flush <- e
			}
			jobEventMap = make(map[string]JobIdentity, EventMapSize)
		}
	}
}

func (m *jobManagerImpl) syncJobStatus(index int, ch <-chan JobIdentity) {
	for {
		select {
		case job, ok := <-ch:
			if !ok {
				m.logger.Info(fmt.Sprintf("channel closed: job sync worker %d will quite", index))
				return
			}
			m.logger.Info(fmt.Sprintf("worker %d received job %s/%s change event", index, job.Domain, job.ID))
			oldJob, err := m.store.GetJob(context.TODO(), job)
			if err != nil {
				m.logger.Error(fmt.Sprintf("unable to get job info for update %s", err))
				break
			}
			jobRes, err := m.engine.GetJobStatus(context.TODO(), oldJob)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					m.logger.Info(fmt.Sprintf("failed to get job %s/%s information, it maybe deleted",
						job.Domain, job.ID))
				} else {
					m.logger.Error(fmt.Sprintf("failed to get job %s/%s information %s", job.Domain, job.ID, err))
				}
			} else {
				oldJob, err := m.store.GetJob(context.TODO(), jobRes.JobIdentity)
				oldJob.Version += 1
				if err != nil {
					m.logger.Error(fmt.Sprintf("unable to get job info for update %s", err))
				} else {
					oldJob.StartTime = jobRes.StartTime
					oldJob.EndTime = jobRes.EndTime
					oldJob.State = jobRes.State
					if len(jobRes.Steps) != 0 {
						oldJob.Steps = jobRes.Steps
					}
					err := m.store.UpdateJobStatus(context.TODO(), &oldJob, oldJob.Version-1)
					if err != nil {
						m.logger.Error(fmt.Sprintf("failed to update job status to store due to: %s", err))
					}
					m.jobChangeListener.Notify(*jobRes)

				}
			}
		}
	}
}

func (m *jobManagerImpl) RegisterJobChangeNotifyChannel(ch chan<- Job) {
	m.jobChangeListener.Push(ch)
}
