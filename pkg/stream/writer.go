package stream

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	kafka "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/compress"
	"github.com/sirupsen/logrus"
)

const (
	WriteTimeout time.Duration = 5 * time.Second
)

//go:generate mockgen -source=writer.go -package=stream -destination=mock_writer.go

// mocking kafka-go producer for testing
type Producer interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type EventStreamWriter interface {
	Write(ctx context.Context, key []byte, value interface{}) error
	Close()
}

type KafkaWriter struct {
	producer Producer
	logger   *logrus.Logger
}

func (w *KafkaWriter) Write(ctx context.Context, key []byte, value interface{}) error {
	encodedValue, err := json.Marshal(value)
	if err != nil {
		w.logger.WithFields(logrus.Fields{
			"value": value,
			"err":   err,
		}).Error("failed to encode json")

		return err
	}
	msg := kafka.Message{
		Key:   key,
		Value: encodedValue,
	}

	w.logger.WithFields(logrus.Fields{
		"msg": msg,
	}).Warning("sending notification")

	if kafkaWriter, ok := w.producer.(*kafka.Writer); ok {
		w.logger.WithFields(logrus.Fields{
			"msg":   msg,
			"topic": kafkaWriter.Topic,
		}).Warning("sending notification")

	}
	// If Async is true, this will always return nil
	return w.producer.WriteMessages(ctx, msg)
}

func (w *KafkaWriter) Close() {
	w.producer.Close()
}

func newProducer(config *KafkaConfig) (Producer, error) {
	brokers := strings.Split(config.BootstrapServer, ",")
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        config.DestinationTopic,
		Balancer:     &kafka.ReferenceHash{},
		Compression:  compress.Gzip,
		Async:        false,
		WriteTimeout: WriteTimeout,
	}
	mechanism, err := getMechanism(config)
	if err != nil {
		return nil, err
	}
	if mechanism != nil {
		writer.Transport = &kafka.Transport{
			SASL: mechanism,
			// let config pick default root CA, but define it to force TLS
			TLS: &tls.Config{},
		}
	}
	return writer, nil
}

func NewWriter(logger *logrus.Logger) (*KafkaWriter, error) {
	config := &KafkaConfig{}
	err := envconfig.Process("", config)
	if err != nil {
		return nil, err
	}

	p, err := newProducer(config)
	if err != nil {
		return nil, err
	}
	return &KafkaWriter{
		producer: p,
		logger:   logger,
	}, nil
}
