package metric

type GetDojoWorkerCountResp struct {
	NumDojoWorkers int `json:"numDojoWorkers"`
}

type GetCompletedTaskResp struct {
	NumCompletedTasks int `json:"numCompletedTasks"`
}

type GetTaskResultResp struct {
	NumTaskResults int `json:"numTaskResults"`
}

type GetAvgTaskCompletionResp struct {
	AvgTaskCompletionTime int `json:"averageTaskCompletionTime"`
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
