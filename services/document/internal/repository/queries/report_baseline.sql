-- name: GetReportJobByID :one
SELECT
    id,
    request_id,
    source,
    job_type,
    target_type,
    target_id,
    asynq_task_id,
    queue_name,
    report_id,
    template_id,
    status,
    error_code,
    error_message,
    retry_count,
    max_attempts,
    started_at,
    finished_at,
    created_at
FROM report_jobs
WHERE id = $1;
