package utils

import (
// "time"
// "encoding/json"
)

type TaskRequest struct {
	Title        string      `json:"title"`
	Body         string      `json:"body"`
	ExpireAt     interface{} `json:"expireAt"`
	TaskData     []TaskData  `json:"taskData"`
	MaxResults   int         `json:"maxResults"`
	TotalRewards float64     `json:"totalRewards"`
}

type TaskData struct {
	Prompt    string         `json:"prompt,omitempty"`
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
