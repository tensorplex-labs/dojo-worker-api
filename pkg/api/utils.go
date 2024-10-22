package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"dojo-api/db"
	"dojo-api/pkg/auth"
	"dojo-api/pkg/event"
	"dojo-api/pkg/metric"
	"dojo-api/pkg/miner"
	"dojo-api/utils"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Define a common response structure
type ApiResponse struct {
	Success bool        `json:"success" swaggertype:"boolean"`
	Body    interface{} `json:"body"`
	Error   interface{} `json:"error"`
}

func defaultErrorResponse(errorMsg interface{}) ApiResponse {
	return ApiResponse{Success: false, Body: nil, Error: errorMsg}
}

func defaultSuccessResponse(body interface{}) ApiResponse {
	return ApiResponse{Success: true, Body: body, Error: nil}
}

func handleCurrentSession(c *gin.Context) (*auth.SecureCookieSession, error) {
	session, exists := c.Get("session")
	if !exists {
		return nil, errors.New("no session found")
	}

	currSession, ok := session.(auth.SecureCookieSession)
	if !ok {
		return nil, errors.New("invalid session")
	}
	return &currSession, nil
}

func buildApiKeyResponse(apiKeys []db.APIKeyModel) miner.MinerApiKeysResponse {
	keys := make([]string, 0)
	for _, apiKey := range apiKeys {
		keys = append(keys, apiKey.Key)
	}
	return miner.MinerApiKeysResponse{
		ApiKeys: keys,
	}
}

func buildSubscriptionKeyResponse(subScriptionKeys []db.SubscriptionKeyModel) miner.MinerSubscriptionKeysResponse {
	keys := make([]string, 0)
	for _, subScriptionKey := range subScriptionKeys {
		keys = append(keys, subScriptionKey.Key)
	}
	return miner.MinerSubscriptionKeysResponse{
		SubscriptionKeys: keys,
	}
}

func handleMetricData(currentTask *db.TaskModel, updatedTask *db.TaskModel) {
	// We want to make sure task status just changed to completion
	metricService := metric.NewMetricService()
	eventService := event.NewEventService()
	ctx := context.Background()

	go func() {
		if err := metricService.UpdateTotalTaskResultsCount(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to update total tasks results count")
		} else {
			log.Info().Msg("Updated total task results count")
		}
	}()

	if (currentTask.Status != db.TaskStatusCompleted) && updatedTask.Status == db.TaskStatusCompleted {
		go func() {
			// Update the completed task count
			if err := metricService.UpdateCompletedTaskCount(ctx); err != nil {
				log.Error().Err(err).Msg("Failed to update completed task count")
			} else {
				log.Info().Msg("Updated completed task count")
			}
		}()

		go func() {
			// Update the task completion event
			if err := eventService.CreateTaskCompletionEvent(ctx, *updatedTask); err != nil {
				log.Error().Err(err).Msg("Failed to create task completion event")
			} else {
				log.Info().Msg("Created task completion event")
			}
		}()

		go func() {
			// Update the avg task completion
			if err := metricService.UpdateAvgTaskCompletionTime(ctx); err != nil {
				log.Error().Err(err).Msg("Failed to update average task completion time")
			} else {
				log.Info().Msg("Updated average task completion time")
			}
		}()
	}
}

// Get the user's IP address from the gin request headers
func getCallerIP(c *gin.Context) string {
	if runtimeEnv := utils.LoadDotEnv("RUNTIME_ENV"); runtimeEnv == "aws" {
		callerIp := c.Request.Header.Get("X-Original-Forwarded-For")
		log.Info().Msgf("Got caller IP from X-Original-Forwarded-For header: %s", callerIp)
		return callerIp
	}
	return c.ClientIP()
}

// CustomGinLogger logs a gin HTTP request in format.
// Allows to set the logger for testing purposes.
func CustomGinLogger(logger *zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now() // Start timer
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Read the request body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("Error reading request body: %v", err)
			c.AbortWithStatus(500)
			return
		}

		// Log the size of the request body
		requestSize := len(body)
		log.Printf("Request size: %d bytes", requestSize)

		// Restore the request body to the context
		c.Request.Body = io.NopCloser(io.NopCloser(bytes.NewBuffer(body)))

		// Process request
		c.Next()

		// Fill the params
		param := gin.LogFormatterParams{}

		param.TimeStamp = time.Now() // Stop timer
		param.Latency = param.TimeStamp.Sub(start)
		if param.Latency > time.Minute {
			param.Latency = param.Latency.Truncate(time.Second)
		}

		param.ClientIP = getCallerIP(c)
		param.Method = c.Request.Method
		param.StatusCode = c.Writer.Status()
		param.ErrorMessage = c.Errors.ByType(gin.ErrorTypePrivate).String()
		param.BodySize = c.Writer.Size()
		if raw != "" {
			path = path + "?" + raw
		}
		param.Path = path

		// Log using the params
		var logEvent *zerolog.Event
		if c.Writer.Status() >= 500 {
			logEvent = logger.Error()
		} else {
			logEvent = logger.Info()
		}

		logEvent.Int("status_code", param.StatusCode).
			Str("latency", param.Latency.String()). // processing time
			Str("ip", param.ClientIP).              // ip addr, depending on runtime
			Str("method", param.Method).
			Str("path", param.Path).          // path with params
			Int("resp_size", param.BodySize). // response size bytes
			Msg(param.ErrorMessage)           // any error messages
		if requestSize > 0 {
			logEvent.Int("req_size", requestSize) // request size bytes
		}
	}
}
