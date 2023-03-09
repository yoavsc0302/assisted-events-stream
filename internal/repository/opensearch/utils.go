package opensearch

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/kelseyhightower/envconfig"
	opensearch "github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchutil"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

type OpensearchEnvConfig struct {
	NumWorkers            int           `envconfig:"OPENSEARCH_BULK_WORKERS" default:"1"`
	FlushBytes            int           `envconfig:"OPENSEARCH_BULK_FLUSH_BYTES" default:"10000000"`
	FlushInterval         time.Duration `envconfig:"OPENSEARCH_BULK_FLUSH_INTERVAL" default:"120s"`
	BulkTimeout           time.Duration `envconfig:"OPENSEARCH_BULK_TIMEOUT" default:"90s"`
	Username              string        `envconfig:"OPENSEARCH_USERNAME" required:"true"`
	Password              string        `envconfig:"OPENSEARCH_PASSWORD" required:"true"`
	Address               string        `envconfig:"OPENSEARCH_ADDRESS" required:"true"`
	ResponseTimeout       time.Duration `envconfig:"OPENSEARCH_RESPONSE_TIMEOUT" default:"90s"`
	DialTimeout           time.Duration `envconfig:"OPENSEARCH_DIAL_TIMEOUT" default:"1s"`
	SSLInsecureSkipVerify bool          `envconfig:"OPENSEARCH_SSL_INSECURE_SKIP_VERIFY" default:"false"`
	IndexPrefix           string        `envconfig:"OPENSEARCH_INDEX_PREFIX required:"true"`
	ConfigIndex           string        `envconfig:"OPENSEARCH_CONFIG_INDEX required:"true"`
	DocId                 string        `envconfig:"OPENSEARCH_CONFIG_DOC_ID default:"projection_config"`
}

func getConfigFromEnv(logger *logrus.Logger) *OpensearchEnvConfig {
	envConfig := OpensearchEnvConfig{}
	err := envconfig.Process("", &envConfig)
	if err != nil {
		logger.WithError(err).Error("error parsing opensearch env config")
	}
	return &envConfig
}

func NewOpensearchClientFromEnv(logger *logrus.Logger) *opensearch.Client {
	envConfig := getConfigFromEnv(logger)
	addresses := []string{
		envConfig.Address,
	}
	cfg := opensearch.Config{
		Addresses: addresses,
		Username:  envConfig.Username,
		Password:  envConfig.Password,
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   10,
			ResponseHeaderTimeout: envConfig.ResponseTimeout,
			DialContext:           (&net.Dialer{Timeout: envConfig.DialTimeout}).DialContext,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: envConfig.SSLInsecureSkipVerify,
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
	envConfig := getConfigFromEnv(logger)
	opensearch := NewOpensearchClientFromEnv(logger)
	return NewEnrichedEventRepository(logger, opensearch, envConfig.IndexPrefix, ackChannel)
}

func NewBulkIndexerFromEnv(opensearch *opensearch.Client, logger *logrus.Logger) (opensearchutil.BulkIndexer, error) {
	envConfig := getConfigFromEnv(logger)
	return opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{
		NumWorkers:    envConfig.NumWorkers,
		Client:        opensearch,
		FlushBytes:    envConfig.FlushBytes,
		FlushInterval: envConfig.FlushInterval,
		Refresh:       "false",
		Timeout:       envConfig.BulkTimeout,
		OnError: func(ctx context.Context, err error) {
			logger.WithError(err).Warning("bulk indexer failed")
		},
		OnFlushStart: func(ctx context.Context) context.Context {
			logger.Debug("Starting to flush...")
			return ctx
		},
		OnFlushEnd: func(ctx context.Context) {
			logger.Debug("Flushed.")
		},
	})
}
