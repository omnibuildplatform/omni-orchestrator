package common

import (
	"context"
	"fmt"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
	"io"
	"sync"
)

const (
	JobStepID      = "%s/%s"
	JobLogReadSize = 8 * 1024
	JobChannelSize = 100
)

type JobStepInfo struct {
	Domain   string
	JobName  string
	StepName string
}

type logManagerImpl struct {
	engine      JobEngine
	logger      *zap.Logger
	store       JobStore
	config      appconfig.LogManager
	closeCh     chan struct{}
	closed      bool
	cancelFunc  context.CancelFunc
	jobChangeCh chan Job
	stepLogCh   chan JobStepInfo
	//Job logs are split into steps
	jobLogMap sync.Map
}

func NewLogManagerImpl(engine JobEngine, store JobStore, config appconfig.LogManager, logger *zap.Logger) (LogManager, error) {
	return &logManagerImpl{
		engine:      engine,
		logger:      logger,
		config:      config,
		store:       store,
		closeCh:     make(chan struct{}, 1),
		closed:      false,
		jobChangeCh: make(chan Job, JobChannelSize),
		stepLogCh:   make(chan JobStepInfo, JobChannelSize),
	}, nil
}

func (l *logManagerImpl) Close() {
	l.closed = true
	if l.cancelFunc != nil {
		l.cancelFunc()
	}
	close(l.closeCh)
	close(l.jobChangeCh)
	close(l.stepLogCh)
}

func (l *logManagerImpl) GetName() string {
	return "log-manager"
}

func (l *logManagerImpl) StartLoop() error {
	var cancelCtx context.Context
	go l.FetchRunningSteps()
	//start up worker to sync job log
	l.logger.Info(fmt.Sprintf("starting to initialzie %d log manager worker(s) to sync job status.", l.config.Worker))
	cancelCtx, l.cancelFunc = context.WithCancel(context.TODO())
	for i := 1; i <= l.config.Worker; i++ {
		go l.SyncJobSteplog(cancelCtx, i, l.stepLogCh)
	}
	//TODO: query database to get all unlogged jobs
	l.logger.Info("log manager fully starts")
	return nil
}

func (l *logManagerImpl) FetchRunningSteps() {
	for {
		select {
		case job, ok := <-l.jobChangeCh:
			if !ok {
				l.logger.Info("channel closed log step status checker will quit")
				return
			}
			for _, step := range job.Steps {
				if step.State != StepCreated {
					identity := fmt.Sprintf(JobStepID, job.ID, step.Name)
					if _, loaded := l.jobLogMap.LoadOrStore(identity, identity); !loaded {
						//start to collect job step logs
						l.stepLogCh <- JobStepInfo{
							Domain:   job.Domain,
							JobName:  job.ID,
							StepName: step.Name,
						}
					}
				}
			}

		}
	}
}

func (l *logManagerImpl) SyncJobSteplog(ctx context.Context, index int, ch chan JobStepInfo) {
	for {
		select {
		case jobStep, ok := <-ch:
			if !ok {
				l.logger.Info(fmt.Sprintf("channel closed: log sync worker %d will quit", index))
				return
			} else {
				logReader, err := l.engine.FetchJobStepLog(ctx, jobStep.Domain, jobStep.JobName, jobStep.StepName)
				if err != nil {
					l.logger.Info(fmt.Sprintf("can't fetch job %s/%s step logs, error: %s", jobStep.Domain, jobStep.JobName, err))
				} else {
					err = l.ReadJobStepLog(ctx, logReader)
					if err != nil {
						l.logger.Info(fmt.Sprintf("can't fetch job %s/%s step logs, error: %s", jobStep.Domain, jobStep.JobName, err))
					}
				}
			}
			l.jobLogMap.Delete(fmt.Sprintf(JobStepID, jobStep.JobName, jobStep.StepName))
		}
	}
}

func (l *logManagerImpl) ReadJobStepLog(context context.Context, reader io.ReadCloser) error {
	defer reader.Close()
	for {
		select {
		case <-context.Done():
			l.logger.Info("context canceled, sync job will quit")
			return nil
		default:
			buf := make([]byte, JobLogReadSize)
			numBytes, err := reader.Read(buf)
			if numBytes == 0 {
				return nil
			}
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			message := string(buf[:numBytes])
			fmt.Println(message)
			//Save into db
		}
	}
}

func (l *logManagerImpl) GetJobChangeChannel() chan<- Job {
	return l.jobChangeCh
}
