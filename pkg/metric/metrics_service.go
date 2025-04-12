package metric

import (
	"context"
	"encoding/json"
	"fmt"

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
	cache := cache.GetCacheInstance()
	cacheKey := string(cache.Keys.CompletedTasksTotal)
	metricORM := orm.NewMetricsORM()

	// Try to get current count from Redis
	currentCount, err := cache.Redis.Get(ctx, cacheKey).Int64()
	log.Info().Int64("CompletedTasksCount", currentCount).Msg("Current completed tasks count")
	if err == redis.Nil { // Key doesn't exist
		// Get the last metric from database
		lastMetric, err := metricORM.GetMetricsDataByMetricType(ctx, db.MetricsTypeTotalNumCompletedTasks)
		if err != nil && !db.IsErrNotFound(err) {
			return err
		}

		// Initialize Redis counter with last known value from database
		var initialCount int64 = 0
		if lastMetric != nil {
			var lastMetricData MetricCompletedTasksCount
			if err := json.Unmarshal(lastMetric.MetricsData, &lastMetricData); err != nil {
				return err
			}
			log.Info().Interface("LastMetricData", lastMetricData).Msg("Last Completed Tasks Count in Metrics")
			initialCount = int64(lastMetricData.TotalNumCompletedTasks)
		}

		if err := cache.Redis.Set(ctx, cacheKey, initialCount, 0).Err(); err != nil {
			return err
		}
		log.Info().Int64("initial_count", initialCount).Msg("Initialized completed tasks counter")
	} else if err != nil {
		return err
	}

	// Increment the counter
	count, err := cache.Redis.Incr(ctx, cacheKey).Result()
	if err != nil {
		log.Error().Err(err).Msg("Failed to increment completed tasks count")
		return err
	}

	// Store in database
	newMetricData := MetricCompletedTasksCount{TotalNumCompletedTasks: int(count)}
	log.Info().Interface("CompletedTaskCount", newMetricData).Msg("Updating completed tasks count metric")

	return metricORM.CreateNewMetric(ctx, db.MetricsTypeTotalNumCompletedTasks, newMetricData)
}

func (metricService *MetricService) UpdateTotalTaskResultsCount(ctx context.Context) error {
	cache := cache.GetCacheInstance()
	cacheKey := string(cache.Keys.TaskResultsTotal)
	metricORM := orm.NewMetricsORM()

	// Try to get current count from Redis
	currentCount, err := cache.Redis.Get(ctx, cacheKey).Int64()
	log.Info().Int64("TaskResultsCount", currentCount).Msg("Current task results count")
	if err == redis.Nil { // Key doesn't exist (e.g., after Redis restart)
		// Get the last metric from database
		lastMetric, err := metricORM.GetMetricsDataByMetricType(ctx, db.MetricsTypeTotalNumTaskResults)
		if err != nil && !db.IsErrNotFound(err) {
			return err
		}

		// Initialize Redis counter with last known value from database
		// If no metric is found, counter will start from 0
		var initialCount int64 = 0
		if lastMetric != nil {
			var lastMetricData MetricTaskResultsCount
			if err := json.Unmarshal(lastMetric.MetricsData, &lastMetricData); err != nil {
				return err
			}
			log.Info().Interface("LastMetricData", lastMetricData).Msg("Last Task Results Count in Metrics")
			initialCount = int64(lastMetricData.TotalNumTasksResults)
		}

		if err := cache.Redis.Set(ctx, cacheKey, initialCount, 0).Err(); err != nil {
			return err
		}
		log.Info().Int64("initial_count", initialCount).Msg("Initialized task results counter")
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

	// calculate average task completion time directly in db
	avgCompletionTime, err := eventORM.GetAverageTaskCompletionTime(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get average task completion time")
		return err
	}

	// events, err := eventORM.GetEventsByType(ctx, db.EventsTypeTaskCompletionTime)
	// if err != nil {
	// 	log.Error().Err(err).Msg("Failed to get task completion time events")
	// 	return err
	// }
	// totalCompletionTime, err := CalculateTotalTaskCompletionTime(events)
	// if err != nil {
	// 	log.Error().Err(err).Msg("Failed to calculate total task completion time")
	// 	return err
	// }
	// avgCompletionTime := *totalCompletionTime / len(events)

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

// GetCompletedTasksCountByInterval returns the number of completed tasks for each interval between dateFrom and dateTo
func (metricService *MetricService) GetCompletedTasksCountByInterval(ctx context.Context, fromUnix int64, toUnix int64, intervalDays int) ([]IntervalDataPoint, error) {
	if intervalDays <= 0 {
		return nil, fmt.Errorf("interval must be greater than 0")
	}

	taskORM := orm.NewTaskORM()

	intervalResults, err := taskORM.GetCompletedTasksCountByIntervals(ctx, fromUnix, toUnix, intervalDays)
	if err != nil {
		log.Error().Err(err).
			Int64("dateFrom", fromUnix).
			Int64("dateTo", toUnix).
			Int("intervalDays", intervalDays).
			Msg("Failed to get completed tasks count by intervals")
		return nil, err
	}

	dataPoints := make([]IntervalDataPoint, len(intervalResults))
	for i, result := range intervalResults {
		dataPoints[i] = IntervalDataPoint{
			Timestamp:         result.IntervalEnd,
			NumCompletedTasks: result.Count,
		}
	}

	return dataPoints, nil
}
