// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: query.sql

package tutorial

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const deleteExpiredTasks = `-- name: DeleteExpiredTasks :exec
DELETE FROM public."Task"
WHERE id IN (
        SELECT
            id
        FROM
            public."Task"
        WHERE
            public."Task".expire_at <= $1
            AND status = ANY ($2::public. "TaskStatus"[])
            AND id NOT IN ( SELECT DISTINCT
                    task_id
                FROM
                    public."TaskResult")
            LIMIT $3)
`

type DeleteExpiredTasksParams struct {
	ExpireAt pgtype.Timestamp `db:"expire_at" json:"expire_at"`
	Column2  []TaskStatus     `db:"column_2" json:"column_2"`
	Limit    int32            `db:"limit" json:"limit"`
}

// consider that tasks can be in progress with completions, or expired with no completions
func (q *Queries) DeleteExpiredTasks(ctx context.Context, arg DeleteExpiredTasksParams) error {
	_, err := q.db.Exec(ctx, deleteExpiredTasks, arg.ExpireAt, arg.Column2, arg.Limit)
	return err
}

const getMinerUserByHotkey = `-- name: GetMinerUserByHotkey :one
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
LIMIT $2 OFFSET $3
`

type GetMinerUserByHotkeyParams struct {
	Hotkey string `db:"hotkey" json:"hotkey"`
	Limit  int32  `db:"limit" json:"limit"`
	Offset int32  `db:"offset" json:"offset"`
}

func (q *Queries) GetMinerUserByHotkey(ctx context.Context, arg GetMinerUserByHotkeyParams) (MinerUser, error) {
	row := q.db.QueryRow(ctx, getMinerUserByHotkey, arg.Hotkey, arg.Limit, arg.Offset)
	var i MinerUser
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Hotkey,
		&i.Email,
		&i.OrganizationName,
	)
	return i, err
}

const getTotalNumDojoWorkers = `-- name: GetTotalNumDojoWorkers :one
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
LIMIT $2 OFFSET $3
`

type GetTotalNumDojoWorkersParams struct {
	Column1 string `db:"column_1" json:"column_1"`
	Limit   int32  `db:"limit" json:"limit"`
	Offset  int32  `db:"offset" json:"offset"`
}

type GetTotalNumDojoWorkersRow struct {
	ID                string           `db:"id" json:"id"`
	CreatedAt         pgtype.Timestamp `db:"created_at" json:"created_at"`
	UpdatedAt         pgtype.Timestamp `db:"updated_at" json:"updated_at"`
	PublicMetricsType string           `db:"public_Metrics_type" json:"public_Metrics_type"`
	MetricsData       []byte           `db:"metrics_data" json:"metrics_data"`
}

func (q *Queries) GetTotalNumDojoWorkers(ctx context.Context, arg GetTotalNumDojoWorkersParams) (GetTotalNumDojoWorkersRow, error) {
	row := q.db.QueryRow(ctx, getTotalNumDojoWorkers, arg.Column1, arg.Limit, arg.Offset)
	var i GetTotalNumDojoWorkersRow
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PublicMetricsType,
		&i.MetricsData,
	)
	return i, err
}

const setTaskStatusToExpired = `-- name: SetTaskStatusToExpired :exec
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
            LIMIT $4)
`

type SetTaskStatusToExpiredParams struct {
	Column1   TaskStatus       `db:"column_1" json:"column_1"`
	UpdatedAt pgtype.Timestamp `db:"updated_at" json:"updated_at"`
	Column3   TaskStatus       `db:"column_3" json:"column_3"`
	Limit     int32            `db:"limit" json:"limit"`
}

func (q *Queries) SetTaskStatusToExpired(ctx context.Context, arg SetTaskStatusToExpiredParams) error {
	_, err := q.db.Exec(ctx, setTaskStatusToExpired,
		arg.Column1,
		arg.UpdatedAt,
		arg.Column3,
		arg.Limit,
	)
	return err
}
