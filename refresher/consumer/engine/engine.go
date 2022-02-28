package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/infralight/pulumi/refresher/common"
	engine "github.com/infralight/pulumi/refresher/consumer/internal"
	"github.com/rs/zerolog"
	"time"
)

var component = "pulumi-mapper-consumer"

func ProcessMessage(ctx context.Context, logger *zerolog.Logger, consumer *common.Consumer, message string) error {
	var event engine.PulumiMapperEvent
	err := json.Unmarshal([]byte(message), &event)
	if err != nil {
		logger.Debug().Msg("failed to unmarshall producer message")
	}

	start := time.Now()
	*logger = logger.With().
		Str("accountId", event.AccountId).
		Str("component", component).
		Str("integrationId", event.IntegrationId).
		Str("stackName", event.StackName).
		Str("stackId", event.StackId).
		Str("organizationName", event.OrganizationName).
		Str("projectName", event.ProjectName).Logger()

	defer func() {
		logger.Info().
			TimeDiff("duration", time.Now(), start).
			Msg("Finished processing job")
	}()

	logger.Info().
		Str("body", message).
		Msg("Handling message")

	if event.AccountId == "" || event.IntegrationId == "" || event.StackName == "" || event.ProjectName == "" || event.OrganizationName == "" || event.StackId == "" {
		return errors.New("failed, invalid message attributes missing [account id / integration id / stackName / projectName / organizationName]")
	}

	err = PulumiMapper(ctx, logger, consumer, &event.LastUpdated, &event.ResourceCount)
	if err != nil {
		return fmt.Errorf("failed processing job: %w", err)
	}

	return nil
}
