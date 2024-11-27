package orm

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"dojo-api/db"

	"dojo-api/pkg/cache"

	"github.com/rs/zerolog/log"
)

type DojoWorkerCacheKey string

const (
	WorkerByWalletCacheKey DojoWorkerCacheKey = "worker_by_wallet" // Short key for worker by wallet
	WorkerCountCacheKey    DojoWorkerCacheKey = "worker_count"     // Short key for worker count
)

type DojoWorkerCache struct {
	key           DojoWorkerCacheKey
	walletAddress string
}

func NewDojoWorkerCache(key DojoWorkerCacheKey) *DojoWorkerCache {
	return &DojoWorkerCache{
		key: key,
	}
}

func (dc *DojoWorkerCache) GetCacheKey() string {
	switch dc.key {
	case WorkerByWalletCacheKey:
		return fmt.Sprintf("%s:%s", dc.key, dc.walletAddress)
	case WorkerCountCacheKey:
		return string(dc.key)
	default:
		return fmt.Sprintf("dw:%s", dc.walletAddress)
	}
}

func (dc *DojoWorkerCache) GetExpiration() time.Duration {
	switch dc.key {
	case WorkerByWalletCacheKey:
		return 5 * time.Minute
	case WorkerCountCacheKey:
		return 1 * time.Minute
	default:
		return 3 * time.Minute
	}
}

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
	workerCache := NewDojoWorkerCache(WorkerByWalletCacheKey)
	workerCache.walletAddress = walletAddress

	var worker *db.DojoWorkerModel
	cache := cache.GetCacheInstance()

	// Try to get from cache first
	if err := cache.GetCache(workerCache, &worker); err == nil {
		return worker, nil
	}

	// Cache miss, fetch from database
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

	// Store in cache
	if err := cache.SetCache(workerCache, worker); err != nil {
		log.Warn().Err(err).Msg("Failed to set worker cache")
	}

	return worker, nil
}

func (s *DojoWorkerORM) GetDojoWorkers() (int, error) {
	workerCache := NewDojoWorkerCache(WorkerCountCacheKey)
	var count int
	cache := cache.GetCacheInstance()

	// Try to get from cache first
	if err := cache.GetCache(workerCache, &count); err == nil {
		return count, nil
	}

	// Cache miss, fetch from database
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
	count, err = strconv.Atoi(workerCountStr)
	if err != nil {
		return 0, err
	}

	// Store in cache
	if err := cache.SetCache(workerCache, count); err != nil {
		log.Warn().Err(err).Msg("Failed to set worker count cache")
	}

	return count, nil
}
