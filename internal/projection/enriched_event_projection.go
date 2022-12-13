package projection

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openshift-assisted/assisted-events-streams/internal/repository"
	"github.com/openshift-assisted/assisted-events-streams/internal/types"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

const (
	ClusterEvent  = "Event"
	ClusterState  = "ClusterState"
	HostState     = "HostState"
	InfraEnvState = "InfraEnv"
)

type EnrichedEventsProjection struct {
	logger                  *logrus.Logger
	eventEnricher           EventEnricherInterface
	snapshotRepository      repository.SnapshotRepositoryInterface
	enrichedEventRepository repository.EnrichedEventRepositoryInterface
}

func NewEnrichedEventsProjection(logger *logrus.Logger, snapshotRepo repository.SnapshotRepositoryInterface, enrichedEventRepo repository.EnrichedEventRepositoryInterface) *EnrichedEventsProjection {
	eventEnricher := NewEventEnricher(logger)
	return &EnrichedEventsProjection{
		logger:                  logger,
		eventEnricher:           eventEnricher,
		snapshotRepository:      snapshotRepo,
		enrichedEventRepository: enrichedEventRepo,
	}
}

func (p *EnrichedEventsProjection) ProcessMessage(ctx context.Context, msg *kafka.Message) error {

	event, err := getEventFromMessage(msg)
	if err != nil {
		return err
	}
	return p.ProcessEvent(ctx, event)
}

func (p *EnrichedEventsProjection) ProcessEvent(ctx context.Context, event *types.Event) error {

	var process func(ctx context.Context, event *types.Event) error
	switch event.Name {
	case ClusterEvent:
		process = p.ProcessClusterEvent
	case ClusterState:
		process = p.ProcessClusterState
	case HostState:
		process = p.ProcessHostState
	case InfraEnvState:
		process = p.ProcessInfraEnvState
	default:
		return fmt.Errorf("Unknown event name: %s (%s)", event.Name, event.Payload)
	}
	err := process(ctx, event)
	if _, ok := err.(*MalformedEventError); ok {
		p.logger.WithError(err).Warn("malformed event discarded")
		return nil
	}
	return err
}

func (p *EnrichedEventsProjection) ProcessClusterEvent(ctx context.Context, event *types.Event) error {
	clusterID, err := getValueFromPayload("cluster_id", event.Payload)
	if err != nil {
		return err
	}
	cluster, err := p.snapshotRepository.GetCluster(ctx, clusterID)
	if err != nil {
		p.logger.WithFields(logrus.Fields{
			"cluster_id": clusterID,
		}).WithError(err).Warn("Could not retrieve cluster")
	}

	hosts, err := p.snapshotRepository.GetHosts(ctx, clusterID)
	if err != nil {
		p.logger.WithFields(logrus.Fields{
			"cluster_id": clusterID,
		}).WithError(err).Warn("Could not retrieve hosts")
	}
	infraEnvs, err := p.snapshotRepository.GetInfraEnvs(ctx, clusterID)
	if err != nil {
		p.logger.WithFields(logrus.Fields{
			"cluster_id": clusterID,
		}).WithError(err).Warn("Could not retrieve infraEnvs")
	}

	enrichedEvent := p.eventEnricher.GetEnrichedEvent(event, cluster, hosts, infraEnvs)
	err = p.enrichedEventRepository.Store(ctx, enrichedEvent)
	if err != nil {
		p.logger.WithError(err).Warn("Something went wrong while trying to store the event")
		return err
	}
	p.logger.WithFields(logrus.Fields{
		"event_id": enrichedEvent.ID,
	}).Info("Event stored")
	return nil
}

func (p *EnrichedEventsProjection) ProcessClusterState(ctx context.Context, event *types.Event) error {
	clusterID, err := getValueFromPayload("id", event.Payload)
	if err != nil {
		return err
	}
	return p.snapshotRepository.SetCluster(ctx, clusterID, event)
}

func (p *EnrichedEventsProjection) ProcessHostState(ctx context.Context, event *types.Event) error {
	hostID, err := getValueFromPayload("id", event.Payload)
	if err != nil {
		return err
	}
	clusterID, err := getValueFromPayload("cluster_id", event.Payload)
	if err != nil {
		return err
	}
	return p.snapshotRepository.SetHost(ctx, clusterID, hostID, event)
}

func (p *EnrichedEventsProjection) ProcessInfraEnvState(ctx context.Context, event *types.Event) error {
	infraEnvID, err := getValueFromPayload("id", event.Payload)
	if err != nil {
		return err
	}

	clusterID, err := getValueFromPayload("cluster_id", event.Payload)
	if err != nil {
		return err
	}

	return p.snapshotRepository.SetInfraEnv(ctx, clusterID, infraEnvID, event)
}

func getValueFromPayload(key string, payload interface{}) (string, error) {
	if payload, ok := (payload).(map[string]interface{}); ok {
		if v, ok := payload[key]; ok {
			if value, ok := v.(string); ok {
				return value, nil
			}
		}
	}
	return "", NewMalformedEventError(fmt.Sprintf("Error retrieving key %s from payload (%v)", key, payload))
}

func getEventFromMessage(msg *kafka.Message) (*types.Event, error) {
	event := &types.Event{}
	err := json.Unmarshal(msg.Value, event)
	if err != nil {
		return nil, err
	}
	return event, nil
}
