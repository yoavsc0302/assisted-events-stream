package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	opensearch "github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchutil"
	"github.com/openshift-assisted/assisted-events-streams/internal/types"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

const (
	NumWorkers    = 1
	BulkTimeout   = time.Second * 20
	FlushInterval = time.Second * 30
	MaxBytes      = 10e6 // Flush records after this size
)

//go:generate mockgen -source=enriched_event_repository.go -package=opensearch -destination=mock_enriched_event_repository.go

type EnrichedEventRepositoryInterface interface {
	Store(ctx context.Context, enrichedEvent *types.EnrichedEvent, msg *kafka.Message) error
	Close(ctx context.Context)
}

type EnrichedEventRepository struct {
	indexPrefix string
	opensearch  *opensearch.Client
	bulk        opensearchutil.BulkIndexer
	logger      *logrus.Logger
	ackChannel  chan kafka.Message
}

func NewEnrichedEventRepository(logger *logrus.Logger, opensearch *opensearch.Client, indexPrefix string, ackChannel chan kafka.Message) *EnrichedEventRepository {
	bulkIndexer, err := opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{
		NumWorkers:    NumWorkers,
		Client:        opensearch,
		FlushBytes:    MaxBytes,
		FlushInterval: FlushInterval,
		Refresh:       "false",
		Timeout:       BulkTimeout,
		OnError: func(ctx context.Context, err error) {
			logger.Println(fmt.Errorf("bulk item indexer failed %w", err))
		},
		OnFlushStart: func(ctx context.Context) context.Context {
			logger.Info("Starting to flush...")
			return ctx
		},
		OnFlushEnd: func(ctx context.Context) {
			logger.Info("Flushed.")
		},
	})
	if err != nil {
		logger.WithError(err).Warning("error initializing bulk indexer")
	}

	return &EnrichedEventRepository{
		indexPrefix: indexPrefix,
		opensearch:  opensearch,
		bulk:        bulkIndexer,
		logger:      logger,
		ackChannel:  ackChannel,
	}
}

func (r *EnrichedEventRepository) Close(ctx context.Context) {
	r.bulk.Close(context.Background())
}

func (r *EnrichedEventRepository) Store(ctx context.Context, enrichedEvent *types.EnrichedEvent, msg *kafka.Message) error {
	r.logger.WithFields(logrus.Fields{
		"id": enrichedEvent.ID,
	}).Debug("adding event to bulk indexer")

	jsonEvent, err := json.Marshal(enrichedEvent)
	if err != nil {
		return err
	}
	document := bytes.NewReader(jsonEvent)

	item := opensearchutil.BulkIndexerItem{
		Index:      r.getIndexName(enrichedEvent.EventTime),
		DocumentID: enrichedEvent.ID,
		Action:     "index",
		Body:       document,
		OnSuccess: func(context.Context, opensearchutil.BulkIndexerItem, opensearchutil.BulkIndexerResponseItem) {
			r.ackChannel <- *msg
		},
		OnFailure: func(ctx context.Context, item opensearchutil.BulkIndexerItem, resp opensearchutil.BulkIndexerResponseItem, err error) {
			r.logger.WithError(err).WithFields(logrus.Fields{
				"document_id": item.DocumentID,
				"action":      item.Action,
				"index":       item.Index,
				"response":    resp,
			}).Debug("error bulk indexing document")
		},
	}
	r.bulk.Add(ctx, item)

	r.logger.WithFields(logrus.Fields{
		"id":    enrichedEvent.ID,
		"stats": r.bulk.Stats(),
	}).Debug("added event to bulk indexer")

	return nil
}

func (r *EnrichedEventRepository) getIndexName(eventTime string) string {
	t, _ := time.Parse(time.RFC3339, eventTime)
	indexSuffix := fmt.Sprintf("%d-%02d", t.Year(), t.Month())

	return r.indexPrefix + indexSuffix
}
