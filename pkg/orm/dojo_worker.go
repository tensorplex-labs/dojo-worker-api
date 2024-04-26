package orm

import (
    "context"
    "dojo-api/db"
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