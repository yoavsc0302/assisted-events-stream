package opensearch

import (
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	//"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearch "github.com/opensearch-project/opensearch-go"
	"github.com/openshift-assisted/assisted-events-streams/internal/types"
	"github.com/sirupsen/logrus"
)

type MockTransport struct {
	Response    *http.Response
	RoundTripFn func(req *http.Request) (*http.Response, error)
}

func (t *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.RoundTripFn(req)
}

func getMockOpensearchClient(mockResponseBody string, statusCode int) *opensearch.Client {
	transport := MockTransport{
		Response: &http.Response{
			StatusCode: statusCode,
			Body:       ioutil.NopCloser(strings.NewReader(mockResponseBody)),
		},
	}
	transport.RoundTripFn = func(req *http.Request) (*http.Response, error) { return transport.Response, nil }

	client, err := opensearch.NewClient(opensearch.Config{
		Transport: &transport,
	})
	Expect(err).To(BeNil())
	return client
}

func getProjectionConfigRepoWithMockOpensearchResponse(mockResponse string, mockStatus int) *ProjectionConfigRepository {
	logger := logrus.New()
	logger.Out = ioutil.Discard
	mockOpensearchClient := getMockOpensearchClient(mockResponse, mockStatus)
	return &ProjectionConfigRepository{
		logger:           logger,
		opensearchClient: mockOpensearchClient,
		index:            "foobar",
	}

}

var _ = Describe("Getting and setting config object", func() {
	var (
		ctx context.Context
	)
	BeforeEach(func() {
		ctx = context.Background()
	})
	When("getting config when not set", func() {
		It("should return error but default config", func() {
			mockResponse := `{"_index" : "assisted-event-streams-config","_type":"_doc","_id":"abc","found" : false}`
			configRepo := getProjectionConfigRepoWithMockOpensearchResponse(mockResponse, http.StatusNotFound)
			cfg := configRepo.Get(ctx)
			Expect(cfg.Mode).To(BeEquivalentTo(types.ProjectionModeOnline))
		})
	})

	When("getting config when set", func() {
		It("should return whatever is set, and no error", func() {
			mockResponse := `
{
  "_index" : "assisted-event-streams-config",
  "_type" : "_doc",
  "_id" : "projection_config",
  "_version" : 1,
  "_seq_no" : 0,
  "_primary_term" : 1,
  "found" : true,
  "_source" : {
    "mode" : "offline"
  }
}`
			configRepo := getProjectionConfigRepoWithMockOpensearchResponse(mockResponse, http.StatusOK)
			cfg := configRepo.Get(ctx)
			Expect(cfg.Mode).To(BeEquivalentTo(types.ProjectionModeOffline))

			mockResponse = `
{
  "_index" : "assisted-event-streams-config",
  "_type" : "_doc",
  "_id" : "projection_config",
  "_version" : 1,
  "_seq_no" : 0,
  "_primary_term" : 1,
  "found" : true,
  "_source" : {
    "mode" : "online"
  }
}`
			configRepo = getProjectionConfigRepoWithMockOpensearchResponse(mockResponse, http.StatusOK)
			cfg = configRepo.Get(ctx)
			Expect(cfg.Mode).To(BeEquivalentTo(types.ProjectionModeOnline))
		})
	})

})

func TestRepositories(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test opensearch repositories")
}
