package onprem

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-assisted/assisted-events-streams/internal/types"
	"github.com/openshift-assisted/assisted-events-streams/pkg/stream"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

type MockEventExtractor struct {
	expectedTarFilename string
	eventChannel        chan types.EventEnvelope
	event               types.Event
	key                 []byte
}

func (e *MockEventExtractor) ExtractEvents(tarFilename string) (chan types.EventEnvelope, error) {
	Expect(e.expectedTarFilename).To(Equal(tarFilename))
	eventEnvelope := types.EventEnvelope{
		Key:   e.key,
		Event: e.event,
	}
	e.eventChannel <- eventEnvelope
	close(e.eventChannel)
	return e.eventChannel, nil
}

var _ = Describe("test event hydrator", func() {
	var (
		expectedURL         string = "https://example.com/foobar.tar.gz"
		expectedTarFilename string = "/tmp/randomdir/foobar.tar.gz"

		ctx                context.Context
		ctrl               *gomock.Controller
		hydrator           *OnPremEventsHydrator
		mockWriter         *stream.MockEventStreamWriter
		mockDownloader     *MockIFileDownloader
		mockEventExtractor *MockEventExtractor

		ackChannel      chan kafka.Message
		downloadChannel chan DownloadUrlMessage
		untarChannel    chan FilenameMessage
		eventChannel    chan types.EventEnvelope
		doneChannel     chan struct{}
	)
	BeforeEach(func() {
		ctx = context.Background()
		ctrl = gomock.NewController(GinkgoT())

		logger := logrus.New()
		logger.Out = ioutil.Discard
		ackChannel = make(chan kafka.Message, 1000)
		downloadChannel = make(chan DownloadUrlMessage, 1000)
		untarChannel = make(chan FilenameMessage, 1000)
		eventChannel = make(chan types.EventEnvelope, 1000)
		doneChannel = make(chan struct{}, 1)

		mockWriter = stream.NewMockEventStreamWriter(ctrl)
		mockDownloader = NewMockIFileDownloader(ctrl)
		mockEventExtractor = &MockEventExtractor{
			expectedTarFilename: expectedTarFilename,
			eventChannel:        eventChannel,
		}
		hydrator = &OnPremEventsHydrator{
			ctx:             ctx,
			logger:          logger,
			ackChannel:      ackChannel,
			downloadChannel: downloadChannel,
			untarChannel:    untarChannel,
			done:            doneChannel,
			writer:          mockWriter,
			downloader:      mockDownloader,
			eventExtractor:  mockEventExtractor,
		}
	})
	When("hydrating events", func() {
		It("should just work", func() {
			expectedKey := []byte("foobar")

			eventPayload := map[string]interface{}{}
			eventPayload["id"] = "1234567890"

			expectedEvent := types.Event{
				Name:    "ClusterState",
				Payload: eventPayload,
			}
			mockEventExtractor.key = expectedKey
			mockEventExtractor.event = expectedEvent
			mockDownloader.EXPECT().DownloadFile(expectedURL).Times(1).Return(expectedTarFilename, nil)
			mockWriter.EXPECT().Write(ctx, expectedKey, expectedEvent)
			go hydrator.Listen()
			message := kafka.Message{
				Key:   []byte("Foobar"),
				Value: []byte("Foobar"),
			}

			hydrator.enqueueDownload(expectedURL, message)

			msg := <-ackChannel
			Expect(msg).To(Equal(message))

			hydrator.Close(ctx)
		})

	})
})

func TestOnPrem(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Onprem test suite")
}
