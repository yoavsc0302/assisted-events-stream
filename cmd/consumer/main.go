package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	opensearch "github.com/opensearch-project/opensearch-go"
	"github.com/openshift-assisted/assisted-events-streams/internal/projection"
	opensearch_repo "github.com/openshift-assisted/assisted-events-streams/internal/repository/opensearch"
	redis_repo "github.com/openshift-assisted/assisted-events-streams/internal/repository/redis"
	"github.com/openshift-assisted/assisted-events-streams/internal/utils"
	"github.com/openshift-assisted/assisted-events-streams/pkg/stream"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

const AckChannelBufferSize = 1000

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

func NewEnrichedEventRepositoryFromEnv(logger *logrus.Logger, ackChannel chan kafka.Message) *opensearch_repo.EnrichedEventRepository {
	indexPrefix := os.Getenv("OPENSEARCH_INDEX_PREFIX")
	opensearch := NewOpensearchClientFromEnv(logger)
	return opensearch_repo.NewEnrichedEventRepository(logger, opensearch, indexPrefix, ackChannel)
}

func NewSnapshotRepositoryFromEnv(ctx context.Context, logger *logrus.Logger) *redis_repo.SnapshotRepository {
	redis := NewRedisClientFromEnv(ctx, logger)
	return redis_repo.NewSnapshotRepository(logger, redis)
}

func NewEnrichedEventsProjectionFromEnv(ctx context.Context, logger *logrus.Logger, ackChannel chan kafka.Message) *projection.EnrichedEventsProjection {
	enrichedEventRepository := NewEnrichedEventRepositoryFromEnv(logger, ackChannel)
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
	ackChannel := make(chan kafka.Message, AckChannelBufferSize)
	reader, err := stream.NewKafkaReader(log, ackChannel)
	if err != nil {
		log.WithError(err).Fatal("Could not connect to kafka")
	}

	projection := NewEnrichedEventsProjectionFromEnv(ctx, log, ackChannel)

	err = reader.Consume(ctx, projection.ProcessMessage)
	if err != nil {
		log.WithError(err).Fatal(err)
	}
}
