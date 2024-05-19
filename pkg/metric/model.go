package metric

type GetDojoWorkerCountResp struct {
	NumDojoWorkers int `json:"numDojoWorkers"`
}

type MetricWorkerCount struct {
	TotalNumDojoWorkers int `json:"total_num_dojo_workers"`
}

type MetricCompletedTasks struct {
	TotalNumCompletedTasks int `json:"total_num_completed_tasks"`
}

type MetricTaskResults struct {
	TotalNumTasksResults int `json:"total_num_tasks_results"`
}

type MetricAvgTaskCompletionTime struct {
	AverageTaskCompletionTime int `json:"average_task_completion_time"`
}

type TaskCompletionEventData struct {
	TaskId             string `json:"task_id"`
	TaskCompletionTime int    `json:"task_completion_time"`
}
