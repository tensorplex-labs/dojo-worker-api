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
	client := GetPrismaClient()
	return &WorkerPartnerORM{dbClient: client}
}

func (m *WorkerPartnerORM) Create(workerId string, minerId string, optionalName string) (*db.WorkerPartnerModel, error) {
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
		db.WorkerPartner.Name.Set(optionalName),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return workerPartner, nil
}

func (m *WorkerPartnerORM) Update(workerId string, minerSubscriptionKey string, newMinerSubscriptionKey string, name string) (*db.WorkerPartnerModel, error) {
	ctx := context.Background()

	var updateParams []db.WorkerPartnerSetParam

	if minerSubscriptionKey != "" {
		existingWorkerPartner, err := m.dbClient.WorkerPartner.FindFirst(
			db.WorkerPartner.MinerSubscriptionKey.Equals(minerSubscriptionKey),
			db.WorkerPartner.WorkerID.Equals(workerId),
		).Exec(ctx)
		if err != nil || existingWorkerPartner == nil {
			return nil, fmt.Errorf("no existing miner key: %s with this worker", minerSubscriptionKey)
		}

		if name != "" {
			updateParams = append(updateParams, db.WorkerPartner.Name.Set(name))
		}

		if newMinerSubscriptionKey != "" {
			updateParams = append(updateParams, db.WorkerPartner.MinerSubscriptionKey.Set(newMinerSubscriptionKey))
		}
	}

	if len(updateParams) > 0 {
		updatedWorkerPartner, err := m.dbClient.WorkerPartner.FindUnique(
			db.WorkerPartner.ID.Equals(workerId),
		).Update(
			updateParams...,
		).Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to update worker partner: %w", err)
		}
		return updatedWorkerPartner, nil
	} else {
		return nil, fmt.Errorf("no update parameters provided")
	}
}

func (m *WorkerPartnerORM) WorkerPartnerDisableUpdate(workerId string, minerSubscriptionKey string, toDisable bool) (int, error) {
	ctx := context.Background()

	updateParams := []db.WorkerPartnerSetParam{
		db.WorkerPartner.IsDeleteByWorker.Set(toDisable),
	}

	result, err := m.dbClient.WorkerPartner.FindMany(
		db.WorkerPartner.WorkerID.Equals(workerId),
		db.WorkerPartner.MinerSubscriptionKey.Equals(minerSubscriptionKey),
	).Update(
		updateParams...,
	).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to update worker partner: %w", err)
	}

	return result.Count, nil
}

func (m *WorkerPartnerORM) GetWorkerPartnerByWorkerId(workerId string) ([]db.WorkerPartnerModel, error) {
	ctx := context.Background()

	workerPartners, err := m.dbClient.WorkerPartner.FindMany(
		db.WorkerPartner.WorkerID.Equals(workerId),
	).Exec(ctx)
	if err != nil && errors.Is(err, db.ErrNotFound) {
		return nil, fmt.Errorf("worker partners with worker ID %s not found", workerId)
	}
	return workerPartners, nil
}

func (m *WorkerPartnerORM) GetWorkerPartnerByWorkerIdAndSubscriptionKey(workerId string, minerSubscriptionKey string) (*db.WorkerPartnerModel, error) {
	ctx := context.Background()
	workerPartner, err := m.dbClient.WorkerPartner.FindFirst(
		db.WorkerPartner.MinerSubscriptionKey.Equals(minerSubscriptionKey),
		db.WorkerPartner.WorkerID.Equals(workerId),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return workerPartner, nil
}
