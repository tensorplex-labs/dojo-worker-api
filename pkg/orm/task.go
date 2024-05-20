package orm

import (
	"context"
	"strconv"
	"strings"

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

	// TODO commented out for now, testing raw query speed
	// totalTasks, err := o.dbClient.Task.FindMany(
	// 	filterParams...,
	// ).Exec(ctx)

	// preparing the raw query params
	var queryBuilder strings.Builder
	var taskTypeParams []string
	for _, taskType := range taskTypes {
		taskTypeParams = append(taskTypeParams, string(taskType))
	}

	queryBuilder.WriteString("select count(*) as total_tasks")
	queryBuilder.WriteString(" from \"Task\"")
	// start miner user id clause
	queryBuilder.WriteString(" where miner_user_id in (")
	queryBuilder.WriteString(" select id")
	queryBuilder.WriteString(" from \"MinerUser\"")
	queryBuilder.WriteString(" where subscription_key in (")
	queryBuilder.WriteString(" '" + strings.Join(subscriptionKeys, "', '") + "'")
	queryBuilder.WriteString(" )")
	queryBuilder.WriteString(" )")
	// end miner user id clause

	if len(taskTypes) > 0 {
		// start task type where clause
		queryBuilder.WriteString(" and type in (")
		queryBuilder.WriteString(" '" + strings.Join(taskTypeParams, "', '") + "'")
		queryBuilder.WriteString(" )")
		// end task type where clause
	}
	queryBuilder.WriteString(" ;")

	log.Debug().Msgf("Query Builder built raw SQL query: %s", queryBuilder.String())

	var res []struct {
		TotalTasks db.RawString `json:"total_tasks"`
	}

	err = o.clientWrapper.Client.Prisma.QueryRaw(queryBuilder.String()).Exec(ctx, &res)
	if err != nil {
		log.Error().Err(err).Msg("Error executing raw query for total tasks")
		return nil, 0, err
	}

	if len(res) == 0 {
		// probably didn't name the right fields in the raw query,
		// "total_tasks" need to match in res and named alias from count(*)
		log.Error().Msg("No tasks found")
		return nil, 0, err
	}

	totalTasksStr := string(res[0].TotalTasks)
	log.Info().Interface("totalTasks", totalTasksStr).Msg("Total tasks fetched")

	totalTasks, err := strconv.Atoi(totalTasksStr)
	if err != nil {
		log.Error().Err(err).Msg("Error converting total tasks to integer")
		return nil, 0, err
	}
	log.Info().Int("totalTasks", totalTasks).Msg("Total tasks fetched, converted to int")
	return tasks, totalTasks, nil
}
