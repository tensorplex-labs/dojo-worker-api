package model

import (
	"dojo-api/db"
	"time"
)

// TaskResponse reflects the task structure used in API responses
type TaskResponse struct {
	ID         string          `json:"taskId"`
	Title      string          `json:"title"`
	Body       string          `json:"body"`
	Modality   db.TaskModality `json:"modality"`
	ExpireAt   time.Time       `json:"expireAt"`
	Criteria   db.JSON         `json:"criteria"`
	TaskData   db.JSON         `json:"taskData"`
	Status     db.TaskStatus   `json:"status"`
	MaxResults int             `json:"maxResults"`
}
