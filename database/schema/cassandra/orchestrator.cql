CREATE TABLE jobs_info(
  service                   text,
  domain                    text,
  task                      text,
  user_id                   text,
  job_id                    uuid,
  max_attempts              int,
  task_timeout              int,
  spec                      blob,
  started_time              timestamp,
  finished_time             timestamp,
  state                     int,  -- enum Created, Running, Succeed, Failed, Timeout,
  task_result               text,
  failure_detail            blob,
  PRIMARY KEY  ((service, domain, task), job_id)
);