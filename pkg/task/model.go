package task

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

type TaskRequest struct {
	Title        string      `json:"title"`
	Body         string      `json:"body"`
	ExpireAt     interface{} `json:"expireAt"`
	TaskData     []TaskData  `json:"taskData"`
	MaxResults   int         `json:"maxResults"`
	TotalRewards float64     `json:"totalRewards"`
}

type TaskData struct {
	Prompt    string         `json:"prompt"`
	Dialogue  []dialogueData `json:"dialogue,omitempty"`
	Responses []taskResponse `json:"responses,omitempty"`
	Task      string         `json:"task"`
	Criteria  []taskCriteria `json:"criteria"`
}

type taskResponse struct {
	Model      string      `json:"model"`
	Completion interface{} `json:"completion"`
}

type dialogueData struct {
	Role    string `json:"role"`
	Message string `json:"message"`
}

type taskCriteria struct {
	Type    string   `json:"type"`
	Options []string `json:"options,omitempty"`
	Min     float64  `json:"min,omitempty"`
	Max     float64  `json:"max,omitempty"`
}

type Task struct {
	Title        string      `json:"title"`
	Body         string      `json:"body"`
	Modality     db.TaskType `json:"modality"`
	ExpireAt     time.Time   `json:"expireAt"`
	Criteria     []byte      `json:"criteria"`
	TaskData     []byte      `json:"taskData"`
	MaxResults   int         `json:"maxResults"`
	TotalRewards float64     `json:"totalRewards"`
}

type TaskResult struct {
	ID             string    `json:"id"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	Status         string    `json:"status"`
	ResultData     []byte    `json:"resultData"`
	TaskID         string    `json:"taskId"`
	DojoWorkerID   string    `json:"dojoWorkerId"`
	StakeAmount    *float64  `json:"stakeAmount"`
	PotentialYield *float64  `json:"potentialYield"`
	PotentialLoss  *float64  `json:"potentialLoss"`
	FinalisedYield *float64  `json:"finalisedYield"`
	FinalisedLoss  *float64  `json:"finalisedLoss"`
}
