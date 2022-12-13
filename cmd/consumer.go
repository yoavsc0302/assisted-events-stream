package main

import (
	"context"
	"crypto/tls"
	//"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	opensearch "github.com/opensearch-project/opensearch-go"
	"github.com/openshift-assisted/assisted-events-streams/internal/projection"
	"github.com/openshift-assisted/assisted-events-streams/internal/repository"
	"github.com/openshift-assisted/assisted-events-streams/internal/utils"
	"github.com/openshift-assisted/assisted-events-streams/pkg/stream"
	"github.com/sirupsen/logrus"
)

func NewOpensearchClientFromEnv(logger *logrus.Logger) *opensearch.Client {
	addresses := []string{
		os.Getenv("OPENSEARCH_ADDRESS"),
	}
	cfg := opensearch.Config{
		Addresses: addresses,
		Username:  os.Getenv("OPENSEARCH_USERNAME"),
		Password:  os.Getenv("OPENSEARCH_PASSWORD"),
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   10,
			ResponseHeaderTimeout: time.Second,
			DialContext:           (&net.Dialer{Timeout: time.Second}).DialContext,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // When local env
				MinVersion:         tls.VersionTLS11,
			},
		},
	}

	client, err := opensearch.NewClient(cfg)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"addresses": addresses,
		}).WithError(err).Fatal("failed to initialize opensearch client")
	}
	_, err = client.Info()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"addresses": addresses,
		}).WithError(err).Fatal("failed to get info from opensearch server")
	}
	return client

}

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

func NewEnrichedEventRepositoryFromEnv(logger *logrus.Logger) *repository.EnrichedEventRepository {
	indexPrefix := os.Getenv("OPENSEARCH_INDEX_PREFIX")
	opensearch := NewOpensearchClientFromEnv(logger)
	return repository.NewEnrichedEventRepository(logger, opensearch, indexPrefix)
}

func NewSnapshotRepositoryFromEnv(ctx context.Context, logger *logrus.Logger) *repository.SnapshotRepository {
	redis := NewRedisClientFromEnv(ctx, logger)
	return repository.NewSnapshotRepository(logger, redis)
}

func NewEnrichedEventsProjectionFromEnv(ctx context.Context, logger *logrus.Logger) *projection.EnrichedEventsProjection {
	enrichedEventRepository := NewEnrichedEventRepositoryFromEnv(logger)
	snapshotRepository := NewSnapshotRepositoryFromEnv(ctx, logger)
	return projection.NewEnrichedEventsProjection(
		logger,
		snapshotRepository,
		enrichedEventRepository,
	)
}

func main() {
	ctx := context.Background()
	log := utils.NewLogger()
	reader, err := stream.NewKafkaReader(log)
	if err != nil {
		log.WithError(err).Fatal("Could not connect to kafka")
	}

	projection := NewEnrichedEventsProjectionFromEnv(ctx, log)

	err = reader.Consume(ctx, projection.ProcessMessage)
	if err != nil {
		log.WithError(err).Fatal(err)
	}
}
