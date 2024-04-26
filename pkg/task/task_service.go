package task

import (
	"context"
	"dojo-api/db"
	"dojo-api/pkg/orm"
	"math"

	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

type TaskService struct {
	client *db.PrismaClient
}

func NewTaskService() *TaskService {
	return &TaskService{
		client: orm.NewPrismaClient(),
	}
}

// get task by id
func (taskService *TaskService) GetTaskResponseById(ctx context.Context, id string) (*TaskResponse, error) {
	task, err := taskService.client.Task.FindUnique(db.Task.ID.Equals(id)).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error converting string to int64")
		return nil, err
	}
	// Ensure task is not nil if Prisma does not handle not found errors automatically
	if task == nil {
		return nil, fmt.Errorf("no task found with ID %s", id)
	}

	return &TaskResponse{
		ID:         task.ID,
		Title:      task.Title,
		Body:       task.Body,
		ExpireAt:   task.ExpireAt,
		Type:       task.Type,
		TaskData:   task.TaskData,
		Status:     task.Status,
		MaxResults: task.MaxResults,
	}, nil
}

// TODO: Implement yieldMin, yieldMax
func (taskService *TaskService) GetTasksByPagination(ctx context.Context, page int, limit int, types []string, sort string) (*TaskPagination, error) {
	// Calculate offset based on the page and limit
	offset := (page - 1) * limit

	// Determine the sort order dynamically
	var sortQuery db.TaskOrderByParam
	switch sort {
	case "createdAt":
		sortQuery = db.Task.CreatedAt.Order(db.SortOrderDesc)
	case "numResults":
		sortQuery = db.Task.NumResults.Order(db.SortOrderDesc)
	default:
		sortQuery = db.Task.CreatedAt.Order(db.SortOrderDesc)
	}

	taskTypes := convertStringToTaskType(types)

	tasks, err := taskService.client.Task.FindMany(
		db.Task.Type.In(taskTypes),
	).OrderBy(sortQuery).
		Skip(offset).
		Take(limit).
		Exec(ctx)

	if err != nil {
		log.Error().Err(err).Msg("Error getting tasks by pagination")
		return nil, err
	}

	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks found")
	}

	// Convert tasks to TaskResponse model
	var taskResponses []TaskResponse
	for _, task := range tasks {
		taskResponse := TaskResponse{
			ID:         task.ID,
			Title:      task.Title,
			Body:       task.Body,
			Type:       task.Type,
			ExpireAt:   task.ExpireAt,
			TaskData:   task.TaskData,
			Status:     task.Status,
			MaxResults: task.MaxResults,
		}
		taskResponses = append(taskResponses, taskResponse)
	}

	totalTasks := len(tasks)
	totalPages := int(math.Ceil(float64(totalTasks) / float64(limit)))

	// Construct pagination metadata
	pagination := Pagination{
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		TotalItems: totalTasks,
	}

	return &TaskPagination{
		Tasks:      taskResponses,
		Pagination: pagination,
	}, nil
}

func convertStringToTaskType(taskTypes []string) []db.TaskType {
	var convertedTypes []db.TaskType
	for _, t := range taskTypes {
		convertedTypes = append(convertedTypes, db.TaskType(t))
	}
	return convertedTypes
}

