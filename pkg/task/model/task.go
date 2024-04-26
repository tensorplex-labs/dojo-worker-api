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

type SortField string

const (
	SortCreatedAt    SortField = "createdAt"
	SortNumResult    SortField = "numResult"
	SortHighestYield SortField = "highestYield"
)

type Pagination struct {
	Page       int `json:"pageNumber"`
	Limit      int `json:"pageSize"`
	TotalPages int `json:"totalPages"`
	TotalItems int `json:"totalItems"`
}

type TaskPagination struct {
	Tasks      []TaskResponse `json:"tasks"`
	Pagination Pagination     `json:"pagination"`
}
