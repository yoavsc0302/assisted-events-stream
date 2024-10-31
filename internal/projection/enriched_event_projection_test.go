package projection

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearch_repo "github.com/openshift-assisted/assisted-events-streams/internal/repository/opensearch"
	redis_repo "github.com/openshift-assisted/assisted-events-streams/internal/repository/redis"
	"github.com/openshift-assisted/assisted-events-streams/internal/types"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Process message", func() {
	var (
		ctx                   context.Context
		ctrl                  *gomock.Controller
		logger                *logrus.Logger
		projection            *EnrichedEventsProjection
		mockEnricher          *MockEventEnricherInterface
		mockSnapshotRepo      *redis_repo.MockSnapshotRepositoryInterface
		mockEnrichedEventRepo *opensearch_repo.MockEnrichedEventRepositoryInterface
		mockCluster           map[string]interface{}
		mockHosts             []map[string]interface{}
		mockInfraEnvs         []map[string]interface{}
		mockEnrichedEvent     *types.EnrichedEvent
		ackChannel            chan kafka.Message
	)
	BeforeEach(func() {
		logger = logrus.New()
		logger.Out = io.Discard
		ctx = context.Background()
		ctrl = gomock.NewController(GinkgoT())
		mockSnapshotRepo = redis_repo.NewMockSnapshotRepositoryInterface(ctrl)
		mockEnrichedEventRepo = opensearch_repo.NewMockEnrichedEventRepositoryInterface(ctrl)
		ackChannel = make(chan kafka.Message, 100)
		mockEnricher = NewMockEventEnricherInterface(ctrl)
		projection = &EnrichedEventsProjection{
			logger:                  logger,
			eventEnricher:           mockEnricher,
			snapshotRepository:      mockSnapshotRepo,
			enrichedEventRepository: mockEnrichedEventRepo,
			ackChannel:              ackChannel,
		}

		mockCluster = getMockCluster()
		mockHosts = getMockHosts()
		mockInfraEnvs = getMockInfraEnvs()
		mockEnrichedEvent = getMockEnrichedEvent()
	})
	AfterEach(func() {
		close(ackChannel)
	})
	When("Processing an invalid message", func() {
		It("should not return error", func() {
			msg := getKafkaMessage("not json")
			err := projection.ProcessMessage(ctx, msg)
			ackedMsg := <-ackChannel
			Expect(ackedMsg).To(Equal(*msg))
			Expect(err).To(BeNil())
		})
	})
	When("Processing a cluster event", func() {
		It("should retrieve all related info and store it", func() {
			eventPayload := getBasicClusterEventPayload()
			msg := getKafkaMessage(eventPayload)

			mockSnapshotRepo.EXPECT().GetCluster(ctx, "3a930088-e49d-4584-bbd3-54a568dbe833").Times(1).Return(mockCluster, nil)
			mockSnapshotRepo.EXPECT().GetHosts(ctx, "3a930088-e49d-4584-bbd3-54a568dbe833").Times(1).Return(mockHosts, nil)
			mockSnapshotRepo.EXPECT().GetInfraEnvs(ctx, "3a930088-e49d-4584-bbd3-54a568dbe833").Times(1).Return(mockInfraEnvs, nil)
			mockEnricher.EXPECT().GetEnrichedEvent(gomock.Any(), mockCluster, mockHosts, mockInfraEnvs).Times(1).Return(mockEnrichedEvent)

			mockEnrichedEventRepo.EXPECT().Store(ctx, mockEnrichedEvent, msg).Times(1).Return(nil)
			err := projection.ProcessMessage(ctx, msg)
			Expect(err).To(BeNil())
		})
	})

	When("Processing a cluster state event", func() {
		It("should store a snapshot of the cluster state, but should not store enriched events", func() {
			eventPayload := getClusterStatePayload()

			msg := getKafkaMessage(eventPayload)

			mockSnapshotRepo.EXPECT().SetCluster(ctx, "391d46b5-169b-4ffb-bce4-43ebdfe66b5c", gomock.Any()).Times(1).Return(nil)

			mockEnricher.EXPECT().GetEnrichedEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			mockEnrichedEventRepo.EXPECT().Store(ctx, gomock.Any(), msg).Times(0)
			err := projection.ProcessMessage(ctx, msg)
			Expect(err).To(BeNil())

			ackedMsg := <-ackChannel
			Expect(ackedMsg).To(Equal(*msg))
		})
	})

	When("Processing a host state event", func() {
		It("should store a snapshot of the host state, but should not store enriched events", func() {
			eventPayload := getHostStatePayload()

			msg := getKafkaMessage(eventPayload)

			mockSnapshotRepo.EXPECT().SetHost(ctx, "3a930088-e49d-4584-bbd3-54a568dbe833", "c64ffb6e-e9b0-4edb-9328-43f90d293783", gomock.Any()).Times(1).Return(nil)

			mockEnricher.EXPECT().GetEnrichedEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			mockEnrichedEventRepo.EXPECT().Store(ctx, gomock.Any(), msg).Times(0)

			err := projection.ProcessMessage(ctx, msg)
			Expect(err).To(BeNil())

			ackedMsg := <-ackChannel
			Expect(ackedMsg).To(Equal(*msg))
		})
	})

	When("Processing a infraenv event", func() {
		It("should store a snapshot of the infraenv, but should not store enriched events", func() {
			eventPayload := getInfraEnvPayload()

			msg := getKafkaMessage(eventPayload)

			mockSnapshotRepo.EXPECT().SetInfraEnv(ctx, "3a930088-e49d-4584-bbd3-54a568dbe833", "bf835bc3-96d4-4926-a71b-4f5829dee688", gomock.Any()).Times(1).Return(nil)

			mockEnricher.EXPECT().GetEnrichedEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			mockEnrichedEventRepo.EXPECT().Store(ctx, gomock.Any(), msg).Times(0).Return(nil)

			err := projection.ProcessMessage(ctx, msg)
			Expect(err).To(BeNil())

			ackedMsg := <-ackChannel
			Expect(ackedMsg).To(Equal(*msg))
		})
	})

	When("Some user names are excluded", func() {
		BeforeEach(func() {
			projection.excludedUserNames = []string{"excluded user"}
		})

		Context("Pushing an event from another user", func() {
			BeforeEach(func() {
				mockCluster = map[string]interface{}{
					"id":        "41355074-0b4a-4774-99f5-50c7bed8bb07",
					"foo":       "bar",
					"user_name": "another user",
				}
			})

			It("shouldn't change anything", func() {
				eventPayload := getBasicClusterEventPayload()
				msg := getKafkaMessage(eventPayload)

				mockSnapshotRepo.EXPECT().GetCluster(ctx, "3a930088-e49d-4584-bbd3-54a568dbe833").Times(1).Return(mockCluster, nil)
				mockSnapshotRepo.EXPECT().GetHosts(ctx, "3a930088-e49d-4584-bbd3-54a568dbe833").Times(1).Return(mockHosts, nil)
				mockSnapshotRepo.EXPECT().GetInfraEnvs(ctx, "3a930088-e49d-4584-bbd3-54a568dbe833").Times(1).Return(mockInfraEnvs, nil)
				mockEnricher.EXPECT().GetEnrichedEvent(gomock.Any(), mockCluster, mockHosts, mockInfraEnvs).Times(1).Return(mockEnrichedEvent)

				mockEnrichedEventRepo.EXPECT().Store(ctx, mockEnrichedEvent, msg).Times(1).Return(nil)
				err := projection.ProcessMessage(ctx, msg)
				Expect(err).To(BeNil())
			})
		})

		Context("Pushing an event from the excluded user", func() {
			BeforeEach(func() {
				mockCluster = map[string]interface{}{
					"id":        "41355074-0b4a-4774-99f5-50c7bed8bb07",
					"foo":       "bar",
					"user_name": "excluded user",
				}
			})

			It("should not process the message further", func() {
				eventPayload := getBasicClusterEventPayload()
				msg := getKafkaMessage(eventPayload)

				mockSnapshotRepo.EXPECT().GetCluster(ctx, "3a930088-e49d-4584-bbd3-54a568dbe833").Times(1).Return(mockCluster, nil)
				mockSnapshotRepo.EXPECT().GetHosts(ctx, "3a930088-e49d-4584-bbd3-54a568dbe833").Times(0)
				mockSnapshotRepo.EXPECT().GetInfraEnvs(ctx, "3a930088-e49d-4584-bbd3-54a568dbe833").Times(0)
				mockEnricher.EXPECT().GetEnrichedEvent(gomock.Any(), mockCluster, mockHosts, mockInfraEnvs).Times(0)

				mockEnrichedEventRepo.EXPECT().Store(ctx, mockEnrichedEvent, msg).Times(0)
				err := projection.ProcessMessage(ctx, msg)
				Expect(err).To(BeNil())
			})
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})
})

