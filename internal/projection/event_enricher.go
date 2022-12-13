package projection

import (
	"github.com/google/uuid"
	"github.com/openshift-assisted/assisted-events-streams/internal/types"
	"github.com/sirupsen/logrus"
)

//go:generate mockgen -source=event_enricher.go -package=projection -destination=mock_event_enricher.go

type EventEnricherInterface interface {
	GetEnrichedEvent(event *types.Event, cluster map[string]interface{}, hosts []map[string]interface{}, infraEnvs []map[string]interface{}) *types.EnrichedEvent
}

type EventEnricher struct {
	logger *logrus.Logger
}

func NewEventEnricher(logger *logrus.Logger) *EventEnricher {
	return &EventEnricher{
		logger: logger,
	}
}

func (e *EventEnricher) GetEnrichedEvent(event *types.Event, cluster map[string]interface{}, hosts []map[string]interface{}, infraEnvs []map[string]interface{}) *types.EnrichedEvent {
	namespace, err := uuid.FromBytes([]byte("abcdefghilmnopqr"))
	if err != nil {
		// seed is wrong not 16 most likely
	}
	message, _ := getValueFromPayload("message", event.Payload)
	event_time, _ := getValueFromPayload("event_time", event.Payload)

	cluster["hosts"] = hosts
	enrichedEvent := &types.EnrichedEvent{}
	enrichedEvent.ID = uuid.NewSHA1(namespace, []byte(message+event_time)).String()
	enrichedEvent.Message = message
	enrichedEvent.Category, _ = getValueFromPayload("category", event.Payload)
	enrichedEvent.EventTime = event_time
	enrichedEvent.Name, _ = getValueFromPayload("name", event.Payload)
	enrichedEvent.RequestID, _ = getValueFromPayload("request_id", event.Payload)
	enrichedEvent.Severity, _ = getValueFromPayload("severity", event.Payload)

	if versions, ok := event.Metadata["versions"]; ok {
		if v, ok := versions.(map[string]string); ok {
			enrichedEvent.Versions = v
		}
	}
	enrichedEvent.Cluster = cluster
	enrichedEvent.InfraEnvs = infraEnvs
	return enrichedEvent

}
