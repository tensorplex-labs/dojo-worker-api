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
	"dojo-api/pkg/sandbox"
	"dojo-api/utils"

	"github.com/rs/zerolog/log"
)

type TaskService struct {
	taskORM       *orm.TaskORM
	taskResultORM *orm.TaskResultORM
}

func NewTaskService() *TaskService {
	return &TaskService{
		taskORM:       orm.NewTaskORM(),
		taskResultORM: orm.NewTaskResultORM(),
	}
}

// get task by id
func (taskService *TaskService) GetTaskResponseById(ctx context.Context, id string, workerId string) (*TaskResponse, error) {
	taskORM := orm.NewTaskORM()

	task, err := taskORM.GetTaskByIdWithSub(ctx, id, workerId)

	if err != nil {
		log.Error().Err(err).Msg("Error in getting task by Id")
		return nil, err
	}
	// Ensure task is not nil if Prisma does not handle not found errors automatically
	if task == nil {
		return nil, fmt.Errorf("no task found with ID %s", id)
	}

	var rawJSON json.RawMessage
	err = json.Unmarshal([]byte(task.TaskData), &rawJSON)
	if err != nil {
		log.Error().Err(err).Msg("Error parsing task data")
		return nil, err
	}

	return &TaskResponse{
		ID:         task.ID,
		Title:      task.Title,
		Body:       task.Body,
		ExpireAt:   task.ExpireAt,
		Type:       task.Type,
		TaskData:   rawJSON,
		Status:     task.Status,
		MaxResults: task.MaxResults,
	}, nil
}

// TODO: Implement yieldMin, yieldMax
func (taskService *TaskService) GetTasksByPagination(ctx context.Context, workerId string, page int, limit int, types []string, sort string) (*TaskPagination, error) {
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

	tasks, err := taskService.taskORM.GetTasksByWorkerSubscription(ctx, workerId, offset, limit, sortQuery, taskTypes)
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
		var rawJSON json.RawMessage
		err = json.Unmarshal([]byte(task.TaskData), &rawJSON)
		if err != nil {
			log.Error().Err(err).Msg("Error parsing task data")
			return nil, err
		}
		taskResponse := TaskResponse{
			ID:         task.ID,
			Title:      task.Title,
			Body:       task.Body,
			Type:       task.Type,
			ExpireAt:   task.ExpireAt,
			TaskData:   rawJSON,
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
func (s *TaskService) CreateTasks(request CreateTaskRequest, minerUserId string) ([]*db.TaskModel, []error) {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tasks := make([]*db.TaskModel, 0)
	errors := make([]error, 0)

	taskORM := orm.NewTaskORM()
	for _, currTask := range request.TaskData {
		taskType := TaskType(currTask.Task)

		_, err := json.Marshal(currTask.Criteria)
		if err != nil {
			log.Error().Err(err).Msgf("Error marshaling criteria")
			errors = append(errors, err)
		}

		taskData, err := json.Marshal(currTask)
		if err != nil {
			log.Error().Err(err).Msgf("Error marshaling task data")
			errors = append(errors, err)
		}

		expireAt := utils.ParseDate(request.ExpireAt.(string))
		log.Info().Msgf("ExpireAt: %v", expireAt)
		if expireAt == nil {
			log.Error().Msg("Error parsing expireAt")
			errors = append(errors, fmt.Errorf("error parsing expireAt"))
			continue
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
			errors = append(errors, err)
		}
		tasks = append(tasks, task)
	}
	return tasks, errors
}

func (t *TaskService) GetTaskById(ctx context.Context, id string) (*db.TaskModel, error) {
	task, err := t.taskORM.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, fmt.Errorf("task with ID %s not found", id)
		}
		return nil, err
	}

	return task, nil
}

