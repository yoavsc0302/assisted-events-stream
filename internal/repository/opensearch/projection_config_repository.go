package opensearch

import (
	"context"
	"encoding/json"
	"net/http"

	opensearch "github.com/opensearch-project/opensearch-go"
	"github.com/openshift-assisted/assisted-events-streams/internal/types"
	"github.com/sirupsen/logrus"
)

type ProjectionConfigRepository struct {
	opensearchClient *opensearch.Client
	logger           *logrus.Logger
	index            string
	docId            string
}

func NewProjectionConfigRepository(logger *logrus.Logger, opensearchClient *opensearch.Client) *ProjectionConfigRepository {
	envConfig := getConfigFromEnv(logger)
	return &ProjectionConfigRepository{
		logger:           logger,
		opensearchClient: opensearchClient,
		index:            envConfig.ConfigIndex,
		docId:            envConfig.DocId,
	}
}

func (r *ProjectionConfigRepository) Get(ctx context.Context) types.ProjectionConfig {
	defaultConfig := types.ProjectionConfig{
		Mode: types.ProjectionModeOnline,
	}
	res, err := r.opensearchClient.Get(r.index, r.docId)
	if res != nil && !res.IsError() && res.StatusCode < http.StatusBadRequest && err == nil {
		var rawRecord types.OpensearchRawConfigDocument
		err = json.NewDecoder(res.Body).Decode(&rawRecord)
		if err != nil {
			r.logger.WithError(err).Warn("projection config retrieved but could not parse it, providing default")
			return defaultConfig
		}
		return types.ProjectionConfig{
			Mode: rawRecord.Source.Mode,
		}
	}
	r.logger.WithError(err).Warn("could not retrieve projection config, providing default")
	return defaultConfig
}
