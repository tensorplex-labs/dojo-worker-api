package metric

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
