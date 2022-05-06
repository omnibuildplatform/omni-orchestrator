CREATE TYPE IF NOT EXISTS step_info(
    step_id                 int,
    name                    text,
    state                   text,
    started_time            timestamp,
    finished_time           timestamp,
    message                 text,
);
