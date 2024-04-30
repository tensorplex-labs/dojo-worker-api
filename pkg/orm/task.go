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

// TODO: Optimization
func (o *TaskORM) GetTaskByIdWithSub(ctx context.Context, taskId string, workerId string) (*db.TaskModel, error) {
	// Fetch the task along with its associated MinerUser
	task, err := o.dbClient.Task.FindUnique(
		db.Task.ID.Equals(taskId),
	).With(
		db.Task.MinerUser.Fetch(),
	).Exec(ctx)

	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, err
	}

	// Retrieve the MinerUser from the fetched task
	minerUser, ok := task.MinerUser()
	if !ok || minerUser == nil {
		return nil, err
	}

	// Check if there's a WorkerPartner link for the given MinerUser and DojoWorker
	exists, err := o.dbClient.WorkerPartner.FindFirst(
		db.WorkerPartner.MinerSubscriptionKey.Equals(minerUser.SubscriptionKey),
		db.WorkerPartner.WorkerID.Equals(workerId),
		db.WorkerPartner.IsDeleteByMiner.Equals(false),
		db.WorkerPartner.IsActiveByWorker.Equals(true),
	).Exec(ctx)

	if err != nil {
		return nil, err
	}
	if exists == nil {
		return nil, err
	}

	// If all checks pass, return the task
	return task, nil
}

// TODO: Optimization
func (o *TaskORM) GetTasksByWorkerSubscription(ctx context.Context, workerId string, offset, limit int, sortQuery db.TaskOrderByParam, taskTypes []db.TaskType) ([]db.TaskModel, error) {
	// Fetch all active WorkerPartner records to retrieve MinerUser's subscription keys.
	partners, err := o.dbClient.WorkerPartner.FindMany(
		db.WorkerPartner.WorkerID.Equals(workerId),
		db.WorkerPartner.IsDeleteByMiner.Equals(false),
		db.WorkerPartner.IsActiveByWorker.Equals(true),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}

	// Collect Subscription keys from the fetched WorkerPartner records
	var subscriptionKeys []string
	for _, partner := range partners {
		subscriptionKeys = append(subscriptionKeys, partner.MinerSubscriptionKey)
	}

	if len(subscriptionKeys) == 0 {
		return nil, err
	}

	// Fetch tasks associated with these subscription keys
	tasks, err := o.dbClient.Task.FindMany(
		db.Task.MinerUser.Where(
			db.MinerUser.SubscriptionKey.In(subscriptionKeys),
		), db.Task.Type.In(taskTypes),
	).OrderBy(sortQuery).
		Skip(offset).
		Take(limit).
		Exec(ctx)

	if err != nil {
		return nil, err
	}

	return tasks, nil
}
