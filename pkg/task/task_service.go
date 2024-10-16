package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"mime/multipart"
	"os"
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
func (taskService *TaskService) GetTasksByPagination(ctx context.Context, workerId string, params PaginationParams) (*TaskPagination, []error) {
	// Calculate offset based on the page and limit
	offset := (params.Page - 1) * params.Limit

	// Determine the sort order dynamically
	var sortQuery db.TaskOrderByParam
	switch params.Sort {
	case "createdAt":
		sortQuery = db.Task.CreatedAt.Order(params.Order)
	case "numResults":
		sortQuery = db.Task.NumResults.Order(params.Order)
	case "numCriteria":
		sortQuery = db.Task.NumCriteria.Order(params.Order)
	default:
		sortQuery = db.Task.CreatedAt.Order(params.Order)
	}

	taskTypes, errs := convertStringToTaskTypes(params.Types)
	if len(errs) > 0 {
		return nil, errs
	}

	// Fetch all completed task by this worker
	completedTaskMap, _ := taskService.GetCompletedTaskMap(ctx, workerId)

	log.Debug().Interface("completedTaskMap", completedTaskMap).Msg("Completed Task Mapping -------")

	tasks, totalTasks, err := taskService.taskORM.GetTasksByWorkerSubscription(ctx, workerId, offset, params.Limit, sortQuery, taskTypes)
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

	totalPages := int(math.Ceil(float64(totalTasks) / float64(params.Limit)))

	// Construct pagination metadata
	pagination := Pagination{
		Page:       params.Page,
		Limit:      params.Limit,
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

func (s *TaskService) CreateTasksWithTimeout(request CreateTaskRequest, minerUserId string, timeout time.Duration) ([]*db.TaskModel, []error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type result struct {
		tasks []*db.TaskModel
		errs  []error
	}

	resultChan := make(chan result, 1)

	go func() {
		tasks, errs := s.CreateTasks(ctx, request, minerUserId)
		resultChan <- result{tasks: tasks, errs: errs}
	}()

	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			log.Error().Dur("timeout", timeout).Msg("CreateTasks timed out due to deadline")
			return nil, []error{fmt.Errorf("operation timed out after %v", timeout)}
		}
		log.Error().Err(ctx.Err()).Msg("Context canceled while creating tasks")
		return nil, []error{ctx.Err()}
	case res := <-resultChan:
		if len(res.tasks) == 0 && len(res.errs) == 0 {
			log.Warn().Msg("No tasks created and no errors reported")
			return nil, []error{fmt.Errorf("no tasks were created and no errors were reported")}
		}
		return res.tasks, res.errs
	}
}

