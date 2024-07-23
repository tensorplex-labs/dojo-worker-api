package task

import (
	"dojo-api/db"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"
)

// TaskResponse reflects the task structure used in API responses
type TaskResponse struct {
	ID          string        `json:"taskId"`
	Title       string        `json:"title"`
	Body        string        `json:"body"`
	ExpireAt    time.Time     `json:"expireAt"`
	Type        db.TaskType   `json:"type"`
	TaskData    interface{}   `json:"taskData"`
	Status      db.TaskStatus `json:"status"`
	NumResults  int           `json:"numResults"`
	MaxResults  int           `json:"maxResults"`
	NumCriteria int           `json:"numCriteria"`
}

type TaskPaginationResponse struct {
	TaskResponse
	IsCompletedByWorker bool `json:"isCompletedByWorker"`
}

type SortField string

const (
	SortCreatedAt    SortField = "createdAt"
	SortNumResult    SortField = "numResult"
	SortHighestYield SortField = "highestYield"
	SENTINEL_VALUE   float64   = -math.MaxFloat64
)

var ValidTaskTypes = []db.TaskType{db.TaskTypeCodeGeneration, db.TaskTypeTextToImage, db.TaskTypeDialogue}

type Pagination struct {
	Page       int `json:"pageNumber"`
	Limit      int `json:"pageSize"`
	TotalPages int `json:"totalPages"`
	TotalItems int `json:"totalItems"`
}

type TaskPagination struct {
	Tasks      []TaskPaginationResponse `json:"tasks"`
	Pagination Pagination               `json:"pagination"`
}

type CreateTaskRequest struct {
	Title        string      `json:"title"`
	Body         string      `json:"body"`
	ExpireAt     interface{} `json:"expireAt"`
	TaskData     []TaskData  `json:"taskData"`
	MaxResults   int         `json:"maxResults"`
	TotalRewards float64     `json:"totalRewards"`
}

type TaskData struct {
	Prompt    string          `json:"prompt"`
	Responses []ModelResponse `json:"responses,omitempty"`
	Task      db.TaskType     `json:"task"`
	Criteria  []Criteria      `json:"criteria"`
}

type ModelResponse struct {
	Model      string      `json:"model"`
	Completion interface{} `json:"completion"`
}

type Message struct {
	Role    string `json:"role"`
	Message string `json:"message"`
}

type Criteria struct {
	Type    CriteriaType `json:"type"`
	Options []string     `json:"options,omitempty"`
	Text    string       `json:"text,omitempty"`
	Min     float64      `json:"min,omitempty"`
	Max     float64      `json:"max,omitempty"`
}

type CriteriaType string

const (
	CriteriaTypeRanking     CriteriaType = "ranking"
	CriteriaTypeMultiSelect CriteriaType = "multi-select"
	CriteriaTypeScore       CriteriaType = "score"
	CriteriaMultiScore      CriteriaType = "multi-score"
)

type Result struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// embed TaskResultModel to reuse its fields
// override ResultData, also will shadow the original "result_data" JSON field
type TaskResult struct {
	db.TaskResultModel
	ResultData []Result `json:"result_data"`
}

type TaskResultResponse struct {
	TaskResults []TaskResult `json:"taskResults"`
}

type SubmitTaskResultRequest struct {
	ResultData []Result `json:"resultData" binding:"required"`
}

type SubmitTaskResultResponse struct {
	NumResults int `json:"numResults"`
}

type (
	ScoreValue       float64
	RankingValue     map[string]string
	MultiScoreValue  map[string]float64
	MultiSelectValue []string
)

type NextTaskResponse struct {
	NextInProgressTaskId string `json:"nextInProgressTaskId"`
}

type PaginationParams struct {
	Page  int          `json:"page"`
	Limit int          `json:"limit"`
	Types []string     `json:"types"`
	Sort  string       `json:"sort"`
	Order db.SortOrder `json:"order"`
}

func parseJsonStringOrFloat(v json.RawMessage) (float64, error) {
	var floatStr string
	err := json.Unmarshal(v, &floatStr)
	if err == nil {
		res, err := strconv.ParseFloat(floatStr, 64)
		if err != nil {
			return SENTINEL_VALUE, err
		}
		return res, nil
	}

	var floatVal float64
	if err := json.Unmarshal(v, &floatVal); err != nil {
		return SENTINEL_VALUE, err
	}
	return floatVal, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for Result,
// allowing for custom unmarshalling logic based on the type of value.
func (r *Result) UnmarshalJSON(data []byte) error {
	// Helper struct to avoid recursion into UnmarshalJSON
	type tempResult struct {
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	}
	var i tempResult
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}

	r.Type = i.Type

	tempType := CriteriaType(i.Type)

	switch tempType {
	case CriteriaTypeScore:
		value, err := parseJsonStringOrFloat(i.Value)
		if err != nil {
			return err
		}
		r.Value = ScoreValue(value)
	case CriteriaTypeRanking:
		var v RankingValue
		if err := json.Unmarshal(i.Value, &v); err != nil {
			return err
		}
		r.Value = v
	case CriteriaTypeMultiSelect:
		var v MultiSelectValue
		if err := json.Unmarshal(i.Value, &v); err != nil {
			return err
		}
		r.Value = v
	case CriteriaMultiScore:
		var intermediate map[string]json.RawMessage
		if err := json.Unmarshal(i.Value, &intermediate); err != nil {
			return err
		}

		v := make(MultiScoreValue)
		for k, vRaw := range intermediate {
			value, err := parseJsonStringOrFloat(vRaw)
			if err != nil {
				return err
			}
			v[k] = value
		}

		r.Value = v
	default:
		return fmt.Errorf("unknown type: %s", i.Type)
	}

	return nil
}
