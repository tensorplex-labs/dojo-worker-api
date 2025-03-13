package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dojo-api/db"
	"dojo-api/pkg/auth"
	"dojo-api/pkg/blockchain/siws"
	"dojo-api/pkg/cache"
	"dojo-api/pkg/metric"
	"dojo-api/pkg/miner"
	"dojo-api/pkg/orm"
	"dojo-api/pkg/task"
	"dojo-api/pkg/worker"
	"dojo-api/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
	"github.com/rs/zerolog/log"
	"github.com/spruceid/siwe-go"
)

// WorkerLoginController godoc
//
//	@Summary		Worker login
//	@Description	Log in a worker by providing their wallet address, chain ID, message, signature, and timestamp
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			body	body		worker.WorkerLoginRequest							true	"Request body containing the worker login details"
//	@Success		200		{object}	ApiResponse{body=worker.WorkerLoginSuccessResponse}	"Worker logged in successfully"
//	@Failure		400		{object}	ApiResponse											"Invalid wallet address or chain ID"
//	@Failure		401		{object}	ApiResponse											"Unauthorized access"
//	@Failure		403		{object}	ApiResponse											"Forbidden access"
//	@Failure		409		{object}	ApiResponse											"Worker already exists"
//	@Failure		500		{object}	ApiResponse											"Failed to create worker"
//	@Router			/worker/login/auth [post]
func WorkerLoginController(c *gin.Context) {
	walletAddressInterface, _ := c.Get("WalletAddress")
	chainIdInterface, _ := c.Get("ChainId")
	token, _ := c.Get("JWTToken")

	walletAddress, ok := walletAddressInterface.(string)
	if !ok {
		log.Error().Msg("Invalid wallet address provided")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid wallet address"))
		return
	}
	chainId, ok := chainIdInterface.(string)
	if !ok {
		log.Error().Msg("Invalid chainId provided")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid chainId"))
		return
	}

	workerORM := orm.NewDojoWorkerORM()
	_, err := workerORM.CreateDojoWorker(walletAddress, chainId)
	_, alreadyExists := db.IsErrUniqueConstraint(err)
	if err != nil {
		if !alreadyExists {
			log.Error().Err(err).Msg("Failed to create worker")
			c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to create worker"))
			return
		}
		log.Warn().Err(err).Msg("Worker already exists")
	}

	metricService := metric.NewMetricService()
	go func() {
		if err := metricService.UpdateDojoWorkerCount(c); err != nil {
			log.Error().Err(err).Msg("Failed to update dojo worker count")
		} else {
			log.Info().Msg("Successfully updated dojo worker count")
		}
	}()

	log.Info().Str("walletAddress", walletAddress).Str("alreadyExists", fmt.Sprintf("%+v", alreadyExists)).Msg("Worker created successfully or already exists")

	c.JSON(http.StatusOK, defaultSuccessResponse(worker.WorkerLoginSuccessResponse{
		Token: token,
	}))
}

// CreateTasksController godoc
//
//	@Summary		Create Tasks
//	@Description	Create tasks by providing the necessary task details along with files to upload. This endpoint accepts multipart/form-data, and multiple files can be uploaded.
//	@Tags			Tasks
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			x-api-key		header		string						true	"API Key for Miner Authentication"
//	@Param			Content-Type	header		string						true	"Content-Type: multipart/form-data"
//	@Param			Title			formData	string						true	"Title of the task"
//	@Param			Body			formData	string						true	"Body of the task"
//	@Param			ExpireAt		formData	string						true	"Expiration date of the task"
//	@Param			TaskData		formData	string						true	"Task data in JSON format"
//	@Param			MaxResults		formData	int							true	"Maximum results"
//	@Param			TotalRewards	formData	float64						true	"Total rewards"
//	@Param			files			formData	[]file						true	"Files to upload (can upload multiple files)"
//	@Success		200				{object}	ApiResponse{body=[]string}	"Tasks created successfully"
//	@Failure		400				{object}	ApiResponse					"Bad request, invalid form data, or failed to process request"
//	@Failure		401				{object}	ApiResponse					"Unauthorized access"
//	@Failure		500				{object}	ApiResponse					"Internal server error, failed to upload files"
//	@Router			/tasks/create [post]
func CreateTasksController(c *gin.Context) {
	log.Info().Msg("Creating Tasks")

	log.Debug().Interface("request body", c.Request.Body).Msg("Creating tasks with request body")

	minerUserInterface, exists := c.Get("minerUser")
	minerUser, _ := minerUserInterface.(*db.MinerUserModel)
	if !exists {
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		c.Abort()
		return
	}

	requestBody, err := task.ProcessRequestBody(c)
	log.Debug().Interface("request body", requestBody).Msg("Request body processed")

	if err != nil {
		log.Error().Err(err).Msg("Failed to process request body")
		c.JSON(http.StatusBadRequest, defaultErrorResponse(err.Error()))
		c.Abort()
		return
	}

	if err := task.ValidateTaskRequest(requestBody); err != nil {
		log.Error().Err(err).Msg("Failed to validate task request")
		c.JSON(http.StatusBadRequest, defaultErrorResponse(err.Error()))
		c.Abort()
		return
	}

	requestBody, err = task.ProcessTaskRequest(requestBody)
	if err != nil {
		log.Error().Err(err).Msg("Failed to process task request")
		c.JSON(http.StatusBadRequest, defaultErrorResponse(err.Error()))
		c.Abort()
		return
	}

	// Here we will handle file upload
	// Parse files from the form
	form, err := c.MultipartForm()
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse multipart form")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid form data"))
		c.Abort()
		return
	}

	files := form.File["file"]
	// Upload files to S3 and update responses with URLs
	requestBody, err = task.ProcessFileUpload(requestBody, files)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upload files")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to upload files"))
		c.Abort()
		return
	}

	taskService := task.NewTaskService()
	tasks, errors := taskService.CreateTasksWithTimeout(requestBody, minerUser.ID, 60*time.Second)

	if len(tasks) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse(errors))
		return
	}

	log.Info().Msg("Tasks created successfully")
	taskIds := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskIds = append(taskIds, task.ID)
	}

	c.JSON(http.StatusOK, defaultSuccessResponse(taskIds))
}

