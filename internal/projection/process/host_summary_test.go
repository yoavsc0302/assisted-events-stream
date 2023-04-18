package process

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-assisted/assisted-events-streams/internal/types"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Process hosts summary", func() {
	When("Summary of an empty cluster", func() {
		It("should have 0 host count", func() {
			enrichedEvent := types.EnrichedEvent{}
			err := AddHostsSummary(&enrichedEvent)
			Expect(err).NotTo(HaveOccurred())

			Expect(enrichedEvent.HostsSummary.HostCount).To(Equal(0))
		})
	})
	When("Summary of an emtpy cluster", func() {
		It("Should generate proper summary", func() {
			enrichedEvent, err := getEnrichedEvent()
			Expect(err).ShouldNot(HaveOccurred())
			err = AddHostsSummary(enrichedEvent)
			Expect(err).NotTo(HaveOccurred())

			assertHostsSummary(enrichedEvent)
		})
	})

})

func assertHostsSummary(enrichedEvent *types.EnrichedEvent) {
	Expect(enrichedEvent.HostsSummary.HostCount).To(Equal(2))

	isoType, ok := enrichedEvent.HostsSummary.InfraEnv["type"]
	Expect(ok).To(BeTrue())
	fullIso, ok := isoType["full-iso"]
	Expect(ok).To(BeTrue())
	Expect(fullIso).To(Equal(1.0))

	Expect(enrichedEvent.HostsSummary.IsoType).To(Equal("full-iso"))

	cpuArch, ok := enrichedEvent.HostsSummary.InfraEnv["cpu_architecture"]
	Expect(ok).To(BeTrue())
	arm64, ok := cpuArch["arm64"]
	Expect(ok).To(BeTrue())
	Expect(arm64).To(Equal(.5))

	x86, ok := cpuArch["x86"]
	Expect(ok).To(BeTrue())
	Expect(x86).To(Equal(.5))

	Expect(enrichedEvent.HostsSummary.HasEtherogeneousArchitecture).To(BeTrue())

	openshiftVersion, ok := enrichedEvent.HostsSummary.InfraEnv["openshift_version"]
	Expect(ok).To(BeTrue())
	version, ok := openshiftVersion["4_12"]
	Expect(ok).To(BeTrue())
	Expect(version).To(Equal(1.0))
	_, ok = openshiftVersion["4.12"]
	Expect(ok).NotTo(BeTrue())
}

func getEnrichedEvent() (*types.EnrichedEvent, error) {
	cluster := map[string]interface{}{
		"id": "foobar",
	}
	eventEnricher := EventEnricher{logger: logrus.New()}
	event, _ := eventEnricher.GetBaseEnrichedEvent(
		getEvent("foobar", "my message"),
		cluster,
		getHosts(),
		getInfraEnvs(),
	)
	outEvent := types.EnrichedEvent{}

	// Marshal and unmarshal to have more realistic types
	jsonBytes, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonBytes, &outEvent)
	if err != nil {
		return nil, err
	}

	return &outEvent, nil
}

func TestProjectionProcessHostsSummary(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Projection process Hosts Summary")
}
