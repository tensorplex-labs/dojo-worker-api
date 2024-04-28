package orm

import (
	"context"
	"dojo-api/db"
	"errors"

	"github.com/rs/zerolog/log"
)

type DojoWorkerService struct {
	dbClient *db.PrismaClient
}

func NewDojoWorkerService() *DojoWorkerService {
	client := NewPrismaClient()
	return &DojoWorkerService{dbClient: client}
}

func (s *DojoWorkerService) CreateDojoWorker(walletAddress string, chainId string) (*db.DojoWorkerModel, error) {
	ctx := context.Background()
	worker, err := s.dbClient.DojoWorker.CreateOne(
		db.DojoWorker.WalletAddress.Set(walletAddress),
		db.DojoWorker.ChainID.Set(chainId),
	).Exec(ctx)
	return worker, err
}

func (s *DojoWorkerService) GetDojoWorkerByWalletAddress(walletAddress string) (*db.DojoWorkerModel, error) {
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