// SubmitTaskResultController godoc
//
//	@Summary		Submit task result
//	@Description	Submit the result of a task
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string											true	"Bearer token"
//	@Param			task-id			path		string											true	"Task ID"
//	@Param			body			body		task.SubmitTaskResultRequest					true	"Request body containing the task result data"
//	@Success		200				{object}	ApiResponse{body=task.SubmitTaskResultResponse}	"Task result submitted successfully"
//	@Failure		400				{object}	ApiResponse										"Invalid request body or task is expired"
//	@Failure		401				{object}	ApiResponse										"Unauthorized"
//	@Failure		404				{object}	ApiResponse										"Task not found"
//	@Failure		409				{object}	ApiResponse										"Task result already completed by worker"
//	@Failure		409				{object}	ApiResponse										"Task has reached max results"
//	@Failure		500				{object}	ApiResponse										"Internal server error"
//	@Router			/tasks/submit-result/{task-id} [put]
func SubmitTaskResultController(c *gin.Context) {
	jwtClaims, ok := c.Get("userInfo")
	if !ok {
		log.Error().Str("userInfo", fmt.Sprintf("%+v", jwtClaims)).Msg("No user info found in context")
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		c.Abort()
		return
	}

	userInfo, ok := jwtClaims.(*jwt.RegisteredClaims)
	if !ok {
		log.Error().Str("userInfo", fmt.Sprintf("%+v", userInfo)).Msg("Failed to assert type for userInfo")
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		c.Abort()
		return
	}
	worker, err := orm.NewDojoWorkerORM().GetDojoWorkerByWalletAddress(userInfo.Subject)
	if err != nil {
		log.Error().Err(err).Str("walletAddress", userInfo.Subject).Msg("Failed to get worker by wallet address")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get worker"))
		c.Abort()
		return
	}

	// Validate the request body for required fields [resultData]
	var requestBody task.SubmitTaskResultRequest
	if err := c.BindJSON(&requestBody); err != nil {
		log.Error().Err(err).Msg("Failed to bind JSON to requestBody")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		c.Abort()
		return
	}

	taskId := c.Param("task-id")
	ctx := c.Request.Context()
	taskService := task.NewTaskService()

	// Fetch the task data
	taskData, err := taskService.GetTaskById(ctx, taskId)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			log.Error().Err(err).Str("taskId", taskId).Msg("Task not found")
			c.JSON(http.StatusNotFound, defaultErrorResponse(err.Error()))
			c.Abort()
			return
		}
		log.Error().Err(err).Str("taskId", taskId).Msg("Error getting Task")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse(err.Error()))
		c.Abort()
		return
	}
	// Check if the task is expired
	if taskData.ExpireAt.Before(time.Now()) || taskData.Status == db.TaskStatusExpired {
		log.Info().Str("taskId", taskId).Msg("Task is expired")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Task is expired"))
		c.Abort()
		return
	}

	// Check if the task has reached max results
	if taskData.MaxResults == taskData.NumResults || taskData.Status == db.TaskStatusCompleted {
		log.Info().Str("taskId", taskId).Msg("Task has reached max results")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Task has reached max results"))
		c.Abort()
		return
	}

	// Check if the task result is already completed by the worker
	isCompletedTResult, err := taskService.ValidateCompletedTResultByWorker(ctx, taskId, worker.ID)
	if err != nil {
		log.Error().Err(err).Str("taskId", taskId).Msg("Error validating completed task result")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse(err.Error()))
		c.Abort()
		return
	}

	if isCompletedTResult {
		log.Info().Str("taskId", taskId).Str("workerId", worker.ID).Msg("Task Result is already completed by worker")
		c.JSON(http.StatusConflict, defaultErrorResponse("Task Result is already completed by worker"))
		c.Abort()
		return
	}

	log.Info().Str("Dojo Worker ID", worker.ID).Str("Task ID", taskId).Msg("Dojo Worker and Task ID pulled")

	// Update the task with the result data
	updatedTask, err := taskService.UpdateTaskResults(ctx, taskData, worker.ID, requestBody.ResultData)
	if err != nil {
		log.Error().Err(err).Str("Dojo Worker ID", worker.ID).Str("Task ID", taskId).Msg("Error updating task with result data")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse(err.Error()))
		c.Abort()
		return
	}

	// Remove from cache
	cache := cache.GetCacheInstance()
	cache.DeleteWithSuffix(cache.Keys.TaskResultByWorker, worker.ID)

	// Update the metric data with goroutine
	handleMetricData(taskData, updatedTask)

	c.JSON(http.StatusOK, defaultSuccessResponse(task.SubmitTaskResultResponse{
		NumResults: updatedTask.NumResults,
	}))
}

// WorkerPartnerCreateController godoc
//
//	@Summary		Create worker-miner partnership
//	@Description	Create a partnership between a worker and a miner
//	@Tags			Worker Partner
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string								true	"Bearer token"
//	@Param			body			body		worker.WorkerPartnerCreateRequest	true	"Request body containing the name and miner subscription key"
//	@Success		200				{object}	ApiResponse{body=string}			"Successfully created worker-miner partnership"
//	@Failure		400				{object}	ApiResponse							"Invalid request body or missing required fields"
//	@Failure		401				{object}	ApiResponse							"Unauthorized"
//	@Failure		404				{object}	ApiResponse							"Miner subscription key is invalid"
//	@Failure		500				{object}	ApiResponse							"Internal server error"
//	@Router			/worker/partner [post]
func WorkerPartnerCreateController(c *gin.Context) {
	jwtClaims, ok := c.Get("userInfo")
	var walletAddress string
	if ok {
		userInfo, ok := jwtClaims.(*jwt.RegisteredClaims)
		if !ok {
			log.Error().Msg("Failed to assert type for userInfo")
			c.AbortWithStatusJSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
			return
		}
		walletAddress = userInfo.Subject
	}

	if walletAddress == "" {
		log.Error().Msg("Missing wallet address, so cannot find worker by wallet address")
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse("Missing wallet address"))
		return
	}

	workerData, err := orm.NewDojoWorkerORM().GetDojoWorkerByWalletAddress(walletAddress)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.AbortWithStatusJSON(http.StatusNotFound, defaultErrorResponse("Worker not found"))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get worker"))
		return
	}

	var requestBody worker.WorkerPartnerCreateRequest
	if err := c.BindJSON(&requestBody); err != nil {
		log.Error().Err(err).Msg("Failed to bind JSON to requestBody")
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		return
	}

	// Continue with your function if there was no error or if the "not found" condition was handled
	foundSubscription, _ := orm.NewSubscriptionKeyORM().GetSubscriptionByKey(requestBody.MinerSubscriptionKey)
	if foundSubscription == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, defaultErrorResponse("Subscription key is invalid"))
		return
	}

	existingPartner, _ := orm.NewWorkerPartnerORM().GetWorkerPartnerByWorkerIdAndSubscriptionKey(workerData.ID, requestBody.MinerSubscriptionKey)
	if existingPartner != nil {
		log.Debug().Interface("existingPartner", existingPartner).Msg("Existing partnership found")
		numRowsChanged, err := orm.NewWorkerPartnerORM().DisablePartnerByWorker(workerData.ID, requestBody.MinerSubscriptionKey, false)
		if numRowsChanged > 0 && err == nil {
			log.Info().Int("numRowsChanged", numRowsChanged).Err(err).Msg("Worker-miner partnership re-enabled")
			c.AbortWithStatusJSON(http.StatusOK, defaultSuccessResponse("Worker-miner partnership re-enabled"))
			return
		}
		log.Error().Int("numRowsChanged", numRowsChanged).Err(err).Msg("Failed to re-enable worker-miner partnership")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to re-enable worker-miner partnership"))
		return
	}

	_, err = orm.NewWorkerPartnerORM().CreateWorkerPartner(workerData.ID, requestBody.MinerSubscriptionKey, requestBody.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to create worker-miner partnership"))
		return
	}

	c.JSON(http.StatusOK, defaultSuccessResponse("Successfully created worker-miner partnership"))
}

