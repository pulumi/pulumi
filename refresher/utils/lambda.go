package utils

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/infralight/pulumi/refresher/config"
)

func TriggerFireflyEngine(cfg *config.Config) error {
	sess := cfg.LoadAwsSession()
	lambdaClient := lambda.New(sess)
	payload, err := json.Marshal(map[string]interface{}{
		"accountId":     cfg.AccountId,
		"integrationId": cfg.ClientAWSIntegrationId,
		"provider": "aws",
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