var _ = Describe("Create Projection", func() {
	When("No user names are excluded", func() {
		os.Unsetenv("EXCLUDED_USER_NAMES")

		Context("Creating a new enriched event projection", func() {
			projection, err := NewEnrichedEventsProjection(nil, nil, nil, nil)

			It("Should not fail", func() {
				Expect(err).To(BeNil())
			})

			It("Should create a projection with an empty excluded user name list", func() {
				Expect(len(projection.excludedUserNames)).To(Equal(0))
			})
		})
	})

	When("EXCLUDED_USER_NAMES is set but empty", func() {
		os.Setenv("EXCLUDED_USER_NAMES", "")

		Context("Creating a new enriched event projection", func() {
			projection, err := NewEnrichedEventsProjection(nil, nil, nil, nil)

			It("Should not fail", func() {
				Expect(err).To(BeNil())
			})

			It("Should create a projection with an empty excluded user name list", func() {
				Expect(len(projection.excludedUserNames)).To(Equal(0))
			})
		})
	})

	When("1 user name is excluded", func() {
		os.Setenv("EXCLUDED_USER_NAMES", "my user")

		Context("Creating a new enriched event projection", func() {
			projection, err := NewEnrichedEventsProjection(nil, nil, nil, nil)

			It("Should not fail", func() {
				Expect(err).To(BeNil())
			})

			It("Should create a projection with 1 excluded user name", func() {
				Expect(projection.excludedUserNames).To(Equal([]string{"my user"}))
			})
		})
	})

	When("several user names are excluded", func() {
		os.Setenv("EXCLUDED_USER_NAMES", "test 1,test2,  test  3 éè, test4")

		Context("Creating a new enriched event projection", func() {
			projection, err := NewEnrichedEventsProjection(nil, nil, nil, nil)

			It("Should not fail", func() {
				Expect(err).To(BeNil())
			})

			It("Should create a projection with 3 excluded user names", func() {
				Expect(projection.excludedUserNames).To(ConsistOf([]string{"test 1", "test2", "  test  3 éè", " test4"}))
			})
		})
	})
})

