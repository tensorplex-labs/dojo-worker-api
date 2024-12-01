package orm

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"dojo-api/db"
	"dojo-api/pkg/cache"

	"github.com/rs/zerolog/log"
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
	switch taskResult.Status {
	case db.TaskResultStatusInvalid:
		return t.CreateTaskResultWithInvalid(ctx, taskResult)
	case db.TaskResultStatusCompleted:
		return t.CreateTaskResultWithCompleted(ctx, taskResult)
	default:
		return nil, fmt.Errorf("unsupported status: %v", taskResult.Status)
	}
}

func (t *TaskResultORM) GetTaskResultsByTaskId(ctx context.Context, taskId string) ([]db.TaskResultModel, error) {
	t.clientWrapper.BeforeQuery()
	defer t.clientWrapper.AfterQuery()

	return t.client.TaskResult.FindMany(db.TaskResult.TaskID.Equals(taskId)).Exec(ctx)
}

func (t *TaskResultORM) GetCompletedTResultByTaskAndWorker(ctx context.Context, taskId string, workerId string) ([]db.TaskResultModel, error) {
	cacheKey := cache.BuildCacheKey(cache.TaskResultByTaskAndWorker, taskId, workerId)

	var results []db.TaskResultModel
	cacheInstance := cache.GetCacheInstance()

	// Try to get from cache
	if err := cacheInstance.GetCacheValue(cacheKey, &results); err == nil {
		return results, nil
	}

	// Cache miss, fetch from database
	t.clientWrapper.BeforeQuery()
	defer t.clientWrapper.AfterQuery()

	results, err := t.client.TaskResult.FindMany(
		db.TaskResult.TaskID.Equals(taskId),
		db.TaskResult.WorkerID.Equals(workerId),
		db.TaskResult.Status.Equals(db.TaskResultStatusCompleted),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}

	// Set cache
	if err := cacheInstance.SetCacheValue(cacheKey, results); err != nil {
		log.Warn().Err(err).Msg("Failed to set cache")
	}

	return results, nil
}

func (t *TaskResultORM) GetCompletedTResultByWorker(ctx context.Context, workerId string) ([]db.TaskResultModel, error) {
	cacheKey := cache.BuildCacheKey(cache.TaskResultByWorker, workerId)

	var results []db.TaskResultModel
	cache := cache.GetCacheInstance()

	// Try to get from cache first
	if err := cache.GetCacheValue(cacheKey, &results); err == nil {
		return results, nil
	}

	// Cache miss, fetch from database
	t.clientWrapper.BeforeQuery()
	defer t.clientWrapper.AfterQuery()

	results, err := t.client.TaskResult.FindMany(
		db.TaskResult.WorkerID.Equals(workerId),
		db.TaskResult.Status.Equals(db.TaskResultStatusCompleted),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}

	// Store in cache
	if err := cache.SetCacheValue(cacheKey, results); err != nil {
		log.Warn().Err(err).Msg("Failed to set task result cache")
	}

	return results, nil
}

func (t *TaskResultORM) CreateTaskResultWithInvalid(ctx context.Context, taskResult *db.InnerTaskResult) (*db.TaskResultModel, error) {
	t.clientWrapper.BeforeQuery()
	defer t.clientWrapper.AfterQuery()

	createdTaskResult, err := t.client.TaskResult.CreateOne(
		db.TaskResult.Status.Set(db.TaskResultStatusInvalid),
		db.TaskResult.ResultData.Set(taskResult.ResultData),
		db.TaskResult.Task.Link(
			db.Task.ID.Equals(taskResult.TaskID),
		),
		db.TaskResult.DojoWorker.Link(
			db.DojoWorker.ID.Equals(taskResult.WorkerID),
		),
	).With(
		db.TaskResult.Task.Fetch(),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return createdTaskResult, nil
}

func (t *TaskResultORM) CreateTaskResultWithCompleted(ctx context.Context, taskResult *db.InnerTaskResult) (*db.TaskResultModel, error) {
	t.clientWrapper.BeforeQuery()
	defer t.clientWrapper.AfterQuery()

	// Retrieve the task object from the appropriate source
	task, err := t.client.Task.FindUnique(db.Task.ID.Equals(taskResult.TaskID)).Exec(ctx)
	if err != nil {
		return nil, err
	}

	updateTaskParams := []db.TaskSetParam{
		db.Task.NumResults.Increment(1),
		db.Task.UpdatedAt.Set(time.Now()),
	}
	// Check if num_results equals max_results before updating the task status
	if task.NumResults+1 == task.MaxResults && task.Status != db.TaskStatusCompleted {
		updateTaskParams = append(updateTaskParams, db.Task.Status.Set(db.TaskStatusCompleted))
	}

	// TODO add web3 integration fields when the time comes
	updateTaskTx := t.client.Task.FindUnique(db.Task.ID.Equals(taskResult.TaskID)).Update(updateTaskParams...).Tx()

	createResultTx := t.client.TaskResult.CreateOne(
		db.TaskResult.Status.Set(db.TaskResultStatusCompleted),
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

func (t *TaskResultORM) GetCompletedTResultCount(ctx context.Context) (int, error) {
	t.clientWrapper.BeforeQuery()
	defer t.clientWrapper.AfterQuery()

	var result []struct {
		Count db.RawString `json:"count"`
	}

	query := "SELECT COUNT(*) as count FROM \"TaskResult\" WHERE status = 'COMPLETED';"
	err := t.clientWrapper.Client.Prisma.QueryRaw(query).Exec(ctx, &result)
	if err != nil {
		return 0, err
	}

	if len(result) == 0 {
		return 0, fmt.Errorf("no results found for completed tasks count query")
	}

	taskResultCountStr := string(result[0].Count)
	taskResultCountInt, err := strconv.Atoi(taskResultCountStr)
	if err != nil {
		return 0, err
	}

	return taskResultCountInt, nil
}
