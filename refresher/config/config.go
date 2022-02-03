package config

import (
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"os"
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
		merr = multierror.Append(merr, errors.New(fmt.Sprintf("failed, environment variable %s must be provided" ,httpstate.AccessTokenEnvVar)))
	}

	if cfg.Env = os.Getenv("INFRALIGHT_ENV"); cfg.Env == "" {
		merr = multierror.Append(merr, errors.New("failed, environment variable INFRALIGHT_ENV must be provided"))
	}

	if cfg.FetchedResourcesBucket = os.Getenv("FETCHED_RESOURCES_BUCKET"); cfg.FetchedResourcesBucket == "" {
		if strings.Contains(cfg.Env, "prod") {
			cfg.FetchedResourcesBucket = "prod-fetched-resources"
		} else {
			cfg.FetchedResourcesBucket = "stag-fetched-resources"
		}
	}



	return cfg, merr.ErrorOrNil()
}
