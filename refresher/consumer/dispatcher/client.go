package dispatcher

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/infralight/pulumi/refresher/common"
	"github.com/infralight/pulumi/refresher/consumer/queue"
	"github.com/rs/zerolog"
	"sync"
)

type ConsumerType string

const (
	// SyncConsumer Consumers to consume messages one by one.
	// A single goroutine handles all messages.
	// Progression is slower and requires less system resource.
	// Ideal for quiet/non-critical queues.
	SyncConsumer ConsumerType = "blocking"
	// AsyncConsumer Consumers to consume messages at the same time.
	// Runs an individual goroutine per message.
	// Progression is faster and requires more system resource.
	// Ideal for busy/critical queues.
	AsyncConsumer ConsumerType = "non-blocking"
)

type ConsumerConfig struct {
	// Instructs whether to consume messages come from a worker synchronously or asynchronous.
	Type ConsumerType
	// Queue URL to receive messages from.
	QueueURL string
	// Maximum workers that will independently receive messages from a queue.
	MaxWorker int
	// Maximum messages that will be picked up by a worker in one-go.
	MaxMsg int
}

type Consumer struct {
	client queue.SQS
	config ConsumerConfig
	cs     *common.Consumer
	logger *zerolog.Logger
	runner func(context.Context, *zerolog.Logger, *common.Consumer, string) error
}

func NewConsumer(client queue.SQS, config ConsumerConfig, logger *zerolog.Logger, cs *common.Consumer, runner func(context.Context, *zerolog.Logger, *common.Consumer, string) error) Consumer {
	return Consumer{
		client: client,
		config: config,
		cs:     cs,
		logger: logger,
		runner: runner,
	}
}

func (c Consumer) Start(ctx context.Context) {
	wg := &sync.WaitGroup{}
	wg.Add(c.config.MaxWorker)

	for i := 1; i <= c.config.MaxWorker; i++ {
		go c.worker(ctx, wg, fmt.Sprintf("%d", i))
	}

	wg.Wait()
}

func (c Consumer) worker(ctx context.Context, wg *sync.WaitGroup, id string) {
	defer wg.Done()

	c.logger.Info().Str("id", id).Msg("worker started")

	for {
		select {
		case <-ctx.Done():
			c.logger.Info().Str("id", id).Msg("worker stopped")
			return
		default:
		}

		msgs, err := c.client.Receive(ctx, c.config.QueueURL, int64(c.config.MaxMsg))
		if err != nil {
			c.logger.Warn().Err(err).Str("id", id).Msg("worker failed")
			continue
		}

		if len(msgs) == 0 {
			continue
		}

		if c.config.Type == SyncConsumer {
			c.sync(ctx, msgs)
		} else {
			c.async(ctx, msgs)
		}
	}
}

func (c Consumer) sync(ctx context.Context, msgs []*sqs.Message) {
	for _, msg := range msgs {
		c.consume(ctx, msg)
	}
}

func (c Consumer) async(ctx context.Context, msgs []*sqs.Message) {
	wg := &sync.WaitGroup{}
	wg.Add(len(msgs))

	for _, msg := range msgs {
		go func(msg *sqs.Message) {
			defer wg.Done()

			c.consume(ctx, msg)
		}(msg)
	}

	wg.Wait()
}

func (c Consumer) consume(ctx context.Context, msg *sqs.Message) {
	logger := c.logger.With().Str("message_id", *msg.MessageId).Logger()
	err := c.runner(ctx, &logger, c.cs, *msg.Body)
	if err != nil {
		c.logger.Warn().Err(err).Msg("failed to process runner function")
		return
	}

	if err := c.client.Delete(ctx, c.config.QueueURL, *msg.ReceiptHandle); err != nil {
		c.logger.Warn().Err(err).Msg("failed to delete message from sqs")
	}
}
