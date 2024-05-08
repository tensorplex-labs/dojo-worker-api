package orm

import (
	"context"

	"dojo-api/db"
)

type TaskResultORM struct {
	client        *db.PrismaClient
	clientWrapper *PrismaClientWrapper
}

func NewTaskResultORM() *TaskResultORM {
	clientWrapper := GetPrismaClient()
	return &TaskResultORM{
		client:        clientWrapper.Client,
		clientWrapper: clientWrapper,
	}
}

// In a transaction creates the TaskResult and updates the Task.NumResults
func (t *TaskResultORM) CreateTaskResult(ctx context.Context, taskResult *db.InnerTaskResult) (*db.TaskResultModel, error) {
	t.clientWrapper.BeforeQuery()
	defer t.clientWrapper.AfterQuery()

	// TODO add web3 integration fields when the time comes
	updateTaskTx := t.client.Task.FindUnique(db.Task.ID.Equals(taskResult.TaskID)).Update(db.Task.NumResults.Increment(1)).Tx()
	createResultTx := t.client.TaskResult.CreateOne(
		db.TaskResult.Status.Set(taskResult.Status),
		db.TaskResult.ResultData.Set(taskResult.ResultData),
		db.TaskResult.Task.Link(
			db.Task.ID.Equals(taskResult.TaskID),
		),
		db.TaskResult.DojoWorker.Link(
			db.DojoWorker.ID.Equals(taskResult.WorkerID),
		),
	).With(
		db.TaskResult.Task.Fetch(),
	).Tx()
	if err := t.client.Prisma.Transaction(updateTaskTx, createResultTx).Exec(ctx); err != nil {
		return nil, err
	}
	return createResultTx.Result(), nil
}

func (t *TaskResultORM) GetTaskResultsByTaskId(ctx context.Context, taskId string) ([]db.TaskResultModel, error) {
	t.clientWrapper.BeforeQuery()
	defer t.clientWrapper.AfterQuery()
	return t.client.TaskResult.FindMany(db.TaskResult.TaskID.Equals(taskId)).Exec(ctx)
}

func (orm *TaskResultORM) GetCompletedTResultByTaskAndWorker(ctx context.Context, taskId string, workerId string) ([]db.TaskResultModel, error) {
	return orm.client.TaskResult.FindMany(
		db.TaskResult.TaskID.Equals(taskId),
		db.TaskResult.WorkerID.Equals(workerId),
		db.TaskResult.Status.Equals(db.TaskResultStatusCompleted),
	).Exec(ctx)
}

func (orm *TaskResultORM) GetCompletedTResultByWorker(ctx context.Context, workerId string) ([]db.TaskResultModel, error) {
	return orm.client.TaskResult.FindMany(
		db.TaskResult.WorkerID.Equals(workerId),
		db.TaskResult.Status.Equals(db.TaskResultStatusCompleted),
	).Exec(ctx)
}
