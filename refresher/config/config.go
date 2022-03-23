package config

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"os"
	"strconv"
)

var PULUMI_AWS_CREDS_ENVS = []string{
	"AWS_ACCESS_KEY_ID",
	"AWS_ACCESS_KEY",
	"AWS_SECRET_ACCESS_KEY",
	"AWS_SECRET_KEY",
	"AWS_SESSION_TOKEN",
}

type Config struct {
	RunImmediately             bool
	DebugMode                  bool
	MongoURI                   string
	PulumiToken                string
	PulumiUrl                  string
	FetchedResourcesBucket     string
	AwsRegion                  string
	PulumiIntegrationId        string
	AccountId                  string
	ProjectName                string
	StackName                  string
	OrganizationName           string
	StackId                    string
	ResourceCount              int
	LastUpdate                 int
	ClientAWSIntegrationId     string
	FireflyAWSRoleARN          string
	FireflyAWSWebIdentityToken string
	ElasticsearchUrl           string
	EngineAccumulatorDynamo    string
	EngineAccumulatorTTL       int
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

	if cfg.FireflyAWSRoleARN = os.Getenv("AWS_ROLE_ARN"); cfg.FireflyAWSRoleARN == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable AWS_ROLE_ARN must be provided"))
	}

	if cfg.FireflyAWSWebIdentityToken = os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE"); cfg.FireflyAWSWebIdentityToken == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable AWS_WEB_IDENTITY_TOKEN_FILE must be provided"))
	}

	cfg.ResourceCount, err = strconv.Atoi(os.Getenv("RESOURCE_COUNT"))
	if err != nil {
		merr = multierror.Append(merr, errors.New("failed, environment variable RESOURCE_COUNT must be provided"))
	}

	cfg.LastUpdate, err = strconv.Atoi(os.Getenv("LAST_UPDATE"))
	if err != nil {
		merr = multierror.Append(merr, errors.New("failed, environment variable LAST_UPDATE must be provided"))
	}

	if cfg.ElasticsearchUrl = os.Getenv("ELASTICSEARCH_URL"); cfg.ElasticsearchUrl == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable ELASTICSEARCH_URL must be provided"))
	}

	if cfg.EngineAccumulatorDynamo = os.Getenv("ACCUMULATOR_DYNAMODB_NAME"); cfg.EngineAccumulatorDynamo == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable ACCUMULATOR_DYNAMODB_NAME must be provided"))
	}

	cfg.EngineAccumulatorTTL, err = strconv.Atoi(os.Getenv("DYNAMO_EXPIRATION_IN_SECONDS"))
	if err != nil {
		merr = multierror.Append(merr, errors.New("failed, environment variable DYNAMO_EXPIRATION_IN_SECONDS must be provided"))
	}

	cfg.ClientAWSIntegrationId = os.Getenv("AWS_INTEGRATION_ID")
	return cfg, merr.ErrorOrNil()
}

func (cfg *Config) LoadAwsSession() *session.Session {
	// We clear the static AWS credentials of the AWS pulumi integration in order to use the web token identity
	if !cfg.RunImmediately {
		for _, env := range PULUMI_AWS_CREDS_ENVS {
			os.Unsetenv(env)
		}
	}

	config := aws.NewConfig()
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
