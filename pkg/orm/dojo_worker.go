package orm

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"dojo-api/db"

	"github.com/rs/zerolog/log"
)

type DojoWorkerORM struct {
	dbClient      *db.PrismaClient
	clientWrapper *PrismaClientWrapper
}

func NewDojoWorkerORM() *DojoWorkerORM {
	clientWrapper := GetPrismaClient()
	return &DojoWorkerORM{dbClient: clientWrapper.Client, clientWrapper: clientWrapper}
}

func (s *DojoWorkerORM) CreateDojoWorker(walletAddress string, chainId string) (*db.DojoWorkerModel, error) {
	s.clientWrapper.BeforeQuery()
	defer s.clientWrapper.AfterQuery()

	ctx := context.Background()
	worker, err := s.dbClient.DojoWorker.CreateOne(
		db.DojoWorker.WalletAddress.Set(walletAddress),
		db.DojoWorker.ChainID.Set(chainId),
	).Exec(ctx)
	return worker, err
}

func (s *DojoWorkerORM) GetDojoWorkerByWalletAddress(walletAddress string) (*db.DojoWorkerModel, error) {
	s.clientWrapper.BeforeQuery()
	defer s.clientWrapper.AfterQuery()

	ctx := context.Background()
	worker, err := s.dbClient.DojoWorker.FindFirst(
		db.DojoWorker.WalletAddress.Equals(walletAddress),
	).Exec(ctx)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			log.Error().Err(err).Msg("Worker not found")
			return nil, err
		}
		return nil, err
	}
	return worker, nil
}

func (s *DojoWorkerORM) GetDojoWorkers() (int, error) {
	s.clientWrapper.BeforeQuery()
	defer s.clientWrapper.AfterQuery()

	ctx := context.Background()
	var result []struct {
		Count db.RawString `json:"count"`
	}

	query := "SELECT COUNT(*) as count FROM \"DojoWorker\";"
	err := s.clientWrapper.Client.Prisma.QueryRaw(query).Exec(ctx, &result)

	if err != nil {
		return 0, err
	}

	if len(result) == 0 {
		return 0, fmt.Errorf("no results found from getting dojoWorker count")
	}

	workerCountStr := string(result[0].Count)
	workerCountInt, err := strconv.Atoi(workerCountStr)

	if err != nil {
		return 0, err
	}

	return workerCountInt, nil
}
