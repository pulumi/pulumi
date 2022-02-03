package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/infralight/pulumi/refresher"
	"github.com/infralight/pulumi/refresher/common"
	"github.com/infralight/pulumi/refresher/config"
	"github.com/infralight/pulumi/refresher/consumer/internal/engine"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
)

var (
	cfg       *config.Config
	consumer  *common.Consumer
	logger    zerolog.Logger
	component = "pulumi-mapper-consumer"
)

func init() {
	var err error

	logger.With().Str("component", component).Logger()

	cfg, err = config.LoadConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load configuration from environment variables")
	}

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

func handler(ctx context.Context, sqsEvent events.SQSEvent) (string, error) {
	errs := make(map[string]error)

	for _, message := range sqsEvent.Records {
		logger = log.With().Str("messageId", message.MessageId).
			Str("eventSource", message.EventSource).
			Str("component", component).
			Logger()

		err := engine.ProcessMessage(ctx, &logger, consumer, message.Body)
		if err != nil {
			// we want to continue processing other messages, so only log this,
			// we'll only return an error if _all_ messages failed processing
			logger.Warn().Err(err).Msg("failed processing message")
			errs[message.MessageId] = err
			continue
		}
	}

	if len(errs) == len(sqsEvent.Records) {
		return "failure", fmt.Errorf("all %d messages failed processing", len(sqsEvent.Records))
	}

	return "success" ,nil

}

func main() {
	var ProjectName string
	ProjectName = "firefly"
	os.Setenv("ProjectName", ProjectName)
	var c = refresher.NewClient(context.Background(), "https://api.pulumi.com")

	//Login
	var b, err = c.Login()
	if err != nil {
		fmt.Errorf("could not connect to pulumi. error=%w", err)
	}

	stacks, token, err := c.ListStacks(b)
	if err != nil {
		fmt.Errorf("could not get list stacks. error=%w", err)
	}

	fmt.Println(fmt.Sprintf("--DEBUG-- Token %s", token))

	//Something
	c.TempRunner(b, stacks)

}
