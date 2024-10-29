package projection

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/kelseyhightower/envconfig"
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

	userName = "user_name"
)

//go:generate mockgen -source=enriched_event_projection.go -package=projection -destination=mock_event_enricher.go

type EventEnricherInterface interface {
	GetEnrichedEvent(event *types.Event, cluster map[string]interface{}, hosts []map[string]interface{}, infraEnvs []map[string]interface{}) *types.EnrichedEvent
}

type ProjectionConfig struct {
	ExcludedUserNames []string `envconfig:"EXCLUDED_USER_NAMES" default:""`
}

type EnrichedEventsProjection struct {
	logger                  *logrus.Logger
	eventEnricher           EventEnricherInterface
	snapshotRepository      redis_repo.SnapshotRepositoryInterface
	enrichedEventRepository opensearch_repo.EnrichedEventRepositoryInterface
	ackChannel              chan kafka.Message
	excludedUserNames       []string
}

func NewEnrichedEventsProjection(logger *logrus.Logger, snapshotRepo redis_repo.SnapshotRepositoryInterface, enrichedEventRepo opensearch_repo.EnrichedEventRepositoryInterface, ackChannel chan kafka.Message) (*EnrichedEventsProjection, error) {
	eventEnricher := process.NewEventEnricher(logger)

	config := ProjectionConfig{}

	err := envconfig.Process("", &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse env: %w", err)
	}

	return &EnrichedEventsProjection{
		logger:                  logger,
		eventEnricher:           eventEnricher,
		snapshotRepository:      snapshotRepo,
		enrichedEventRepository: enrichedEventRepo,
		ackChannel:              ackChannel,
		excludedUserNames:       config.ExcludedUserNames,
	}, nil
}

func (p *EnrichedEventsProjection) Close(ctx context.Context) {
	p.enrichedEventRepository.Close(ctx)
}

func (p *EnrichedEventsProjection) ProcessMessage(ctx context.Context, msg *kafka.Message) error {

	event, err := getEventFromMessage(msg)
	if err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"message": msg,
		}).Error("could not get event from message")
		// acknowledge malformed message
		p.ackMsg(msg)
		return nil
	}
	return p.ProcessEvent(ctx, event, msg)
}

func (p *EnrichedEventsProjection) ProcessEvent(ctx context.Context, event *types.Event, msg *kafka.Message) error {
	var err error
	p.logger.WithFields(logrus.Fields{
		"name": event.Name,
	}).Debug("processing event")
	switch event.Name {
	case ClusterEvent:
		err = p.ProcessClusterEvent(ctx, event, msg)
	case ClusterState:
		err = p.ProcessClusterState(ctx, event)
	case HostState:
		err = p.ProcessHostState(ctx, event)
	case InfraEnvState:
		err = p.ProcessInfraEnvState(ctx, event)
	default:
		return fmt.Errorf("unknown event name: %s (%s)", event.Name, event.Payload)
	}
	if event.Name != ClusterEvent {
		p.ackMsg(msg)
	}
	if _, ok := err.(*process.MalformedEventError); ok {
		p.logger.WithError(err).Warn("malformed event discarded")
		p.ackMsg(msg)
		return nil
	}
	return err
}

func (p *EnrichedEventsProjection) ackMsg(msg *kafka.Message) {
	p.ackChannel <- *msg
}

func (p *EnrichedEventsProjection) ProcessClusterEvent(ctx context.Context, event *types.Event, msg *kafka.Message) error {
	clusterID, err := process.GetValueFromPayload("cluster_id", event.Payload)
	if err != nil {
		return err
	}
	p.logger.WithFields(logrus.Fields{
		"name":       event.Name,
		"cluster_id": clusterID,
	}).Debug("processing cluster event")

	cluster, err := p.snapshotRepository.GetCluster(ctx, clusterID)
	if err != nil {
		p.logger.WithFields(logrus.Fields{
			"cluster_id": clusterID,
		}).WithError(err).Warn("Could not retrieve cluster")
	}

	// Hack: Don't process further events from specific users
	filterOut := p.skipEvent(cluster)
	if filterOut {
		p.logger.WithFields(logrus.Fields{
			"cluster_id": clusterID,
		}).Debug("skipping event")

		return nil
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
	err = p.enrichedEventRepository.Store(ctx, enrichedEvent, msg)
	if err != nil {
		p.logger.WithError(err).Warn("something went wrong while trying to store the event")
		return err
	}
	return nil
}

func (p *EnrichedEventsProjection) ProcessClusterState(ctx context.Context, event *types.Event) error {
	clusterID, err := process.GetValueFromPayload("id", event.Payload)
	if err != nil {
		return err
	}
	p.logger.WithFields(logrus.Fields{
		"name":       event.Name,
		"cluster_id": clusterID,
	}).Debug("processing cluster state")
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
	p.logger.WithFields(logrus.Fields{
		"name":       event.Name,
		"host_id":    hostID,
		"cluster_id": clusterID,
	}).Debug("processing host state")
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

	p.logger.WithFields(logrus.Fields{
		"name":         event.Name,
		"infra_env_id": infraEnvID,
		"cluster_id":   clusterID,
	}).Debug("processing infra-env state")
	return p.snapshotRepository.SetInfraEnv(ctx, clusterID, infraEnvID, event)
}

func (p *EnrichedEventsProjection) skipEvent(cluster map[string]interface{}) bool {
	name, ok := cluster[userName]
	if !ok {
		return false
	}

	nameStr, ok := name.(string)
	if !ok {
		return false
	}

	return slices.Contains(p.excludedUserNames, nameStr)
}

func getEventFromMessage(msg *kafka.Message) (*types.Event, error) {
	event := &types.Event{}
	err := json.Unmarshal(msg.Value, event)
	if err != nil {
		return nil, err
	}
	return event, nil
}