// GetWorkerPartnerListController godoc
//
//	@Summary		Get worker-miner partnership list
//	@Description	Retrieve a list of partnerships between a worker and miners
//	@Tags			Worker Partner
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string												true	"Bearer token"
//	@Success		200				{object}	ApiResponse{body=worker.ListWorkerPartnersResponse}	"Successfully retrieved worker-miner partnership list"
//	@Failure		400				{object}	ApiResponse											"Invalid request or missing required fields"
//	@Failure		401				{object}	ApiResponse											"Unauthorized"
//	@Failure		404				{object}	ApiResponse											"Worker not found"
//	@Failure		500				{object}	ApiResponse											"Internal server error"
//	@Router			/worker/partner/list [get]
func GetWorkerPartnerListController(c *gin.Context) {
	jwtClaims, ok := c.Get("userInfo")
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		return
	}

	userInfo, ok := jwtClaims.(*jwt.RegisteredClaims)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		return
	}

	walletAddress := userInfo.Subject
	foundWorker, err := orm.NewDojoWorkerORM().GetDojoWorkerByWalletAddress(walletAddress)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get worker")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get worker"))
		return
	}

	if foundWorker == nil {
		log.Error().Msg("Worker not found")
		c.AbortWithStatusJSON(http.StatusNotFound, defaultErrorResponse("Worker not found"))
		return
	}
	workerPartners, err := orm.NewWorkerPartnerORM().GetWorkerPartnerByWorkerId(foundWorker.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get worker partners")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get worker partners"))
		return
	}

	listWorkerPartnersResponse := &worker.ListWorkerPartnersResponse{
		Partners: make([]worker.WorkerPartner, 0),
	}
	for _, workerPartner := range workerPartners {
		if workerPartner.IsDeleteByWorker {
			continue
		}
		name, _ := workerPartner.Name()
		listWorkerPartnersResponse.Partners = append(listWorkerPartnersResponse.Partners, worker.WorkerPartner{
			Id:              workerPartner.ID,
			CreatedAt:       workerPartner.CreatedAt,
			SubscriptionKey: workerPartner.MinerSubscriptionKey,
			Name:            name,
		})
	}

	c.JSON(http.StatusOK, defaultSuccessResponse(listWorkerPartnersResponse))
}

// GetTaskById godoc
//
//	@Summary		Retrieve task by ID
//	@Description	Get details of a task by its ID
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Param			task-id	path		string								true	"Task ID"
//	@Success		200		{object}	ApiResponse{body=task.TaskResponse}	"Successfully retrieved task response"
//	@Failure		404		{object}	ApiResponse{error=string}			"Task not found"
//	@Failure		500		{object}	ApiResponse{error=string}			"Internal server error"
//	@Router			/tasks/{task-id} [get]
func GetTaskByIdController(c *gin.Context) {
	taskID := c.Param("task-id")
	taskService := task.NewTaskService()

	task, err := taskService.GetTaskResponseById(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Internal server error"))
		c.Abort()
		return
	}

	if task == nil {
		c.JSON(http.StatusNotFound, defaultErrorResponse("Task not found"))
		return
	}

	// Successful response
	c.JSON(http.StatusOK, defaultSuccessResponse(task))
}

