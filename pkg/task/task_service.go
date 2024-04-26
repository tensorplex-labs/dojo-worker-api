package task

import (
	"context"
	"dojo-api/db"
	"dojo-api/pkg/orm"
	"dojo-api/utils"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type TaskService struct {
	client *db.PrismaClient
}

func NewTaskService() *TaskService {
	return &TaskService{
		client: orm.NewPrismaClient(),
	}
}

type Task struct {
	Title        string          `json:"title"`
	Body         string          `json:"body"`
	Modality     db.TaskModality `json:"modality"`
	ExpireAt     time.Time       `json:"expireAt"`
	Criteria     []byte          `json:"criteria"`
	TaskData     []byte          `json:"taskData"`
	MaxResults   int             `json:"maxResults"`
	TotalRewards float64         `json:"totalRewards"`
}

// create task
func (t *TaskService) CreateTask(taskData utils.TaskRequest, userid string) ([]string, error) {
	var logger = utils.GetLogger()
	client := db.NewClient()
	ctx := context.Background()
	defer func() {
		if err := client.Prisma.Disconnect(); err != nil {
			logger.Error().Msgf("Error disconnecting: %v", err)
		}
	}()
	client.Prisma.Connect()
	var createdIds []string
	for _, taskInterface := range taskData.TaskData {
		modality := db.TaskModality(taskInterface.Task)

		criteriaJSON, err := json.Marshal(taskInterface.Criteria)
		if err != nil {
			logger.Error().Msgf("Error marshaling criteria: %v", err)
			return nil, errors.New("invalid criteria format")
		}

		taskInfoJSON, err := json.Marshal(taskInterface)
		if err != nil {
			logger.Error().Msgf("Error marshaling task data: %v", err)
			return nil, errors.New("invalid task data format")
		}

		parsedExpireAt, err := time.Parse(time.DateTime, taskData.ExpireAt.(string))
		if err != nil {
			logger.Error().Msgf("Error parsing expireAt: %v", err)
			return nil, errors.New("invalid expireAt format")
		}

		newTask := Task{
			Title:        taskData.Title,
			Body:         taskData.Body,
			Modality:     modality,
			ExpireAt:     parsedExpireAt,
			Criteria:     criteriaJSON,
			TaskData:     taskInfoJSON,
			MaxResults:   taskData.MaxResults,
			TotalRewards: taskData.TotalRewards,
		}

		id, err := insertTaskData(newTask, userid, client, ctx)

		if err != nil {
			logger.Error().Msgf("Error creating task: %v", err)
			return nil, err
		}

		createdIds = append(createdIds, fmt.Sprintf("Task created with ID: %s", id))
	}

	return createdIds, nil
}

func insertTaskData(newTask Task, userid string, client *db.PrismaClient, ctx context.Context) (string, error) {
	task, err := client.Task.CreateOne(
		db.Task.Title.Set(newTask.Title),
		db.Task.Body.Set(newTask.Body),
		db.Task.Modality.Set(newTask.Modality),
		db.Task.ExpireAt.Set(newTask.ExpireAt),
		db.Task.Criteria.Set(newTask.Criteria),
		db.Task.TaskData.Set(newTask.TaskData),
		db.Task.Status.Set("PENDING"),
		db.Task.MaxResults.Set(newTask.MaxResults),
		db.Task.NumResults.Set(0),
		db.Task.TotalYield.Set(newTask.TotalRewards),
		db.Task.NetworkUser.Link(
			db.NetworkUser.ID.Equals(userid),
		),
	).Exec(ctx)

	if err != nil {
		return "", err
	}

	if task == nil {
		return "", errors.New("failed to create task")
	}

	return task.ID, err
}
