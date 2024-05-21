package orm

import (
	"context"
	"time"

	"dojo-api/db"

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

func (o *TaskORM) GetByPage(ctx context.Context, offset, limit int, sortQuery db.TaskOrderByParam, taskTypes []db.TaskType) ([]db.TaskModel, error) {

	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()
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

	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()
	// Fetch the task along with its associated MinerUser
	task, err := o.dbClient.Task.FindUnique(
		db.Task.ID.Equals(taskId),
	).With(
		db.Task.MinerUser.Fetch(),
	).Exec(ctx)

	if err != nil {
		log.Error().Err(err).Msg("Error in fetching task by taskId")
		return nil, err
	}

	if task == nil {
		log.Error().Err(err).Msg("No Task found with the given taskId")
		return nil, err
	}

	// Retrieve the MinerUser from the fetched task
	minerUser, ok := task.MinerUser()
	if !ok || minerUser == nil {
		log.Error().Err(err).Msg("Error in fetching MinerUser by MinerSubscriptionKey")
		return nil, err
	}

	// Check if there's a WorkerPartner link for the given MinerUser and DojoWorker
	exists, err := o.dbClient.WorkerPartner.FindFirst(
		db.WorkerPartner.MinerSubscriptionKey.Equals(minerUser.SubscriptionKey),
		db.WorkerPartner.WorkerID.Equals(workerId),
		db.WorkerPartner.IsDeleteByMiner.Equals(false),
		db.WorkerPartner.IsDeleteByWorker.Equals(false),
	).Exec(ctx)

	if err != nil {
		log.Error().Err(err).Msg("Error in fetching WorkerPartner by MinerSubscriptionKey and WorkerID")
		return nil, err
	}
	if exists == nil {
		log.Error().Err(err).Msg("No WorkerPartner found with the given MinerSubscriptionKey and WorkerID")
		return nil, err
	}

	// If all checks pass, return the task
	return task, nil
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
			db.MinerUser.SubscriptionKey.In(subscriptionKeys),
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

	totalTasks, err := o.dbClient.Task.FindMany(
		filterParams...,
	).Exec(ctx)

	if err != nil {
		log.Error().Err(err).Msg("Error in fetching total Tasks")
		return nil, 0, err
	}

	return tasks, len(totalTasks), nil
}

// check every three mins for expired tasks
func (o *TaskORM) UpdateExpiredTasks(ctx context.Context) {
	for {
		o.clientWrapper.BeforeQuery()
		// Fetch all expired tasks
		tasks, err := o.dbClient.Task.FindMany(
			db.Task.ExpireAt.Lte(time.Now()),
		).Exec(ctx)

		if err != nil {
			log.Error().Err(err).Msg("Error in fetching expired tasks")
		}
		
		var txns []db.PrismaTransaction
		for _, taskModel := range tasks {
			if taskModel.Status != db.TaskStatusExpired{
				transaction := o.dbClient.Task.FindUnique(
					db.Task.ID.Equals(taskModel.ID),
				).Update(
					db.Task.Status.Set(db.TaskStatusExpired),
				).Tx()

				txns = append(txns, transaction)

				if err != nil {
					log.Error().Err(err).Msg("Error in updating task status to expired")
				}
			}
		}

		if err := o.dbClient.Prisma.Transaction(txns...).Exec(ctx); err != nil {
			log.Error().Err(err).Msg("Error in fetching expired tasks")
		}

		o.clientWrapper.AfterQuery()

		time.Sleep(3 * time.Second)
	}
}
