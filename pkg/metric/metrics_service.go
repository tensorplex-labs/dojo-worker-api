package metric

import (
	"context"
	"dojo-api/db"
	"dojo-api/pkg/event"
	"dojo-api/pkg/orm"
	"encoding/json"

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
	taskResultORM := orm.NewTaskResultORM()
	metricORM := orm.NewMetricsORM()

	completedTResultCount, err := taskResultORM.GetCompletedTResultCount(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get completed task result count")
		return err
	}
	newMetricData := MetricTaskResultsCount{TotalNumTasksResults: completedTResultCount}
	log.Info().Interface("TotalTaskResultsCount", newMetricData).Msg("Updating total task results count metric")

	err = metricORM.CreateNewMetric(ctx, db.MetricsTypeTotalNumTaskResults, newMetricData)
	return err
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