// GetTasksByPageController godoc
//
//	@Summary		Retrieve tasks by pagination
//	@Description	Get a paginated list of tasks based on the specified parameters
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string									true	"Bearer token"
//	@Param			task			query		string									true	"Comma-separated list of task types (e.g., CODE_GENERATION,TEXT_TO_IMAGE,DIALOGUE). Use 'All' to include all types."
//	@Param			page			query		int										false	"Page number (default is 1)"
//	@Param			limit			query		int										false	"Number of tasks per page (default is 10)"
//	@Param			sort			query		string									false	"Sort field (default is createdAt)"
//	@Param			order			query		string									false	"Order field (default is desc order) e.g., asc or desc"
//	@Success		200				{object}	ApiResponse{body=task.TaskPagination}	"Successfully retrieved task pagination response"
//	@Failure		400				{object}	ApiResponse								"Invalid request parameters"
//	@Failure		401				{object}	ApiResponse								"Unauthorized"
//	@Failure		404				{object}	ApiResponse								"No tasks found"
//	@Failure		500				{object}	ApiResponse								"Internal server error"
//	@Router			/tasks [get]
func GetTasksByPageController(c *gin.Context) {
	jwtClaims, ok := c.Get("userInfo")
	if !ok {
		log.Error().Str("userInfo", fmt.Sprintf("%+v", jwtClaims)).Msg("No user info found in context")
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		return
	}

	userInfo, ok := jwtClaims.(*jwt.RegisteredClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		return
	}

	worker, err := orm.NewDojoWorkerORM().GetDojoWorkerByWalletAddress(userInfo.Subject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get worker"))
		return
	}

	// Get the task query parameter as a single string
	taskParam := c.Query("task")
	if taskParam == "" {
		c.JSON(http.StatusBadRequest, defaultErrorResponse("task parameter is required"))
		return
	}
	// Split the string into a slice of strings
	taskTypes := strings.Split(taskParam, ",")
	if len(taskTypes) == 0 {
		c.JSON(http.StatusBadRequest, defaultErrorResponse("task parameter is required"))
		return
	}

	if len(taskTypes) == 1 && taskTypes[0] == "All" {
		taskTypes = []string{"CODE_GENERATION", "TEXT_TO_IMAGE", "DIALOGUE", "TEXT_TO_THREE_D"}
	}

	// Parsing "page" and "limit" as integers with default values
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
	sort := c.DefaultQuery("sort", "createdAt")
	orderStr := c.DefaultQuery("order", "desc")
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		log.Error().Err(err).Msg("Error converting page to integer:")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid page parameter"))
		return
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		log.Error().Err(err).Msg("Error converting page to integer:")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid limit parameter"))
		return
	}

	if orderStr != "asc" && orderStr != "desc" {
		log.Error().Msgf("Invalid order parameter: %s", orderStr)
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid order parameter"))
		return
	}

	order := db.SortOrderDesc
	if orderStr == "asc" {
		order = db.SortOrderAsc
	}

	paginationParams := task.PaginationParams{
		Page:  page,
		Limit: limit,
		Sort:  sort,
		Types: taskTypes,
		Order: order,
	}

	// fetching tasks by pagination
	taskService := task.NewTaskService()
	taskPagination, taskErrors := taskService.GetTasksByPagination(c.Request.Context(), worker.ID, paginationParams)
	if len(taskErrors) > 0 {
		isBadRequest := false
		errorDetails := make([]string, 0)
		for _, err := range taskErrors {
			errorDetails = append(errorDetails, err.Error())
			if _, ok := err.(*task.ErrInvalidTaskType); ok {
				isBadRequest = true
			}
		}
		log.Error().Interface("errors", errorDetails).Msg("Error getting tasks by pagination")
		if isBadRequest {
			c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse(errorDetails))
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse(errorDetails))
		return
	}

	if taskPagination == nil {
		log.Error().Err(err).Msg("Error getting tasks by pagination")
		c.JSON(http.StatusNotFound, defaultErrorResponse("no tasks found"))
		return
	}

	// Successful response
	c.JSON(http.StatusOK, defaultSuccessResponse(taskPagination))
}

func GetTaskResultsController(c *gin.Context) {
	taskId := c.Param("task-id")
	if taskId == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse("task id is required"))
		return
	}

	taskResultORM := orm.NewTaskResultORM()
	taskResults, err := taskResultORM.GetTaskResultsByTaskId(c.Request.Context(), taskId)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("failed to fetch task results"))
		return
	}

	var formattedTaskResults []task.TaskResult

	for _, taskResult := range taskResults {
		var resultDataItem []task.Result
		err = json.Unmarshal([]byte(string(taskResult.ResultData)), &resultDataItem)
		if err != nil {
			log.Error().Err(err).Str("taskResult.ResultData", string(taskResult.ResultData)).Msg("failed to convert task results")
			c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("failed to convert result data to tempResult"))
			return
		}

		tempResult := task.TaskResult{
			ResultData:      resultDataItem,
			TaskResultModel: taskResult,
		}
		formattedTaskResults = append(formattedTaskResults, tempResult)
	}

	c.JSON(http.StatusOK, defaultSuccessResponse(task.TaskResultResponse{TaskResults: formattedTaskResults}))
}

// UpdateWorkerPartnerController godoc
//
//	@Summary		Update worker partner details
//	@Description	Update the subscription key and name of a worker partner
//	@Tags			Worker Partner
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string													true	"Bearer token"
//	@Param			body			body		worker.UpdateWorkerPartnerRequest						true	"Request body containing the details to update"
//	@Success		200				{object}	ApiResponse{body=worker.UpdateWorkerPartnerResponse}	"Successfully updated worker partner"
//	@Failure		400				{object}	ApiResponse												"Invalid request body or missing required parameters"
//	@Failure		401				{object}	ApiResponse												"Unauthorized"
//	@Failure		500				{object}	ApiResponse												"Internal server error - failed to update worker partner"
//	@Router			/partner/edit [put]
func UpdateWorkerPartnerController(c *gin.Context) {
	jwtClaims, _ := c.Get("userInfo")

	var requestBody worker.UpdateWorkerPartnerRequest
	if err := c.BindJSON(&requestBody); err != nil {
		log.Error().Err(err).Msg("Failed to bind JSON to requestBody")
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		return
	}

	minerSubscriptionKey := requestBody.MinerSubscriptionKey
	newMinerSubscriptionKey := requestBody.NewMinerSubscriptionKey
	name := requestBody.Name

	userInfo, ok := jwtClaims.(*jwt.RegisteredClaims)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		return
	}
	dojoWorker, err := orm.NewDojoWorkerORM().GetDojoWorkerByWalletAddress(userInfo.Subject)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get worker"))
		return
	}

	workerPartnerORM := orm.NewWorkerPartnerORM()
	if minerSubscriptionKey == "" && newMinerSubscriptionKey == "" && name == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse("Missing required param for update"))
		return
	}
	updatedWorkerPartner, err := workerPartnerORM.UpdateSubscriptionKey(dojoWorker.ID, minerSubscriptionKey, newMinerSubscriptionKey, name)
	if err != nil {
		log.Error().Err(err).Msg("Failed updating subscription key for worker")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse(err.Error()))
		return
	}

	log.Info().Msg("Worker partner updated successfully")
	c.JSON(http.StatusOK, defaultSuccessResponse(worker.UpdateWorkerPartnerResponse{
		WorkerPartner: worker.WorkerPartner{
			Id:              updatedWorkerPartner.ID,
			CreatedAt:       updatedWorkerPartner.CreatedAt,
			SubscriptionKey: updatedWorkerPartner.MinerSubscriptionKey,
			Name:            *updatedWorkerPartner.InnerWorkerPartner.Name,
		},
	}))
}

