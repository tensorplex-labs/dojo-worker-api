package api

import (
	"context"
	"dojo-api/db"
	"dojo-api/pkg/auth"
	"dojo-api/pkg/event"
	"dojo-api/pkg/metric"
	"dojo-api/pkg/miner"
	"errors"

	"github.com/gin-gonic/gin"
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
