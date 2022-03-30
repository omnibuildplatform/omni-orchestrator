package common

import (
	"context"
	"errors"
	"fmt"
	"github.com/mitchellh/mapstructure"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
	"sync"
	"time"
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
}

func NewJobManagerImpl(engine JobEngine, store JobStore, config appconfig.JobManager, logger *zap.Logger) (JobManager, error) {
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
	}, nil
}

func (m *jobManagerImpl) GetName() string {
	return ""
}

func (m *jobManagerImpl) CreateJob(ctx context.Context, job *Job, kind JobKind) error {
	var err error
	err = m.store.CreateJob(ctx, job)
	if err != nil {
		m.logger.Error(fmt.Sprintf("unable to save job info %s", err))
		return err
	}
	switch kind {
	case JobImageBuild:
		err = m.createBuildISOJob(ctx, job)
		break
	}
	if err != nil {
		job.State = JobFailed
		job.Detail = err.Error()
		updateErr := m.store.UpdateJob(ctx, job)
		if updateErr != nil {
			m.logger.Error(fmt.Sprintf("failed to update job info into database %s", updateErr))
		}
		return err
	} else {
		job.State = JobCreated
		updateErr := m.store.UpdateJob(ctx, job)
		if updateErr != nil {
			m.logger.Error(fmt.Sprintf("failed to update job info into database %s", updateErr))
			return updateErr
		}
	}
	return nil
}

func (m jobManagerImpl) createBuildISOJob(ctx context.Context, job *Job) error {
	var err error
	var para = JobImageBuildPara{}
	err = mapstructure.Decode(job.Spec, &para)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to decode job specification %s", err))
	}
	return m.engine.BuildOSImage(ctx, job, para)

}

func (m *jobManagerImpl) jobSupported(kind JobKind) bool {
	for _, j := range m.engine.GetSupportedJobs() {
		if kind == j {
			return true
		}
	}
	return false
}
func (m *jobManagerImpl) AcceptableJob(ctx context.Context, job Job) JobKind {
	if job.Engine != m.engine.GetName() {
		m.logger.Info(fmt.Sprintf("configured engine %s while job asked for engine %s", m.engine.GetName(), job.Engine))
		return JobUnrecognized
	}
	for _, j := range AvailableJobs {
		if string(j) == job.Task && m.jobSupported(j) {
			return j
		}
	}
	m.logger.Info(fmt.Sprintf("configured engine doesn't support job task %s", job.Task))
	return JobUnrecognized
}
func (m *jobManagerImpl) DeleteJob(ctx context.Context, jobID string) error {
	return nil
}
func (m *jobManagerImpl) GetJob(ctx context.Context, service, task, domain, jobID string) (Job, error) {
	return m.store.GetJob(ctx, service, task, domain, jobID)
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

func (m *jobManagerImpl) StartLoop() error {
	if m.engine == nil {
		return errors.New("task engine is empty")
	}
	eventChannel := m.engine.GetJobEventChannel()
	flushChannel := make(chan JobEvent)
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

func (m *jobManagerImpl) SyncRequestReduce(flush chan<- JobEvent, ch <-chan JobEvent) {
	ticker := time.NewTicker(time.Duration(m.config.SyncInterval) * time.Second)
	jobEventMap := make(map[string]JobEvent, 1000)
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
			jobEventMap = make(map[string]JobEvent, 1000)
		}
	}
}

func (m *jobManagerImpl) syncJobStatus(index int, ch <-chan JobEvent) {
	for {
		select {
		case job, ok := <-ch:
			if !ok {
				m.logger.Info(fmt.Sprintf("channel closed: job sync worker %d will quite", index))
				return
			}
			m.logger.Info(fmt.Sprintf("worker %d received job %s/%s change event", index, job.Domain, job.ID))
			jobRes, err := m.engine.GetJob(context.TODO(), job.Domain, job.ID)
			if err != nil {
				m.logger.Warn(fmt.Sprintf("failed to get job %s/%s information %s", job.Domain, job.ID, err))
			} else {
				err := m.store.UpdateJob(context.TODO(), jobRes)
				if err != nil {
					m.logger.Error(fmt.Sprintf("failed to update job status to store due to: %s", err))
				}
				m.jobChangeListener.Notify(*jobRes)
			}
		}
	}
}

func (m *jobManagerImpl) RegisterJobChangeNotifyChannel(ch chan<- Job) {
	m.jobChangeListener.Push(ch)
}
