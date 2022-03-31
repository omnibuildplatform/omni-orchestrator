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
  state                     text,
  steps                     frozen<list<step_info>>,
  detail                    text,
  PRIMARY KEY  ((service, task), domain, job_date, job_id)
);



CREATE TABLE IF NOT EXISTS omni_orchestrator.log_info(
 service                   text,
 task                      text,
 domain                    text,
 job_id                    timeuuid,
 step_id                   int,
 log_time                  timeuuid,
 data                      blob,
 PRIMARY KEY  ((service, task), domain, job_id, step_id, log_time)
) WITH CLUSTERING ORDER BY (domain ASC, job_id ASC, step_id ASC, log_time ASC);

