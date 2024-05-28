package event

type EventTaskCompletionTime struct {
	TaskId             string `json:"task_id"`
	TaskCompletionTime int    `json:"task_completion_time"`
}
