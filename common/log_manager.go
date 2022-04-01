package common

import (
	"context"
	"fmt"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
	"io"
	"strconv"
	"sync"
	"time"
)

const (
	JobStepID           = "%s/%s"
	JobLogReadSize      = 8 * 1024
	JobChannelSize      = 100
	JobLogStoreInterval = 10
)

type JobStepInfo struct {
	Service  string
	Task     string
	Domain   string
	JobID    string
	StepID   string
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
					//skip finished jobs
					if !l.store.JobStepLogFinished(context.TODO(), job.JobIdentity, strconv.Itoa(step.ID)) {
						identity := fmt.Sprintf(JobStepID, job.ID, step.Name)
						if _, loaded := l.jobLogMap.LoadOrStore(identity, identity); !loaded {
							//start to collect job step logs
							l.stepLogCh <- JobStepInfo{
								Service:  job.Service,
								Task:     job.Task,
								Domain:   job.Domain,
								JobID:    job.ID,
								StepID:   strconv.Itoa(step.ID),
								StepName: step.Name,
							}
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
				logReader, err := l.engine.FetchJobStepLog(ctx, jobStep.Domain, jobStep.JobID, jobStep.StepName)
				if err != nil {
					l.logger.Info(fmt.Sprintf("can't fetch job %s/%s step logs, error: %s", jobStep.Domain, jobStep.JobID, err))
				} else {
					err = l.ReadJobStepLog(ctx, jobStep, logReader)
					if err != nil {
						l.logger.Info(fmt.Sprintf("can't fetch job %s/%s step logs, error: %s", jobStep.Domain, jobStep.JobID, err))
					}
				}
			}
			l.jobLogMap.Delete(fmt.Sprintf(JobStepID, jobStep.JobID, jobStep.StepID))
		}
	}
}

func (l *logManagerImpl) GetJobStepLogs(ctx context.Context, jobID JobIdentity, stepID string, startTime string, maxRecord int) (*JobLogPart, error) {
	return l.store.GetJobStepLogs(ctx, jobID, stepID, startTime, maxRecord)
}

func (l *logManagerImpl) InsertLogPart(context context.Context, jobStep JobStepInfo, data []byte, logTime time.Time) {
	log := JobStepLog{
		JobIdentity: JobIdentity{
			Service: jobStep.Service,
			Task:    jobStep.Task,
			Domain:  jobStep.Domain,
			ID:      jobStep.JobID,
		},
		StepID:  jobStep.StepID,
		LogTime: logTime,
		Data:    data,
	}
	err := l.store.InsertJobStepLog(context, &log)
	if err != nil {
		l.logger.Error(fmt.Sprintf("failed to insert job log into job store: %s", err))
	}
	l.logger.Info(fmt.Sprintf("job: %s/%s %d log data saved into log store", jobStep.JobID, jobStep.StepName, len(data)))
}

func (l *logManagerImpl) ReadJobStepLog(context context.Context, jobStep JobStepInfo, reader io.ReadCloser) error {
	defer reader.Close()
	var logs []byte
	ticker := time.NewTicker(JobLogStoreInterval * time.Second)
	for {
		select {
		case <-ticker.C:
			if len(logs) == 0 {
				break
			}
			logsData := make([]byte, len(logs))
			copy(logsData, logs)
			logTime := time.Now().Add(time.Duration(JobLogStoreInterval) * time.Second * -1)
			logs = []byte{}
			go l.InsertLogPart(context, jobStep, logsData, logTime)
		case <-context.Done():
			l.logger.Info("context canceled, sync job will quit")
			goto LASTLOG
		default:
			buf := make([]byte, JobLogReadSize)
			numBytes, err := reader.Read(buf)
			if numBytes == 0 {
				goto LASTLOG
			}
			if err == io.EOF {
				goto LASTLOG
			}
			if err != nil {
				l.logger.Error(fmt.Sprintf("failed to collect logs for job %s/%s, err %s", jobStep.JobID, jobStep.StepID, err))
				goto LASTLOG
			}
			logs = append(logs, buf[:numBytes]...)
		}
	}
LASTLOG:
	l.logger.Info(fmt.Sprintf("collect job step %s/%s log finished", jobStep.JobID, jobStep.StepID))
	if len(logs) != 0 {
		go l.InsertLogPart(context, jobStep, logs, time.Now())
	}
	//Append finished log
	go l.InsertLogPart(context, jobStep, []byte(LogCompleteFlag), time.Now())
	return nil
}

func (l *logManagerImpl) GetJobChangeChannel() chan<- Job {
	return l.jobChangeCh
}