// create task
func (s *TaskService) CreateTasks(ctx context.Context, request CreateTaskRequest, minerUserId string) ([]*db.TaskModel, []error) {
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

		task, err := taskORM.CreateTask(ctx, taskToCreate, minerUserId)
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

// TODO: Update this function with the new Resultdata structure
func (t *TaskService) UpdateTaskResults(ctx context.Context, task *db.TaskModel, dojoWorkerId string, results []Result) (*db.TaskModel, error) {
	validatedResults, err := ValidateResultData(results, task)
	if err != nil {
		log.Error().Err(err).Msg("Error validating result data")
		return nil, err
	}

	// Process and scale the scores
	processedResults, err := ProcessScores(validatedResults, task)
	if err != nil {
		log.Error().Err(err).Msg("Error processing scores")
		return nil, err
	}

	jsonResults, err := json.Marshal(processedResults)
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

	modelNames := make([]string, 0, len(taskData.Responses))
	for _, response := range taskData.Responses {
		modelNames = append(modelNames, response.Model)
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
			if score < 1.0 || score > 10.0 {
				return nil, fmt.Errorf("score %v is out of the valid range [1, 10]", score)
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

				// Validate that options match model names
				if !slices.Equal(criteria.Options, modelNames) {
					return nil, fmt.Errorf("multi-score options does not match the model names in responses")
				}

				for option, score := range multiScore {
					if score < 1.0 || score > 10.0 {
						return nil, fmt.Errorf("score %v for option %s is out of the valid range [1, 10]", score, option)
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

func ProcessScores(results []Result, task *db.TaskModel) ([]Result, error) {
	var taskData TaskData
	err := json.Unmarshal(task.TaskData, &taskData)
	if err != nil {
		log.Error().Err(err).Msg("Error unmarshaling task data")
		return nil, err
	}

	for i, item := range results {
		switch CriteriaType(item.Type) {
		case CriteriaTypeScore:
			score, _ := item.Value.(ScoreValue)
			for _, criteria := range taskData.Criteria {
				if criteria.Type == CriteriaTypeScore {
					scaledScore := scaleScore(float64(score), 1, 10, criteria.Min, criteria.Max)
					results[i].Value = ScoreValue(scaledScore)
					break
				}
			}

		case CriteriaMultiScore:
			multiScore, _ := item.Value.(MultiScoreValue)
			for _, criteria := range taskData.Criteria {
				if criteria.Type == CriteriaMultiScore {
					scaledMultiScore := make(MultiScoreValue)
					for option, score := range multiScore {
						scaledScore := scaleScore(float64(score), 1, 10, criteria.Min, criteria.Max)
						scaledMultiScore[option] = float64(ScoreValue(scaledScore))
					}
					results[i].Value = scaledMultiScore
					break
				}
			}
		}
	}

	return results, nil
}

func scaleScore(score, oldMin, oldMax, newMin, newMax float64) float64 {
	return ((score-oldMin)/(oldMax-oldMin))*(newMax-newMin) + newMin
}

// Validates a single task, reads the `type` field to determine different flows.
//
//nolint:gocyclo
func ValidateTaskData(taskData TaskData) error {
	if taskData.Task == "" {
		return errors.New("task is required")
	}

	isValid, err := IsValidTaskType(taskData.Task)
	if !isValid {
		return err
	}

	if taskData.Prompt == "" {
		return errors.New("prompt is required")
	}

	if len(taskData.Responses) == 0 {
		return errors.New("responses shouldn't be empty")
	}

	task := taskData.Task
	for _, taskresponse := range taskData.Responses {
		switch task {
		case db.TaskTypeTextToImage:
			if _, ok := taskresponse.Completion.(map[string]interface{}); !ok {
				return fmt.Errorf("invalid completion format: %v", taskresponse.Completion)
			}
		case db.TaskTypeCodeGeneration:
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
		case db.TaskTypeDialogue:
			messages, ok := taskresponse.Completion.([]interface{})
			if !ok {
				return fmt.Errorf("invalid completion format: %v", taskresponse.Completion)
			}

			for _, msg := range messages {
				message, ok := msg.(map[string]interface{})
				if !ok {
					return fmt.Errorf("invalid message format: %v", msg)
				}

				if _, ok := message["role"].(string); !ok {
					return errors.New("role is required for each message")
				}

				if _, ok := message["message"].(string); !ok {
					return errors.New("message is required for each message")
				}
			}
		case db.TaskTypeTextToThreeD:
			if _, ok := taskresponse.Completion.(map[string]interface{}); !ok {
				return fmt.Errorf("invalid completion format: %v", taskresponse.Completion)
			}
		}
	}

	if len(taskData.Criteria) == 0 {
		return errors.New("criteria is required")
	}

	modelNames := make([]string, 0, len(taskData.Responses))
	for _, response := range taskData.Responses {
		modelNames = append(modelNames, response.Model)
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
			// Block ranking criteria
			return fmt.Errorf("ranking criteria is not supported")
		case CriteriaMultiScore:
			if len(criteria.Options) == 0 {
				return errors.New("options is required for multiple choice criteria")
			}
			if task != db.TaskTypeDialogue {
				if len(criteria.Options) != len(taskData.Responses) {
					return fmt.Errorf("number of options should match number of responses: %v", len(taskData.Responses))
				}

				// Validate that options match model names
				if !slices.Equal(criteria.Options, modelNames) {
					return fmt.Errorf("multi-score options must match the model names in responses")
				}
			}

			if (criteria.Min < 0 || criteria.Max < 0) || (criteria.Min == 0 && criteria.Max == 0) {
				return errors.New("valid min or max is required for numeric criteria")
			}

			if criteria.Min >= criteria.Max {
				return errors.New("min must be less than max")
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
			// Combine the files
			combinedResponse, err := sandbox.CombineFiles(completionMap)
			if err != nil {
				log.Error().Msg("Error combining files")
				return taskData, err
			}
			if combinedResponse.CombinedHTML != "" {
				completionMap["combined_html"] = combinedResponse.CombinedHTML
			} else {
				log.Info().Interface("combinedResponse", combinedResponse).Msg("Combined Response")
				log.Error().Msg("Error combining files")
				return taskData, errors.New("error combining files")
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
	// set max memory to 64 MB
	if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
		log.Error().Err(err).Msg("Failed to parse multipart form")
		return CreateTaskRequest{}, fmt.Errorf("failed to parse form: %w", err)
	}

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
	publicURL := os.Getenv("S3_PUBLIC_URL")
	if publicURL == "" {
		log.Error().Msg("S3_PUBLIC_URL not set")
		return CreateTaskRequest{}, errors.New("S3_PUBLIC_URL not set")
	}
	for i, t := range requestBody.TaskData {
		if t.Task == db.TaskTypeTextToImage || t.Task == db.TaskTypeTextToThreeD {
			for j, response := range t.Responses {
				completionMap, ok := response.Completion.(map[string]interface{})
				if !ok {
					return CreateTaskRequest{}, fmt.Errorf("unexpected type for response.Completion: %T", response.Completion)
				}

				filename, ok := completionMap["filename"].(string)
				if !ok {
					log.Error().Msg("Filename not found in completion map or not a string")
					return CreateTaskRequest{}, errors.New("filename not found in completion map or not a string")
				}

				log.Info().Str("filename", filename).Interface("files", files).Msg("Debugging file matching")
				var fileHeader *multipart.FileHeader
				// Find the file with the matching completion filename
				for _, file := range files {
					if file.Filename == filename {
						fileHeader = file
						break
					}
				}

				if fileHeader == nil {
					log.Error().Str("filename", filename).Msg("Failed to find file header for response")
					return CreateTaskRequest{}, errors.New("failed to find file header for response")
				}

				// Upload the file to S3
				fileObj, err := utils.UploadFileToS3(fileHeader)
				if err != nil {
					log.Error().Err(err).Msg("Failed to upload file to S3")
					return CreateTaskRequest{}, err
				}

				log.Info().Interface("fileObj", fileObj).Msg("File uploaded successfully")
				fileURL := fmt.Sprintf("%s/%s", publicURL, *fileObj.Key)
				log.Info().Str("fileURL", fileURL).Msg("File URL")

				// Update the response completion with the S3 URL
				completionMap["url"] = fileURL
				requestBody.TaskData[i].Responses[j].Completion = completionMap
			}
		}
	}
	return requestBody, nil
}
