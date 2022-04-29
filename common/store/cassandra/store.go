package cassandra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gocql/gocql"
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
	"time"
)

const (
	MaxJobStepLogRecord    = 20
	InsertJobQueryTemplate = `INSERT INTO job_info (` +
		`service, task, domain, job_date, job_id, user_id, engine, spec, started_time, finished_time, state, steps, detail, version) ` +
		`VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS USING TTL ?`
	InsertJobStepLogQueryTemplate = `INSERT INTO log_info (` +
		`service, task, domain, job_id, step_id, log_time, data) ` +
		`VALUES(?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS USING TTL ?`

	DeleteJobStepLogQueryTemplate = `DELETE FROM log_info ` +
		`WHERE service = ? ` +
		`and task = ? ` +
		`and domain = ? ` +
		`and job_id = ? ` +
		`and step_id = ?`

	DeleteJobLogQueryTemplate = `DELETE FROM log_info ` +
		`WHERE service = ? ` +
		`and task = ? ` +
		`and domain = ? ` +
		`and job_id = ? `

	DeleteJobQueryTemplate = `DELETE FROM job_info ` +
		`WHERE service = ? ` +
		`and task = ? ` +
		`and domain = ? ` +
		`and job_date = ? ` +
		`and job_id = ?`

	UpdateJobQueryTemplate = `UPDATE job_info ` +
		`SET started_time = ?, ` +
		`finished_time = ?, ` +
		`state = ?, ` +
		`steps = ?, ` +
		`detail = ?, ` +
		`version = ?` +
		`WHERE service = ? ` +
		`and task = ? ` +
		`and domain = ? ` +
		`and job_date = ? ` +
		`and job_id = ?` +
		`IF version = ?`
	GetJobQueryTemplate = `SELECT service, task, domain, job_id, user_id, engine, spec, started_time, finished_time, state, steps, detail, version ` +
		`FROM job_info ` +
		`WHERE service = ? ` +
		`and task = ? ` +
		`and domain = ? ` +
		`and job_date = ? ` +
		`and job_id IN ? `

	GetJobLogQueryTemplate = `SELECT log_time, data ` +
		`FROM log_info ` +
		`WHERE service = ? ` +
		`and task = ? ` +
		`and domain = ? ` +
		`and job_id = ? ` +
		`and step_id = ?` +
		`and log_time > ?` +
		`LIMIT ?`

	GetJobLastLogQueryTemplate = `SELECT data ` +
		`FROM log_info ` +
		`WHERE service = ? ` +
		`and task = ? ` +
		`and domain = ? ` +
		`and job_id = ? ` +
		`and step_id = ?` +
		`ORDER BY log_time DESC LIMIT 1`
)

type Store struct {
	session *Session
	logger  *zap.Logger
	config  appconfig.PersistentStore
}

func NewJobStore(config appconfig.PersistentStore, logger *zap.Logger) (common.JobStore, error) {
	session, err := NewSession(config, logger)
	if err != nil {
		return nil, err
	}
	return &Store{
		session: session,
		logger:  logger,
		config:  config,
	}, nil
}

func (s *Store) Close() {
	if s.session != nil {
		s.logger.Info("job store will quit")
		s.session.Close()
	}
}

func (s *Store) Reload() {
	s.logger.Info("job store configuration reloaded")
}

func (s *Store) GetReloadDirs() []string {
	return []string{}
}

func (s *Store) GetName() string {
	return "cassandra"
}

func (s *Store) Initialize() error {
	return nil
}

