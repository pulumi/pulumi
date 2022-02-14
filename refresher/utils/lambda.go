package utils

import (
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	goKit "github.com/infralight/go-kit/aws"
	"github.com/infralight/pulumi/refresher/config"
	"github.com/rs/zerolog"
)

func InvokeEngineLambda(cfg *config.Config, assetTypes []string, logger *zerolog.Logger) error {
	var merr *multierror.Error
	awsSession := cfg.LoadAwsSession()
	for _, asset := range assetTypes {
		var payload interface{} = map[string]interface{}{
			"assetType":     asset,
			"accountId":     cfg.AccountId,
			"integrationId": cfg.ClientAWSIntegrationId,
		}
		logger.Info().Str("assetType", asset).Interface("payload", payload).Str("accountId", cfg.AccountId).Str("awsIntegrationId", cfg.ClientAWSIntegrationId).Msg("invoking engine")
		err := goKit.InvokeLambdaAsync(cfg.FireflyEngineLambdaArn, payload, awsSession)
		if err != nil {
			logger.Err(err).Str("assetType", asset).Str("accountId", cfg.AccountId).Str("awsIntegrationId", cfg.ClientAWSIntegrationId).Msg("failed to invoke firefly engine producer")
			merr = multierror.Append(merr, errors.New(fmt.Sprintf("failed to invoke lambda: accountId: %s awsIntegrationId: %s assetType: %s" ,cfg.AccountId, cfg.ClientAWSIntegrationId, asset)))
			continue

		}
	}
	return merr.ErrorOrNil()
}
