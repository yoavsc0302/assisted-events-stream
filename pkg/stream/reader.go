package stream

import (
	"context"
	"crypto/tls"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	kafka "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/sirupsen/logrus"
)

const (
	MinBytes = 10e3 // 10KB
	MaxBytes = 10e6 // 10MB
)

type EventStreamReader interface {
	Consume(ctx context.Context, processMessageFn func(ctx context.Context, msg *kafka.Message) error) error
}

type KafkaReader struct {
	quitChannel chan struct{}
	logger      *logrus.Logger
	kafkaReader *kafka.Reader
	config      *KafkaConfig
	ackChannel  chan kafka.Message
}

type KafkaConfig struct {
	BootstrapServer string `envconfig:"KAFKA_BOOTSTRAP_SERVER" required:"true"`
	ClientID        string `envconfig:"KAFKA_CLIENT_ID" default:""`
	ClientSecret    string `envconfig:"KAFKA_CLIENT_SECRET" default:""`
	Topic           string `envconfig:"KAFKA_EVENT_STREAM_TOPIC" required:"true"`
	GroupID         string `envconfig:"KAFKA_GROUP_ID" required:"true"`
}

func NewKafkaReader(logger *logrus.Logger, ackChannel chan kafka.Message) (*KafkaReader, error) {
	envConfig := &KafkaConfig{}
	err := envconfig.Process("", envConfig)
	if err != nil {
		return nil, err
	}
	brokers := strings.Split(envConfig.BootstrapServer, ",")
	config := kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          envConfig.Topic,
		MinBytes:       MinBytes,
		MaxBytes:       MaxBytes,
		GroupID:        envConfig.GroupID,
		CommitInterval: time.Second * 10,
	}
	if envConfig.ClientID != "" && envConfig.ClientSecret != "" {
		mechanism := &plain.Mechanism{
			Username: envConfig.ClientID,
			Password: envConfig.ClientSecret,
		}

		dialer := &kafka.Dialer{
			SASLMechanism: mechanism,
			TLS:           &tls.Config{},
		}
		config.Dialer = dialer
	}
	kafkaReader := kafka.NewReader(config)
	return &KafkaReader{
		quitChannel: make(chan struct{}),
		logger:      logger,
		kafkaReader: kafkaReader,
		config:      envConfig,
		ackChannel:  ackChannel,
	}, nil
}

func (r *KafkaReader) Close(ctx context.Context) {
	close(r.quitChannel)
	r.kafkaReader.Close()
}

func (r *KafkaReader) Consume(ctx context.Context, processMessageFn func(ctx context.Context, msg *kafka.Message) error) error {
	r.logger.WithFields(logrus.Fields{
		"bootstrap_server": r.config.BootstrapServer,
		"topic":            r.config.Topic,
		"group_id":         r.config.GroupID,
	}).Printf("Connecting to kafka")

	go r.listenForCommit(ctx)
	for {
		select {
		case <-r.quitChannel:
			r.logger.Info("stop consuming")
			return nil
		default:
			msg, err := r.kafkaReader.FetchMessage(ctx)
			fields := logrus.Fields{
				"offset": msg.Offset,
				"key":    msg.Key,
			}
			if err != nil {
				return err
			}
			r.logger.WithFields(fields).Debug("processing message")
			err = processMessageFn(ctx, &msg)
			if err != nil {
				r.logger.WithError(err).WithFields(fields).Error("error processing message")
				return err
			}

			r.logger.WithFields(fields).Debug("message processed")
		}
	}
	return nil
}

func (r *KafkaReader) listenForCommit(ctx context.Context) {
	for msg := range r.ackChannel {
		fields := logrus.Fields{
			"offset": msg.Offset,
			"key":    msg.Key,
		}
		if err := r.kafkaReader.CommitMessages(ctx, msg); err != nil {
			r.logger.WithError(err).WithFields(fields).Error("error committing message")
		}
		r.logger.WithFields(fields).Debug("message committed")
	}
}
