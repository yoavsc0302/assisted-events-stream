package stream

import (
	"context"
	"crypto/tls"
	"strings"

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
	logger      *logrus.Logger
	kafkaReader *kafka.Reader
	config      *KafkaConfig
}

type KafkaConfig struct {
	BootstrapServer string `envconfig:"KAFKA_BOOTSTRAP_SERVER" required:"true"`
	ClientID        string `envconfig:"KAFKA_CLIENT_ID" default:""`
	ClientSecret    string `envconfig:"KAFKA_CLIENT_SECRET" default:""`
	Topic           string `envconfig:"KAFKA_EVENT_STREAM_TOPIC" required:"true"`
	GroupID         string `envconfig:"KAFKA_GROUP_ID" required:"true"`
}

func NewKafkaReader(logger *logrus.Logger) (*KafkaReader, error) {
	envConfig := &KafkaConfig{}
	err := envconfig.Process("", envConfig)
	if err != nil {
		return nil, err
	}
	brokers := strings.Split(envConfig.BootstrapServer, ",")
	config := kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    envConfig.Topic,
		MinBytes: MinBytes,
		MaxBytes: MaxBytes,
		GroupID:  envConfig.GroupID,
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
		logger:      logger,
		kafkaReader: kafkaReader,
		config:      envConfig,
	}, nil
}

func (r *KafkaReader) Consume(ctx context.Context, processMessageFn func(ctx context.Context, msg *kafka.Message) error) error {
	r.logger.WithFields(logrus.Fields{
		"bootstrap_server": r.config.BootstrapServer,
		"topic":            r.config.Topic,
		"group_id":         r.config.GroupID,
	}).Printf("Connecting to kafka")
	defer r.kafkaReader.Close()

	for {
		msg, err := r.kafkaReader.FetchMessage(ctx)
		if err != nil {
			return err
		}
		r.logger.Debug("Processing message")
		err = processMessageFn(ctx, &msg)
		if err != nil {
			r.logger.WithError(err).Error("Error processing message")
			return err
		}
		r.logger.Debug("Message processed")
		if err := r.kafkaReader.CommitMessages(ctx, msg); err != nil {
			r.logger.WithError(err).Error("Error committing message")
			return err
		}
		r.logger.Debug("Message committed")
	}
	return nil
}