func getKafkaMessage(payload string) *kafka.Message {
	message := &kafka.Message{}
	message.Value = []byte(payload)
	return message
}

func getBasicClusterEventPayload() string {
	return `{
  "name": "Event",
  "payload": {
    "ID": 3375324,
    "CreatedAt": "2023-01-20T02:19:23.972689685Z",
    "UpdatedAt": "2023-01-20T02:19:23.972689685Z",
    "DeletedAt": null,
    "category": "user",
    "cluster_id": "3a930088-e49d-4584-bbd3-54a568dbe833",
    "event_time": "2023-01-20T02:19:23.972Z",
    "message": "Successfully registered cluster",
    "name": "cluster_registration_succeeded",
    "request_id": "0b22419a-5490-4cae-9e75-cb3ff3a8007f",
    "severity": "info"
  },
  "metadata": {
    "versions": {
      "SelfVersion": "quay.io/app-sre/assisted-service:3c49745",
      "AgentDockerImg": "registry-proxy.engineering.redhat.com/rh-osbs/openshift4-assisted-installer-agent-rhel8:latest",
      "InstallerImage": "registry-proxy.engineering.redhat.com/rh-osbs/openshift4-assisted-installer-rhel8:latest",
      "ControllerImage": "registry-proxy.engineering.redhat.com/rh-osbs/openshift4-assisted-installer-reporter-rhel8:latest",
      "ReleaseTag": "master"
    }
  }
}`

}

