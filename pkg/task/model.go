package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"dojo-api/db"
)

// TaskResponse reflects the task structure used in API responses
type TaskResponse struct {
	ID          string        `json:"taskId"`
	Title       string        `json:"title"`
	Body        string        `json:"body"`
	ExpireAt    time.Time     `json:"expireAt"`
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

var ValidTaskModalities = []db.TaskModality{db.TaskModalityCodeGeneration, db.TaskModalityImage, db.TaskModalityThreeD}

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
	Prompt       string          `json:"prompt"`
	Responses    []ModelResponse `json:"responses,omitempty"`
	TaskModality db.TaskModality `json:"task_modality"`
}

type ModelResponse struct {
	Model      string      `json:"model"`
	Completion interface{} `json:"completion"`
	Criteria   []Criteria  `json:"criteria"`
}

type rawModelResponse struct {
	Model      string            `json:"model"`
	Completion interface{}       `json:"completion"`
	Criteria   []json.RawMessage `json:"criteria"`
}

type Message struct {
	Role    string `json:"role"`
	Message string `json:"message"`
}

type Criteria interface {
	GetType() CriteriaType
	Validate() error
}

type ScoreCriteria struct {
	Type       CriteriaType `json:"type"`
	Min        float64      `json:"min,omitempty"`
	Max        float64      `json:"max,omitempty"`
	Text       string       `json:"text,omitempty"`
	MinerScore float64      `json:"value,omitempty"`
}

type TextCriteria struct {
	Type         CriteriaType `json:"type"`
	Query        string       `json:"query,omitempty"`
	TextFeedback string       `json:"text_feedback"`
}

type CriteriaType string

const (
	CriteriaTypeScore CriteriaType = "score"
	CriteriaTypeText  CriteriaType = "text"
)

type Result struct {
	Model    string     `json:"model"`
	Criteria []Criteria `json:"criteria"`
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
	Page       int          `json:"page"`
	Limit      int          `json:"limit"`
	Modalities []string     `json:"modalities"`
	Sort       string       `json:"sort"`
	Order      db.SortOrder `json:"order"`
}

// Implement GetType for all criteria types
func (s ScoreCriteria) GetType() CriteriaType {
	return CriteriaTypeScore
}

// GetType for TextCriteria
func (t TextCriteria) GetType() CriteriaType {
	return CriteriaTypeText
}

// Implement Validate for each type
func (c ScoreCriteria) Validate() error {
	if (c.Min < 0 || c.Max < 0) || (c.Min == 0 && c.Max == 0) {
		return errors.New("valid min and max are required for score criteria")
	}
	if c.Min >= c.Max {
		return errors.New("min must be less than max for score criteria")
	}
	return nil
}

// Validate for TextCriteria
func (t TextCriteria) Validate() error {
	if t.Query == "" {
		return fmt.Errorf("query is required for text criteria")
	}
	return nil
}

// Add custom unmarshaling for ModelResponse
func (mr *ModelResponse) UnmarshalJSON(data []byte) error {
	var raw rawModelResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	mr.Model = raw.Model
	mr.Completion = raw.Completion
	mr.Criteria = make([]Criteria, 0)

	for _, criteriaData := range raw.Criteria {
		// unmarshal to get the type
		var temp struct {
			Type CriteriaType `json:"type"`
		}
		if err := json.Unmarshal(criteriaData, &temp); err != nil {
			return err
		}

		// Based on the type, unmarshal to the correct criteria struct
		var criteria Criteria
		switch temp.Type {
		case CriteriaTypeScore:
			var sc ScoreCriteria
			if err := json.Unmarshal(criteriaData, &sc); err != nil {
				return err
			}
			criteria = sc
		case CriteriaTypeText:
			var tc TextCriteria
			if err := json.Unmarshal(criteriaData, &tc); err != nil {
				return err
			}
			criteria = tc
		default:
			return fmt.Errorf("unknown criteria type: %s", temp.Type)
		}

		mr.Criteria = append(mr.Criteria, criteria)
	}

	return nil
}

func (r *Result) UnmarshalJSON(data []byte) error {
	var raw struct {
		Model    string            `json:"model"`
		Criteria []json.RawMessage `json:"criteria"`
	}
	// unmarshal the outer structure
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	r.Model = raw.Model
	r.Criteria = make([]Criteria, 0)

	for _, criteriaData := range raw.Criteria {
		// Peek at the type field first
		var temp struct {
			Type CriteriaType `json:"type"`
		}
		if err := json.Unmarshal(criteriaData, &temp); err != nil {
			return err
		}

		// Based on the type, unmarshal into the correct concrete type
		switch temp.Type {
		case CriteriaTypeScore:
			var sc ScoreCriteria
			if err := json.Unmarshal(criteriaData, &sc); err != nil {
				return err
			}
			r.Criteria = append(r.Criteria, sc)
		case CriteriaTypeText:
			var tc TextCriteria
			if err := json.Unmarshal(criteriaData, &tc); err != nil {
				return err
			}
			r.Criteria = append(r.Criteria, tc)
		default:
			return fmt.Errorf("unknown criteria type: %s", temp.Type)
		}
	}

	return nil
}
