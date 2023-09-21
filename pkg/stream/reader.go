package stream

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	kafka "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
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
	BootstrapServer  string `envconfig:"KAFKA_BOOTSTRAP_SERVER" required:"true"`
	ClientID         string `envconfig:"KAFKA_CLIENT_ID" default:""`
	ClientSecret     string `envconfig:"KAFKA_CLIENT_SECRET" default:""`
	SaslMechanism    string `envconfig:"KAFKA_SASL_MECHANISM" default:"PLAIN"`
	Topic            string `envconfig:"KAFKA_EVENT_STREAM_TOPIC" required:"true"`
	GroupID          string `envconfig:"KAFKA_GROUP_ID" required:"true"`
	DestinationTopic string `envconfig:"KAFKA_EVENT_STREAM_TOPIC_DESTINATION" required:"true"`
}

func getMechanism(envConfig *KafkaConfig) (sasl.Mechanism, error) {
	if envConfig.SaslMechanism == "PLAIN" {
		return &plain.Mechanism{
			Username: envConfig.ClientID,
			Password: envConfig.ClientSecret,
		}, nil
	}
	if envConfig.SaslMechanism == "SCRAM" {
		return scram.Mechanism(scram.SHA512, envConfig.ClientID, envConfig.ClientSecret)
	}
	return nil, fmt.Errorf("SASL Mechanism %s not implemented", envConfig.SaslMechanism)

}
func NewKafkaReader(logger *logrus.Logger, ackChannel chan kafka.Message) (*KafkaReader, error) {
	envConfig := &KafkaConfig{}
	err := envconfig.Process("", envConfig)
	if err != nil {
		return nil, err
	}
	brokers := strings.Split(envConfig.BootstrapServer, ",")
	logger.WithFields(logrus.Fields{
		"bootstrap_server": envConfig.BootstrapServer,
		"topic":            envConfig.Topic,
		"group_id":         envConfig.GroupID,
	}).Printf("connecting to kafka")

	config := kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          envConfig.Topic,
		MinBytes:       MinBytes,
		MaxBytes:       MaxBytes,
		GroupID:        envConfig.GroupID,
		CommitInterval: time.Second * 10,
	}
	if envConfig.ClientID != "" && envConfig.ClientSecret != "" {
		mechanism, err := getMechanism(envConfig)
		if err != nil {
			return nil, err
		}
		dialer := &kafka.Dialer{
			SASLMechanism: mechanism,
			TLS:           &tls.Config{},
		}
		config.Dialer = dialer
		logger.WithFields(logrus.Fields{
			"mechanism": mechanism.Name(),
			"user":      envConfig.ClientID,
		}).Printf("using TLS connection")
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
