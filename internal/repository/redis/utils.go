package redis

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

const defaultExpirationStr = "720h" // 30 days

func NewRedisClientFromEnv(ctx context.Context, logger *logrus.Logger) *redis.Client {
	addr := os.Getenv("VALKEY_ADDRESS")
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("VALKEY_PASSWORD"),
		DB:       0,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		logger.WithFields(logrus.Fields{
			"addr": addr,
		}).WithError(err).Fatal("could not ping redis compatible server")
	}
	return client
}

func NewSnapshotRepositoryFromEnv(ctx context.Context, logger *logrus.Logger) (*SnapshotRepository, error) {
	redis := NewRedisClientFromEnv(ctx, logger)

	expirationStr := os.Getenv("VALKEY_EXPIRATION")
	if expirationStr == "" {
		expirationStr = defaultExpirationStr
	}

	expiration, err := time.ParseDuration(expirationStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse expiration duration: %w", err)
	}

	return NewSnapshotRepository(logger, redis, expiration), nil
}