// DisableMinerByWorkerController godoc
//
//	@Summary		Disable miner by worker
//	@Description	Disable a miner by providing the worker subscription key and a disable flag
//	@Tags			Worker Partner
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string											true	"Bearer token"
//	@Param			body			body		worker.DisableMinerRequest						true	"Request body containing the miner subscription key and disable flag"
//	@Success		200				{object}	ApiResponse{body=worker.DisableSuccessResponse}	"Miner disabled successfully"
//	@Failure		400				{object}	ApiResponse										"Invalid request body or parameters"
//	@Failure		401				{object}	ApiResponse										"Unauthorized"
//	@Failure		404				{object}	ApiResponse										"Failed to disable worker partner, no records updated"
//	@Failure		500				{object}	ApiResponse										"Internal server error - failed to disable worker partner"
//	@Router			/worker/partner/disable [put]
func DisableMinerByWorkerController(c *gin.Context) {
	var requestBody worker.DisableMinerRequest
	if err := c.BindJSON(&requestBody); err != nil {
		log.Error().Err(err).Msg("Failed to bind JSON to requestBody")
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		return
	}

	jwtClaims, _ := c.Get("userInfo")

	userInfo, ok := jwtClaims.(*jwt.RegisteredClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		return
	}
	workerData, err := orm.NewDojoWorkerORM().GetDojoWorkerByWalletAddress(userInfo.Subject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get worker"))
		return
	}

	log.Info().Interface("requestBody", requestBody).Msg("Disabling miner by worker")

	if requestBody.ToDisable {
		count, err := orm.NewWorkerPartnerORM().DisablePartnerByWorker(workerData.ID, requestBody.MinerSubscriptionKey, requestBody.ToDisable)
		if err != nil {
			c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to disable worker partner"))
			return
		}
		if count > 0 {
			c.JSON(http.StatusOK, defaultSuccessResponse(worker.DisableSuccessResponse{Message: "Miner disabled successfully"}))
			return
		}
		c.JSON(http.StatusNotFound, defaultErrorResponse("Failed to disable worker partner, no records updated"))
	} else {
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid request param"))
	}
}

// GenerateNonceController godoc
//
//	@Summary		Generate nonce
//	@Description	Generate a nonce for a given wallet address
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			address	path		string											true	"Wallet address"
//	@Success		200		{object}	ApiResponse{body=worker.GenerateNonceResponse}	"Nonce generated successfully"
//	@Failure		400		{object}	ApiResponse										"Address parameter is required"
//	@Failure		500		{object}	ApiResponse										"Failed to store nonce"
//	@Router			/auth/{address} [get]
func GenerateNonceController(c *gin.Context) {
	address := c.Param("address")
	log.Info().Str("address", address).Msg("Getting address from param")
	if address == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "address parameter is required"})
		return
	}

	cache := cache.GetCacheInstance()
	nonce := siwe.GenerateNonce()
	log.Info().Msgf("Wallet address %s generated nonce %s", address, nonce)
	err := cache.SetWithExpire(address, nonce, 1*time.Minute)
	if err != nil {
		log.Error().Str("address", address).Str("nonce", nonce).Err(err).Msg("Failed to store nonce")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to store nonce"))
		return
	}

	log.Info().Str("address", address).Str("nonce", nonce).Msg("Nonce generated successfully")
	c.JSON(http.StatusOK, defaultSuccessResponse(worker.GenerateNonceResponse{Nonce: nonce}))
}

// GetDojoWorkerCountController godoc
//
//	@Summary		Get the total number of Dojo workers
//	@Description	Retrieves the total number of Dojo workers from the metrics data
//	@Tags			Metrics
//	@Produce		json
//	@Success		200	{object}	ApiResponse{body=metric.DojoWorkerCountResponse}	"Total number of Dojo workers retrieved successfully"
//	@Failure		500	{object}	ApiResponse											"Failed to get worker count or unmarshal data"
//	@Router			/metrics/dojo-worker-count [get]
func GetDojoWorkerCountController(c *gin.Context) {
	metricData, err := orm.NewMetricsORM().GetMetricsDataByMetricType(c, db.MetricsTypeTotalNumDojoWorkers)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get worker count")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get worker count"))
		return
	}
	var workerCountData metric.MetricWorkerCount
	if err = json.Unmarshal([]byte(metricData.MetricsData), &workerCountData); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal worker count data")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to unmarshal worker count data"))
	}

	c.JSON(http.StatusOK, defaultSuccessResponse(metric.DojoWorkerCountResponse{NumDojoWorkers: workerCountData.TotalNumDojoWorkers}))
}

// GetTotalCompletedTasksController godoc
//
//	@Summary		Get the total number of completed tasks
//	@Description	Retrieves the total number of completed tasks from the metrics data
//	@Tags			Metrics
//	@Produce		json
//	@Success		200	{object}	ApiResponse{body=metric.CompletedTaskCountResponse}	"Total number of completed tasks retrieved successfully"
//	@Failure		500	{object}	ApiResponse											"Failed to get completed tasks count or unmarshal data"
//	@Router			/metrics/completed-tasks-count [get]
func GetTotalCompletedTasksController(c *gin.Context) {
	metricData, err := orm.NewMetricsORM().GetMetricsDataByMetricType(c, db.MetricsTypeTotalNumCompletedTasks)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get completed tasks count")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get completed tasks count"))
		return
	}

	var completedTasksData metric.MetricCompletedTasksCount
	if err = json.Unmarshal([]byte(metricData.MetricsData), &completedTasksData); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal completed tasks data")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to unmarshal completed tasks data"))
		return
	}

	c.JSON(http.StatusOK, defaultSuccessResponse(metric.CompletedTaskCountResponse{NumCompletedTasks: completedTasksData.TotalNumCompletedTasks}))
}

// GetTotalTasksResultsController godoc
//
//	@Summary		Get the total number of task results
//	@Description	Retrieves the total number of task results from the metrics data
//	@Tags			Metrics
//	@Produce		json
//	@Success		200	{object}	ApiResponse{body=metric.TaskResultCountResponse}	"Total number of task results retrieved successfully"
//	@Failure		500	{object}	ApiResponse											"Failed to get task results count or unmarshal data"
//	@Router			/metrics/task-result-count [get]
func GetTotalTasksResultsController(c *gin.Context) {
	metricData, err := orm.NewMetricsORM().GetMetricsDataByMetricType(c, db.MetricsTypeTotalNumTaskResults)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get task results count")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get task results count"))
		return
	}

	var totalTasksResults metric.MetricTaskResultsCount
	if err = json.Unmarshal([]byte(metricData.MetricsData), &totalTasksResults); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal task results data")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to unmarshal task results data"))
		return
	}

	c.JSON(http.StatusOK, defaultSuccessResponse(metric.TaskResultCountResponse{NumTaskResults: totalTasksResults.TotalNumTasksResults}))
}

