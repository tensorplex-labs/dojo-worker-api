package orm

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dojo-api/db"

	sq "github.com/Masterminds/squirrel"

	"github.com/rs/zerolog/log"
)

type TaskORM struct {
	dbClient      *db.PrismaClient
	clientWrapper *PrismaClientWrapper
}

func NewTaskORM() *TaskORM {
	clientWrapper := GetPrismaClient()
	return &TaskORM{dbClient: clientWrapper.Client, clientWrapper: clientWrapper}
}

// DOES NOT USE ANY DEFAULT VALUES, SO REMEMBER TO SET THE RIGHT STATUS
// CreateTask creates a new task in the database with the provided details.
// Ignores `Status` and `NumResults` fields as they are set to default values.
func (o *TaskORM) CreateTask(ctx context.Context, task db.InnerTask, minerUserId string) (*db.TaskModel, error) {
	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()

	createdTask, err := o.dbClient.Task.CreateOne(
		db.Task.ExpireAt.Set(task.ExpireAt),
		db.Task.Title.Set(task.Title),
		db.Task.Body.Set(task.Body),
		db.Task.Type.Set(task.Type),
		db.Task.TaskData.Set(task.TaskData),
		db.Task.Status.Set(task.Status),
		db.Task.MaxResults.Set(task.MaxResults),
		db.Task.NumResults.Set(task.NumResults),
		db.Task.NumCriteria.Set(task.NumCriteria),
		db.Task.MinerUser.Link(
			db.MinerUser.ID.Equals(minerUserId),
		),
	).Exec(ctx)
	return createdTask, err
}

func (o *TaskORM) GetById(ctx context.Context, taskId string) (*db.TaskModel, error) {
	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()
	task, err := o.dbClient.Task.FindUnique(
		db.Task.ID.Equals(taskId),
	).Exec(ctx)
	return task, err
}

// TODO: Optimization
func (o *TaskORM) GetTasksByWorkerSubscription(ctx context.Context, workerId string, offset, limit int, sortQuery db.TaskOrderByParam, taskTypes []db.TaskType) ([]db.TaskModel, int, error) {
	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()
	// Fetch all active WorkerPartner records to retrieve MinerUser's subscription keys.
	partners, err := o.dbClient.WorkerPartner.FindMany(
		db.WorkerPartner.WorkerID.Equals(workerId),
		db.WorkerPartner.IsDeleteByMiner.Equals(false),
		db.WorkerPartner.IsDeleteByWorker.Equals(false),
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error in fetching WorkerPartner by WorkerID")
		return nil, 0, err
	}

	// Collect Subscription keys from the fetched WorkerPartner records
	var subscriptionKeys []string
	for _, partner := range partners {
		subscriptionKeys = append(subscriptionKeys, partner.MinerSubscriptionKey)
	}

	if len(subscriptionKeys) == 0 {
		log.Error().Err(err).Msg("No WorkerPartner found with the given WorkerID")
		return nil, 0, err
	}

	filterParams := []db.TaskWhereParam{
		db.Task.MinerUser.Where(
			db.MinerUser.SubscriptionKeys.Some(
				db.SubscriptionKey.Key.In(subscriptionKeys), // SubscriptionKey should be one of the keys in the subscriptionKeys slice.
			),
		),
	}

	if len(taskTypes) > 0 {
		filterParams = append(filterParams, db.Task.Type.In(taskTypes))
	}

	log.Debug().Interface("taskTypes", taskTypes).Msgf("Filter Params: %v", filterParams)

	// Fetch tasks associated with these subscription keys
	tasks, err := o.dbClient.Task.FindMany(
		filterParams...,
	).OrderBy(sortQuery).
		Skip(offset).
		Take(limit).
		Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error in fetching tasks by WorkerSubscriptionKey")
		return nil, 0, err
	}

	// TODO commented out for now, testing raw query speed
	// totalTasks, err := o.dbClient.Task.FindMany(
	// 	filterParams...,
	// ).Exec(ctx)

	totalTasks, err := o.countTasksByWorkerSubscription(ctx, taskTypes, subscriptionKeys)
	if err != nil {
		log.Error().Err(err).Msg("Error in fetching total tasks by WorkerSubscriptionKey")
		return nil, 0, err
	}

	log.Info().Int("totalTasks", totalTasks).Msgf("Successfully fetched total tasks fetched for worker ID %v", workerId)
	return tasks, totalTasks, nil
}

