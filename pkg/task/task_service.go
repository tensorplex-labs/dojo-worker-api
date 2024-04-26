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

// create task

// get task by id
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
