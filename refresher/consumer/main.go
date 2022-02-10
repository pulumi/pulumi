package main

import (
	"context"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/infralight/pulumi/refresher/common"
	"github.com/infralight/pulumi/refresher/config"
	"github.com/infralight/pulumi/refresher/consumer/engine"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
)

var (
	cfg       *config.Config
	consumer  *common.Consumer
	logger    zerolog.Logger
	sess      *session.Session
	component = "pulumi-mapper-consumer"
)

func init() {
	var err error
	logger.With().Str("component", component).Logger()

	cfg, err = config.LoadConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load configuration from environment variables")
	}

	// Load aws credentials
	sess = cfg.LoadAwsSession()

	consumer, err = common.NewConsumer(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create new consumer")
	}

	if cfg.DebugMode {
		// log verbosely when debug mode is enabled, and use the pretty-print
		// console writer instead of the default JSON writer
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

}

func handler(ctx context.Context) (string, error) {
	lastUpdate := int64(cfg.LastUpdate)
	resourceCount := cfg.ResourceCount
	err := engine.PulumiMapper(ctx, &logger, consumer, cfg.AccountId, cfg.PulumiIntegrationId, cfg.StackName, cfg.ProjectName, cfg.OrganizationName, cfg.StackId, &lastUpdate, &resourceCount )
	if err != nil {
		return "failed", err
	}
	return "success", nil

}

func main() {
	logger = log.With().
		Str("component", component).Logger()
	handler(context.Background())

}