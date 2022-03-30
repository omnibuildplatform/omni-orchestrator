CREATE TYPE IF NOT EXISTS omni_orchestrator.step_info(
    step_id                 int,
    name                    text,
    state                   text,
    started_time            timestamp,
    finished_time           timestamp,
    message                 text,
);

CREATE TABLE IF NOT EXISTS omni_orchestrator.job_info(
  service                   text,
  task                      text,
  domain                    text,
  job_date                  date,
  job_id                    timeuuid,
  user_id                   text,
  engine                    text,
  spec                      blob,
  started_time              timestamp,
  finished_time             timestamp,
  state                     text,  -- enum Created, Running, Succeed, Failed,
  steps                     frozen<list<step_info>>,
  detail                    text,
  PRIMARY KEY  ((service, task), domain, job_date, job_id)
);
