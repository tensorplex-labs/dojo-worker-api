package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"mime/multipart"
	"slices"
	"strconv"
	"time"

	"dojo-api/db"
	"dojo-api/pkg/orm"
	"dojo-api/pkg/sandbox"
	"dojo-api/utils"

	"github.com/gin-gonic/gin"
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
func (taskService *TaskService) GetTaskResponseById(ctx context.Context, id string) (*TaskResponse, error) {
	taskORM := orm.NewTaskORM()

	task, err := taskORM.GetById(ctx, id)

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
		ID:          task.ID,
		Title:       task.Title,
		Body:        task.Body,
		ExpireAt:    task.ExpireAt,
		Type:        task.Type,
		TaskData:    rawJSON,
		Status:      task.Status,
		MaxResults:  task.MaxResults,
		NumResults:  task.NumResults,
		NumCriteria: task.NumCriteria,
	}, nil
}

// TODO: Implement yieldMin, yieldMax
func (taskService *TaskService) GetTasksByPagination(ctx context.Context, workerId string, page int, limit int, types []string, sort string) (*TaskPagination, []error) {
	// Calculate offset based on the page and limit
	offset := (page - 1) * limit
	// taskORM := orm.NewTaskORM()

	// Determine the sort order dynamically
	var sortQuery db.TaskOrderByParam
	switch sort {
	case "createdAt":
		sortQuery = db.Task.CreatedAt.Order(db.SortOrderDesc)
	case "numResults":
		sortQuery = db.Task.NumResults.Order(db.SortOrderDesc)
	case "numCriteria":
		sortQuery = db.Task.NumCriteria.Order(db.SortOrderDesc)
	default:
		sortQuery = db.Task.CreatedAt.Order(db.SortOrderDesc)
	}

	taskTypes, errs := convertStringToTaskTypes(types)
	if len(errs) > 0 {
		return nil, errs
	}

	// Fetch all completed task by this worker
	completedTaskMap, _ := taskService.GetCompletedTaskMap(ctx, workerId)

	log.Debug().Interface("completedTaskMap", completedTaskMap).Msg("Completed Task Mapping -------")

	tasks, totalTasks, err := taskService.taskORM.GetTasksByWorkerSubscription(ctx, workerId, offset, limit, sortQuery, taskTypes)
	if err != nil {
		log.Error().Err(err).Msg("Error getting tasks by pagination")
		return nil, []error{err}
	}

	// Convert tasks to TaskResponse model
	taskResponses := make([]TaskPaginationResponse, 0)
	for _, task := range tasks {
		var rawJSON json.RawMessage
		err = json.Unmarshal([]byte(task.TaskData), &rawJSON)
		if err != nil {
			log.Error().Err(err).Msg("Error parsing task data")
			return nil, []error{err}
		}
		taskResponse := TaskPaginationResponse{
			TaskResponse: TaskResponse{ // Fill the embedded TaskResponse structure.
				ID:          task.ID,
				Title:       task.Title,
				Body:        task.Body,
				ExpireAt:    task.ExpireAt,
				Type:        task.Type,
				TaskData:    rawJSON,
				Status:      task.Status,
				NumResults:  task.NumResults,
				MaxResults:  task.MaxResults,
				NumCriteria: task.NumCriteria,
			},
			IsCompletedByWorker: completedTaskMap[task.ID], // Set the completion status.
		}
		taskResponses = append(taskResponses, taskResponse)
	}

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
	}, []error{}
}

func convertStringToTaskTypes(taskTypes []string) ([]db.TaskType, []error) {
	convertedTypes := make([]db.TaskType, 0)
	errors := make([]error, 0)
	for _, t := range taskTypes {
		isValid, err := IsValidTaskType(t)
		if !isValid {
			errors = append(errors, err)
			continue
		}
		convertedTypes = append(convertedTypes, db.TaskType(t))
	}
	return convertedTypes, errors
}

type ErrInvalidTaskType struct {
	Type interface{}
}