// GetAvgTaskCompletionTimeController godoc
//
//	@Summary		Get the average task completion time
//	@Description	Retrieves the average task completion time from the metrics data
//	@Tags			Metrics
//	@Produce		json
//	@Success		200	{object}	ApiResponse{body=metric.AvgTaskCompletionTimeResponse}	"Average task completion time retrieved successfully"
//	@Failure		500	{object}	ApiResponse												"Failed to get average task completion time or unmarshal data"
//	@Router			/metrics/average-task-completion-time [get]
func GetAvgTaskCompletionTimeController(c *gin.Context) {
	metricData, err := orm.NewMetricsORM().GetMetricsDataByMetricType(c, db.MetricsTypeAverageTaskCompletionTime)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get average task completion time")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get average task completion time"))
		return
	}

	var avgCompletionTime metric.MetricAvgTaskCompletionTime
	if err = json.Unmarshal([]byte(metricData.MetricsData), &avgCompletionTime); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal average task completion time data")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to unmarshal average task completion time data"))
		return
	}

	c.JSON(http.StatusOK, defaultSuccessResponse(metric.AvgTaskCompletionTimeResponse{AvgTaskCompletionTime: avgCompletionTime.AverageTaskCompletionTime}))
}

// GenerateCookieAuth godoc
//
//	@Summary		Generates a session given valid proof of ownership
//
//	@Description	Generates cookies that can be used to authenticate a user, given a valid signature, message for a specific hotkey
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//
//	@Param			body	body		auth.GenerateCookieAuthRequest					true	"Request body containing the hotkey, signature, and message"
//	@Success		200		{object}	ApiResponse{body=task.SubmitTaskResultResponse}	"Task result submitted successfully"
//
//	@Param			address	path		string											true	"Wallet address"
//	@Success		200		{object}	ApiResponse{body=worker.GenerateNonceResponse}	"Nonce generated successfully"
//	@Failure		400		{object}	ApiResponse										"Invalid request body"
//	@Failure		401		{object}	ApiResponse										"Unauthorized"
//	@Failure		500		{object}	ApiResponse										"Error verifying signature"
//	@Failure		500		{object}	ApiResponse										"Failed to generate session"
//	@Router			/api/v1/auth/{address} [get]
func GenerateCookieAuth(c *gin.Context) {
	var requestBody auth.GenerateCookieAuthRequest
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		var missingFields []string
		if requestBody.Hotkey == "" {
			missingFields = append(missingFields, "hotkey")
		}
		if requestBody.Signature == "" {
			missingFields = append(missingFields, "signature")
		}
		if requestBody.Message == "" {
			missingFields = append(missingFields, "message")
		}
		errorMessage := "Invalid request body"
		if len(missingFields) > 0 {
			errorMessage += ": missing or invalid fields - " + strings.Join(missingFields, ", ")
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse(errorMessage))
		return
	}

	isVerified, err := siws.SS58VerifySignature(requestBody.Message, requestBody.Hotkey, requestBody.Signature)
	if err != nil {
		log.Error().Err(err).Msg("Error verifying signature")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Error verifying signature"))
		return
	}

	if !isVerified {
		log.Error().Err(err).Msg("Verification result is false")
		c.AbortWithStatusJSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		return
	}

	// successfully authorized, now generate a session for them to use
	cache := cache.GetCacheInstance()
	hashKey := securecookie.GenerateRandomKey(64)
	blockKey := securecookie.GenerateRandomKey(32)
	s := securecookie.New(hashKey, blockKey)
	sessionID := uuid.New().String()
	cookieData := auth.CookieData{SessionId: sessionID, Hotkey: requestBody.Hotkey}

	if encoded, err := s.Encode(auth.CookieName, cookieData); err == nil {
		redisData := auth.SecureCookieSession{
			HashKey:  hashKey,
			BlockKey: blockKey,
			CookieData: auth.CookieData{
				SessionId: sessionID,
				Hotkey:    requestBody.Hotkey,
			},
		}

		jsonData, err := json.Marshal(redisData)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal auth data...")
			c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to generate session"))
			return
		}

		expirationTime := 5 * time.Minute
		// migrating from rueidis to go-redis

		if _, err := cache.Redis.Set(context.Background(), encoded, jsonData, expirationTime).Result(); err != nil {
			log.Error().Err(err).Msg("Failed to store session in redis")
			c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to generate session"))
			return
		}

		cookie := &http.Cookie{
			Name:     auth.CookieName,
			Value:    encoded,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			Expires:  time.Now().Add(expirationTime),
		}
		http.SetCookie(c.Writer, cookie)
		log.Info().Msgf("Session generated successfully for hotkey %v", requestBody.Hotkey)
		minerUser, err := orm.NewMinerUserORM().CreateNewMiner(requestBody.Hotkey)
		_, alreadyExists := db.IsErrUniqueConstraint(err)
		if err != nil {
			if alreadyExists {
				log.Info().Msg("Miner already exists, skipping creation")
				c.JSON(http.StatusOK, defaultSuccessResponse("Session generated successfully"))
				return
			}
			log.Error().Err(err).Msg("Failed to create miner")
			c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to generate session"))
			return
		}
		log.Info().Msgf("Successfully created new miner, id: %v", minerUser.ID)
		c.JSON(http.StatusOK, defaultSuccessResponse("Session generated successfully"))
		return
	} else {
		log.Error().Err(err).Msg("Failed to encode cookie")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to generate session"))
		return
	}
}

// MinerApiKeyListController godoc
//
//	@Summary		Retrieve API keys for a miner
//	@Description	Get a list of API keys associated with the miner's hotkey
//	@Tags			Miner
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string											true	"Bearer token"
//	@Success		200				{object}	ApiResponse{body=miner.MinerApiKeysResponse}	"Successfully retrieved API keys"
//	@Failure		401				{object}	ApiResponse										"Unauthorized"
//	@Failure		500				{object}	ApiResponse										"Internal server error"
//	@Router			/miner/api-keys [get]

func MinerApiKeyListController(c *gin.Context) {
	session, err := handleCurrentSession(c)
	if err != nil {
		log.Error().Err(err).Msg("Failed to authenticate current session")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Unauthorized"))
		return
	}

	apiKeys, err := orm.NewApiKeyORM().GetApiKeysByMinerHotkey(session.Hotkey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get api keys by miner hotkey")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get api keys"))
		return
	}

	response := buildApiKeyResponse(apiKeys)
	log.Info().Msgf("%d API Keys retrieved successfully for hotkey %s", len(apiKeys), session.Hotkey)
	c.JSON(http.StatusOK, defaultSuccessResponse(response))
}

