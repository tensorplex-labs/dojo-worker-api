package orm

import (
	"context"

	"dojo-api/db"
)

type TaskORM struct {
	dbClient *db.PrismaClient
}

func NewTaskORM() *TaskORM {
	client := NewPrismaClient()
	return &TaskORM{dbClient: client}
}

// DOES NOT USE ANY DEFAULT VALUES, SO REMEMBER TO SET THE RIGHT STATUS
// CreateTask creates a new task in the database with the provided details.
// Ignores `Status` and `NumResults` fields as they are set to default values.
func (o *TaskORM) CreateTask(ctx context.Context, task db.InnerTask, minerUserId string) (*db.TaskModel, error) {
	createdTask, err := o.dbClient.Task.CreateOne(
		db.Task.ExpireAt.Set(task.ExpireAt),
		db.Task.Title.Set(task.Title),
		db.Task.Body.Set(task.Body),
		db.Task.Type.Set(task.Type),
		db.Task.TaskData.Set(task.TaskData),
		db.Task.Status.Set(task.Status),
		db.Task.MaxResults.Set(task.MaxResults),
		db.Task.NumResults.Set(task.NumResults),
		db.Task.MinerUser.Link(
			db.MinerUser.ID.Equals(minerUserId),
		),
	).Exec(ctx)
	return createdTask, err
}

func (o *TaskORM) GetById(ctx context.Context, taskId string) (*db.TaskModel, error) {
	task, err := o.dbClient.Task.FindUnique(
		db.Task.ID.Equals(taskId),
	).Exec(ctx)
	return task, err
}

func (o *TaskORM) GetByPage(ctx context.Context, offset, limit int, sortQuery db.TaskOrderByParam, taskTypes []db.TaskType) ([]db.TaskModel, error) {
	tasks, err := o.dbClient.Task.FindMany(
		db.Task.Type.In(taskTypes),
	).OrderBy(sortQuery).
		Skip(offset).
		Take(limit).
		Exec(ctx)
	return tasks, err
}