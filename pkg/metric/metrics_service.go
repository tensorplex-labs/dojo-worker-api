package metric

import (
	"dojo-api/db"
	"encoding/json"
	"fmt"
)

func ValidateMetricData(metricType db.MetricsType, data db.JSON) (interface{}, error) {
	switch metricType {
	case db.MetricsTypeTotalNumDojoWorkers:
		var workerCountData MetricWorkerCount
		if err := json.Unmarshal(data, &workerCountData); err != nil {
			return nil, fmt.Errorf("invalid worker count data format: %v", err)
		}
		return workerCountData, nil
	case db.MetricsTypeTotalNumCompletedTasks:
		var completedTasksData MetricCompletedTasks
		if err := json.Unmarshal(data, &completedTasksData); err != nil {
			return nil, fmt.Errorf("invalid completed tasks data format: %v", err)
		}
		return completedTasksData, nil
	case db.MetricsTypeTotalNumTaskResults:
		var tasksResultsData MetricTaskResults
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
