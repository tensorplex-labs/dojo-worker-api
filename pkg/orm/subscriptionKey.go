package orm

import (
	"context"
	"dojo-api/db"
)

type SubscriptionKeyORM struct {
	dbClient      *db.PrismaClient
	clientWrapper *PrismaClientWrapper
}

func NewSubscriptionKeyORM() *SubscriptionKeyORM {
	clientWrapper := GetPrismaClient()
	return &SubscriptionKeyORM{dbClient: clientWrapper.Client, clientWrapper: clientWrapper}
}

func (o *SubscriptionKeyORM) GetSubscriptionKeyByMinerId(ctx context.Context, minerId string) (*db.SubscriptionKeyModel, error) {
	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()
	subscription, err := o.dbClient.SubscriptionKey.FindFirst(
		db.SubscriptionKey.MinerUserID.Equals(minerId),
	).Exec(ctx)

	return subscription, err
}

func (o *SubscriptionKeyORM) GetSubscriptionByKey(ctx context.Context, key string) (*db.SubscriptionKeyModel, error) {
	o.clientWrapper.BeforeQuery()
	defer o.clientWrapper.AfterQuery()
	subscription, err := o.dbClient.SubscriptionKey.FindFirst(
		db.SubscriptionKey.Key.Equals(key),
	).Exec(ctx)

	return subscription, err
}
