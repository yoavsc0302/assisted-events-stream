package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	opensearch "github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"github.com/openshift-assisted/assisted-events-streams/internal/types"
	"github.com/sirupsen/logrus"
)

//go:generate mockgen -source=enriched_event_repository.go -package=repository -destination=mock_enriched_event_repository.go

type EnrichedEventRepositoryInterface interface {
	Store(ctx context.Context, enrichedEvent *types.EnrichedEvent) error
}

type EnrichedEventRepository struct {
	indexPrefix string
	opensearch  *opensearch.Client
	logger      *logrus.Logger
}

func NewEnrichedEventRepository(logger *logrus.Logger, opensearch *opensearch.Client, indexPrefix string) *EnrichedEventRepository {
	return &EnrichedEventRepository{
		indexPrefix: indexPrefix,
		opensearch:  opensearch,
		logger:      logger,
	}
}

func (r *EnrichedEventRepository) Store(ctx context.Context, enrichedEvent *types.EnrichedEvent) error {
	jsonEvent, err := json.Marshal(enrichedEvent)
	document := bytes.NewReader(jsonEvent)
	indexName := r.getIndexName(enrichedEvent.EventTime)
	req := opensearchapi.IndexRequest{
		Index:      indexName,
		DocumentID: enrichedEvent.ID,
		Body:       document,
		Timeout:    500 * time.Millisecond,
	}
	r.logger.WithFields(logrus.Fields{
		"request": req,
	}).Debug("Trying to store event")

	response, err := req.Do(ctx, r.opensearch)

	buf := new(strings.Builder)
	io.Copy(buf, response.Body)
	responseBody := buf.String()

	r.logger.WithError(err).WithFields(logrus.Fields{
		"response": response,
		"body":     responseBody,
	}).WithError(err).Debug("Response obtained")
	return err
}

func (r *EnrichedEventRepository) getIndexName(eventTime string) string {
	t, _ := time.Parse(time.RFC3339, eventTime)
	indexSuffix := fmt.Sprintf("%d-%02d", t.Year(), t.Month())

	return r.indexPrefix + indexSuffix
}
