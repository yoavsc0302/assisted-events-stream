package redis

import (
	"context"
	"os"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

func NewRedisClientFromEnv(ctx context.Context, logger *logrus.Logger) *redis.Client {
	addr := os.Getenv("REDIS_ADDRESS")
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		logger.WithFields(logrus.Fields{
			"addr": addr,
		}).WithError(err).Fatal("could not ping redis server")
	}
	return client
}

func NewSnapshotRepositoryFromEnv(ctx context.Context, logger *logrus.Logger) *SnapshotRepository {
	redis := NewRedisClientFromEnv(ctx, logger)
	return NewSnapshotRepository(logger, redis)
}