// create task
func (s *TaskService) BatchCreateTask(taskData TaskRequest, userId string) ([]string, error) {
	ctx := context.Background()
	var createdIds []string
	for _, taskInterface := range taskData.TaskData {
		modality := db.TaskType(taskInterface.Task)

		criteriaJSON, err := json.Marshal(taskInterface.Criteria)
		if err != nil {
			log.Error().Msgf("Error marshaling criteria: %v", err)
			return nil, errors.New("invalid criteria format")
		}

		taskInfoJSON, err := json.Marshal(taskInterface)
		if err != nil {
			log.Error().Msgf("Error marshaling task data: %v", err)
			return nil, errors.New("invalid task data format")
		}

		parsedExpireAt, err := time.Parse(time.DateTime, taskData.ExpireAt.(string))
		if err != nil {
			log.Error().Msgf("Error parsing expireAt: %v", err)
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

		id, err := insertTaskData(newTask, userId, s.client, ctx)

		if err != nil {
			log.Error().Msgf("Error creating task: %v", err)
			return nil, err
		}

		createdIds = append(createdIds, fmt.Sprintf("Task created with ID: %s", id))
	}

	return createdIds, nil
}

func insertTaskData(newTask Task, userid string, client *db.PrismaClient, ctx context.Context) (string, error) {
	task, err := client.Task.CreateOne(
		db.Task.ExpireAt.Set(newTask.ExpireAt),
		db.Task.Title.Set(newTask.Title),
		db.Task.Body.Set(newTask.Body),
		db.Task.Type.Set(newTask.Modality),
		// db.Task.Criteria.Set(newTask.Criteria),
		db.Task.TaskData.Set(newTask.TaskData),
		db.Task.Status.Set("PENDING"),
		db.Task.MaxResults.Set(newTask.MaxResults),
		db.Task.NumResults.Set(0),
		db.Task.TotalReward.Set(newTask.TotalRewards),
		db.Task.MinerUser.Link(
			db.MinerUser.ID.Equals(userid),
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

func insertTaskResultData(newTaskResult TaskResult, client *db.PrismaClient, ctx context.Context) (string, error) {
	taskResult, err := client.TaskResult.CreateOne(
		db.TaskResult.Status.Set("COMPLETED"),
		db.TaskResult.ResultData.Set(newTaskResult.ResultData),
		db.TaskResult.Task.Link(
			db.Task.ID.Equals(newTaskResult.TaskID),
		),
		db.TaskResult.DojoWorker.Link(
			db.DojoWorker.ID.Equals(newTaskResult.DojoWorkerID),
		),
	).Exec(ctx)

	return taskResult.ID, err
}

func (t *TaskService) GetDojoWorkerById(ctx context.Context, id string) (*db.DojoWorkerModel, error) {
	// print the dojo worker id
	fmt.Println("Dojo Worker ID: ", id)
	worker, err := t.client.DojoWorker.FindUnique(
		db.DojoWorker.ID.Equals(id),
	).Exec(ctx)

	if err != nil {
		if err == db.ErrNotFound {
			return nil, fmt.Errorf("DojoWorker with ID %s not found", id)
		}
		return nil, err
	}

	return worker, nil
}

func (t *TaskService) GetTaskById(ctx context.Context, id string) (*db.TaskModel, error) {
	task, err := t.client.Task.FindUnique(
		db.Task.ID.Equals(id),
	).Exec(ctx)

	if err != nil {
		if err == db.ErrNotFound {
			return nil, fmt.Errorf("Task with ID %s not found", id)
		}
		return nil, err
	}

	return task, nil
}

func (t *TaskService) UpdateTaskResultData(ctx context.Context, taskId string, dojoWorkerId string, resultData map[string]interface{}) (int, error) {

	// Convert your map to json
	jsonResultData, err := json.Marshal(resultData)
	if err != nil {
		return 0, err
	}

	newTaskResultData := TaskResult{
		Status:       "COMPLETED", // Check with evan if it's completed
		ResultData:   jsonResultData,
		TaskID:       taskId,
		DojoWorkerID: dojoWorkerId,
	}

	// Insert the task result data
	_, err = insertTaskResultData(newTaskResultData, t.client, ctx)
	if err != nil {
		return 0, err
	}

	// Increment numResults
	updatedTask, err := t.client.Task.FindUnique(db.Task.ID.Equals(taskId)).Update(
		db.Task.NumResults.Increment(1),
	).Exec(ctx)
	if err != nil {
		return 0, err
	}

	// If no errors occurred, return the updated numResults and nil
	return updatedTask.NumResults, nil
}

func ValidateTaskData(taskData TaskData) error {
	if taskData.Prompt == "" && len(taskData.Dialogue) == 0 {
		return errors.New("prompt is required")
	}

	if taskData.Task == "" {
		return errors.New("task is required")
	}
	task := taskData.Task
	if task == "CODE_GENERATION" || task == "TEXT_TO_IMAGE" {
		for _, taskresponse := range taskData.Responses {
			var ok bool
			if task == "CODE_GENERATION" {
				fmt.Println(taskresponse.Completion)
				_, ok = taskresponse.Completion.(map[string]interface{})
			} else if task == "TEXT_TO_IMAGE" {
				_, ok = taskresponse.Completion.(string)
			}
			if !ok {
				return fmt.Errorf("invalid completion format: %v", taskresponse.Completion)
			}
		}

		if len(taskData.Dialogue) != 0 {
			return errors.New("dialogue should be empty for code generation and text to image tasks")
		}
		// TODO: change to dialogue when schema is updated
	} else if task == "CONVERSATION" {
		if len(taskData.Responses) != 0 {
			return errors.New("responses should be empty for dialogue task")
		}

		if len(taskData.Dialogue) == 0 {
			return errors.New("dialogue is required for dialogue task")
		}
	} else {
		return errors.New("invalid task")
	}

	if len(taskData.Criteria) == 0 {
		return errors.New("criteria is required")
	}

	for _, criteria := range taskData.Criteria {
		if criteria.Type == "" {
			return errors.New("type is required for criteria")
		}

		if criteria.Type == "multi-select" || criteria.Type == "ranking" {
			if len(criteria.Options) == 0 {
				return errors.New("options is required for multiple choice criteria")
			}
		} else if criteria.Type == "score" {
			if criteria.Min == 0 && criteria.Max == 0 {
				return errors.New("min or max is required for numeric criteria")
			}
		}
	}

	return nil
}

func ValidateTaskRequest(taskData TaskRequest) error {
	if taskData.Title == "" {
		return errors.New("title is required")
	}

	if taskData.Body == "" {
		return errors.New("body is required")
	}

	if taskData.ExpireAt == "" {
		return errors.New("expireAt is required")
	}

	for _, taskInterface := range taskData.TaskData {
		err := ValidateTaskData(taskInterface)
		if err != nil {
			return err
		}
	}

	if taskData.MaxResults == 0 {
		return errors.New("maxResults is required")
	}

	if taskData.TotalRewards == 0 {
		return errors.New("totalRewards is required")
	}

	return nil
}