func getClusterStatePayload() string {
	return `{"name":"ClusterState","payload":{"additional_ntp_source":"clock.redhat.com","ams_subscription_id":"2KZTZsY7llbJfFryAtsyzocpgUN","api_vips":[],"base_dns_domain":"qe.lab.redhat.com","cluster_networks":[{"cidr":"10.128.0.0/14","cluster_id":"391d46b5-169b-4ffb-bce4-43ebdfe66b5c","host_prefix":23}],"connectivity_majority_groups":"{\"IPv4\":[],\"IPv6\":[]}","controller_logs_collected_at":"0001-01-01T00:00:00.000Z","controller_logs_started_at":"0001-01-01T00:00:00.000Z","cpu_architecture":"x86_64","created_at":"2023-01-20T02:19:23.714468Z","deleted_at":null,"disk_encryption":{"enable_on":"none","mode":"tpmv2"},"email_domain":"redhat.com","feature_usage":"{\"Additional NTP Source\":{\"data\":{\"source_count\":1},\"id\":\"ADDITIONAL_NTP_SOURCE\",\"name\":\"Additional NTP Source\"},\"Hyperthreading\":{\"data\":{\"hyperthreading_enabled\":\"all\"},\"id\":\"HYPERTHREADING\",\"name\":\"Hyperthreading\"},\"SDN network type\":{\"id\":\"SDN_NETWORK_TYPE\",\"name\":\"SDN network type\"}}","high_availability_mode":"Full","host_networks":null,"hosts":[],"href":"/api/assisted-install/v2/clusters/391d46b5-169b-4ffb-bce4-43ebdfe66b5c","hyperthreading":"all","id":"391d46b5-169b-4ffb-bce4-43ebdfe66b5c","ignition_endpoint":{},"image_info":{"created_at":"2023-01-20T02:19:23.714468Z","expires_at":"0001-01-01T00:00:00.000Z"},"ingress_vips":[],"install_completed_at":"0001-01-01T00:00:00.000Z","install_started_at":"0001-01-01T00:00:00.000Z","ip_collisions":"{}","kind":"Cluster","machine_networks":[],"monitored_operators":[{"cluster_id":"391d46b5-169b-4ffb-bce4-43ebdfe66b5c","name":"console","operator_type":"builtin","status_updated_at":"0001-01-01T00:00:00.000Z","timeout_seconds":3600},{"cluster_id":"391d46b5-169b-4ffb-bce4-43ebdfe66b5c","name":"cvo","operator_type":"builtin","status_updated_at":"0001-01-01T00:00:00.000Z","timeout_seconds":3600}],"name":"test-infra-cluster-fdc699fb","network_type":"OpenShiftSDN","ocp_release_image":"quay.io/openshift-release-dev/ocp-release:4.11.20-x86_64","openshift_version":"4.11.20","org_id":"16020201","platform":{"type":"baremetal"},"progress":{},"pull_secret_set":true,"schedulable_masters":false,"schedulable_masters_forced_true":true,"service_networks":[{"cidr":"172.30.0.0/16","cluster_id":"391d46b5-169b-4ffb-bce4-43ebdfe66b5c"}],"ssh_public_key":"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCvisqzuafhtk+72Y+FAres0kpsSRo/EaIeLs+C49TW30B2FgFpieQ1/a0UouW8eofS2VFVWdOqnS3aCMbI02XLFk8w5pLSOYGpALqJKk7BYYPPDgWzA1lrLvOlgXdKpkfcumsBxxlvPRm69ueNl1F3Ib1bxo1L1MDHQ7Av3CcafkOvzenOGUTpbOdZq6BLFbi2IhDhoIw1WJxvuMu0phrz7q3l5OcYp8rU+nqIo0JShF5vWODeD2jCf8/IS2H5DZvd9qDPUVubQTU51sLvA5vIToPbT680ekKlld2AEzpOkNQf8kBvOAgoPII5aXUNggH2FHIgMBA2FiPQyLTonQAZ","status":"pending-for-input","status_info":"User input required","status_updated_at":"2023-01-20T02:19:33.823Z","updated_at":"2023-01-20T02:19:33.824846Z","user_managed_networking":false,"user_name":"assisted-installer-qe-ci2","validations_info":"{\"configuration\":[{\"id\":\"pull-secret-set\",\"status\":\"success\",\"message\":\"The pull secret is set.\"}],\"hosts-data\":[{\"id\":\"all-hosts-are-ready-to-install\",\"status\":\"success\",\"message\":\"All hosts in the cluster are ready to install.\"},{\"id\":\"sufficient-masters-count\",\"status\":\"failure\",\"message\":\"Clusters must have exactly 3 dedicated control plane nodes. Add or remove hosts, or change their roles configurations to meet the requirement.\"}],\"network\":[{\"id\":\"api-vips-defined\",\"status\":\"failure\",\"message\":\"API virtual IPs are undefined and must be provided.\"},{\"id\":\"api-vips-valid\",\"status\":\"pending\",\"message\":\"API virtual IPs are undefined.\"},{\"id\":\"cluster-cidr-defined\",\"status\":\"success\",\"message\":\"The Cluster Network CIDR is defined.\"},{\"id\":\"dns-domain-defined\",\"status\":\"success\",\"message\":\"The base domain is defined.\"},{\"id\":\"ingress-vips-defined\",\"status\":\"failure\",\"message\":\"Ingress virtual IPs are undefined and must be provided.\"},{\"id\":\"ingress-vips-valid\",\"status\":\"pending\",\"message\":\"Ingress virtual IPs are undefined.\"},{\"id\":\"machine-cidr-defined\",\"status\":\"pending\",\"message\":\"Hosts have not been discovered yet\"},{\"id\":\"machine-cidr-equals-to-calculated-cidr\",\"status\":\"pending\",\"message\":\"The Machine Network CIDR, API virtual IPs, or Ingress virtual IPs are undefined.\"},{\"id\":\"network-prefix-valid\",\"status\":\"success\",\"message\":\"The Cluster Network prefix is valid.\"},{\"id\":\"network-type-valid\",\"status\":\"success\",\"message\":\"The cluster has a valid network type\"},{\"id\":\"networks-same-address-families\",\"status\":\"pending\",\"message\":\"At least one of the CIDRs (Machine Network, Cluster Network, Service Network) is undefined.\"},{\"id\":\"no-cidrs-overlapping\",\"status\":\"pending\",\"message\":\"At least one of the CIDRs (Machine Network, Cluster Network, Service Network) is undefined.\"},{\"id\":\"ntp-server-configured\",\"status\":\"success\",\"message\":\"No ntp problems found\"},{\"id\":\"service-cidr-defined\",\"status\":\"success\",\"message\":\"The Service Network CIDR is defined.\"}],\"operators\":[{\"id\":\"cnv-requirements-satisfied\",\"status\":\"success\",\"message\":\"cnv is disabled\"},{\"id\":\"lso-requirements-satisfied\",\"status\":\"success\",\"message\":\"lso is disabled\"},{\"id\":\"lvm-requirements-satisfied\",\"status\":\"success\",\"message\":\"lvm is disabled\"},{\"id\":\"odf-requirements-satisfied\",\"status\":\"success\",\"message\":\"odf is disabled\"}]}","vip_dhcp_allocation":false,"pull_secret":"{\"auths\":{\"cloud.openshift.com\":{\"auth\":\"b3BlbnNoaWZ0LXJlbGVhc2UtZGV2K29jbV9hY2Nlc3NfZGE5MDU0YWJkMmVkNDM1MmE4MDVlNzJhOWE0ZTRiZmQ6RzVPWFlLSUtZOE5RTzJFQVJRTEdNN1FROTZPUENGWlIyVTUwSFBZUzJEUEE0SDlROFlFM1NHSTZZWlRUQ1Q4Qg==\",\"email\":\"talhil@redhat.com\"},\"quay.io\":{\"auth\":\"b3BlbnNoaWZ0LXJlbGVhc2UtZGV2K29jbV9hY2Nlc3NfZGE5MDU0YWJkMmVkNDM1MmE4MDVlNzJhOWE0ZTRiZmQ6RzVPWFlLSUtZOE5RTzJFQVJRTEdNN1FROTZPUENGWlIyVTUwSFBZUzJEUEE0SDlROFlFM1NHSTZZWlRUQ1Q4Qg==\",\"email\":\"talhil@redhat.com\"},\"registry.connect.redhat.com\":{\"auth\":\"fHVoYy1wb29sLTM3ZDZmZjBhLTNlZjItNGM2OS1hZGE0LWE3MjllMTc4ZDNiYTpleUpoYkdjaU9pSlNVelV4TWlKOS5leUp6ZFdJaU9pSmpabVU0Tmpka1l6QmtNekUwTlRobFlqZzVOamsyTmpNNE9EYzVNek14WlNKOS5PUTIzVTMtak5VZFdPQ05NdHJlWGJsX2lFMUFjbjRKOWZBTE5Xc1kwQkZxb2IxdHoyaDJ6U21kZnVIU2tTcG01VVpXQndJVmE2Q0l3Si1oVmJsR2ZzMWw1NEJmLTZRMzJDcU5uS0c5ejlEb3hpNndJUm9iYjExeTk0aWNVOTh3VmZPTkpXb2JzQV91UDhsazhRcFNYSnJFR3hzN09lQjhPVHF4aUNaS1lMaGlSWlpZblVUaVBRTzU0YWlBVmFLVUdQdmR4bWlVRXVLNUM4VzJYeTZMeHVYejB0eHJhX0xuUzFXZWg1ZjBiSF9CRnhCdktCeGZHWjRFWEpkYklSTW1UU2ZTWUVZMTFYdUlDMmxXbV81ekdWM3hqMlhFMTYxR25ZcHVsOUtyQlcxWDVhUWItcVdoU0NTVUhCS3BrRjRNb0RiSkxMekNrTDEwR3U2S3hoRXdpQ3d4OXRZY09lNVVkUEliekdGb01DS0M0VGRWZnk1d25BRDdvOHFwRF9HT2J4YWQtNF9UN0ZoVXZsbmx1S2VxTElqdFFIUHBTSFp0R1JHa3BBVGxhMWg4Z2RMQmFfUFc3aEttemYzdFpuN0xCQ2ROVnphR29FVW1hSm9BeUlDZmc5TzVTNG5PdzY2YTdKSVkxNm1RZlVVaktqOXZIR1hvSG5ucEJPcTVRaDBMU2ZfVkY2NVhoS1IxVXN1a0lUTlRXdXpQX2lGc1QwMXR5bUd2a2d0WXltLTdDekpUMFI4RWhHNklYOTZ3eW51VTVuT0NvUWpONVJ1Y2ZudzNrM29rWjBJc25tVXdvMXNyM0x5X21GaHRRVVkzUm5FMG9aYl9PdFNIUWNfM3ZSV3RLeHpNMmZoaVlVSmVMRklEVWEydEFHbXA4cVZlaTFwYzNCc2JrREF2eGZjQQ==\",\"email\":\"talhil@redhat.com\"},\"registry.redhat.io\":{\"auth\":\"fHVoYy1wb29sLTM3ZDZmZjBhLTNlZjItNGM2OS1hZGE0LWE3MjllMTc4ZDNiYTpleUpoYkdjaU9pSlNVelV4TWlKOS5leUp6ZFdJaU9pSmpabVU0Tmpka1l6QmtNekUwTlRobFlqZzVOamsyTmpNNE9EYzVNek14WlNKOS5PUTIzVTMtak5VZFdPQ05NdHJlWGJsX2lFMUFjbjRKOWZBTE5Xc1kwQkZxb2IxdHoyaDJ6U21kZnVIU2tTcG01VVpXQndJVmE2Q0l3Si1oVmJsR2ZzMWw1NEJmLTZRMzJDcU5uS0c5ejlEb3hpNndJUm9iYjExeTk0aWNVOTh3VmZPTkpXb2JzQV91UDhsazhRcFNYSnJFR3hzN09lQjhPVHF4aUNaS1lMaGlSWlpZblVUaVBRTzU0YWlBVmFLVUdQdmR4bWlVRXVLNUM4VzJYeTZMeHVYejB0eHJhX0xuUzFXZWg1ZjBiSF9CRnhCdktCeGZHWjRFWEpkYklSTW1UU2ZTWUVZMTFYdUlDMmxXbV81ekdWM3hqMlhFMTYxR25ZcHVsOUtyQlcxWDVhUWItcVdoU0NTVUhCS3BrRjRNb0RiSkxMekNrTDEwR3U2S3hoRXdpQ3d4OXRZY09lNVVkUEliekdGb01DS0M0VGRWZnk1d25BRDdvOHFwRF9HT2J4YWQtNF9UN0ZoVXZsbmx1S2VxTElqdFFIUHBTSFp0R1JHa3BBVGxhMWg4Z2RMQmFfUFc3aEttemYzdFpuN0xCQ2ROVnphR29FVW1hSm9BeUlDZmc5TzVTNG5PdzY2YTdKSVkxNm1RZlVVaktqOXZIR1hvSG5ucEJPcTVRaDBMU2ZfVkY2NVhoS1IxVXN1a0lUTlRXdXpQX2lGc1QwMXR5bUd2a2d0WXltLTdDekpUMFI4RWhHNklYOTZ3eW51VTVuT0NvUWpONVJ1Y2ZudzNrM29rWjBJc25tVXdvMXNyM0x5X21GaHRRVVkzUm5FMG9aYl9PdFNIUWNfM3ZSV3RLeHpNMmZoaVlVSmVMRklEVWEydEFHbXA4cVZlaTFwYzNCc2JrREF2eGZjQQ==\",\"email\":\"talhil@redhat.com\"},\"registry.stage.redhat.io\":{\"auth\":\"cWFAcmVkaGF0LmNvbTpyZWRoYXRxYQ==\"}}}","proxy_hash":"","MachineNetworkCidrUpdatedAt":"2023-01-20T02:19:23.566617Z","ApiVipLease":"","IngressVipLease":"","kube_key_name":"","kube_key_namespace":"","is_ams_subscription_console_url_set":false,"InstallationPreparationCompletionStatus":"","image_generated":false,"TriggerMonitorTimestamp":"2023-01-20T02:19:33.823612Z","static_network_configured":false},"metadata":{"versions":{"SelfVersion":"quay.io/app-sre/assisted-service:3c49745","AgentDockerImg":"registry-proxy.engineering.redhat.com/rh-osbs/openshift4-assisted-installer-agent-rhel8:latest","InstallerImage":"registry-proxy.engineering.redhat.com/rh-osbs/openshift4-assisted-installer-rhel8:latest","ControllerImage":"registry-proxy.engineering.redhat.com/rh-osbs/openshift4-assisted-installer-reporter-rhel8:latest","ReleaseTag":"master"}}}`
}

