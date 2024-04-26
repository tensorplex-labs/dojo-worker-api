package model

import (
	"dojo-api/db"
	"time"
)

// TaskResponse reflects the task structure used in API responses
type TaskResponse struct {
	ID         string        `json:"taskId"`
	Title      string        `json:"title"`
	Body       string        `json:"body"`
	ExpireAt   time.Time     `json:"expireAt"`
	Type       db.TaskType   `json:"type"`
	TaskData   db.JSON       `json:"taskData"`
	Status     db.TaskStatus `json:"status"`
	MaxResults int           `json:"maxResults"`
}

// {
// 	"success": true,
// 	"body": {
// 	  "taskId": "123",
// 	  "title": "Task Title",
// 	  "body": "Detailed task description",
// 	  "expireAt": "YYYY-MM-DD HH:MM:SS",
// 	  "taskData": [....],
// 	  "status": "Pending",
// 	  "maxResults": 10
// 	},
// 	"error": null
//   }
