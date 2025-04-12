package orm

import (
	"context"
	"dojo-api/db"
	"encoding/json"
)

// import cycle not allowed
type EventsORM struct {
	dbClient      *db.PrismaClient
	clientWrapper *PrismaClientWrapper
}

func NewEventsORM() *EventsORM {
	clientWrapper := GetPrismaClient()
	return &EventsORM{
		dbClient:      clientWrapper.Client,
		clientWrapper: clientWrapper,
	}
}

func (o *EventsORM) CreateEventByType(ctx context.Context, eventType db.EventsType, data interface{}) error {
	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()

	eventData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = o.dbClient.Events.CreateOne(
		db.Events.Type.Set(eventType),
		db.Events.EventsData.Set(eventData),
	).Exec(ctx)

	return err
}

func (o *EventsORM) GetEventsByType(ctx context.Context, eventType db.EventsType) ([]db.EventsModel, error) {
	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()

	events, err := o.dbClient.Events.FindMany(db.Events.Type.Equals(eventType)).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return events, nil
}

func (o *EventsORM) GetAverageTaskCompletionTime(ctx context.Context) (int, error) {
	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()

	var avgTime int
	query := `SELECT AVG(CAST(events_data->>'task_completion_time' AS INTEGER)) FROM "Events" WHERE type = 'task_completion_time'`

	err := o.clientWrapper.Client.Prisma.QueryRaw(query).Exec(ctx, &avgTime)
	if err != nil {
		return 0, err
	}

	return avgTime, nil
}