func getHostStatePayload() string {
	return `{
  "name": "HostState",
  "payload": {
    "ID": 3375324,
    "CreatedAt": "2023-01-20T02:19:23.972689685Z",
    "UpdatedAt": "2023-01-20T02:19:23.972689685Z",
    "DeletedAt": null,
    "category": "user",
    "id": "c64ffb6e-e9b0-4edb-9328-43f90d293783",
    "cluster_id": "3a930088-e49d-4584-bbd3-54a568dbe833",
    "event_time": "2023-01-20T02:19:23.972Z",
    "message": "Successfully registered cluster",
    "name": "cluster_registration_succeeded",
    "request_id": "0b22419a-5490-4cae-9e75-cb3ff3a8007f",
    "severity": "info"
  },
  "metadata": {
    "versions": {
      "SelfVersion": "quay.io/app-sre/assisted-service:3c49745",
      "AgentDockerImg": "registry-proxy.engineering.redhat.com/rh-osbs/openshift4-assisted-installer-agent-rhel8:latest",
      "InstallerImage": "registry-proxy.engineering.redhat.com/rh-osbs/openshift4-assisted-installer-rhel8:latest",
      "ControllerImage": "registry-proxy.engineering.redhat.com/rh-osbs/openshift4-assisted-installer-reporter-rhel8:latest",
      "ReleaseTag": "master"
    }
  }
}`

}

