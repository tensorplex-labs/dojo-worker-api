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

func (m *WorkerPartnerORM) Create(workerId string, minerId string, name string) (*db.WorkerPartnerModel, error) {
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
		db.WorkerPartner.Name.Set(name),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return workerPartner, nil
}

func (m *WorkerPartnerORM) Update(workerId string, payload map[string]interface{}) (*db.WorkerPartnerModel, error) {
	ctx := context.Background()

	// Prepare update parameters
	var updateParams []db.WorkerPartnerSetParam

	if name, ok := payload["name"].(string); ok && name != "" {
		updateParams = append(updateParams, db.WorkerPartner.Name.Set(name))
	}

	if minerSubscriptionKey, ok := payload["miner_subscription_key"].(string); ok && minerSubscriptionKey != "" {
		// Check if the miner subscription key exists in the WorkerPartner table
		existingWorkerPartner, err := m.dbClient.WorkerPartner.FindFirst(
			db.WorkerPartner.MinerSubscriptionKey.Equals(minerSubscriptionKey),
			db.WorkerPartner.WorkerID.Equals(workerId),
		).Exec(ctx)
		if err != nil || existingWorkerPartner == nil {
			return nil, fmt.Errorf("no existing miner key: %s with this worker", minerSubscriptionKey)
		}
		// if existingWorkerPartner == nil {
		// 	return nil, fmt.Errorf("miner subscription key %s does not exist", minerSubscriptionKey)
		// }
		if newMinerSubscriptionKey, ok := payload["new_miner_subscription_key"].(string); ok && newMinerSubscriptionKey != "" {
			updateParams = append(updateParams, db.WorkerPartner.MinerSubscriptionKey.Set(newMinerSubscriptionKey))
		}
	}

	// Execute update
	updatedWorkerPartner, err := m.dbClient.WorkerPartner.FindUnique(
		db.WorkerPartner.ID.Equals(workerId),
	).Update(
		updateParams...,
	).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update worker partner: %w", err)
	}

	return updatedWorkerPartner, nil
}

func (m *WorkerPartnerORM) WorkerParnterDisableUpdate(payload map[string]interface{}) (int, error) {
	ctx := context.Background()

	workerId, ok := payload["workerId"].(string)
	if !ok {
		return 0, fmt.Errorf("workerId is required and must be a string")
	}

	minerSubscriptionKey, ok := payload["minerSubscriptionKey"].(string)
	if !ok {
		return 0, fmt.Errorf("minerSubscriptionKey is required and must be a string")
	}

	toDisable, ok := payload["toDisable"].(bool)
	if !ok || !toDisable {
		return 0, fmt.Errorf("toDisable is required and must be true")
	}

	updateParams := []db.WorkerPartnerSetParam{
		db.WorkerPartner.IsDeleteByWorker.Set(true),
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
