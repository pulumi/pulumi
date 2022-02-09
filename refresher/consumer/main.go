package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/infralight/pulumi/refresher/common"
	"github.com/infralight/pulumi/refresher/config"
	"github.com/infralight/pulumi/refresher/consumer/dispatcher"
	"github.com/infralight/pulumi/refresher/consumer/engine"
	"github.com/infralight/pulumi/refresher/consumer/queue"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"time"
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
	sess = config.LoadAwsSession()

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

	return "success", nil

}

func main() {
	logger = log.With().
		Str("component", component).Logger()

	if cfg.RunImmediately {
		//load message body from standard input
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			logger.Fatal().
				Err(err).
				Msg("Message must be provided via standard input")
		}
		_, err = handler(context.Background(), events.SQSEvent{
			Records: []events.SQSMessage{{
				MessageId: "local-test",
				Body:      string(b),
			}},
		})
		if err != nil {
			logger.Fatal().Err(err).Msg("Consumer failed")
			os.Exit(1)
		}

		logger.Info().Msg("Consumer succeeded")
		os.Exit(0)
	}

	if cfg.Lambda {
		lambda.Start(handler)
	} else {
		pubsub := queue.NewSQS(sess, time.Second*10, time.Second*1500)
		logger = log.With().Str("component", component).Int("MaxWorkers", cfg.MaxWorkers).Logger()

		if cfg.PulumiMapperSqsUrl == "" {
			logger.Fatal().Msg("failed missing drift detector sqs url environment variable")
			os.Exit(1)
		}

		dispatcher.NewConsumer(pubsub, dispatcher.ConsumerConfig{
			Type:      dispatcher.AsyncConsumer,
			QueueURL:  cfg.PulumiMapperSqsUrl,
			MaxWorker: cfg.MaxWorkers,
			MaxMsg:    cfg.MaxMsg,
		}, &logger, consumer, engine.ProcessMessage).Start(context.Background())
	}
}