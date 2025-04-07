package metric

import (
	"time"
)

type DojoWorkerCountResponse struct {
	NumDojoWorkers int `json:"numDojoWorkers"`
}

type CompletedTaskCountResponse struct {
	NumCompletedTasks int `json:"numCompletedTasks"`
}

type TaskResultCountResponse struct {
	NumTaskResults int `json:"numTaskResults"`
}

type AvgTaskCompletionTimeResponse struct {
	AvgTaskCompletionTime int `json:"averageTaskCompletionTime"`
}

type MetricData interface{}

type MetricWorkerCount struct {
	TotalNumDojoWorkers int `json:"total_num_dojo_workers"`
}

type MetricCompletedTasksCount struct {
	TotalNumCompletedTasks int `json:"total_num_completed_tasks"`
}

type MetricTaskResultsCount struct {
	TotalNumTasksResults int `json:"total_num_tasks_results"`
}

type MetricAvgTaskCompletionTime struct {
	AverageTaskCompletionTime int `json:"average_task_completion_time"`
}

type CompletedTasksByTimestampResponse struct {
	Timestamp         time.Time `json:"timestamp"`
	NumCompletedTasks int       `json:"numCompletedTasks"`
}

// IntervalDataPoint represents a single data point with timestamp and count
type IntervalDataPoint struct {
	Timestamp         time.Time `json:"timestamp"`
	NumCompletedTasks int       `json:"numCompletedTasks"`
}

// CompletedTasksIntervalResponse represents the response for interval-based task completion metrics
type CompletedTasksIntervalResponse struct {
	IntervalSeconds int                 `json:"intervalSeconds"`
	DateFrom        time.Time           `json:"dateFrom"`
	DateTo          time.Time           `json:"dateTo"`
	DataPoints      []IntervalDataPoint `json:"dataPoints"`
}
