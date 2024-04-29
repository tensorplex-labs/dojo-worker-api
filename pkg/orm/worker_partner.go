package orm

import (
	"context"
	"dojo-api/db"
	"errors"
	"fmt"
)

type WorkerPartnerORM struct {
	dbClient *db.PrismaClient
}

func NewWorkerPartnerORM() *WorkerPartnerORM {
	client := NewPrismaClient()
	return &WorkerPartnerORM{dbClient: client}
}

func (m *WorkerPartnerORM) Create(workerId string, minerId string) (*db.WorkerPartnerModel, error) {
	ctx := context.Background()

	dojoWorker, err := m.dbClient.DojoWorker.FindUnique(
		db.DojoWorker.ID.Equals(workerId),
	).Exec(ctx)
	if err != nil && errors.Is(err, db.ErrNotFound) {
		return nil, fmt.Errorf("worker with ID %s not found", workerId)
	}

	miner, err := m.dbClient.MinerUser.FindUnique(
		db.MinerUser.ID.Equals(minerId),
	).Exec(ctx)
	if err != nil && errors.Is(err, db.ErrNotFound) {
		return nil, fmt.Errorf("miner with ID %s not found", minerId)
	}

	workerPartner, err := m.dbClient.WorkerPartner.CreateOne(
		db.WorkerPartner.MinerUser.Link(
			db.MinerUser.ID.Equals(miner.ID),
		),
		db.WorkerPartner.DojoWorker.Link(
			db.DojoWorker.ID.Equals(dojoWorker.ID),
		),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return workerPartner, nil
}