func getInfraEnvPayload() string {
	return `{
  "name": "InfraEnv",
  "payload": {
    "ID": 3375324,
    "CreatedAt": "2023-01-20T02:19:23.972689685Z",
    "UpdatedAt": "2023-01-20T02:19:23.972689685Z",
    "DeletedAt": null,
    "category": "user",
    "id": "bf835bc3-96d4-4926-a71b-4f5829dee688",
    "cluster_id": "3a930088-e49d-4584-bbd3-54a568dbe833",
    "event_time": "2023-01-20T02:19:23.972Z",
    "message": "Successfully registered cluster",
    "name": "cluster_registration_succeeded",
    "request_id": "0b22419a-5490-4cae-9e75-cb3ff3a8007f",
    "severity": "info"
  },
  "metadata": {
    "versions": {
      "SelfVersion": "quay.io/app-sre/assisted-service:3c49745",
      "AgentDockerImg": "registry-proxy.engineering.redhat.com/rh-osbs/openshift4-assisted-installer-agent-rhel8:latest",
      "InstallerImage": "registry-proxy.engineering.redhat.com/rh-osbs/openshift4-assisted-installer-rhel8:latest",
      "ControllerImage": "registry-proxy.engineering.redhat.com/rh-osbs/openshift4-assisted-installer-reporter-rhel8:latest",
      "ReleaseTag": "master"
    }
  }
}`
}

func TestEnrichedEventProjection(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Enriched Events Projection suite")
}

func getMockCluster() map[string]interface{} {
	return map[string]interface{}{
		"id":  "41355074-0b4a-4774-99f5-50c7bed8bb07",
		"foo": "bar",
	}
}
func getMockHosts() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":  "5d8054e5-6d52-4bdb-8629-65c258d29de9",
			"foo": "foobar",
		},
		{
			"id":  "9024b650-f31f-420b-b279-3296a6e7d2cc",
			"foo": "foobar",
		},
	}
}

func getMockInfraEnvs() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":  "94b5c5b6-f798-41cc-9187-ffe14df68d48",
			"foo": "foobar",
		},
		{
			"id":  "177880c8-7c2c-49d3-b7c2-42625c31eb05",
			"foo": "foobar",
		},
	}
}

func getMockEnrichedEvent() *types.EnrichedEvent {
	return &types.EnrichedEvent{
		ID: "bf1de182-ff4f-40a0-970d-1a3c0321de27",
	}
}
