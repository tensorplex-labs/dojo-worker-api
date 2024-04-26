package task

import (
	"context"
	"dojo-api/db"
	"dojo-api/pkg/orm"
	"dojo-api/pkg/task/model"
	"fmt"
	"math"

	"github.com/rs/zerolog/log"
)

type TaskService struct {
	client *db.PrismaClient
}

func NewTaskService() *TaskService {
	return &TaskService{
		client: orm.NewPrismaClient(),
	}
}

// create task

// get task by id
func (taskService *TaskService) GetTaskById(ctx context.Context, id string) (*model.TaskResponse, error) {
	task, err := taskService.client.Task.FindUnique(db.Task.ID.Equals(id)).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error converting string to int64")
		return nil, err
	}
	// Ensure task is not nil if Prisma does not handle not found errors automatically
	if task == nil {
		return nil, fmt.Errorf("no task found with ID %s", id)
	}

	return &model.TaskResponse{
		ID:         task.ID,
		Title:      task.Title,
		Body:       task.Body,
		ExpireAt:   task.ExpireAt,
		Type:       task.Type,
		TaskData:   task.TaskData,
		Status:     task.Status,
		MaxResults: task.MaxResults,
	}, nil
}

// TODO: Implement yieldMin, yieldMax
func (taskService *TaskService) GetTasksByPagination(ctx context.Context, page int, limit int, types []string, sort string) (*model.TaskPagination, error) {
	// Calculate offset based on the page and limit
	offset := (page - 1) * limit

	// Determine the sort order dynamically
	var sortQuery db.TaskOrderByParam
	switch sort {
	case "createdAt":
		sortQuery = db.Task.CreatedAt.Order(db.SortOrderDesc)
	case "numResults":
		sortQuery = db.Task.NumResults.Order(db.SortOrderDesc)
	default:
		sortQuery = db.Task.CreatedAt.Order(db.SortOrderDesc)
	}

	taskTypes := convertStringToTaskType(types)

	tasks, err := taskService.client.Task.FindMany(
		db.Task.Type.In(taskTypes),
		sortQuery,
	).
		Skip(offset).
		Take(limit).
		Exec(ctx)

	if err != nil {
		log.Error().Err(err).Msg("Error getting tasks by pagination")
		return nil, err
	}

	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks found")
	}

	// Convert tasks to TaskResponse model
	var taskResponses []model.TaskResponse
	for _, task := range tasks {
		taskResponse := model.TaskResponse{
			ID:         task.ID,
			Title:      task.Title,
			Body:       task.Body,
			Type:       task.Type,
			ExpireAt:   task.ExpireAt,
			TaskData:   task.TaskData,
			Status:     task.Status,
			MaxResults: task.MaxResults,
		}
		taskResponses = append(taskResponses, taskResponse)
	}

	totalTasks := len(tasks)
	totalPages := int(math.Ceil(float64(totalTasks) / float64(limit)))

	// Construct pagination metadata
	pagination := model.Pagination{
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		TotalItems: totalTasks,
	}

	return &model.TaskPagination{
		Tasks:      taskResponses,
		Pagination: pagination,
	}, nil
}

func convertStringToTaskType(taskTypes []string) []db.TaskType {
	var convertedTypes []db.TaskType
	for _, t := range taskTypes {
		convertedTypes = append(convertedTypes, db.TaskType(t))
	}
	return convertedTypes
}
