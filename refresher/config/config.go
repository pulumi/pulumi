package config

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"os"
	"runtime"
	"strconv"
	"strings"
)

type Config struct {
	RunImmediately         bool
	DebugMode              bool
	Env                    string
	MongoURI               string
	PulumiToken            string
	FetchedResourcesBucket string
	PulumiMapperSqsUrl     string
	MaxWorkers             int
	MaxMsg                 int
	AwsAccessKeyId         string
	AwsAccessSecretKey     string
	AwsRegion              string
	Lambda                 bool
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

	if cfg.Lambda, err = strconv.ParseBool(os.Getenv("LAMBDA_MODE")); err != nil {
		cfg.Lambda = false
	}

	if cfg.MongoURI = os.Getenv("MONGO_URI"); cfg.MongoURI == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable MONGO_URI must be provided"))
	}

	if cfg.PulumiToken = os.Getenv(httpstate.AccessTokenEnvVar); cfg.PulumiToken == "" {
		merr = multierror.Append(merr, errors.New(fmt.Sprintf("failed, environment variable %s must be provided", httpstate.AccessTokenEnvVar)))
	}

	if cfg.Env = os.Getenv("INFRALIGHT_ENV"); cfg.Env == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable INFRALIGHT_ENV must be provided"))
	}

	if cfg.PulumiMapperSqsUrl = os.Getenv("PULUMI_MAPPER_SQS_QUEUE_URL"); cfg.PulumiMapperSqsUrl == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable PULUMI_MAPPER_SQS_QUEUE_URL must be provided"))
	}

	if cfg.MaxWorkers, err = strconv.Atoi(os.Getenv("MAX_WORKERS")); err != nil {
		cfg.MaxWorkers = runtime.NumCPU()
	}

	if cfg.MaxMsg, err = strconv.Atoi(os.Getenv("MAX_MESSAGES")); err != nil {
		cfg.MaxMsg = 10
	}

	if cfg.AwsRegion = os.Getenv("AWS_REGION"); cfg.AwsRegion == "" {
		if cfg.AwsRegion = os.Getenv("AWS_DEFAULT_REGION"); cfg.AwsRegion == "" {
			cfg.AwsRegion = "us-west-2"
		}
	}

	cfg.AwsAccessKeyId = os.Getenv("AWS_ACCESS_KEY_ID")
	cfg.AwsAccessSecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")

	if cfg.FetchedResourcesBucket = os.Getenv("FETCHED_RESOURCES_BUCKET"); cfg.FetchedResourcesBucket == "" {
		if strings.Contains(cfg.Env, "prod") {
			cfg.FetchedResourcesBucket = "prod-fetched-resources"
		} else {
			cfg.FetchedResourcesBucket = "stag-fetched-resources"
		}
	}

	return cfg, merr.ErrorOrNil()
}

func LoadAwsSession() *session.Session {
	config := aws.Config{}
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}

	if region != "" {
		config = aws.Config{
			Region: aws.String(region),
		}
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config:            config,
		SharedConfigState: session.SharedConfigEnable,
	}))

	return sess
}