// MinerApiKeyGenerateController godoc
//
//	@Summary		Generate a new API key for a miner
//	@Description	Generate a new API key associated with the miner's hotkey
//	@Tags			Miner
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string											true	"Bearer token"
//	@Success		200				{object}	ApiResponse{body=miner.MinerApiKeysResponse}	"Successfully generated API key"
//	@Failure		401				{object}	ApiResponse										"Unauthorized"
//	@Failure		500				{object}	ApiResponse										"Internal server error"
//	@Router			/miner/api-keys [post]
func MinerApiKeyGenerateController(c *gin.Context) {
	session, err := handleCurrentSession(c)
	if err != nil {
		log.Error().Err(err).Msg("Failed to authenticate current session")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Unauthorized"))
		return
	}

	apiKey, _, err := generateRandomApiKey()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate random api key")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to generate api key"))
		return
	}

	createdApiKey, err := orm.NewApiKeyORM().CreateApiKeyByHotkey(session.Hotkey, apiKey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create api key")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to create api key"))
		return
	}
	log.Info().Msgf("API Key %s generated successfully", createdApiKey.Key)

	apiKeys, err := orm.NewApiKeyORM().GetApiKeysByMinerHotkey(session.Hotkey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get api keys by miner hotkey")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get api keys"))
		return
	}
	response := buildApiKeyResponse(apiKeys)
	c.JSON(http.StatusOK, defaultSuccessResponse(response))
}

// MinerApiKeyDisableController godoc
//
//	@Summary		Disable an API key for a miner
//	@Description	Disable a specific API key associated with the miner's hotkey
//	@Tags			Miner
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string											true	"Bearer token"
//	@Param			body			body		miner.MinerApiKeyDisableRequest					true	"Disable API key request body"
//	@Success		200				{object}	ApiResponse{body=miner.MinerApiKeysResponse}	"Successfully disabled API key"
//	@Failure		400				{object}	ApiResponse										"Invalid request body"
//	@Failure		401				{object}	ApiResponse										"Unauthorized"
//	@Failure		404				{object}	ApiResponse										"API key not found"
//	@Failure		500				{object}	ApiResponse										"Internal server error"
//	@Router			/miner/api-keys/disable [put]
func MinerApiKeyDisableController(c *gin.Context) {
	session, err := handleCurrentSession(c)
	if err != nil {
		log.Error().Err(err).Msg("Failed to authenticate current session")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Unauthorized"))
		return
	}

	var request miner.MinerApiKeyDisableRequest
	if err := c.BindJSON(&request); err != nil {
		log.Error().Err(err).Msg("Failed to bind JSON to request")
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		return
	}

	apiKeys, err := orm.NewApiKeyORM().GetApiKeysByMinerHotkey(session.Hotkey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get api keys by miner hotkey")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get api keys"))
		return
	}

	var foundApiKey *db.APIKeyModel
	for _, key := range apiKeys {
		if key.Key == request.ApiKey {
			foundApiKey = &key
			break
		}
	}

	if foundApiKey == nil {
		log.Error().Msg("API Key belonging to miner not found")
		c.AbortWithStatusJSON(http.StatusNotFound, defaultErrorResponse("API Key belonging to miner not found"))
		return
	}

	disabledKey, err := orm.NewApiKeyORM().DisableApiKeyByHotkey(session.Hotkey, request.ApiKey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to disable api key")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to disable api key"))
		return
	}
	log.Info().Msgf("API Key %s disabled successfully", disabledKey.Key)

	// create new array without request.ApiKey
	updatedApiKeys := make([]string, 0)
	for _, key := range apiKeys {
		if key.Key != request.ApiKey && !key.IsDelete {
			updatedApiKeys = append(updatedApiKeys, key.Key)
		}
	}
	c.JSON(http.StatusOK, defaultSuccessResponse(miner.MinerApiKeysResponse{ApiKeys: updatedApiKeys}))
}

// MinerSubscriptionKeyListController godoc
//
//	@Summary		Retrieve subscription keys for a miner
//	@Description	Get a list of subscription keys associated with the miner's hotkey
//	@Tags			Miner
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string													true	"Bearer token"
//	@Success		200				{object}	ApiResponse{body=miner.MinerSubscriptionKeysResponse}	"Successfully retrieved subscription keys"
//	@Failure		401				{object}	ApiResponse												"Unauthorized"
//	@Failure		500				{object}	ApiResponse												"Internal server error"
//	@Router			/miner/subscription-keys [get]
func MinerSubscriptionKeyListController(c *gin.Context) {
	session, err := handleCurrentSession(c)
	if err != nil {
		log.Error().Err(err).Msg("Failed to authenticate current session")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Unauthorized"))
		return
	}

	subscriptionKeys, err := orm.NewSubscriptionKeyORM().GetSubscriptionKeysByMinerHotkey(session.Hotkey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get subscription keys by miner hotkey")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get subscription keys"))
		return
	}

	response := buildSubscriptionKeyResponse(subscriptionKeys)
	log.Info().Msgf("%d Subscription Keys retrieved successfully for hotkey %s", len(subscriptionKeys), session.Hotkey)
	c.JSON(http.StatusOK, defaultSuccessResponse(response))
}

// MinerSubscriptionKeyGenerateController godoc
//
//	@Summary		Generate a new subscription key for a miner
//	@Description	Generate a new subscription key associated with the miner's hotkey
//	@Tags			Miner
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string													true	"Bearer token"
//	@Success		200				{object}	ApiResponse{body=miner.MinerSubscriptionKeysResponse}	"Successfully generated subscription key"
//	@Failure		401				{object}	ApiResponse												"Unauthorized"
//	@Failure		500				{object}	ApiResponse												"Internal server error"
//	@Router			/miner/subscription-keys [post]
func MinerSubscriptionKeyGenerateController(c *gin.Context) {
	session, err := handleCurrentSession(c)
	if err != nil {
		log.Error().Err(err).Msg("Failed to authenticate current session")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Unauthorized"))
		return
	}

	subscriptionKey, err := utils.GenerateRandomMinerSubscriptionKey()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate random subscriptionKey key")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to generate subscriptionKey key"))
		return
	}

	createdSubscriptionKey, err := orm.NewSubscriptionKeyORM().CreateSubscriptionKeyByHotkey(session.Hotkey, subscriptionKey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create subscription key")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to create subscription key"))
		return
	}
	log.Info().Msgf("Subscription Key %s generated successfully", createdSubscriptionKey.Key)

	// Reset cache for both subscription key caches
	cache := cache.GetCacheInstance()
	cacheKeyByHotkey := cache.BuildCacheKey(cache.Keys.SubByHotkey, session.Hotkey)
	cacheKeyByKey := cache.BuildCacheKey(cache.Keys.SubByKey, subscriptionKey)

	if err := cache.Delete(cacheKeyByHotkey); err != nil {
		log.Error().Err(err).Msg("Failed to delete hotkey subscription cache")
	}
	if err := cache.Delete(cacheKeyByKey); err != nil {
		log.Error().Err(err).Msg("Failed to delete subscription key cache")
	}

	subscriptionKeys, err := orm.NewSubscriptionKeyORM().GetSubscriptionKeysByMinerHotkey(session.Hotkey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get api keys by miner hotkey")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get api keys"))
		return
	}
	response := buildSubscriptionKeyResponse(subscriptionKeys)
	c.JSON(http.StatusOK, defaultSuccessResponse(response))
}

