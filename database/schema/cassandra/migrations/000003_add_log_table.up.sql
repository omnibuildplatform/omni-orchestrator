CREATE TABLE IF NOT EXISTS log_info(
    service                   text,
    task                      text,
    domain                    text,
    job_id                    timeuuid,
    step_id                   int,
    log_time                  timeuuid,
    data                      blob,
    PRIMARY KEY  ((service, task), domain, job_id, step_id, log_time)
) WITH CLUSTERING ORDER BY (domain ASC, job_id ASC, step_id ASC, log_time ASC);