func (s *Store) UpdateJobStatus(ctx context.Context, job *common.Job, version int32) error {
	if job.ID == "" || job.Domain == "" || job.Service == "" || job.Task == "" {
		return errors.New("job id, domain, service, task is empty")
	}
	jobTime, err := gocql.ParseUUID(job.ID)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to parse job id %s", err.Error()))
	}
	jobDate := jobTime.Time().Format("2006-01-02")
	var steps []map[string]interface{}
	if job != nil {
		for _, step := range job.Steps {
			value := make(map[string]interface{})
			value["step_id"] = step.ID
			value["name"] = step.Name
			value["state"] = step.State
			value["started_time"] = step.StartTime.UnixMilli()
			value["finished_time"] = step.EndTime.UnixMilli()
			value["message"] = step.Message
			steps = append(steps, value)
		}
	}
	query := s.session.Query(UpdateJobQueryTemplate, job.StartTime.UnixMilli(), job.EndTime.UnixMilli(),
		job.State, steps, job.Detail, job.Version, job.Service, job.Task, job.Domain, jobDate, job.ID, version).WithContext(ctx)
	applied, err := query.MapScanCAS(make(map[string]interface{}))
	if err != nil {
		return err
	}
	if !applied {
		return fmt.Errorf("update job operation failed because of job not found")
	}
	return nil

}

func (s *Store) InsertJobStepLog(ctx context.Context, log *common.JobStepLog, ttl int64) error {
	query := s.session.Query(InsertJobStepLogQueryTemplate, log.Service, log.Task, log.Domain, log.ID,
		log.StepID, gocql.UUIDFromTime(log.LogTime).String(), log.Data, ttl).WithContext(ctx)
	applied, err := query.MapScanCAS(make(map[string]interface{}))
	if err != nil {
		return err
	}
	if !applied {
		return fmt.Errorf("insert job log operation failed")
	}
	return nil
}

func (s *Store) getJobDateStringFromID(jobID string) (string, error) {
	jobTime, err := gocql.ParseUUID(jobID)
	if err != nil {
		return "", errors.New(fmt.Sprintf("unable to parse job id %s", err.Error()))
	}
	return jobTime.Time().Format("2006-01-02"), nil
}

func (s *Store) DeleteJobStepLog(ctx context.Context, log *common.JobStepLog) error {
	query := s.session.Query(DeleteJobStepLogQueryTemplate, log.Service, log.Task, log.Domain, log.ID,
		log.StepID).WithContext(ctx)
	return query.Exec()
}
func (s *Store) CreateJob(ctx context.Context, job *common.Job, ttl int64) error {
	job.ID = gocql.TimeUUID().String()
	jobDate := time.Now().Format("2006-01-02")
	jobSpec, err := json.Marshal(job.Spec)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to encode job spec %s", err))
	}
	//always zero when insert
	job.Version = 0
	var steps []map[string]interface{}
	if job != nil {
		for _, step := range job.Steps {
			value := make(map[string]interface{})
			value["step_id"] = step.ID
			value["name"] = step.Name
			value["state"] = step.State
			value["started_time"] = step.StartTime.UnixMilli()
			value["finished_time"] = step.EndTime.UnixMilli()
			value["message"] = step.Message
			steps = append(steps, value)
		}
	}
	query := s.session.Query(InsertJobQueryTemplate, job.Service, job.Task, job.Domain, jobDate, job.ID, job.UserID, job.Engine,
		jobSpec, job.StartTime.UnixMilli(), job.EndTime.UnixMilli(), job.State, steps, job.Detail, job.Version, ttl).WithContext(ctx)
	applied, err := query.MapScanCAS(make(map[string]interface{}))
	if err != nil {
		return err
	}
	if !applied {
		return fmt.Errorf("create job operation failed because of uuid collision")
	}
	return nil
}

func (s *Store) DeleteJob(ctx context.Context, jobID common.JobIdentity) error {
	jobDate, err := s.getJobDateStringFromID(jobID.ID)
	if err != nil {
		return err
	}
	query := s.session.Query(DeleteJobQueryTemplate, jobID.Service, jobID.Task, jobID.Domain, jobDate, jobID.ID).WithContext(ctx)
	return query.Exec()
}

func (s *Store) DeleteJobLog(ctx context.Context, jobID common.JobIdentity) error {
	query := s.session.Query(DeleteJobLogQueryTemplate, jobID.Service, jobID.Task, jobID.Domain, jobID.ID).WithContext(ctx)
	return query.Exec()
}