// MinerSubscriptionKeyDisableController godoc
//
//	@Summary		Disable a subscription key for a miner
//	@Description	Disable a specific subscription key associated with the miner's hotkey
//	@Tags			Miner
//	@Accept			json
//	@Produce		json
//	@Param			Authorization	header		string													true	"Bearer token"
//	@Param			body			body		miner.MinerSubscriptionDisableRequest					true	"Disable subscription key request body"
//	@Success		200				{object}	ApiResponse{body=miner.MinerSubscriptionKeysResponse}	"Successfully disabled subscription key"
//	@Failure		400				{object}	ApiResponse												"Invalid request body"
//	@Failure		401				{object}	ApiResponse												"Unauthorized"
//	@Failure		404				{object}	ApiResponse												"Subscription key not found"
//	@Failure		500				{object}	ApiResponse												"Internal server error"
//	@Router			/miner/subscription-keys/disable [put]
func MinerSubscriptionKeyDisableController(c *gin.Context) {
	session, err := handleCurrentSession(c)
	if err != nil {
		log.Error().Err(err).Msg("Failed to authenticate current session")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Unauthorized"))
		return
	}

	var request miner.MinerSubscriptionDisableRequest
	if err := c.BindJSON(&request); err != nil {
		log.Error().Err(err).Msg("Failed to bind JSON to request")
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		return
	}

	subscriptionKeys, err := orm.NewSubscriptionKeyORM().GetSubscriptionKeysByMinerHotkey(session.Hotkey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get api keys by miner hotkey")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get api keys"))
		return
	}

	var foundApiKey *db.SubscriptionKeyModel
	for _, key := range subscriptionKeys {
		if key.Key == request.SubscriptionKey {
			foundApiKey = &key
			break
		}
	}

	if foundApiKey == nil {
		log.Error().Msg("subscription Key belonging to miner not found")
		c.AbortWithStatusJSON(http.StatusNotFound, defaultErrorResponse("subscription Key belonging to miner not found"))
		return
	}

	disabledKey, err := orm.NewSubscriptionKeyORM().DisableSubscriptionKeyByHotkey(session.Hotkey, request.SubscriptionKey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to disable subscription key")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to disable subscription key"))
		return
	}
	log.Info().Msgf("Subscription Key %s disabled successfully", disabledKey.Key)

	// Reset cache for both subscription key caches
	cache := cache.GetCacheInstance()
	cacheKeyByHotkey := cache.BuildCacheKey(cache.Keys.SubByHotkey, session.Hotkey)
	cacheKeyByKey := cache.BuildCacheKey(cache.Keys.SubByKey, request.SubscriptionKey)

	if err := cache.Delete(cacheKeyByHotkey); err != nil {
		log.Error().Err(err).Msg("Failed to delete hotkey subscription cache")
	}
	if err := cache.Delete(cacheKeyByKey); err != nil {
		log.Error().Err(err).Msg("Failed to delete subscription key cache")
	}

	newSubscriptionKeys, err := orm.NewSubscriptionKeyORM().GetSubscriptionKeysByMinerHotkey(session.Hotkey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get new subscription keys by miner hotkey")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get new subscription keys"))
		return
	}

	updatedSubscriptionKeys := make([]string, 0)
	for _, key := range newSubscriptionKeys {
		if key.Key != request.SubscriptionKey && !key.IsDelete {
			updatedSubscriptionKeys = append(updatedSubscriptionKeys, key.Key)
		}
	}

	c.JSON(http.StatusOK, defaultSuccessResponse(miner.MinerSubscriptionKeysResponse{SubscriptionKeys: updatedSubscriptionKeys}))
}

// GetNextInProgressTaskController handles GET request to fetch the next in-progress task by task ID.
//
//	@Summary		Get next in-progress task by task ID
//	@Description	Fetch the next in-progress task by providing the task ID
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Param			task-id	path		string									true	"Task ID"
//	@Success		200		{object}	ApiResponse{body=task.NextTaskResponse}	"Successful operation"
//	@Failure		400		{object}	ApiResponse								"Invalid request, task id is required"
//	@Failure		401		{object}	ApiResponse								"Unauthorized"
//	@Failure		500		{object}	ApiResponse								"Failed to get next in-progress task"
//	@Router			/next-in-progress-task/{task-id} [get]
func GetNextInProgressTaskController(c *gin.Context) {
	// session, err := handleCurrentSession(c)
	jwtClaims, ok := c.Get("userInfo")
	if !ok {
		log.Error().Str("userInfo", fmt.Sprintf("%+v", jwtClaims)).Msg("No user info found in context")
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		return
	}

	userInfo, ok := jwtClaims.(*jwt.RegisteredClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		return
	}

	worker, err := orm.NewDojoWorkerORM().GetDojoWorkerByWalletAddress(userInfo.Subject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get worker"))
		return
	}

	taskId := c.Param("task-id")
	if taskId == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse("task id is required"))
		return
	}
	taskData, err := orm.NewTaskORM().GetNextInProgressTask(c, taskId, worker.ID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			log.Info().Msg("No in progress tasks found")
			c.JSON(http.StatusOK, defaultSuccessResponse(task.NextTaskResponse{NextInProgressTaskId: ""}))
			return
		}
		log.Error().Err(err).Msg("Failed to get next in-progress task")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get next in-progress task"))
		return
	}

	if taskData == nil {
		log.Info().Msg("No in progress tasks found")
		c.JSON(http.StatusOK, defaultSuccessResponse(task.NextTaskResponse{NextInProgressTaskId: ""}))
		return
	}

	c.JSON(http.StatusOK, defaultSuccessResponse(task.NextTaskResponse{NextInProgressTaskId: taskData.ID}))
}