func (t *TaskService) UpdateTaskResults(ctx context.Context, task *db.TaskModel, dojoWorkerId string, results []Result) (*db.TaskModel, error) {
	_, err := ValidateResultData(results, task)
	if err != nil {
		log.Error().Err(err).Msg("Error validating result data")
		return nil, err
	}

	jsonResults, err := json.Marshal(results)
	if err != nil {
		log.Error().Err(err).Msg("Error marshaling result items")
		return nil, err
	}
	newTaskResultData := db.InnerTaskResult{
		Status:     db.TaskResultStatusCompleted,
		ResultData: jsonResults,
		TaskID:     task.ID,
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

func ValidateResultData(results []Result, task *db.TaskModel) ([]Result, error) {
	var taskData TaskData
	err := json.Unmarshal(task.TaskData, &taskData)
	if err != nil {
		log.Error().Err(err).Msg("Error unmarshaling task data")
		return nil, err
	}

	for _, item := range results {
		itemType := CriteriaType(item.Type)
		if !IsValidCriteriaType(itemType) {
			log.Error().Msgf("Invalid criteria type: %v", item.Type)
			continue
		}
		switch itemType {
		case CriteriaTypeScore:
			score, _ := item.Value.(ScoreValue)
			for _, criteria := range taskData.Criteria {
				if criteria.Type != itemType {
					continue
				}
				minScore, maxScore := criteria.Min, criteria.Max
				if float64(score) < minScore || float64(score) > maxScore {
					return nil, fmt.Errorf("score %v is out of the valid range [%v, %v]", score, minScore, maxScore)
				}

			}

		case CriteriaTypeRanking:
			ranking, _ := item.Value.(RankingValue)
			if len(ranking) == 0 {
				return nil, fmt.Errorf("ranking criteria provided but no rankings found")
			}
			for _, criteria := range taskData.Criteria {
				if criteria.Type != itemType {
					continue
				}

				if len(ranking) != len(criteria.Options) {
					return nil, fmt.Errorf("number of rankings provided does not match number of options")
				}
			}
		case CriteriaTypeMultiSelect:
			multiSelect, _ := item.Value.(MultiSelectValue)
			if len(multiSelect) == 0 {
				return nil, fmt.Errorf("multi-select criteria provided but no selections found")
			}

			for _, criteria := range taskData.Criteria {
				if criteria.Type != itemType {
					continue
				}

				if len(multiSelect) > len(criteria.Options) {
					return nil, fmt.Errorf("number of selections provided exceeds number of options")
				}
			}

		default:
			return nil, fmt.Errorf("unknown result data type: %s", item.Type)
		}
	}

	log.Info().Str("resultData", fmt.Sprintf("%v", results)).Msgf("Result data validated successfully")
	return results, nil
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
	if task == TaskTypeTextToImage || task == TaskTypeCodeGen {
		for _, taskresponse := range taskData.Responses {
			if task == TaskTypeTextToImage {
				if _, ok := taskresponse.Completion.(string); !ok {
					return fmt.Errorf("invalid completion format: %v", taskresponse.Completion)
				}
			} else if task == TaskTypeCodeGen {
				if _, ok := taskresponse.Completion.(map[string]interface{}); !ok {
					return fmt.Errorf("invalid completion format: %v", taskresponse.Completion)
				}

				files, ok := taskresponse.Completion.(map[string]interface{})["files"]
				if !ok {
					return errors.New("files is required for code generation task")
				}

				if _, ok = files.([]interface{}); !ok {
					return errors.New("files must be an array")
				}
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

func ProcessTaskRequest(taskData CreateTaskRequest) (CreateTaskRequest, error) {
	processedTaskData := make([]TaskData, 0)
	for _, taskInterface := range taskData.TaskData {
		if taskInterface.Task == TaskTypeCodeGen {
			processedTaskEntry, err := ProcessCodeCompletion(taskInterface)
			if err != nil {
				log.Error().Msg("Error processing code completion")
				return taskData, err
			}
			processedTaskData = append(processedTaskData, processedTaskEntry)
		} else {
			processedTaskData = append(processedTaskData, taskInterface)
		}
	}
	taskData.TaskData = processedTaskData
	return taskData, nil
}

func ProcessCodeCompletion(taskData TaskData) (TaskData, error) {
	responses := taskData.Responses
	for i, response := range responses {
		completionMap, ok := response.Completion.(map[string]interface{})
		if !ok {
			log.Error().Msg("You sure this is code generation?")
			return taskData, errors.New("invalid completion format")
		}
		if _, ok := completionMap["files"]; ok {
			sandboxResponse, err := sandbox.GetCodesandbox(completionMap)
			if err != nil {
				log.Error().Msg(fmt.Sprintf("Error getting sandbox response: %v", err))
				return taskData, err
			}
			if sandboxResponse.Url != "" {
				completionMap["sandbox_url"] = sandboxResponse.Url
			} else {
				fmt.Println(sandboxResponse)
				log.Error().Msg("Error getting sandbox response")
				return taskData, errors.New("error getting sandbox response")
			}
		} else {
			log.Error().Msg("Invalid completion format")
			return taskData, errors.New("invalid completion format")
		}
		taskData.Responses[i].Completion = completionMap
	}
	return taskData, nil
}