func (s *Store) BatchGetJobs(ctx context.Context, jobID common.JobIdentity, IDs []string) ([]common.Job, error) {
	jobDataMap := make(map[string][]string)
	var jobs []common.Job
	for _, id := range IDs {
		jobDate, err := s.getJobDateStringFromID(id)
		if err != nil {
			s.logger.Warn(fmt.Sprintf("job ID %s format is invalid, will be skipped, error %s", id, err))
			continue
		}
		if _, ok := jobDataMap[jobDate]; ok {
			jobDataMap[jobDate] = append(jobDataMap[jobDate], id)
		} else {
			jobDataMap[jobDate] = []string{id}
		}
	}
	for date, ids := range jobDataMap {
		jobsPart, err := s.getJobs(ctx, jobID.Service, jobID.Task, jobID.Domain, date, ids)
		if err != nil {
			return []common.Job{}, err
		}
		jobs = append(jobs, jobsPart...)
	}
	return jobs, nil
}

func (s *Store) GetJob(ctx context.Context, jobID common.JobIdentity) (common.Job, error) {
	jobDate, err := s.getJobDateStringFromID(jobID.ID)
	if err != nil {
		return common.Job{}, err
	}
	jobs, err := s.getJobs(ctx, jobID.Service, jobID.Task, jobID.Domain, jobDate, []string{jobID.ID})
	if err != nil {
		return common.Job{}, err
	}
	if len(jobs) == 0 {
		return common.Job{}, errors.New("job not found")
	}
	return jobs[0], nil
}

func (s *Store) getJobs(ctx context.Context, service, task, domain, date string, IDs []string) ([]common.Job, error) {
	var jobs []common.Job
	query := s.session.Query(GetJobQueryTemplate, service, task, domain, date, IDs).PageSize(len(IDs)).WithContext(context.TODO())
	iter := query.Iter()
	if iter == nil {
		return []common.Job{}, errors.New("unable to find job, iter empty")
	}
	for {
		result := make(map[string]interface{})
		if !iter.MapScan(result) {
			break
		} else {
			job := common.Job{}
			//job ID
			if value, ok := (result["job_id"]).(gocql.UUID); ok {
				job.ID = value.String()
			} else {
				s.logger.Error("unable to decode job ID, job will be skipped")
				break
			}
			//service
			if value, ok := (result["service"]).(string); ok {
				job.Service = value
			} else {
				s.logger.Warn(fmt.Sprintf("unable to decode job %s service %s", job.ID, result))
			}
			//task
			if value, ok := (result["task"]).(string); ok {
				job.Task = value
			} else {
				s.logger.Warn(fmt.Sprintf("unable to decode job %s task %s", job.ID, result))
			}
			//domain
			if value, ok := (result["domain"]).(string); ok {
				job.Domain = value
			} else {
				s.logger.Warn(fmt.Sprintf("unable to decode job %s domain %s", job.ID, result))
			}
			//user ID
			if value, ok := (result["user_id"]).(string); ok {
				job.UserID = value
			} else {
				s.logger.Warn(fmt.Sprintf("unable to decode job %s user id %s", job.ID, result))
			}
			//engine
			if value, ok := (result["engine"]).(string); ok {
				job.Engine = value
			} else {
				s.logger.Warn(fmt.Sprintf("unable to decode job %s engine %s", job.ID, result))
			}
			//started_time
			if value, ok := (result["started_time"]).(time.Time); ok {
				job.StartTime = value
			} else {
				s.logger.Warn(fmt.Sprintf("unable to decode job %s start time %s", job.ID, result))
			}
			//finished_time
			if value, ok := (result["finished_time"]).(time.Time); ok {
				job.EndTime = value
			} else {
				s.logger.Warn(fmt.Sprintf("unable to decode job %s finish time %s", job.ID, result))
			}
			//state
			if value, ok := (result["state"]).(string); ok {
				job.State = common.JobState(value)
			} else {
				s.logger.Warn(fmt.Sprintf("unable to decode job %s state %s", job.ID, result))
			}
			//detail
			if value, ok := (result["detail"]).(string); ok {
				job.Detail = value
			} else {
				s.logger.Warn(fmt.Sprintf("unable to decode job %s detail %s", job.ID, result))
			}
			//version
			if value, ok := (result["version"]).(int); ok {
				job.Version = int32(value)
			} else {
				s.logger.Warn(fmt.Sprintf("unable to decode job %s version %s", job.ID, result))
			}
			//append steps
			stepsDB, ok := result["steps"].([]map[string]interface{})
			if ok {
				job.Steps = s.collectJobSteps(stepsDB)
			} else {
				s.logger.Warn(fmt.Sprintf("unable to decode job %s steps %s", job.ID, result))
			}
			//append specs
			var spec map[string]interface{}
			err := json.Unmarshal(result["spec"].([]byte), &spec)
			if err != nil {
				s.logger.Warn(fmt.Sprintf("unable to decode job %s spec %s", job.ID, result))
			} else {
				job.Spec = spec
			}
			jobs = append(jobs, job)

		}
	}
	return jobs, nil
}

