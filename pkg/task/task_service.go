package task

import (
	"dojo-api/db"
	"dojo-api/pkg/orm"
	"dojo-api/utils"
	"time"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type TaskService struct {
	client *db.PrismaClient
}

func NewTaskService() *TaskService {
	return &TaskService{
		client: orm.NewPrismaClient(),
	}
}

type Task struct{
	Title       string      `json:"title"`
	Body        string      `json:"body"`
	Modality    db.TaskModality `json:"modality"`
	ExpireAt    time.Time   `json:"expireAt"`
	Criteria    []byte      `json:"criteria"`
	TaskData    []byte `json:"taskData"`
	MaxResults  int         `json:"maxResults"`
	TotalRewards float64    `json:"totalRewards"`
}

// create task 
func (t *TaskService) CreateTask(taskData map[string]interface{}, userid string) ([]string, error){
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
	for _, taskInterface := range taskData["taskData"].([]interface{}) {
		task, ok := taskInterface.(map[string]interface{})
		if !ok {
            logger.Error().Msg("Invalid task data format")
            return nil, errors.New("invalid task data format")
        }
        expireAtStr, ok := taskData["expireAt"].(string)
        if !ok {
            logger.Error().Msg("Missing or invalid expireAt field")
            return nil, errors.New("missing or invalid expireAt field")
        }
        parsedTime, err := time.Parse(time.DateTime, expireAtStr)
        if err != nil {
            logger.Error().Msgf("Error parsing time: %v", err)
            return nil, errors.New("invalid time format")
        }
		modalityStr, ok := task["task"].(string)
        if !ok {
            logger.Error().Msg("Missing or invalid modality field")
            return nil, errors.New("missing or invalid modality field")
        }
        modality := db.TaskModality(modalityStr)

		criteriaJSON, err := json.Marshal(task["criteria"])
        if err != nil {
            logger.Error().Msgf("Error marshaling criteria: %v", err)
            return nil, errors.New("invalid criteria format")
        }

		taskInfoJSON, err := json.Marshal(task["taskData"])
		if err != nil {
			logger.Error().Msgf("Error marshaling task data: %v", err)
			return nil, errors.New("invalid task data format")
		}

		newTask := Task{
			Title: taskData["title"].(string),
			Body: taskData["body"].(string),
			Modality: modality,
			ExpireAt: parsedTime, 
			Criteria: criteriaJSON,
			TaskData: taskInfoJSON,
			MaxResults: int(taskData["maxResults"].(float64)),
			TotalRewards: taskData["totalRewards"].(float64),
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

	return task.ID, err
}
