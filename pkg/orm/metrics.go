package orm

import (
	"context"
	"dojo-api/db"
	"dojo-api/pkg/metric"
	"encoding/json"
	"fmt"
	"time"
)

type MetricsORM struct {
	dbClient      *db.PrismaClient
	clientWrapper *PrismaClientWrapper
}

func NewMetricsORM() *MetricsORM {
	clientWrapper := GetPrismaClient()
	return &MetricsORM{
		dbClient:      clientWrapper.Client,
		clientWrapper: clientWrapper,
	}
}

func (o *MetricsORM) GetMetricsDataByMetricType(ctx context.Context, metricType db.MetricsType) (*db.MetricsModel, error) {
	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()

	metrics, err := o.dbClient.Metrics.FindUnique(
		db.Metrics.Type.Equals(metricType),
	).Exec(ctx)

	if err != nil {
		return nil, err
	}

	return metrics, nil
}

func (o *MetricsORM) UpdateDojoWorkerCount(ctx context.Context, increment int) error {
	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()

	metrics, err := o.dbClient.Metrics.FindUnique(
		db.Metrics.Type.Equals(db.MetricsTypeTotalNumDojoWorkers),
	).Exec(ctx)

	if err != nil {
		if db.IsErrNotFound(err) {
			// Create new metric data if it doesn't exist
			workers, err := o.dbClient.DojoWorker.FindMany().Exec(ctx)
			if err != nil {
				return err
			}
			newMetricData := metric.MetricWorkerCount{TotalNumDojoWorkers: len(workers)}
			data, err := json.Marshal(newMetricData)
			if err != nil {
				return err
			}
			_, err = o.dbClient.Metrics.CreateOne(
				db.Metrics.Type.Set(db.MetricsTypeTotalNumDojoWorkers),
				db.Metrics.MetricsData.Set(data),
			).Exec(ctx)
			return err
		}
		return err
	}

	var workerCountData metric.MetricWorkerCount
	if err := json.Unmarshal(metrics.MetricsData, &workerCountData); err != nil {
		return fmt.Errorf("invalid worker count data format: %v", err)
	}

	workerCountData.TotalNumDojoWorkers += increment
	updatedData, err := json.Marshal(workerCountData)
	if err != nil {
		return err
	}

	_, err = o.dbClient.Metrics.FindUnique(
		db.Metrics.Type.Equals(db.MetricsTypeTotalNumDojoWorkers),
	).Update(
		db.Metrics.MetricsData.Set(updatedData),
		db.Metrics.UpdatedAt.Set(time.Now()),
	).Exec(ctx)
	return err
}

func (o *MetricsORM) UpdateCompletedTaskCount(ctx context.Context, increment int) error {
	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()

	metrics, err := o.dbClient.Metrics.FindUnique(
		db.Metrics.Type.Equals(db.MetricsTypeTotalNumCompletedTasks),
	).Exec(ctx)

	if err != nil {
		if db.IsErrNotFound(err) {
			// Create new metric data if it doesn't exist
			completedTasks, err := o.dbClient.Task.FindMany(db.Task.Status.Equals(db.TaskStatusCompleted)).Exec(ctx)
			if err != nil {
				return err
			}
			newMetricData := metric.MetricCompletedTasks{TotalNumCompletedTasks: len(completedTasks)}
			data, err := json.Marshal(newMetricData)
			if err != nil {
				return err
			}
			_, err = o.dbClient.Metrics.CreateOne(
				db.Metrics.Type.Set(db.MetricsTypeTotalNumCompletedTasks),
				db.Metrics.MetricsData.Set(data),
			).Exec(ctx)
			return err
		}
		return err
	}

	var completedTasksData metric.MetricCompletedTasks
	if err := json.Unmarshal(metrics.MetricsData, &completedTasksData); err != nil {
		return fmt.Errorf("invalid completed tasks data format: %v", err)
	}

	completedTasksData.TotalNumCompletedTasks += increment
	updatedData, err := json.Marshal(completedTasksData)
	if err != nil {
		return err
	}

	_, err = o.dbClient.Metrics.FindUnique(
		db.Metrics.Type.Equals(db.MetricsTypeTotalNumCompletedTasks),
	).Update(
		db.Metrics.MetricsData.Set(updatedData),
		db.Metrics.UpdatedAt.Set(time.Now()),
	).Exec(ctx)
	return err
}

func (o *MetricsORM) UpdateTotalTaskResultsCount(ctx context.Context, increment int) error {
	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()

	metrics, err := o.dbClient.Metrics.FindUnique(
		db.Metrics.Type.Equals(db.MetricsTypeTotalNumTaskResults),
	).Exec(ctx)

	if err != nil {
		if db.IsErrNotFound(err) {
			// Create new metric data if it doesn't exist
			taskResults, err := o.dbClient.TaskResult.FindMany(db.TaskResult.Status.Equals(db.TaskResultStatusCompleted)).Exec(ctx)
			if err != nil {
				return err
			}
			newMetricData := metric.MetricTaskResults{TotalNumTasksResults: len(taskResults)}
			data, err := json.Marshal(newMetricData)
			if err != nil {
				return err
			}
			_, err = o.dbClient.Metrics.CreateOne(
				db.Metrics.Type.Set(db.MetricsTypeTotalNumTaskResults),
				db.Metrics.MetricsData.Set(data),
			).Exec(ctx)
			return err
		}
		return err
	}

	var tasksResultsData metric.MetricTaskResults
	if err := json.Unmarshal(metrics.MetricsData, &tasksResultsData); err != nil {
		return fmt.Errorf("invalid tasks results data format: %v", err)
	}

	tasksResultsData.TotalNumTasksResults += increment
	updatedData, err := json.Marshal(tasksResultsData)
	if err != nil {
		return err
	}

	_, err = o.dbClient.Metrics.FindUnique(
		db.Metrics.Type.Equals(db.MetricsTypeTotalNumTaskResults),
	).Update(
		db.Metrics.MetricsData.Set(updatedData),
		db.Metrics.UpdatedAt.Set(time.Now()),
	).Exec(ctx)
	return err
}

func (o *MetricsORM) CreateTaskCompletionEvent(ctx context.Context, task db.TaskModel) error {
	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()

	taskCompletionTime := int(time.Since(task.CreatedAt).Seconds())

	eventData := metric.TaskCompletionEventData{TaskId: task.ID, TaskCompletionTime: taskCompletionTime}
	taskCompletionEvent, err := json.Marshal(eventData)
	if err != nil {
		return err
	}

	_, err = o.dbClient.Events.CreateOne(
		db.Events.Type.Set(db.EventsTypeTaskCompletionTime),
		db.Events.EventsData.Set(taskCompletionEvent),
	).Exec(ctx)

	return err
}
