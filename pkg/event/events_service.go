package event

import (
	"context"
	"dojo-api/db"
	"dojo-api/pkg/orm"
	"time"
)

type EventService struct {
	eventsORM *orm.EventsORM
}

func NewEventService() *EventService {
	return &EventService{
		eventsORM: orm.NewEventsORM(),
	}
}

func (o *EventService) CreateTaskCompletionEvent(ctx context.Context, task db.TaskModel) error {
	taskCompletionTime := int(time.Since(task.CreatedAt).Seconds())

	eventData := EventTaskCompletionTime{TaskId: task.ID, TaskCompletionTime: taskCompletionTime}

	eventsORM := orm.NewEventsORM()
	err := eventsORM.CreateEventByType(ctx, db.EventsTypeTaskCompletionTime, eventData)

	return err
}
