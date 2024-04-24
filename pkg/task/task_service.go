package task

import (
	"context"
	"dojo-api/db"
	"dojo-api/pkg/orm"
	"dojo-api/pkg/task/model"
	"fmt"
)

type TaskService struct {
	client *db.PrismaClient
}

func NewTaskService() *TaskService {
	return &TaskService{
		client: orm.NewPrismaClient(),
	}
}

// TODO: Implement Error Handling Properly
func (taskService *TaskService) GetTaskById(ctx context.Context, id string) (*model.TaskResponse, error) {
	task, err := taskService.client.Task.FindUnique(db.Task.ID.Equals(id)).Exec(ctx)
	if err != nil {
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
		Modality:   task.Modality,
		ExpireAt:   task.ExpireAt,
		Criteria:   task.Criteria,
		TaskData:   task.TaskData,
		Status:     task.Status,
		MaxResults: task.MaxResults,
	}, nil
}

// GetTasks method of TaskService to fetch paginated tasks
func (taskService *TaskService) GetTasks(ctx context.Context, pagination model.Pagination, filter model.TaskFilter, sortBy string) ([]*model.TaskResponse, model.Pagination, error) {
	// Define the default sorting order (if sortBy parameter is empty)
	defaultSortOrder := db.Task.CreatedAt.Desc()

	// Map the sortBy parameter to Prisma field names
	var sortField db.TaskOrderBy
	switch sortBy {
	case "createdAt":
		sortField = db.Task.CreatedAt
	case "numResults":
		sortField = db.Task.NumResults
	case "highestYield":
		sortField = db.Task.TotalYield
	default:
		// Use the default sorting order if sortBy parameter is not recognized
		sortField = defaultSortOrder
	}

	// Fetch tasks from Prisma based on pagination, filter, and sorting parameters
	tasks, err := taskService.client.Task.FindMany(
		db.Task.Status.Equals(filter.Status),                        // Example filter by status
		db.Task.Modality.In(filter.Modalities),                      // Example filter by modalities
		db.Task.OrderBy(sortField),                                  // Sorting by the specified field
		db.Task.Skip((pagination.PageNumber-1)*pagination.PageSize), // Pagination offset
		db.Task.Take(pagination.PageSize),                           // Pagination limit
	).Exec(ctx)

	if err != nil {
		return nil, Pagination{}, err
	}

	// Convert tasks to TaskResponse format
	taskResponses := make([]*model.TaskResponse, len(tasks))
	for i, task := range tasks {
		taskResponses[i] = &model.TaskResponse{
			ID:         task.ID,
			Title:      task.Title,
			Body:       task.Body,
			Modality:   task.Modality,
			ExpireAt:   task.ExpireAt,
			Criteria:   task.Criteria,
			TaskData:   task.TaskData,
			Status:     task.Status,
			MaxResults: task.MaxResults,
		}
	}

	// Calculate total number of items
	totalItems, err := taskService.client.Task.Count(
		db.Task.Status.Equals(filter.Status),   // Example filter by status
		db.Task.Modality.In(filter.Modalities), // Example filter by modalities
	).Exec(ctx)
	if err != nil {
		return nil, Pagination{}, err
	}

	// Calculate total number of pages
	totalPages := (totalItems + pagination.PageSize - 1) / pagination.PageSize

	// Create pagination data
	paginationData := Pagination{
		PageNumber: pagination.PageNumber,
		PageSize:   pagination.PageSize,
		TotalPages: totalPages,
		TotalItems: totalItems,
	}

	return taskResponses, paginationData, nil
}
