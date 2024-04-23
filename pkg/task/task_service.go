package task

import (
	"dojo-api/db"
	"dojo-api/pkg/orm"
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