func (e *ErrInvalidTaskType) Error() string {
	return fmt.Sprintf("invalid task type: '%v', supported types are %v", e.Type, ValidTaskTypes)
}

func IsValidTaskType(taskType interface{}) (bool, error) {
	switch t := taskType.(type) {
	case string, db.TaskType:
		for _, validType := range ValidTaskTypes {
			switch v := t.(type) {
			case string:
				if v == string(validType) {
					return true, nil
				}
			case db.TaskType:
				if v == validType {
					return true, nil
				}
			}
		}
		return false, &ErrInvalidTaskType{Type: t}
	default:
		return false, fmt.Errorf("invalid task type argument: %T, supported types are string and db.TaskType", t)
	}
}

func IsValidCriteriaType(criteriaType CriteriaType) bool {
	switch criteriaType {
	case CriteriaTypeMultiSelect, CriteriaTypeRanking, CriteriaTypeScore, CriteriaMultiScore:
		return true
	default:
		return false
	}
}

// create task
func (s *TaskService) CreateTasks(request CreateTaskRequest, minerUserId string) ([]*db.TaskModel, []error) {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tasks := make([]*db.TaskModel, 0)
	errors := make([]error, 0)

	taskORM := orm.NewTaskORM()
	for _, currTask := range request.TaskData {
		taskType := db.TaskType(currTask.Task)

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
			ExpireAt:    *expireAt,
			Title:       request.Title,
			Body:        request.Body,
			Type:        db.TaskType(taskType),
			TaskData:    taskData,
			MaxResults:  request.MaxResults,
			NumResults:  0,
			Status:      db.TaskStatusInProgress,
			NumCriteria: len(currTask.Criteria),
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

	// Check if the task has reached the max results, no way we can have greater than max results, or something's wrong
	if task.NumResults >= task.MaxResults {
		log.Info().Msg("Task has reached max results")
		newTaskResultData.Status = db.TaskResultStatusInvalid
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

			for _, criteria := range taskData.Criteria {
				if criteria.Type != itemType {
					continue
				}

				if len(multiSelect) > len(criteria.Options) {
					return nil, fmt.Errorf("number of selections provided exceeds number of options")
				}
			}
		case CriteriaMultiScore:
			multiScore, _ := item.Value.(MultiScoreValue)
			for _, criteria := range taskData.Criteria {
				if criteria.Type != itemType {
					continue
				}

				if len(multiScore) != len(criteria.Options) {
					return nil, fmt.Errorf("number of scores provided does not match number of options")
				}

				for option, score := range multiScore {
					minScore, maxScore := criteria.Min, criteria.Max
					if float64(score) < minScore || float64(score) > maxScore {
						return nil, fmt.Errorf("score %v is out of the valid range [%v, %v]", score, minScore, maxScore)
					}

					if !slices.Contains(criteria.Options, option) {
						return nil, fmt.Errorf("option %v not found in criteria options", option)
					}
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

	isValid, err := IsValidTaskType(taskData.Task)
	if !isValid {
		return err
	}

	if taskData.Task == db.TaskTypeDialogue {
		if len(taskData.Dialogue) == 0 {
			return errors.New("dialogue cannot be empty")
		}
	} else {
		if taskData.Prompt == "" {
			return errors.New("prompt is required")
		}
	}

	task := taskData.Task
	if task == db.TaskTypeTextToImage || task == db.TaskTypeCodeGeneration {
		for _, taskresponse := range taskData.Responses {
			if task == db.TaskTypeTextToImage {
				if _, ok := taskresponse.Completion.(string); !ok {
					return fmt.Errorf("invalid completion format: %v", taskresponse.Completion)
				}
			} else if task == db.TaskTypeCodeGeneration {
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
	} else if task == db.TaskTypeDialogue {
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
		case CriteriaTypeRanking, CriteriaMultiScore:
			if len(criteria.Options) == 0 {
				return errors.New("options is required for multiple choice criteria")
			}
			if task != db.TaskTypeDialogue {
				if len(criteria.Options) != len(taskData.Responses) {
					return fmt.Errorf("number of options should match number of responses: %v", len(taskData.Responses))
				}
			}

			if criteria.Type == CriteriaMultiScore {
				if (criteria.Min < 0 || criteria.Max < 0) || (criteria.Min == 0 && criteria.Max == 0) {
					return errors.New("valid min or max is required for numeric criteria")
				}

				if criteria.Min >= criteria.Max {
					return errors.New("min must be less than max")
				}
			}
		case CriteriaTypeScore:
			if (criteria.Min < 0 || criteria.Max < 0) || (criteria.Min == 0 && criteria.Max == 0) {
				return errors.New("valid min or max is required for numeric criteria")
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

	return nil
}

func ProcessTaskRequest(taskData CreateTaskRequest) (CreateTaskRequest, error) {
	processedTaskData := make([]TaskData, 0)
	for _, taskInterface := range taskData.TaskData {
		if taskInterface.Task == db.TaskTypeCodeGeneration {
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

func (t *TaskService) ValidateCompletedTResultByWorker(ctx context.Context, taskId string, workerId string) (bool, error) {
	taskResult, err := t.taskResultORM.GetCompletedTResultByTaskAndWorker(ctx, taskId, workerId)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return false, nil // No existing task result found
		}
		return false, err // An error occurred while fetching the task result
	}
	return len(taskResult) > 0, nil // Task result exists
}

func (t *TaskService) GetCompletedTaskMap(ctx context.Context, workerId string) (map[string]bool, error) {
	// Fetch all completed task by this worker
	completedtResult, err := t.taskResultORM.GetCompletedTResultByWorker(ctx, workerId)
	// Convert to a map for quick lookup
	completedTaskMap := make(map[string]bool)

	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return completedTaskMap, nil // No existing task result found
		}
		return nil, err
	}

	if len(completedtResult) > 0 {
		for _, ts := range completedtResult {
			completedTaskMap[ts.TaskID] = true
		}
	}
	return completedTaskMap, nil // Task result exists
}

func ProcessRequestBody(c *gin.Context) (CreateTaskRequest, error) {
	var reqbody CreateTaskRequest
	title := c.PostForm("title")
	body := c.PostForm("body")
	expireAt := c.PostForm("expireAt")
	maxResults, _ := strconv.Atoi(c.PostForm("maxResults"))
	totalRewards, _ := strconv.ParseFloat(c.PostForm("totalRewards"), 64)

	var taskData []TaskData
	if err := json.Unmarshal([]byte(c.PostForm("taskData")), &taskData); err != nil {
		log.Error().Err(err).Msg("Invalid taskData")
		return reqbody, err
	}

	reqbody = CreateTaskRequest{
		Title:        title,
		Body:         body,
		ExpireAt:     expireAt,
		TaskData:     taskData,
		MaxResults:   maxResults,
		TotalRewards: totalRewards,
	}

	return reqbody, nil
}

func ProcessFileUpload(requestBody CreateTaskRequest, files []*multipart.FileHeader) (CreateTaskRequest, error) {
	for i, t := range requestBody.TaskData {
		if t.Task == db.TaskTypeTextToImage {
			for j, response := range t.Responses {
				var fileHeader *multipart.FileHeader
				// Find the file with the matching completion filename
				for _, file := range files {
					if file.Filename == response.Completion {
						fileHeader = file
						break
					}
				}

				if fileHeader == nil {
					log.Error().Interface("response", response.Completion).Msg("Failed to find file header for response")
					return CreateTaskRequest{}, errors.New("failed to find file header for response")
				}

				// Upload the file to S3
				fileObj, err := utils.UploadFileToS3(fileHeader)
				if err != nil {
					log.Error().Err(err).Msg("Failed to upload file to S3")
					return CreateTaskRequest{}, err
				}

				log.Info().Interface("fileObj", fileObj).Msg("File uploaded successfully")
				// Update the response completion with the S3 URL
				requestBody.TaskData[i].Responses[j].Completion = fileObj.Location
			}
		}
	}
	return requestBody, nil
}
