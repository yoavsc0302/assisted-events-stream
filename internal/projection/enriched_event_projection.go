package projection

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openshift-assisted/assisted-events-streams/internal/projection/process"
	opensearch_repo "github.com/openshift-assisted/assisted-events-streams/internal/repository/opensearch"
	redis_repo "github.com/openshift-assisted/assisted-events-streams/internal/repository/redis"
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

//go:generate mockgen -source=enriched_event_projection.go -package=projection -destination=mock_event_enricher.go

type EventEnricherInterface interface {
	GetEnrichedEvent(event *types.Event, cluster map[string]interface{}, hosts []map[string]interface{}, infraEnvs []map[string]interface{}) *types.EnrichedEvent
}

type EnrichedEventsProjection struct {
	logger                  *logrus.Logger
	eventEnricher           EventEnricherInterface
	snapshotRepository      redis_repo.SnapshotRepositoryInterface
	enrichedEventRepository opensearch_repo.EnrichedEventRepositoryInterface
}

func NewEnrichedEventsProjection(logger *logrus.Logger, snapshotRepo redis_repo.SnapshotRepositoryInterface, enrichedEventRepo opensearch_repo.EnrichedEventRepositoryInterface) *EnrichedEventsProjection {
	eventEnricher := process.NewEventEnricher(logger)
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

	var processFn func(ctx context.Context, event *types.Event) error
	switch event.Name {
	case ClusterEvent:
		processFn = p.ProcessClusterEvent
	case ClusterState:
		processFn = p.ProcessClusterState
	case HostState:
		processFn = p.ProcessHostState
	case InfraEnvState:
		processFn = p.ProcessInfraEnvState
	default:
		return fmt.Errorf("Unknown event name: %s (%s)", event.Name, event.Payload)
	}
	err := processFn(ctx, event)
	if _, ok := err.(*process.MalformedEventError); ok {
		p.logger.WithError(err).Warn("malformed event discarded")
		return nil
	}
	return err
}

func (p *EnrichedEventsProjection) ProcessClusterEvent(ctx context.Context, event *types.Event) error {
	clusterID, err := process.GetValueFromPayload("cluster_id", event.Payload)
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
	clusterID, err := process.GetValueFromPayload("id", event.Payload)
	if err != nil {
		return err
	}
	return p.snapshotRepository.SetCluster(ctx, clusterID, event)
}

func (p *EnrichedEventsProjection) ProcessHostState(ctx context.Context, event *types.Event) error {
	hostID, err := process.GetValueFromPayload("id", event.Payload)
	if err != nil {
		return err
	}
	clusterID, err := process.GetValueFromPayload("cluster_id", event.Payload)
	if err != nil {
		return err
	}
	return p.snapshotRepository.SetHost(ctx, clusterID, hostID, event)
}

func (p *EnrichedEventsProjection) ProcessInfraEnvState(ctx context.Context, event *types.Event) error {
	infraEnvID, err := process.GetValueFromPayload("id", event.Payload)
	if err != nil {
		return err
	}

	clusterID, err := process.GetValueFromPayload("cluster_id", event.Payload)
	if err != nil {
		return err
	}

	return p.snapshotRepository.SetInfraEnv(ctx, clusterID, infraEnvID, event)
}

func getEventFromMessage(msg *kafka.Message) (*types.Event, error) {
	event := &types.Event{}
	err := json.Unmarshal(msg.Value, event)
	if err != nil {
		return nil, err
	}
	return event, nil
}
