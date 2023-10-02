package onprem

import (
	"io/ioutil"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-assisted/assisted-events-streams/internal/types"
	"github.com/sirupsen/logrus"
)

const expectedClusterID = "ef477c3a-d433-4d18-bc10-83924deb6c28"
const sampleOnpremTarfile = "84364d0aade5461ca41b69f29e7b9181.gz"
const expectedTotalEvents = 247
const expectedClusterEvents = 1
const expectedHostEvents = 5
const expectedInfraEnvEvents = 1

var _ = Describe("test event extractor", func() {
	var (
		extractor *EventExtractor
		logger    *logrus.Logger
	)
	BeforeEach(func() {
		logger = logrus.New()
		logger.Out = ioutil.Discard

		extractor = &EventExtractor{
			logger:           logger,
			eventsBufferSize: 1000,
		}
	})
	When("untarring a valid file", func() {
		It("should produce valid events in the right sequence", func() {
			filename := filepath.Join("testdata", sampleOnpremTarfile)

			eventChannel, err := extractor.ExtractEvents(filename)
			Expect(err).To(BeNil())

			eventNumber := 0

			for event := range eventChannel {
				eventNumber += 1
				assertEventIsInRightSequence(event, eventNumber, expectedClusterID)
			}
			Expect(eventNumber).To(Equal(expectedTotalEvents))
		})

	})
})

func assertHasVersions(payload map[string]interface{}) {
	versions, ok := payload["versions"].(map[string]string)
	Expect(ok).To(Equal(true))
	assistedServiceVersion, ok := versions["assisted-installer-service"]
	Expect(ok).To(Equal(true))
	Expect(assistedServiceVersion).To(Equal("quay.io/app-sre/assisted-service:fe274eb"))
}

func assertEventIsInRightSequence(event types.EventEnvelope, eventNumber int, expectedClusterID string) {
	Expect(event.Key).To(Equal([]byte(expectedClusterID)))
	payload, ok := event.Event.Payload.(map[string]interface{})
	Expect(ok).To(Equal(true))
	assertHasVersions(payload)
	if eventNumber <= expectedClusterEvents {
		Expect(event.Event.Name).To(Equal("ClusterState"))
		clusterID, ok := payload["id"].(string)
		Expect(ok).To(Equal(true))
		Expect(clusterID).To(Equal(expectedClusterID))
		return
	}
	if eventNumber <= (expectedClusterEvents + expectedHostEvents) {
		Expect(event.Event.Name).To(Equal("HostState"))
		clusterID, ok := payload["cluster_id"].(string)
		Expect(ok).To(Equal(true))
		Expect(clusterID).To(Equal(expectedClusterID))
		return
	}
	if eventNumber <= (expectedClusterEvents + expectedHostEvents + expectedInfraEnvEvents) {
		Expect(event.Event.Name).To(Equal("InfraEnv"))
		clusterID, ok := payload["cluster_id"].(string)
		Expect(ok).To(Equal(true))
		Expect(clusterID).To(Equal(expectedClusterID))
		return
	}
	if eventNumber <= expectedTotalEvents {
		Expect(event.Event.Name).To(Equal("Event"))
		clusterID, ok := payload["cluster_id"].(string)
		Expect(ok).To(Equal(true))
		Expect(clusterID).To(Equal(expectedClusterID))
		return
	}
}
