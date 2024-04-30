package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dojo-api/db"
	"dojo-api/pkg/orm"
	"dojo-api/pkg/task"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
)

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
	log.Info().Str("walletAddress", walletAddress).Str("alreadyExists", fmt.Sprintf("%+v", alreadyExists)).Msg("Worker created successfully or already exists")
	c.JSON(http.StatusOK, defaultSuccessResponse(token))
}

func CreateTaskController(c *gin.Context) {
	minerUserId, exists := c.Get("minerUserID")
	if !exists {
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		c.Abort()
		return
	}

	var requestBody task.CreateTaskRequest
	if err := c.BindJSON(&requestBody); err != nil {
		log.Error().Err(err).Msg("Invalid request body")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		c.Abort()
		return
	}

	err := task.ValidateTaskRequest(requestBody)
	if err != nil {
		log.Error().Err(err).Msg("Failed to validate task request")
		c.JSON(http.StatusBadRequest, defaultErrorResponse(err.Error()))
		c.Abort()
		return
	}

	taskService := task.NewTaskService()
	taskIds, err := taskService.CreateTasks(requestBody, minerUserId.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, defaultErrorResponse(err.Error()))
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, defaultSuccessResponse(taskIds))
}

func SubmitTaskResultController(c *gin.Context) {
	// TODO possibly refactor after merging with oolwin's MR
	jwtClaims, ok := c.Get("userInfo")
	if !ok {
		log.Error().Str("userInfo", fmt.Sprintf("%+v", jwtClaims)).Msg("No user info found in context")
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		return
	}

	userInfo, ok := jwtClaims.(*jwt.RegisteredClaims)
	if !ok {
		log.Error().Str("userInfo", fmt.Sprintf("%+v", userInfo)).Msg("Failed to assert type for userInfo")
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		return
	}
	worker, err := orm.NewDojoWorkerORM().GetDojoWorkerByWalletAddress(userInfo.Subject)
	if err != nil {
		log.Error().Err(err).Str("walletAddress", userInfo.Subject).Msg("Failed to get worker by wallet address")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get worker"))
		return
	}

	var requestBody task.SubmitTaskResultRequest
	if err := c.BindJSON(&requestBody); err != nil {
		log.Error().Err(err).Msg("Failed to bind JSON to requestBody")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		return
	}

	// Validate the request body for required fields [resultData]
	taskId := c.Param("task-id")
	ctx := c.Request.Context()
	taskService := task.NewTaskService()
	task, err := taskService.GetTaskById(ctx, taskId)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			log.Error().Err(err).Str("taskId", taskId).Msg("Task not found")
			c.JSON(http.StatusInternalServerError, defaultErrorResponse(err.Error()))
			c.Abort()
			return
		}
		log.Error().Err(err).Str("taskId", taskId).Msg("Error getting Task")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse(err.Error()))
		c.Abort()
		return
	}
	log.Info().Str("Dojo Worker ID", worker.ID).Str("Task ID", taskId).Msg("Dojo Worker and Task ID pulled")

	// Update the task with the result data
	updatedTask, err := taskService.UpdateTaskResults(ctx, task, worker.ID, requestBody.ResultData)
	if err != nil {
		log.Error().Err(err).Str("Dojo Worker ID", worker.ID).Str("Task ID", taskId).Msg("Error updating task with result data")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, defaultSuccessResponse(map[string]interface{}{
		"numResults": updatedTask.NumResults,
	}))
}

func MinerLoginController(c *gin.Context) {
	verified, _ := c.Get("verified")
	hotkey, _ := c.Get("hotkey")
	coldkey, _ := c.Get("coldkey")
	apiKey, _ := c.Get("apiKey")
	expiry, _ := c.Get("expiry")

	minerUserORM := orm.NewMinerUserORM()
	_, err := minerUserORM.CreateUser(coldkey.(string), hotkey.(string), apiKey.(string), expiry.(time.Time), verified.(bool))
	if err != nil {
		log.Error().Err(err).Msg("Failed to save network user")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to save network user because miner's hot key may already exists"))
		return
	}

	if verified.(bool) {
		c.JSON(http.StatusOK, defaultSuccessResponse(apiKey))
	} else {
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Miner user not verified"))
	}
}

func MinerController(c *gin.Context) {
	apiKey := c.Request.Header.Get("X-API-KEY")

	minerUserORM := orm.NewMinerUserORM()
	minerUser, _ := minerUserORM.GetUserByAPIKey(apiKey)
	if minerUser == nil {
		c.JSON(http.StatusNotFound, defaultErrorResponse("miner not found"))
		return
	}

	log.Info().Str("minerUser", fmt.Sprintf("%+v", minerUser)).Msg("Miner user found")
	c.JSON(http.StatusOK, defaultSuccessResponse(map[string]string{
		"minerId": minerUser.ID,
	}))
}

func WorkerPartnerController(c *gin.Context) {
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

	var requestBody struct {
		MinerId string `json:"minerId"`
	}

	if err := c.BindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		return
	}

	_, err = orm.NewWorkerPartnerORM().Create(worker.ID, requestBody.MinerId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, defaultErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, defaultSuccessResponse("successfully created worker-miner partnership"))
}

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

func GetTasksByPageController(c *gin.Context) {

	// Get the task query parameter as a single string
	taskParam := c.Query("task")
	// Split the string into a slice of strings
	taskTypes := strings.Split(taskParam, ",")

	// Parsing "page" and "limit" as integers with default values
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
	sort := c.DefaultQuery("sort", "createdAt")

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

	// fetching tasks by pagination
	taskService := task.NewTaskService()
	taskPagination, err := taskService.GetTasksByPagination(c.Request.Context(), page, limit, taskTypes, sort)
	if err != nil {
		log.Error().Err(err).Msg("Error getting tasks by pagination")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse(err.Error()))
		return
	}

	if taskPagination == nil {
		log.Error().Err(err).Msg("Error getting tasks by pagination")
		c.JSON(http.StatusNotFound, defaultErrorResponse("no tasks found"))
		return
	}

	// Successful response
	c.JSON(http.StatusOK, defaultSuccessResponse(map[string]interface{}{
		"tasks": taskPagination,
	}))
}
