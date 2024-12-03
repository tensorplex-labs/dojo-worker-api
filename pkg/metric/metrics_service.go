package metric

import (
	"context"
	"encoding/json"

	"dojo-api/db"
	"dojo-api/pkg/cache"
	"dojo-api/pkg/event"
	"dojo-api/pkg/orm"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type MetricService struct {
	metricORM *orm.MetricsORM
}

func NewMetricService() *MetricService {
	return &MetricService{
		metricORM: orm.NewMetricsORM(),
	}
}

func (metricService *MetricService) UpdateDojoWorkerCount(ctx context.Context) error {
	workerORM := orm.NewDojoWorkerORM()
	workerCounts, err := workerORM.GetDojoWorkers()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get dojo workers")
		return err
	}

	metricORM := orm.NewMetricsORM()
	newMetricData := MetricWorkerCount{TotalNumDojoWorkers: workerCounts}
	log.Info().Interface("DojoWorkerCount", newMetricData).Msg("Updating dojo worker count metric")

	err = metricORM.CreateNewMetric(ctx, db.MetricsTypeTotalNumDojoWorkers, newMetricData)
	return err
}

func (metricService *MetricService) UpdateCompletedTaskCount(ctx context.Context) error {
	taskORM := orm.NewTaskORM()
	metricORM := orm.NewMetricsORM()

	completedTasksCount, err := taskORM.GetCompletedTaskCount(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get completed tasks")
		return err
	}
	newMetricData := MetricCompletedTasksCount{TotalNumCompletedTasks: completedTasksCount}
	log.Info().Interface("CompletedTaskCount", newMetricData).Msg("Updating completed task count metric")

	err = metricORM.CreateNewMetric(ctx, db.MetricsTypeTotalNumCompletedTasks, newMetricData)
	return err
}

func (metricService *MetricService) UpdateTotalTaskResultsCount(ctx context.Context) error {
	cache := cache.GetCacheInstance()
	metricORM := orm.NewMetricsORM()

	cacheKey := "metrics:task_results:total"

	// Try to get current count from Redis
	_, err := cache.Redis.Get(ctx, cacheKey).Int64()
	if err == redis.Nil { // Key doesn't exist (e.g., after Redis restart)
		// Get the last metric from database
		lastMetric, err := metricORM.GetMetricsDataByMetricType(ctx, db.MetricsTypeTotalNumTaskResults)
		if err != nil && !db.IsErrNotFound(err) {
			return err
		}

		// Initialize Redis with the last known count from database
		if lastMetric != nil {
			var lastMetricData MetricTaskResultsCount
			if err := json.Unmarshal(lastMetric.MetricsData, &lastMetricData); err != nil {
				return err
			}
			currentCount := int64(lastMetricData.TotalNumTasksResults)
			// Set the Redis counter to last known value
			if err := cache.Redis.Set(ctx, cacheKey, currentCount, 0).Err(); err != nil {
				return err
			}
		}
	} else if err != nil {
		return err
	}

	// Increment the counter
	count, err := cache.Redis.Incr(ctx, cacheKey).Result()
	if err != nil {
		log.Error().Err(err).Msg("Failed to increment task results count")
		return err
	}

	// Store in database
	newMetricData := MetricTaskResultsCount{TotalNumTasksResults: int(count)}
	log.Info().Interface("TotalTaskResultsCount", newMetricData).Msg("Updating total task results count metric")

	return metricORM.CreateNewMetric(ctx, db.MetricsTypeTotalNumTaskResults, newMetricData)
}

func (metricService *MetricService) UpdateAvgTaskCompletionTime(ctx context.Context) error {
	eventORM := orm.NewEventsORM()
	metricORM := orm.NewMetricsORM()

	events, err := eventORM.GetEventsByType(ctx, db.EventsTypeTaskCompletionTime)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get task completion time events")
		return err
	}

	totalCompletionTime, err := CalculateTotalTaskCompletionTime(events)
	if err != nil {
		log.Error().Err(err).Msg("Failed to calculate total task completion time")
		return err
	}

	avgCompletionTime := *totalCompletionTime / len(events)
	newMetricData := MetricAvgTaskCompletionTime{AverageTaskCompletionTime: avgCompletionTime}
	log.Info().Interface("AvgTaskCompletionTime", newMetricData).Msg("Updating average task completion time metric")

	err = metricORM.CreateNewMetric(ctx, db.MetricsTypeAverageTaskCompletionTime, newMetricData)
	return err
}

func CalculateTotalTaskCompletionTime(events []db.EventsModel) (*int, error) {
	var totalCompletionTime int

	for _, e := range events {

		if e.Type != db.EventsTypeTaskCompletionTime {
			continue
		}

		eventData := event.EventTaskCompletionTime{}
		err := json.Unmarshal(e.EventsData, &eventData)
		if err != nil {
			return nil, err
		}

		totalCompletionTime += eventData.TaskCompletionTime
	}
	return &totalCompletionTime, nil
}
