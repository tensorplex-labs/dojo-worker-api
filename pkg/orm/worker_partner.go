package orm

import (
	"context"
	"dojo-api/db"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

type WorkerPartnerORM struct {
	dbClient      *db.PrismaClient
	clientWrapper *PrismaClientWrapper
}

func NewWorkerPartnerORM() *WorkerPartnerORM {
	clientWrapper := GetPrismaClient()
	return &WorkerPartnerORM{dbClient: clientWrapper.Client, clientWrapper: clientWrapper}
}

func (m *WorkerPartnerORM) Create(workerId string, minerId string, optionalName string) (*db.WorkerPartnerModel, error) {
	m.clientWrapper.BeforeQuery()
	defer m.clientWrapper.AfterQuery()

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

func (m *WorkerPartnerORM) UpdateSubscriptionKey(workerId string, minerSubscriptionKey string, newMinerSubscriptionKey string, name string) (*db.WorkerPartnerModel, error) {
	m.clientWrapper.BeforeQuery()
	defer m.clientWrapper.AfterQuery()
	ctx := context.Background()

	type RawQueryParams struct {
		Name                    *string
		NewMinerSubscriptionKey *string
		TargetID                *string
	}

	var rawQueryParams RawQueryParams
	var existingWorkerPartner *db.WorkerPartnerModel
	var err error
	if minerSubscriptionKey != "" {
		existingWorkerPartner, err = m.dbClient.WorkerPartner.FindFirst(
			db.WorkerPartner.MinerSubscriptionKey.Equals(minerSubscriptionKey),
			db.WorkerPartner.WorkerID.Equals(workerId),
		).Exec(ctx)
		if err != nil || existingWorkerPartner == nil {
			return nil, fmt.Errorf("no existing miner key: %s with this worker", minerSubscriptionKey)
		}
	}

	if name != "" {
		rawQueryParams.Name = &name
	}

	if newMinerSubscriptionKey != "" {
		rawQueryParams.NewMinerSubscriptionKey = &newMinerSubscriptionKey
	}

	if rawQueryParams.NewMinerSubscriptionKey == nil && rawQueryParams.Name == nil {
		return nil, fmt.Errorf("no update parameters provided")
	}

	count := 1
	statements := make([]string, 0)
	params := make([]interface{}, 0)
	if rawQueryParams.Name != nil {
		statements = append(statements, fmt.Sprintf(`name = $%d`, count))
		params = append(params, *rawQueryParams.Name)
		count++
	}

	if rawQueryParams.NewMinerSubscriptionKey != nil {
		statements = append(statements, fmt.Sprintf(`miner_subscription_key = $%d`, count))
		params = append(params, *rawQueryParams.NewMinerSubscriptionKey)
		count++
	}

	params = append(params, existingWorkerPartner.ID)

	sep := " , "
	setStatement := strings.TrimSuffix(strings.Join(statements, sep), sep)
	preparedStatement := fmt.Sprintf(`UPDATE "WorkerPartner" SET %s WHERE id = $%d`, setStatement, count)

	log.Info().Msgf("Prepared statement: %s", preparedStatement)
	log.Info().Msgf("Params: %v", params)

	execResult, err := m.dbClient.Prisma.ExecuteRaw(preparedStatement, params...).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update worker partner: %w", err)
	}

	log.Info().Msgf("Updated %d worker partner records", execResult.Count)

	// Assuming only one worker partner is updated, fetch the updated record
	updatedRecord, err := m.dbClient.WorkerPartner.FindFirst(
		db.WorkerPartner.ID.Equals(existingWorkerPartner.ID),
	).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated worker partner: %w", err)
	}
	return updatedRecord, nil
}

func (m *WorkerPartnerORM) DisablePartnerByWorker(workerId string, minerSubscriptionKey string, toDisable bool) (int, error) {
	m.clientWrapper.BeforeQuery()
	defer m.clientWrapper.AfterQuery()

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
		log.Error().Err(err).Msg("failed to update worker partner")
		return 0, fmt.Errorf("failed to update worker partner: %w", err)
	}

	log.Info().Msgf("Updated %d worker partner records", result.Count)
	return result.Count, nil
}

func (m *WorkerPartnerORM) DisablePartnerByMiner(workerId string, minerSubscriptionKey string, toDisable bool) (int, error) {
	m.clientWrapper.BeforeQuery()
	defer m.clientWrapper.AfterQuery()

	ctx := context.Background()

	filterParams := []db.WorkerPartnerWhereParam{
		db.WorkerPartner.WorkerID.Equals(workerId),
		db.WorkerPartner.MinerSubscriptionKey.Equals(minerSubscriptionKey),
	}

	result, err := m.dbClient.WorkerPartner.FindMany(
		filterParams...,
	).Update(
		db.WorkerPartner.IsDeleteByMiner.Set(toDisable),
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to update worker partner while disabling by miner")
		return 0, fmt.Errorf("failed to update worker partner while disabling by miner %w", err)
	}

	log.Info().Msgf("Updated %d worker partner records, disabling by miner", result.Count)
	return result.Count, nil
}

func (m *WorkerPartnerORM) GetWorkerPartnerByWorkerId(workerId string) ([]db.WorkerPartnerModel, error) {
	m.clientWrapper.BeforeQuery()
	defer m.clientWrapper.AfterQuery()

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
	m.clientWrapper.BeforeQuery()
	defer m.clientWrapper.AfterQuery()

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
