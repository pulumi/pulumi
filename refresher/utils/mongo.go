package utils

import (
	"context"
	"github.com/infralight/go-kit/db/mongo"
	"github.com/infralight/pulumi/refresher/config"
	"github.com/rs/zerolog"
)

func GetClusterId(ctx context.Context, cfg *config.Config, integrationId, accountId string, logger *zerolog.Logger) (string, error) {
	client, err := mongo.NewClient(cfg.MongoURI)
	if err != nil {
		logger.Err(err).Msg("failed to create new mongo client")
		return "", err
	}

	integration, err := client.GetK8sIntegration(ctx, integrationId, accountId)
	if err != nil {
		logger.Err(err).Msg("failed to get matching k8s integration ids")
		return "", err
	}
	return integration.ClusterId, nil

}
