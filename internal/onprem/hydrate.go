package onprem

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/kelseyhightower/envconfig"
	"github.com/openshift-assisted/assisted-events-streams/internal/types"
	"github.com/openshift-assisted/assisted-events-streams/pkg/stream"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

type DownloadUrlMessage struct {
	Url string
	Msg kafka.Message
}

type FilenameMessage struct {
	Filename string
	Msg      kafka.Message
}

type OnPremEventsHydrator struct {
	ctx             context.Context
	logger          *logrus.Logger
	ackChannel      chan kafka.Message
	downloadChannel chan DownloadUrlMessage
	untarChannel    chan FilenameMessage
	done            chan struct{}
	eventExtractor  IEventExtractor
	downloader      IFileDownloader
	writer          stream.EventStreamWriter
}

type OnPremPayload struct {
	Url       string `json:"url"`
	RequestID string `json:"request_id"`
}

type ChannelsConfig struct {
	DownloadChannelBufferSize int `envconfig:"DOWNLOAD_CHANNEL_BUFFER_SIZE" default:"1000"`
	UntarChannelBufferSize    int `envconfig:"UNTAR_CHANNEL_BUFFER_SIZE" default:"1000"`
	EventChannelBufferSize    int `envconfig:"EVENT_CHANNEL_BUFFER_SIZE" default:"1000"`
}

func NewOnPremEventsHydrator(ctx context.Context, logger *logrus.Logger, ackChannel chan kafka.Message) *OnPremEventsHydrator {
	channelsConfig := &ChannelsConfig{}
	err := envconfig.Process("", channelsConfig)
	if err != nil {
		return nil
	}

	downloadChannel := make(chan DownloadUrlMessage, channelsConfig.DownloadChannelBufferSize)
	untarChannel := make(chan FilenameMessage, channelsConfig.UntarChannelBufferSize)
	doneChannel := make(chan struct{}, 1)
	downloader, err := NewFileDownloaderFromEnv(logger)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"err": err,
		}).Fatal("error initializing downloader")
	}
	eventExtractor, err := NewEventExtractorFromEnv(logger, channelsConfig)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"err": err,
		}).Fatal("error initializing extractor")
	}
	writer, err := stream.NewWriter(logger)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"err": err,
		}).Fatal("error initializing kafka writer")
	}
	return &OnPremEventsHydrator{
		ctx:             ctx,
		logger:          logger,
		done:            doneChannel,
		ackChannel:      ackChannel,
		downloadChannel: downloadChannel,
		untarChannel:    untarChannel,
		eventExtractor:  eventExtractor,
		downloader:      downloader,
		writer:          writer,
	}
}

func (h *OnPremEventsHydrator) Listen() {
	h.logger.Info("listening to events")
	done := false
	for {
		select {
		case urlMsg := <-h.downloadChannel:
			go h.downloadURL(urlMsg.Url, urlMsg.Msg)
		case filenameMsg := <-h.untarChannel:
			go h.extractEvents(filenameMsg.Filename, filenameMsg.Msg)
		case <-h.done:
			done = true
		}
		if done {
			break
		}
	}
}

func (h *OnPremEventsHydrator) Close(ctx context.Context) {
	h.done <- struct{}{}
	ctx.Done()
}

func (h *OnPremEventsHydrator) extractEvents(filename string, msg kafka.Message) {
	h.logger.WithFields(logrus.Fields{
		"filename": filename,
	}).Debug("extracting events for filename")
	eventChannel, err := h.eventExtractor.ExtractEvents(filename)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"err": err,
		}).Warning("error when extracting events")
		return
	}
	var wg sync.WaitGroup
	for event := range eventChannel {
		wg.Add(1)
		go func(event types.EventEnvelope) {
			h.notifyEvent(event)
			wg.Done()
		}(event)
	}
	wg.Wait()
	h.ackChannel <- msg
}

func (h *OnPremEventsHydrator) notifyEvent(envelope types.EventEnvelope) {
	h.logger.WithFields(logrus.Fields{
		"cluster_id": envelope.Key,
	}).Debug("notfiying event for on-prem cluster")
	if err := h.writer.Write(h.ctx, envelope.Key, envelope.Event); err != nil {
		h.logger.WithFields(logrus.Fields{
			"err": err,
		}).Warning("error when notifying event")
	}
}

func (h *OnPremEventsHydrator) downloadURL(url string, msg kafka.Message) {
	h.logger.WithFields(logrus.Fields{
		"url": url,
	}).Info("downloading file from url")
	downloadedFilename, err := h.downloader.DownloadFile(url)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"url": url,
			"err": err,
		}).Warning("error downloading from url")
		return
	}
	h.untarChannel <- FilenameMessage{Filename: downloadedFilename, Msg: msg}
}

func (h *OnPremEventsHydrator) ProcessMessage(ctx context.Context, msg *kafka.Message) error {
	for _, header := range msg.Headers {
		if header.Key == "service" {
			service := string(header.Value)
			if service == "assisted-installer" {
				payload := OnPremPayload{}
				err := json.Unmarshal(msg.Value, &payload)
				if err != nil {
					h.logger.WithFields(logrus.Fields{
						"msg": msg,
					}).Warning("could not decode message value")
					continue
				}
				h.logger.WithFields(logrus.Fields{
					"payload": payload,
					"msg":     msg,
				}).Info("received and decoded message")
				h.enqueueDownload(payload.Url, *msg)
			}
		}
	}
	return nil
}

func (h *OnPremEventsHydrator) enqueueDownload(fileURL string, msg kafka.Message) {
	h.logger.WithFields(logrus.Fields{
		"url": fileURL,
	}).Debug("enqueued url for download")
	h.downloadChannel <- DownloadUrlMessage{Url: fileURL, Msg: msg}
}
