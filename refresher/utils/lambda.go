package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/hashicorp/go-multierror"
	goKit "github.com/infralight/go-kit/aws"
	"github.com/infralight/pulumi/refresher/config"
	"github.com/rs/zerolog"
)

func TriggerFireflyEngine(cfg *config.Config) error {
	sess := cfg.LoadAwsSession()
	lambdaClient := lambda.New(sess)
	payload, err := json.Marshal(map[string]interface{}{
		"accountId":     cfg.AccountId,
		"integrationId": cfg.ClientAWSIntegrationId,
		"provider":      "aws",
	})
	if err != nil {
		return fmt.Errorf("failed to marshal engine event payload: %v", err)
	}
	_, err = lambdaClient.Invoke(&lambda.InvokeInput{
		FunctionName:   aws.String(cfg.FireflyEngineLambdaArn),
		InvocationType: aws.String(lambda.InvocationTypeEvent),
		Payload:        payload,
	})
	if err != nil {
		return fmt.Errorf("falied to invoke engine lambda %s: %v", cfg.FireflyEngineLambdaArn, err)
	}
	return nil
}

func InvokeEngineLambda(cfg *config.Config, assetTypes []string, logger *zerolog.Logger) error {
	var merr *multierror.Error
	awsSession := cfg.LoadAwsSession()
	for _, asset := range assetTypes {
		var payload interface{} = map[string]interface{}{
			"assetType":     asset,
			"accountId":     cfg.AccountId,
			"integrationId": cfg.ClientAWSIntegrationId,
		}
		err := goKit.InvokeLambdaAsync(cfg.FireflyEngineLambdaArn, payload, awsSession)
		if err != nil {
			logger.Err(err).Str("assetType", asset).Str("accountId", cfg.AccountId).Str("awsIntegrationId", cfg.ClientAWSIntegrationId).Msg("failed to invoke firefly engine producer")
			merr = multierror.Append(merr, errors.New(fmt.Sprintf("failed to invoke lambda: accountId: %s awsIntegrationId: %s assetType: %s" ,cfg.AccountId, cfg.ClientAWSIntegrationId, asset)))
			continue

		}
	}
	return merr.ErrorOrNil()
}
