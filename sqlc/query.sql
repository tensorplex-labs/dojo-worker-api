-- name: GetMinerUserByHotkey :one
SELECT
    public."MinerUser".id,
    public."MinerUser".created_at,
    public."MinerUser".updated_at,
    public."MinerUser".hotkey,
    public."MinerUser".email,
    public."MinerUser"."organizationName"
FROM
    public."MinerUser"
WHERE
    public."MinerUser".hotkey = $1
LIMIT $2 OFFSET $3;

-- name: GetTotalNumDojoWorkers :one
SELECT
    public."Metrics".id,
    public."Metrics".created_at,
    public."Metrics".updated_at,
    public."Metrics".TYPE::text,
    public."Metrics".metrics_data
FROM
    public."Metrics"
WHERE
    public."Metrics".TYPE = CAST($1::text AS public."MetricsType")
LIMIT $2 OFFSET $3;

-- name: DeleteExpiredTasks :exec
-- consider that tasks can be in progress with completions, or expired with no completions
DELETE FROM public."Task"
WHERE id IN (
        SELECT
            id
        FROM
            public."Task"
        WHERE
            public."Task".expire_at <= $1
            AND status = ANY ($2::"TaskStatus"[])
            AND id NOT IN ( SELECT DISTINCT
                    task_id
                FROM
                    public."TaskResult")
            LIMIT $3);

-- name: SetTaskStatusToExpired :exec
UPDATE
    public."Task"
SET
    status = $1::"TaskStatus",
    updated_at = $2
WHERE
    id IN (
        SELECT
            id
        FROM
            "Task"
        WHERE
            expire_at <= $2
            AND status = $3::"TaskStatus"
            AND id IN ( SELECT DISTINCT
                    task_id
                FROM
                    "TaskResult")
            LIMIT $4);

