package orm

import (
	"context"
	"dojo-api/db"
	"encoding/json"
	"time"

	"github.com/rs/zerolog/log"
)

type MetricsORM struct {
	dbClient      *db.PrismaClient
	clientWrapper *PrismaClientWrapper
}

func NewMetricsORM() *MetricsORM {
	clientWrapper := GetPrismaClient()
	return &MetricsORM{
		dbClient:      clientWrapper.Client,
		clientWrapper: clientWrapper,
	}
}

func (orm *MetricsORM) GetMetricsDataByMetricType(ctx context.Context, metricType db.MetricsType) (*db.MetricsModel, error) {
	orm.clientWrapper.BeforeQuery()
	defer orm.clientWrapper.AfterQuery()

	metrics, err := orm.dbClient.Metrics.FindUnique(
		db.Metrics.Type.Equals(metricType),
	).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

func (orm *MetricsORM) CreateNewMetric(ctx context.Context, metricType db.MetricsType, data interface{}) error {
	orm.clientWrapper.BeforeQuery()
	defer orm.clientWrapper.AfterQuery()

	metrics, err := orm.dbClient.Metrics.FindUnique(
		db.Metrics.Type.Equals(metricType),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			return orm.createMetric(ctx, metricType, data)
		}
		return err
	}

	return orm.updateMetric(ctx, metrics, data)
}

func (orm *MetricsORM) createMetric(ctx context.Context, metricType db.MetricsType, data interface{}) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}

	log.Info().Interface("MetricData", data).Msg("Creating new metric")

	_, err = orm.dbClient.Metrics.CreateOne(
		db.Metrics.Type.Set(metricType),
		db.Metrics.MetricsData.Set(dataJSON),
	).Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (orm *MetricsORM) updateMetric(ctx context.Context, metrics *db.MetricsModel, data interface{}) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = orm.dbClient.Metrics.FindUnique(
		db.Metrics.Type.Equals(metrics.Type),
	).Update(
		db.Metrics.MetricsData.Set(dataJSON),
		db.Metrics.UpdatedAt.Set(time.Now()),
	).Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}
