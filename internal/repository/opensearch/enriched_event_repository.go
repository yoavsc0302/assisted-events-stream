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

type EnrichedEventRepository struct {
	indexPrefix string
	opensearch  *opensearch.Client
	bulk        opensearchutil.BulkIndexer
	logger      *logrus.Logger
	ackChannel  chan kafka.Message
}

func NewEnrichedEventRepository(logger *logrus.Logger, opensearch *opensearch.Client, indexPrefix string, ackChannel chan kafka.Message) *EnrichedEventRepository {
	bulkIndexer, err := NewBulkIndexerFromEnv(opensearch, logger)
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
				"item":        item,
				"action":      item.Action,
				"index":       item.Index,
				"response":    resp,
			}).Error("error bulk indexing document")
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
