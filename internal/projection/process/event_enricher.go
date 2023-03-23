package process

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/openshift-assisted/assisted-events-streams/internal/types"
	"github.com/openshift-assisted/assisted-events-streams/pkg/jsonedit"
	"github.com/sirupsen/logrus"
)

type EventEnricher struct {
	logger                 *logrus.Logger
	fieldsToUnpack         []string
	fieldsMapToList        []string
	fieldsMapToListDropKey []string
	fieldsToDelete         []string
	fieldsToAnonymize      map[string]string
}

func NewEventEnricher(logger *logrus.Logger) *EventEnricher {
	return &EventEnricher{
		logger:                 logger,
		fieldsToUnpack:         getDefaultFieldsToUnpack(),
		fieldsMapToList:        getDefaultFieldsMapToList(),
		fieldsMapToListDropKey: getDefaultFieldsMapToListDropKey(),
		fieldsToDelete:         getDefaultFieldsToDelete(),
		fieldsToAnonymize:      getDefaultFieldsToAnonymize(),
	}
}

func (e *EventEnricher) GetEnrichedEvent(event *types.Event, cluster map[string]interface{}, hosts []map[string]interface{}, infraEnvs []map[string]interface{}) *types.EnrichedEvent {
	enrichedEvent, err := GetBaseEnrichedEvent(event, cluster, hosts, infraEnvs)
	if err != nil {
		e.logger.WithError(err).Debug("Error creating base enriched event")
	}
	return e.getTransformedEvent(enrichedEvent)
}

// Get event after having applied all required transformations
func (e *EventEnricher) getTransformedEvent(enrichedEvent *types.EnrichedEvent) *types.EnrichedEvent {
	originalJson, err := json.Marshal(enrichedEvent)
	if err != nil {
		e.logger.WithError(err).Debug("error marshaling enriched event")
		return enrichedEvent
	}

	unpackedJson, err := unpackJson(originalJson, e.fieldsToUnpack)
	if err != nil {
		e.logger.WithError(err).Debug("error unpacking json")
	}
	transformedJson, err := mapToListJsonDropKey(unpackedJson, e.fieldsMapToListDropKey)
	if err != nil {
		e.logger.WithError(err).Debug("error transforming json")
	}
	transformedJson, err = mapToListJson(transformedJson, e.fieldsMapToList)
	if err != nil {
		e.logger.WithError(err).Debug("error transforming json")
	}
	anonymizedJson, err := anonymizeJson(transformedJson, e.fieldsToAnonymize)
	if err != nil {
		e.logger.WithError(err).Debug("error anonymizing json")
	}
	deletedJson, err := jsonedit.Delete(anonymizedJson, e.fieldsToDelete)
	if err != nil {
		e.logger.WithError(err).Debug("error deleting json")
	}
	withHostsSummaryJson, err := e.addHostsSummaryJson(deletedJson)
	if err != nil {
		e.logger.WithError(err).Debug("error adding summary json")
	}

	return e.getEnrichedEventFromJson(withHostsSummaryJson)
}

func (e *EventEnricher) addHostsSummaryJson(eventJson []byte) ([]byte, error) {
	event := e.getEnrichedEventFromJson(eventJson)
	err := AddHostsSummary(event)
	if err != nil {
		return eventJson, err
	}
	outJson, err := json.Marshal(event)
	if err != nil {
		return outJson, err
	}
	return outJson, nil
}

// Get enriched event before applying any transformation
func GetBaseEnrichedEvent(event *types.Event, cluster map[string]interface{}, hosts []map[string]interface{}, infraEnvs []map[string]interface{}) (*types.EnrichedEvent, error) {
	namespace, err := uuid.FromBytes([]byte("abcdefghilmnopqr"))
	if err != nil {
		return nil, fmt.Errorf("Error getting uuid namespace, most likely seed is not 16 characters")
	}

	enrichedEvent := &types.EnrichedEvent{}
	enrichedEvent.Event.Properties = getProps(event.Payload)
	eventBytes, err := json.Marshal(event.Payload)
	if err != nil {
		return enrichedEvent, err
	}
	err = json.Unmarshal(eventBytes, enrichedEvent)
	cluster["hosts"] = getHostsWithEmbeddedInfraEnv(hosts, infraEnvs)

	enrichedEvent.ID = uuid.NewSHA1(namespace, []byte(enrichedEvent.Message+enrichedEvent.EventTime)).String()

	if versions, ok := event.Metadata["versions"]; ok {
		if v, ok := versions.(map[string]interface{}); ok {
			enrichedEvent.Versions = v
		}
	}
	enrichedEvent.Cluster = cluster
	enrichedEvent.InfraEnvs = infraEnvs
	return enrichedEvent, err
}

func getHostsWithEmbeddedInfraEnv(hosts []map[string]interface{}, infraEnvs []map[string]interface{}) []map[string]interface{} {
	for i, host := range hosts {
		if infraEnvID, ok := host["infra_env_id"]; ok {
			for _, infraEnv := range infraEnvs {
				if infraEnv["id"] == infraEnvID {
					hosts[i]["infra_env"] = infraEnv
				}
			}
		}
	}
	return hosts
}

func (e *EventEnricher) getEnrichedEventFromJson(eventJson []byte) *types.EnrichedEvent {
	var outEvent types.EnrichedEvent
	err := json.Unmarshal(eventJson, &outEvent)
	if err != nil {
		e.logger.WithError(err).Debug("error unmarshaling json to enriched event")
	}
	return &outEvent
}

func getProps(eventPayload interface{}) map[string]interface{} {
	defaultProps := map[string]interface{}{}
	if payload, ok := eventPayload.(map[string]interface{}); ok {
		if rawProps, ok := payload["props"]; ok {
			if p, ok := rawProps.(string); ok {
				structuredProps := map[string]interface{}{}
				json.Unmarshal([]byte(p), &structuredProps)
				return structuredProps
			}
		}
	}
	return defaultProps
}
