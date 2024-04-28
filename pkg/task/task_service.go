package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"dojo-api/db"
	"dojo-api/pkg/orm"
	"dojo-api/utils"

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

func IsValidTaskType(taskType TaskType) bool {
	if taskType == TaskTypeCodeGen || taskType == TaskTypeTextToImage || taskType == TaskTypeDialogue {
		return true
	}
	return false
}

func IsValidCriteriaType(criteriaType CriteriaType) bool {
	if criteriaType == CriteriaTypeMultiSelect || criteriaType == CriteriaTypeRanking || criteriaType == CriteriaTypeScore {
		return true
	}
	return false
}

// create task
func (s *TaskService) CreateTasks(request CreateTaskRequest, minerUserId string) ([]string, error) {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var taskIds []string
	taskORM := orm.NewTaskORM()
	for _, currTask := range request.TaskData {
		taskType := TaskType(currTask.Task)

		_, err := json.Marshal(currTask.Criteria)
		if err != nil {
			log.Error().Err(err).Msgf("Error marshaling criteria")
			return nil, errors.New("invalid criteria format")
		}

		taskData, err := json.Marshal(currTask)
		if err != nil {
			log.Error().Err(err).Msgf("Error marshaling task data")
			return nil, errors.New("invalid task data format")
		}

		expireAt := utils.ParseDate(request.ExpireAt.(string))
		if expireAt == nil {
			log.Error().Msg("Error parsing expireAt")
			return nil, errors.New("invalid expireAt format")
		}

		taskToCreate := db.InnerTask{
			ExpireAt:   *expireAt,
			Title:      request.Title,
			Body:       request.Body,
			Type:       db.TaskType(taskType),
			TaskData:   taskData,
			MaxResults: request.MaxResults,
			NumResults: 0,
			Status:     db.TaskStatusInProgress,
		}

		if request.TotalRewards > 0 {
			taskToCreate.TotalReward = &request.TotalRewards
		}

		task, err := taskORM.CreateTask(ctxWithTimeout, taskToCreate, minerUserId)
		if err != nil {
			log.Error().Msgf("Error creating task: %v", err)
			return nil, err
		}

		taskIds = append(taskIds, task.ID)
	}
	return taskIds, nil
}

func (t *TaskService) GetDojoWorkerById(ctx context.Context, id string) (*db.DojoWorkerModel, error) {
	// print the dojo worker id
	fmt.Println("Dojo Worker ID: ", id)
	worker, err := t.client.DojoWorker.FindUnique(
		db.DojoWorker.ID.Equals(id),
	).Exec(ctx)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
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
		if errors.Is(err, db.ErrNotFound) {
			return nil, fmt.Errorf("Task with ID %s not found", id)
		}
		return nil, err
	}

	return task, nil
}

func (t *TaskService) UpdateTaskResultData(ctx context.Context, taskId string, dojoWorkerId string, resultData map[string]interface{}) (*db.TaskModel, error) {
	// Convert your map to json
	jsonResultData, err := json.Marshal(resultData)
	if err != nil {
		log.Error().Err(err).Msg("Error marshaling result data")
		return nil, err
	}

	newTaskResultData := db.InnerTaskResult{
		Status:     db.TaskResultStatusCompleted,
		ResultData: jsonResultData,
		TaskID:     taskId,
		WorkerID:   dojoWorkerId,
	}

	// Insert the task result data
	taskResultORM := orm.NewTaskResultORM()
	createdTaskResult, err := taskResultORM.CreateTaskResult(ctx, &newTaskResultData)
	if err != nil {
		return nil, err
	}

	return createdTaskResult.Task(), nil
}

// Validates a single task, reads the `type` field to determine different flows.
func ValidateTaskData(taskData TaskData) error {
	if taskData.Task == "" {
		return errors.New("task is required")
	}

	if !IsValidTaskType(taskData.Task) {
		return fmt.Errorf("unsupported task: %v", taskData.Task)
	}

	if taskData.Task == TaskTypeDialogue {
		if len(taskData.Dialogue) == 0 {
			return errors.New("dialogue cannot be empty")
		}
	} else {
		if taskData.Prompt == "" {
			return errors.New("prompt is required")
		}
	}

	task := taskData.Task
	if task == TaskTypeCodeGen || task == TaskTypeTextToImage {
		for _, taskresponse := range taskData.Responses {
			var ok bool
			if task == TaskTypeCodeGen {
				fmt.Println(taskresponse.Completion)
				_, ok = taskresponse.Completion.(map[string]interface{})
			} else if task == TaskTypeTextToImage {
				_, ok = taskresponse.Completion.(string)
			}

			if !ok {
				return fmt.Errorf("invalid completion format: %v", taskresponse.Completion)
			}
		}

		if len(taskData.Dialogue) != 0 {
			return errors.New("dialogue should be empty for code generation and text to image tasks")
		}
	} else if task == TaskTypeDialogue {
		if len(taskData.Responses) != 0 {
			return errors.New("responses should be empty for dialogue task")
		}

		if len(taskData.Dialogue) == 0 {
			return errors.New("dialogue is required for dialogue task")
		}
	}

	if len(taskData.Criteria) == 0 {
		return errors.New("criteria is required")
	}

	for _, criteria := range taskData.Criteria {
		if criteria.Type == "" {
			return errors.New("type is required for criteria")
		}

		if !IsValidCriteriaType(criteria.Type) {
			return errors.New("unsupported criteria")
		}

		switch criteria.Type {
		case CriteriaTypeMultiSelect:
			if len(criteria.Options) == 0 {
				return errors.New("options is required for multiple choice criteria")
			}
		case CriteriaTypeRanking:
			if len(criteria.Options) == 0 {
				return errors.New("options is required for multiple choice criteria")
			}
			if task != TaskTypeDialogue {
				if len(criteria.Options) != len(taskData.Responses) {
					return fmt.Errorf("number of options for criteria: %v should match number of responses: %v", CriteriaTypeRanking, len(taskData.Responses))
				}
			}
		case CriteriaTypeScore:
			if criteria.Min == 0 && criteria.Max == 0 {
				return errors.New("min or max is required for numeric criteria")
			}

			if criteria.Min >= criteria.Max {
				return errors.New("min must be less than max")
			}
		}
	}

	return nil
}

func ValidateTaskRequest(request CreateTaskRequest) error {
	if request.Title == "" {
		return errors.New("title is required")
	}

	if request.Body == "" {
		return errors.New("body is required")
	}

	if request.ExpireAt == "" {
		return errors.New("expireAt is required")
	}

	for _, currTask := range request.TaskData {
		err := ValidateTaskData(currTask)
		if err != nil {
			return err
		}
	}

	if request.MaxResults == 0 {
		return errors.New("maxResults is required")
	}

	if request.TotalRewards == 0 {
		return errors.New("totalRewards is required")
	}

	return nil
}
