package config

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"os"
	"strconv"
)

type Config struct {
	RunImmediately         bool
	DebugMode              bool
	MongoURI               string
	PulumiToken            string
	PulumiUrl              string
	FetchedResourcesBucket string
	AwsRegion              string
	PulumiIntegrationId    string
	AccountId              string
	ProjectName            string
	StackName              string
	OrganizationName       string
	StackId                string
	ResourceCount          int
	LastUpdate             int
	FireflyAWSAccessKey    string
	FireflyAWSSecretKey    string
	FireflyAWSSessionToken string
	FireflyEngineLambdaArn string
	ClientAWSIntegrationId string
}

func LoadConfig() (*Config, error) {
	var err error
	var merr *multierror.Error
	cfg := &Config{}

	if cfg.RunImmediately, err = strconv.ParseBool(os.Getenv("RUN_IMMEDIATELY")); err != nil {
		cfg.RunImmediately = false
	}

	if cfg.DebugMode, err = strconv.ParseBool(os.Getenv("DEBUG_MODE")); err != nil {
		cfg.DebugMode = true
	}

	if cfg.MongoURI = os.Getenv("MONGO_URI"); cfg.MongoURI == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable MONGO_URI must be provided"))
	}

	if cfg.PulumiToken = os.Getenv(httpstate.AccessTokenEnvVar); cfg.PulumiToken == "" {
		merr = multierror.Append(merr, errors.New(fmt.Sprintf("failed, environment variable %s must be provided", httpstate.AccessTokenEnvVar)))
	}

	if cfg.PulumiUrl = os.Getenv("PULUMI_URL"); cfg.PulumiUrl == "" {
		cfg.PulumiUrl = "https://api.pulumi.com"
	}

	if cfg.AwsRegion = os.Getenv("AWS_REGION"); cfg.AwsRegion == "" {
		if cfg.AwsRegion = os.Getenv("AWS_DEFAULT_REGION"); cfg.AwsRegion == "" {
			cfg.AwsRegion = "us-west-2"
		}
	}

	if cfg.FetchedResourcesBucket = os.Getenv("FETCHED_RESOURCES_BUCKET"); cfg.FetchedResourcesBucket == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable FETCHED_RESOURCES_BUCKET must be provided"))
	}

	if cfg.PulumiIntegrationId = os.Getenv("INTEGRATION_ID"); cfg.PulumiIntegrationId == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable INTEGRATION_ID must be provided"))
	}

	if cfg.AccountId = os.Getenv("ACCOUNT_ID"); cfg.AccountId == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable ACCOUNT_ID must be provided"))
	}

	if cfg.ProjectName = os.Getenv("PROJECT_NAME"); cfg.ProjectName == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable PROJECT_NAME must be provided"))
	}

	if cfg.StackName = os.Getenv("STACK_NAME"); cfg.StackName == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable STACK_NAME must be provided"))
	}

	if cfg.OrganizationName = os.Getenv("ORGANIZATION_NAME"); cfg.OrganizationName == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable ORGANIZATION_NAME must be provided"))
	}

	if cfg.StackId = os.Getenv("STACK_ID"); cfg.StackId == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable STACK_ID must be provided"))
	}

	if cfg.StackId = os.Getenv("STACK_ID"); cfg.StackId == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable STACK_ID must be provided"))
	}

	if cfg.FireflyAWSAccessKey = os.Getenv("FIREFLY_AWS_ACCESS_KEY_ID"); cfg.FireflyAWSAccessKey == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable FIREFLY_AWS_ACCESS_KEY_ID must be provided"))
	}

	if cfg.FireflyAWSSecretKey = os.Getenv("FIREFLY_AWS_SECRET_ACCESS_KEY"); cfg.FireflyAWSSecretKey == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable FIREFLY_AWS_SECRET_ACCESS_KEY must be provided"))
	}

	if cfg.FireflyAWSSessionToken = os.Getenv("FIREFLY_AWS_SESSION_TOKEN"); cfg.FireflyAWSSessionToken == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable FIREFLY_AWS_SESSION_TOKEN must be provided"))
	}

	if cfg.FireflyEngineLambdaArn = os.Getenv("ENGINE_PRODUCER_LAMBDA_ARN"); cfg.FireflyEngineLambdaArn == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable ENGINE_PRODUCER_LAMBDA_ARN must be provided"))
	}

	if cfg.ClientAWSIntegrationId = os.Getenv("AWS_INTEGRATION_ID"); cfg.ClientAWSIntegrationId == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable AWS_INTEGRATION_ID must be provided"))
	}

	cfg.ResourceCount, err = strconv.Atoi(os.Getenv("RESOURCE_COUNT"))
	if err != nil {
		merr = multierror.Append(merr, errors.New("failed, environment variable RESOURCE_COUNT must be provided"))
	}

	cfg.LastUpdate, err = strconv.Atoi(os.Getenv("LAST_UPDATE"))
	if err != nil {
		merr = multierror.Append(merr, errors.New("failed, environment variable LAST_UPDATE must be provided"))
	}

	return cfg, merr.ErrorOrNil()
}

func (cfg *Config) LoadAwsSession() *session.Session {
	config := aws.NewConfig().
		WithCredentials(credentials.NewStaticCredentialsFromCreds(credentials.Value{
			AccessKeyID:     cfg.FireflyAWSAccessKey,
			SecretAccessKey: cfg.FireflyAWSSecretKey,
			SessionToken:    cfg.FireflyAWSSessionToken,
		}))
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}

	if region != "" {
		config.WithRegion(region)
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config:            *config,
		SharedConfigState: session.SharedConfigEnable,
	}))

	return sess
}
