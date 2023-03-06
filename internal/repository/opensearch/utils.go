package opensearch

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"time"

	opensearch "github.com/opensearch-project/opensearch-go"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

func NewOpensearchClientFromEnv(logger *logrus.Logger) *opensearch.Client {
	addresses := []string{
		os.Getenv("OPENSEARCH_ADDRESS"),
	}
	cfg := opensearch.Config{
		Addresses: addresses,
		Username:  os.Getenv("OPENSEARCH_USERNAME"),
		Password:  os.Getenv("OPENSEARCH_PASSWORD"),
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   10,
			ResponseHeaderTimeout: time.Second * 90,
			DialContext:           (&net.Dialer{Timeout: time.Second}).DialContext,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // When local env
				MinVersion:         tls.VersionTLS11,
			},
		},
	}

	client, err := opensearch.NewClient(cfg)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"addresses": addresses,
		}).WithError(err).Fatal("failed to initialize opensearch client")
	}
	_, err = client.Info()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"addresses": addresses,
		}).WithError(err).Fatal("failed to get info from opensearch server")
	}
	return client
}

func NewEnrichedEventRepositoryFromEnv(logger *logrus.Logger, ackChannel chan kafka.Message) *EnrichedEventRepository {
	indexPrefix := os.Getenv("OPENSEARCH_INDEX_PREFIX")
	opensearch := NewOpensearchClientFromEnv(logger)
	return NewEnrichedEventRepository(logger, opensearch, indexPrefix, ackChannel)
}