func (s *Store) collectJobSteps(dbSteps []map[string]interface{}) []common.Step {
	var steps []common.Step
	for _, ss := range dbSteps {
		var step common.Step
		if value, ok := (ss["started_time"]).(time.Time); ok {
			step.StartTime = value
		}
		if value, ok := (ss["finished_time"]).(time.Time); ok {
			step.EndTime = value
		}
		if value, ok := (ss["message"]).(string); ok {
			step.Message = value
		}
		if value, ok := (ss["name"]).(string); ok {
			step.Name = value
		}
		if value, ok := (ss["step_id"]).(int); ok {
			step.ID = value
		}
		if value, ok := (ss["state"]).(string); ok {
			step.State = common.StepState(value)
		}
		steps = append(steps, step)
	}
	return steps
}

func (s *Store) JobStepLogFinished(ctx context.Context, jobID common.JobIdentity, stepID string) bool {
	query := s.session.Query(GetJobLastLogQueryTemplate, jobID.Service, jobID.Task, jobID.Domain, jobID.ID, stepID).WithContext(context.TODO())
	iter := query.Iter()
	if iter == nil {
		s.logger.Error(fmt.Sprintf("unable to fetch job %s/%s last log information", jobID, stepID))
		return false
	}
	var jobLog []byte
	for iter.Scan(&jobLog) {
		if string(jobLog) == common.LogCompleteFlag {
			return true
		}
	}
	return false
}

func (s *Store) GetJobStepLogs(ctx context.Context, jobID common.JobIdentity, stepID string, startTime string, maxRecord int) (*common.JobLogPart, error) {
	logPart := common.JobLogPart{
		Finished: false,
		Data:     []byte{},
	}
	var startUUID string
	if len(startTime) == 0 {
		startUUID = gocql.MinTimeUUID(time.Now().Add(-1 * time.Minute * 60 * 24 * 365)).String()
	} else {
		uuid, err := gocql.ParseUUID(startTime)
		if err != nil {
			s.logger.Error(fmt.Sprintf("unable to parse job step log time %s", err))
			return &logPart, errors.New(fmt.Sprintf("unable to parse job step log time %s", err))
		} else {
			startUUID = uuid.String()
		}
	}
	if maxRecord <= 0 {
		maxRecord = MaxJobStepLogRecord
	}
	query := s.session.Query(GetJobLogQueryTemplate, jobID.Service, jobID.Task, jobID.Domain, jobID.ID, stepID, startUUID, maxRecord).WithContext(context.TODO())
	iter := query.Iter()
	if iter == nil {
		return &logPart, errors.New("query job logs operation failed.  Not able to create query iterator")
	}
	var jobLog []byte
	var jobTime string
	for iter.Scan(&jobTime, &jobLog) {
		if string(jobLog) == common.LogCompleteFlag {
			logPart.Finished = true
		} else {
			logPart.Data = append(logPart.Data, jobLog...)
		}
	}
	logPart.MaxJobTimeUUID = jobTime
	return &logPart, nil
}
