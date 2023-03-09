package opensearch

import (
	"context"

	"github.com/openshift-assisted/assisted-events-streams/internal/types"
	kafka "github.com/segmentio/kafka-go"
)

//go:generate mockgen -source=interfaces.go -package=opensearch -destination=mock_enriched_event_repository.go

type EnrichedEventRepositoryInterface interface {
	Store(ctx context.Context, enrichedEvent *types.EnrichedEvent, msg *kafka.Message) error
	Close(ctx context.Context)
}
