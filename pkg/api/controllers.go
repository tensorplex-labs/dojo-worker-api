package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dojo-api/db"
	"dojo-api/pkg/email"
	"dojo-api/pkg/orm"
	"dojo-api/pkg/task"
	"dojo-api/pkg/worker"
	"dojo-api/utils"

	"github.com/spruceid/siwe-go"

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
	c.JSON(http.StatusOK, defaultSuccessResponse(map[string]interface{}{
		"token": token,
	}))
}

func CreateTasksController(c *gin.Context) {
	log.Info().Msg("Creating Tasks")

	minerUserInterface, exists := c.Get("minerUser")
	minerUser, _ := minerUserInterface.(*db.MinerUserModel)
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

	if err := task.ValidateTaskRequest(requestBody); err != nil {
		log.Error().Err(err).Msg("Failed to validate task request")
		c.JSON(http.StatusBadRequest, defaultErrorResponse(err.Error()))
		c.Abort()
		return
	}

	requestBody, err := task.ProcessTaskRequest(requestBody)
	if err != nil {
		log.Error().Err(err).Msg("Failed to process task request")
		c.JSON(http.StatusBadRequest, defaultErrorResponse(err.Error()))
		c.Abort()
		return
	}

	log.Info().Str("minerUser", fmt.Sprintf("%+v", minerUser)).Msg("Miner user found")

	taskService := task.NewTaskService()
	tasks, errors := taskService.CreateTasks(requestBody, minerUser.ID)

	log.Info().Interface("tasks", tasks).Msg("Tasks created successfully")
	if len(tasks) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse(errors))
		return
	}

	taskIds := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskIds = append(taskIds, task.ID)
	}

	c.JSON(http.StatusOK, &ApiResponse{
		Success: true,
		Body:    taskIds,
		Error:   errors,
	})
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

// func MinerLoginController(c *gin.Context) {
// 	verified, _ := c.Get("verified")
// 	hotkey, _ := c.Get("hotkey")
// 	apiKey, _ := c.Get("apiKey")
// 	expiry, _ := c.Get("expiry")
// 	email, _ := c.Get("email")
// 	organisation, organisationExists := c.Get("organisationName")

// 	minerUserORM := orm.NewMinerUserORM()
// 	var err error
// 	if organisationExists {
// 		_, err = minerUserORM.CreateUserWithOrganisation(hotkey.(string), apiKey.(string), expiry.(time.Time), verified.(bool), email.(string), organisation.(string))
// 	} else {
// 		_, err = minerUserORM.CreateUser(hotkey.(string), apiKey.(string), expiry.(time.Time), verified.(bool), email.(string))
// 	}

// 	if err != nil {
// 		log.Error().Err(err).Msg("Failed to save miner user")
// 		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to save miner user because miner's hot key may already exists"))
// 		return
// 	}

// 	if verified.(bool) {
// 		c.JSON(http.StatusOK, defaultSuccessResponse(apiKey))
// 	} else {
// 		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Miner user not verified"))
// 	}
// }

func MinerApplicationController(c *gin.Context) {
	requestInterface, exists := c.Get("requestMap")
	if !exists {
		log.Error().Msg("Request map not found in context")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Request map not found in context"))
		c.Abort()
		return
	}

	requestMap, ok := requestInterface.(map[string]string)
	if !ok {
		log.Error().Msg("Invalid request body")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		c.Abort()
		return
	}

	apiKey, expiry, err := generateRandomApiKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to generate API key"))
		return
	}

	minerUserORM := orm.NewMinerUserORM()
	organisation, organisationExists := requestMap["organisationName"]
	subscriptionKey, err := utils.GenerateRandomMinerSubscriptionKey()
	if subscriptionKey == "" {
		log.Error().Err(err).Msg("Failed to generate subscription key")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to generate subscription key"))
		return
	}

	if organisationExists {
		if _, err = minerUserORM.CreateUserWithOrganisation(requestMap["hotkey"], apiKey, expiry, true, requestMap["email"], subscriptionKey, organisation); err != nil {
			c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to save miner user"))
			return
		}
	} else {
		if _, err = minerUserORM.CreateUser(requestMap["hotkey"], apiKey, expiry, false, requestMap["email"], subscriptionKey); err != nil {
			c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to save miner user"))
			return
		}
	}

	person := map[bool]string{true: requestMap["organisationName"], false: "User"}[organisationExists]
	body := fmt.Sprintf("Hi %s,\nHere are your api key and subscription keys \nAPI Key: %s\nSubscription Key: %s", person, apiKey, subscriptionKey)
	err = email.SendEmail(requestMap["email"], body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to send email"))
		return
	}

}

