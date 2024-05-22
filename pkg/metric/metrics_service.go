package metric

import (
	"context"
	"dojo-api/db"
	"dojo-api/pkg/event"
	"dojo-api/pkg/orm"
	"encoding/json"
	"fmt"

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
	workers, err := workerORM.GetDojoWorkers()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get dojo workers")
		return err
	}

	metricORM := orm.NewMetricsORM()
	newMetricData := MetricWorkerCount{TotalNumDojoWorkers: len(workers)}
	dataJSON, err := json.Marshal(newMetricData)
	if err != nil {
		return err
	}

	err = metricORM.CreateNewMetric(ctx, db.MetricsTypeTotalNumDojoWorkers, dataJSON)
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
	dataJSON, err := json.Marshal(newMetricData)
	if err != nil {
		return err
	}

	err = metricORM.CreateNewMetric(ctx, db.MetricsTypeTotalNumDojoWorkers, dataJSON)
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
	dataJSON, err := json.Marshal(newMetricData)
	if err != nil {
		return err
	}

	err = metricORM.CreateNewMetric(ctx, db.MetricsTypeTotalNumDojoWorkers, dataJSON)
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
	dataJSON, err := json.Marshal(newMetricData)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal average task completion time data")
		return err
	}

	err = metricORM.CreateNewMetric(ctx, db.MetricsTypeAverageTaskCompletionTime, dataJSON)
	return err
}

func ValidateMetricData(metricType db.MetricsType, data db.JSON) (interface{}, error) {
	switch metricType {
	case db.MetricsTypeTotalNumDojoWorkers:
		var workerCountData MetricWorkerCount
		if err := json.Unmarshal(data, &workerCountData); err != nil {
			return nil, fmt.Errorf("invalid worker count data format: %v", err)
		}
		return workerCountData, nil
	case db.MetricsTypeTotalNumCompletedTasks:
		var completedTasksData MetricCompletedTasksCount
		if err := json.Unmarshal(data, &completedTasksData); err != nil {
			return nil, fmt.Errorf("invalid completed tasks data format: %v", err)
		}
		return completedTasksData, nil
	case db.MetricsTypeTotalNumTaskResults:
		var tasksResultsData MetricTaskResultsCount
		if err := json.Unmarshal(data, &tasksResultsData); err != nil {
			return nil, fmt.Errorf("invalid tasks results data format: %v", err)
		}
		return tasksResultsData, nil
	case db.MetricsTypeAverageTaskCompletionTime:
		var avgTaskCompletionData MetricAvgTaskCompletionTime
		if err := json.Unmarshal(data, &avgTaskCompletionData); err != nil {
			return nil, fmt.Errorf("invalid avg tasks completion time data format: %v", err)
		}
		return avgTaskCompletionData, nil
	default:
		return nil, fmt.Errorf("unsupported metric type: %v", metricType)
	}
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
