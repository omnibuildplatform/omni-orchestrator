CREATE TABLE IF NOT EXISTS job_info(
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
    version                   int,
    PRIMARY KEY  ((service, task), domain, job_date, job_id)
);