func MinerInfoController(c *gin.Context) {
	minerUserInterface, ok := c.Get("minerUser")
	if !ok {
		log.Error().Msg("Miner user not found in context")
		c.AbortWithStatusJSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		return
	}
	minerUser := minerUserInterface.(*db.MinerUserModel)
	c.JSON(http.StatusOK, defaultSuccessResponse(map[string]string{
		"minerId":         minerUser.ID,
		"subscriptionKey": minerUser.SubscriptionKey,
	}))
}

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
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Missing wallet address"))
		return
	}

	worker, err := orm.NewDojoWorkerORM().GetDojoWorkerByWalletAddress(walletAddress)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get worker"))
		return
	}

	var requestMap map[string]string
	if err := c.BindJSON(&requestMap); err != nil {
		log.Error().Err(err).Msg("Failed to bind JSON to requestMap")
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		return
	}

	name, ok := requestMap["name"]
	if !ok {
		log.Error().Msg("Missing Miner Name")
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse("Missing Miner Name"))
		return
	}

	minerSubscriptionKey, ok := requestMap["minerSubscriptionKey"]
	if !ok {
		log.Error().Msg("Missing minerSubscriptionKey")
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse("Missing minerSubscriptionKey"))
		return
	}

	// Continue with your function if there was no error or if the "not found" condition was handled
	foundMinerUser, _ := orm.NewMinerUserORM().GetUserBySubscriptionKey(minerSubscriptionKey)
	if foundMinerUser == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, defaultErrorResponse("Miner subscription key is invalid"))
		return
	}

	existingPartner, _ := orm.NewWorkerPartnerORM().GetWorkerPartnerByWorkerIdAndSubscriptionKey(worker.ID, minerSubscriptionKey)
	if existingPartner != nil {
		log.Debug().Interface("existingPartner", existingPartner).Msg("Existing partnership found")
		numRowsChanged, err := orm.NewWorkerPartnerORM().DisablePartnerByWorker(worker.ID, minerSubscriptionKey, false)
		if numRowsChanged > 0 && err == nil {
			log.Info().Int("numRowsChanged", numRowsChanged).Err(err).Msg("Worker-miner partnership re-enabled")
			c.AbortWithStatusJSON(http.StatusOK, defaultSuccessResponse("Worker-miner partnership re-enabled"))
			return
		}
		log.Error().Int("numRowsChanged", numRowsChanged).Err(err).Msg("Failed to re-enable worker-miner partnership")
		c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to re-enable worker-miner partnership"))
		return
	}

	_, err = orm.NewWorkerPartnerORM().Create(worker.ID, foundMinerUser.ID, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to create worker-miner partnership"))
		return
	}

	c.JSON(http.StatusOK, defaultSuccessResponse("Successfully created worker-miner partnership"))
}

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

func GetTaskByIdController(c *gin.Context) {
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

	taskID := c.Param("task-id")
	taskService := task.NewTaskService()
	task, err := taskService.GetTaskResponseById(c.Request.Context(), taskID, worker.ID)
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
		taskTypes = []string{"CODE_GENERATION", "TEXT_TO_IMAGE", "DIALOGUE"}
	}

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
	taskPagination, taskErrors := taskService.GetTasksByPagination(c.Request.Context(), worker.ID, page, limit, taskTypes, sort)
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

	// embed TaskResultModel to reuse its fields
	// override ResultData, also will shadow the original "result_data" JSON field
	type taskResultResponse struct {
		db.TaskResultModel
		ResultData []task.Result `json:"result_data"`
	}
	var formattedTaskResults []taskResultResponse

	for _, taskResult := range taskResults {
		var resultDataItem []task.Result
		err = json.Unmarshal([]byte(string(taskResult.ResultData)), &resultDataItem)
		if err != nil {
			log.Error().Err(err).Str("taskResult.ResultData", string(taskResult.ResultData)).Msg("failed to convert task results")
			c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("failed to convert result data to tempResult"))
			return
		}

		tempResult := taskResultResponse{
			ResultData:      resultDataItem,
			TaskResultModel: taskResult,
		}
		formattedTaskResults = append(formattedTaskResults, tempResult)
	}

	c.JSON(http.StatusOK, defaultSuccessResponse(map[string]interface{}{"taskResults": formattedTaskResults}))
}

func UpdateWorkerPartnerController(c *gin.Context) {
	jwtClaims, _ := c.Get("userInfo")

	var requestMap map[string]string
	if err := c.BindJSON(&requestMap); err != nil {
		log.Error().Err(err).Msg("Failed to bind JSON to requestMap")
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		return
	}

	minerSubscriptionKey := requestMap["minerSubscriptionKey"]
	newMinerSubscriptionKey := requestMap["newMinerSubscriptionKey"]
	name := requestMap["name"]

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
	c.JSON(http.StatusOK, defaultSuccessResponse(map[string]interface{}{
		"workerPartner": worker.WorkerPartner{
			Id:              updatedWorkerPartner.ID,
			CreatedAt:       updatedWorkerPartner.CreatedAt,
			SubscriptionKey: updatedWorkerPartner.MinerSubscriptionKey,
			Name:            *updatedWorkerPartner.InnerWorkerPartner.Name,
		},
	}))
}

