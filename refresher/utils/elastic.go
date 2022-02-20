package utils

import (
	"errors"
	"github.com/infralight/go-kit/db/elasticsearch"
	"github.com/infralight/pulumi/refresher/config"
	"github.com/rs/zerolog"
)

func GetK8sIntegrationIds(accountId string, uids []string, kinds []string, logger *zerolog.Logger) ([]string, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Err(err).Msg("failed to load configuration")
		return nil, errors.New("failed to load configuration")
	}
	client, err := elasticsearch.NewClient(cfg.ElasticsearchUrl)
	if err != nil {
		logger.Err(err).Msg("failed to create elastic search client")
		return nil, errors.New("failed to create es client")
	}

	integrationIds, err := client.GetK8sIntegrationIds(accountId, uids, kinds)
	if err != nil{
		logger.Err(err).Msg("failed to get k8s integration ids")
		return nil, err
	}

	return integrationIds, nil

}

