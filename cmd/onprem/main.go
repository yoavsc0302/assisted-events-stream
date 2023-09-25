package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/openshift-assisted/assisted-events-streams/internal/onprem"
	"github.com/openshift-assisted/assisted-events-streams/internal/utils"
	"github.com/openshift-assisted/assisted-events-streams/pkg/stream"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

const AckChannelBufferSize = 1000

func main() {
	ctx := context.Background()
	log := utils.NewLogger()
	ackChannel := make(chan kafka.Message, AckChannelBufferSize)
	reader, err := stream.NewKafkaReader(log, ackChannel)
	if err != nil {
		log.WithError(err).Fatal("Could not connect to kafka")
	}

	onprem := onprem.NewOnPremEventsHydrator(ctx, log, ackChannel)
	go onprem.Listen()
	intChannel := make(chan os.Signal, 1)

	gracefulShutdown := func() {
		for sig := range intChannel {
			log.WithFields(logrus.Fields{
				"signal": sig,
			}).Info("captured signal, shutting down")
			reader.Close(ctx)
			onprem.Close(ctx)
		}
	}

	signal.Notify(intChannel, syscall.SIGTERM, syscall.SIGINT)
	go gracefulShutdown()

	err = reader.Consume(ctx, onprem.ProcessMessage)
	if err != nil {
		log.WithError(err).Fatal(err)
	}
}