func DisableMinerByWorkerController(c *gin.Context) {
	var requestMap map[string]interface{}
	if err := c.BindJSON(&requestMap); err != nil {
		log.Error().Err(err).Msg("Failed to bind JSON to requestMap")
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		return
	}

	const SUB_KEY = "minerSubscriptionKey"
	const DISABLE_KEY = "toDisable"

	minerSubKeyInterface, minerSubscriptionKeyExists := requestMap[SUB_KEY]
	minerSubscriptionKey, ok := minerSubKeyInterface.(string)
	if !minerSubscriptionKeyExists || minerSubscriptionKey == "" || !ok {
		c.JSON(http.StatusBadRequest, defaultErrorResponse(SUB_KEY+" is required, must be a string, and cannot be empty"))
		return
	}

	toDisableInterface, toDisableExists := requestMap[DISABLE_KEY]
	if !toDisableExists {
		c.JSON(http.StatusBadRequest, defaultErrorResponse(DISABLE_KEY+" is required"))
		return
	}
	jwtClaims, _ := c.Get("userInfo")

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

	// check if bool
	toDisableOptional, parseError := parseBool(toDisableInterface)
	if parseError != nil {
		c.JSON(http.StatusBadRequest, defaultErrorResponse(DISABLE_KEY+" must be a boolean value"))

		return
	}
	toDisable := *toDisableOptional

	log.Info().Str("minerSubscriptionKey", minerSubscriptionKey).Bool(DISABLE_KEY, toDisable).Msg("Disabling miner by worker")

	if toDisable {
		count, err := orm.NewWorkerPartnerORM().DisablePartnerByWorker(worker.ID, minerSubscriptionKey, toDisable)

		if err != nil {
			c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to disable worker partner"))
			return
		}
		if count > 0 {
			c.JSON(http.StatusOK, defaultSuccessResponse(map[string]interface{}{"message": "Miner disabled successfully"}))
			return
		}
		c.JSON(http.StatusNotFound, defaultErrorResponse("Failed to disable worker partner, no records updated"))
	} else {
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid request param"))
	}
}

func parseBool(value interface{}) (*bool, error) {
	valueBool, ok := value.(bool)
	if ok {
		return &valueBool, nil
	}
	valueParsed, err := strconv.ParseBool(value.(string))
	if err != nil {
		return nil, err
	}
	return &valueParsed, nil
}

func DisableWorkerByMinerController(c *gin.Context) {
	var requestMap map[string]interface{}
	if err := c.BindJSON(&requestMap); err != nil {
		log.Error().Err(err).Msg("Failed to bind JSON to requestMap")
		c.AbortWithStatusJSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		return
	}

	const WORKER_KEY = "workerId"
	const DISABLE_KEY = "toDisable"

	workerIdInterface, workerIdExists := requestMap[WORKER_KEY]
	workerId, ok := workerIdInterface.(string)
	if !workerIdExists || workerId == "" || !ok {
		c.JSON(http.StatusBadRequest, defaultErrorResponse(WORKER_KEY+" is required and must be a string"))
		return
	}

	toDisableInterface, toDisableExists := requestMap[DISABLE_KEY]
	toDisableOptional, parseError := parseBool(toDisableInterface)
	if !toDisableExists || parseError != nil {
		c.JSON(http.StatusBadRequest, defaultErrorResponse(DISABLE_KEY+" must be a boolean value"))
		return
	}
	toDisable := *toDisableOptional

	minerUserValue, exists := c.Get("minerUser")
	if !exists {
		return
	}
	minerUser, _ := minerUserValue.(*db.MinerUserModel)

	log.Info().Str(WORKER_KEY, workerId).Bool(DISABLE_KEY, toDisable).Str("subscriptionKey", minerUser.SubscriptionKey).Msg("Attempting to disable worker by miner")

	if toDisable {

		count, err := orm.NewWorkerPartnerORM().DisablePartnerByMiner(workerId, minerUser.SubscriptionKey, toDisable)
		if err != nil {
			c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to disable worker partner"))
			return
		}
		if count > 0 {
			c.JSON(http.StatusOK, defaultSuccessResponse(map[string]interface{}{"message": "Worker disabled successfully"}))
		} else {
			c.JSON(http.StatusNotFound, defaultErrorResponse("Failed to disable worker partner, no records updated"))
		}
	} else {
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid request param"))
	}
}

func GenerateNonceController(c *gin.Context) {
	address := c.Param("address")
	log.Info().Str("address", address).Msg("Getting address from param")
	if address == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "address parameter is required"})
		return
	}

	cache := GetCacheInstance()
	nonce := siwe.GenerateNonce()
	log.Info().Msgf("Wallet address %s generated nonce %s", address, nonce)
	err := cache.SetWithExpire(address, nonce, 1*time.Minute)
	log.Info().Interface("keys", cache.Keys()).Msg("Checking cache keys")
	// TODO remove after debugging
	cache.ShowAll()

	if err != nil {
		log.Error().Str("address", address).Str("nonce", nonce).Err(err).Msg("Failed to store nonce")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to store nonce"))
		return
	}

	log.Info().Str("address", address).Str("nonce", nonce).Msg("Nonce generated successfully")
	c.JSON(http.StatusOK, defaultSuccessResponse(map[string]interface{}{"nonce": nonce}))
}
