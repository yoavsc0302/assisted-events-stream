package projection

import (
	"context"

	opensearch_repo "github.com/openshift-assisted/assisted-events-streams/internal/repository/opensearch"
	redis_repo "github.com/openshift-assisted/assisted-events-streams/internal/repository/redis"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

func NewEnrichedEventsProjectionFromEnv(ctx context.Context, logger *logrus.Logger, ackChannel chan kafka.Message) *EnrichedEventsProjection {
	enrichedEventRepository := opensearch_repo.NewEnrichedEventRepositoryFromEnv(logger, ackChannel)
	snapshotRepository := redis_repo.NewSnapshotRepositoryFromEnv(ctx, logger)
	return NewEnrichedEventsProjection(
		logger,
		snapshotRepository,
		enrichedEventRepository,
	)
}
