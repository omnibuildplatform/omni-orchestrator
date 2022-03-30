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
	InsertJobQueryTemplate = `INSERT INTO job_info (` +
		`service, task, domain, job_date, job_id, user_id, engine, spec, started_time, finished_time, state, steps, detail) ` +
		`VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS`
	UpdateJobQueryTemplate = `UPDATE job_info ` +
		`SET started_time = ?, ` +
		`finished_time = ?, ` +
		`state = ?, ` +
		`steps = ?, ` +
		`detail = ? ` +
		`WHERE service = ? ` +
		`and task = ? ` +
		`and domain = ? ` +
		`and job_date = ? ` +
		`and job_id = ?` +
		`IF EXISTS`
	GetJobQueryTemplate = `SELECT service, task, domain, job_id, user_id, engine, spec, started_time, finished_time, state, steps, detail ` +
		`FROM job_info ` +
		`WHERE service = ? ` +
		`and task = ? ` +
		`and domain = ? ` +
		`and job_date = ? ` +
		`and job_id = ?`
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

func (s *Store) GetName() string {
	return "cassandra"
}

func (s *Store) Initialize() error {
	return nil
}

func (s *Store) UpdateJob(ctx context.Context, job *common.Job) error {
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
			value["step_id"] = step.Index
			value["name"] = step.Name
			value["state"] = step.State
			value["started_time"] = step.StartTime.UnixMilli()
			value["finished_time"] = step.EndTime.UnixMilli()
			value["message"] = step.Message
			steps = append(steps, value)
		}
	}
	query := s.session.Query(UpdateJobQueryTemplate, job.StartTime.UnixMilli(), job.EndTime.UnixMilli(),
		job.State, steps, job.Detail, job.Service, job.Task, job.Domain, jobDate, job.ID).WithContext(ctx)
	applied, err := query.MapScanCAS(make(map[string]interface{}))
	if err != nil {
		return err
	}
	if !applied {
		return fmt.Errorf("update job operation failed because of job not found")
	}
	return nil

}
func (s *Store) CreateJob(ctx context.Context, job *common.Job) error {
	job.ID = gocql.TimeUUID().String()
	jobDate := time.Now().Format("2006-01-02")
	jobSpec, err := json.Marshal(job.Spec)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to encode job spec %s", err))
	}
	var steps []map[string]interface{}
	if job != nil {
		for _, step := range job.Steps {
			value := make(map[string]interface{})
			value["step_id"] = step.Index
			value["name"] = step.Name
			value["state"] = step.State
			value["started_time"] = step.StartTime.UnixMilli()
			value["finished_time"] = step.EndTime.UnixMilli()
			value["message"] = step.Message
			steps = append(steps, value)
		}
	}
	query := s.session.Query(InsertJobQueryTemplate, job.Service, job.Task, job.Domain, jobDate, job.ID, job.UserID, job.Engine,
		jobSpec, job.StartTime.UnixMilli(), job.EndTime.UnixMilli(), job.State, steps, job.Detail).WithContext(ctx)
	applied, err := query.MapScanCAS(make(map[string]interface{}))
	if err != nil {
		return err
	}
	if !applied {
		return fmt.Errorf("create job operation failed because of uuid collision")
	}
	return nil
}

func (s *Store) GetJob(ctx context.Context, service, task, domain, jobID string) (common.Job, error) {
	jobTime, err := gocql.ParseUUID(jobID)
	if err != nil {
		return common.Job{}, errors.New(fmt.Sprintf("unable to parse job id %s", err.Error()))
	}
	jobDate := jobTime.Time().Format("2006-01-02")
	query := s.session.Query(GetJobQueryTemplate, service, task, domain, jobDate, jobID).WithContext(context.TODO())
	result := make(map[string]interface{})
	if err := query.MapScan(result); err != nil {
		return common.Job{}, err
	}
	job := common.Job{
		Service: service,
		Task:    task,
		Domain:  domain,
		ID:      jobID,
	}
	if value, ok := (result["user_id"]).(string); ok {
		job.UserID = value
	} else {
		s.logger.Warn("unable to decode job user id")
	}
	if value, ok := (result["engine"]).(string); ok {
		job.Engine = value
	} else {
		s.logger.Warn("unable to decode job engine")
	}
	if value, ok := (result["started_time"]).(time.Time); ok {
		job.StartTime = value
	} else {
		s.logger.Warn("unable to decode job start time")
	}
	if value, ok := (result["finished_time"]).(time.Time); ok {
		job.EndTime = value
	} else {
		s.logger.Warn("unable to decode job finish time")
	}
	if value, ok := (result["state"]).(string); ok {
		job.State = common.JobState(value)
	} else {
		s.logger.Warn("unable to decode job state")
	}
	if value, ok := (result["detail"]).(string); ok {
		job.Detail = value
	} else {
		s.logger.Warn("unable to decode job detail")
	}
	//append steps
	stepsDB, ok := result["steps"].([]map[string]interface{})
	if ok {
		job.Steps = s.collectJobSteps(stepsDB)
	} else {
		s.logger.Warn("unable to decode job steps")
	}
	//append specs
	var spec map[string]interface{}
	err = json.Unmarshal(result["spec"].([]byte), &spec)
	if err != nil {
		s.logger.Warn(fmt.Sprintf("unable to job spec into struct %s", err))
	} else {
		job.Spec = spec
	}
	return job, nil
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
			step.Index = value
		}
		if value, ok := (ss["state"]).(string); ok {
			step.State = common.StepState(value)
		}
		steps = append(steps, step)
	}
	return steps
}