// This function uses raw queries to calculate count(*) since this functionality is missing from the prisma go client
// and using findMany with the filter params and then len(tasks) is facing performance issues
func (o *TaskORM) countTasksByWorkerSubscription(ctx context.Context, taskTypes []db.TaskType, subscriptionKeys []string) (int, error) {
	var taskTypesParam []string
	for _, taskType := range taskTypes {
		taskTypesParam = append(taskTypesParam, string(taskType))
	}

	validSubscriptionKeys := make([]string, 0)
	for _, key := range subscriptionKeys {
		if !strings.HasPrefix(key, "sk-") || len(key[3:]) != 32 {
			continue
		}
		validSubscriptionKeys = append(validSubscriptionKeys, key)
	}

	// need to set subquery to use "$?" and let the main query use dollar to resolve placeholders
	subQuery, subQueryArgs, err := sq.Select("miner_user_id").
		From("\"SubscriptionKey\"").
		Where(sq.Eq{"key": subscriptionKeys}).
		PlaceholderFormat(sq.Question).
		ToSql()
	if err != nil {
		log.Error().Err(err).Msg("Error building subquery")
		return 0, err
	}

	mainQuery := sq.Select("count(*) as total_tasks").
		From("\"Task\"").
		Where(sq.Expr(fmt.Sprintf("miner_user_id IN (%s)", subQuery), subQueryArgs...)).
		// need to do this since TaskType is a custom prisma enum type
		Where(sq.Expr(fmt.Sprintf("type in ('%s')", strings.Join(taskTypesParam, "', '")))).
		PlaceholderFormat(sq.Dollar)

	sql, args, err := mainQuery.ToSql()
	if err != nil {
		log.Error().Err(err).Msg("Error building full SQL query")
		return 0, err
	}

	log.Debug().Interface("args", args).Msgf("Query Builder built raw SQL query: %s", sql)

	// unsure why it's a raw string, when the examples say it's a raw int
	var res []struct {
		TotalTasks db.RawString `json:"total_tasks"`
	}
	err = o.clientWrapper.Client.Prisma.QueryRaw(sql, args...).Exec(ctx, &res)
	if err != nil {
		log.Error().Err(err).Msg("Error executing raw query for total tasks")
		return 0, err
	}

	if len(res) == 0 {
		// probably didn't name the right fields in the raw query,
		// "total_tasks" need to match in res and named alias from count(*)
		log.Error().Msg("No tasks found")
		return 0, err
	}

	totalTasksStr := string(res[0].TotalTasks)
	log.Info().Interface("totalTasks", totalTasksStr).Msg("Total tasks fetched using raw query")

	totalTasks, err := strconv.Atoi(totalTasksStr)
	if err != nil {
		log.Error().Err(err).Msg("Error converting total tasks to integer")
		return 0, err
	}

	return totalTasks, nil
}

// check every three mins for expired tasks
func (o *TaskORM) UpdateExpiredTasks(ctx context.Context) {
	for range time.Tick(3 * time.Minute) {
		log.Info().Msg("Checking for expired tasks")
		o.clientWrapper.BeforeQuery()
		// Fetch all expired tasks
		tasks, err := o.dbClient.Task.
			FindMany(
				db.Task.ExpireAt.Lte(time.Now()),
				db.Task.Status.Equals(db.TaskStatusInProgress),
			).
			OrderBy(db.Task.CreatedAt.Order(db.SortOrderDesc)).
			Exec(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Error in fetching expired tasks")
		}

		if len(tasks) == 0 {
			log.Info().Msg("No newly expired tasks to update skipping...")
			continue
		} else {
			log.Info().Msgf("Fetched %v newly expired tasks", len(tasks))
		}

		var txns []db.PrismaTransaction
		for i, taskModel := range tasks {
			transaction := o.dbClient.Task.FindUnique(
				db.Task.ID.Equals(taskModel.ID),
			).Update(
				db.Task.Status.Set(db.TaskStatusExpired),
				db.Task.UpdatedAt.Set(time.Now()),
			).Tx()

			txns = append(txns, transaction)

			if len(txns) == 100 || (i == len(tasks)-1 && len(txns) > 0) {
				if err := o.dbClient.Prisma.Transaction(txns...).Exec(ctx); err != nil {
					log.Error().Err(err).Msg("Error in updating batch of task status to expired")
				}
				txns = []db.PrismaTransaction{}
			}
		}

		o.clientWrapper.AfterQuery()
	}
}

func (o *TaskORM) GetCompletedTaskCount(ctx context.Context) (int, error) {
	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()

	var result []struct {
		Count db.RawString `json:"count"`
	}

	query := "SELECT COUNT(*) as count FROM \"TaskResult\" WHERE status = 'COMPLETED';"
	err := o.clientWrapper.Client.Prisma.QueryRaw(query).Exec(ctx, &result)
	if err != nil {
		return 0, err
	}

	if len(result) == 0 {
		return 0, fmt.Errorf("no results found for completed tasks count query")
	}

	taskCountStr := string(result[0].Count)
	taskCountInt, err := strconv.Atoi(taskCountStr)
	if err != nil {
		return 0, err
	}

	return taskCountInt, nil
}

func (o *TaskORM) GetNextInProgressTask(ctx context.Context, taskId string) (*db.TaskModel, error) {
	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()

	// Fetch the current task to determine the ordering criteria
	currentTask, err := o.dbClient.Task.FindFirst(
		db.Task.ID.Equals(taskId),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}

	// Fetch the next task that is in-progress based on the created_at timestamp
	nextTask, err := o.dbClient.Task.FindFirst(
		db.Task.CreatedAt.Lt(currentTask.CreatedAt),
		db.Task.Status.Equals(db.TaskStatusInProgress),
	).OrderBy(db.Task.CreatedAt.Order(db.SortOrderDesc)).Exec(ctx)
	if err != nil {
		// If no next task is found, loop back to the first in-progress task
		if errors.Is(err, db.ErrNotFound) {
			nextTask, err = o.dbClient.Task.FindFirst(
				db.Task.Status.Equals(db.TaskStatusInProgress),
			).OrderBy(db.Task.CreatedAt.Order(db.SortOrderDesc)).Exec(ctx)
			if err != nil {
				return nil, err
			}
			return nextTask, nil
		}
		return nil, err
	}

	return nextTask, nil
}
