package engine

import (
	"context"
	"fmt"
	goKitDynamo "github.com/infralight/go-kit/db/dynamo"
	"github.com/infralight/pulumi/refresher"
	"github.com/infralight/pulumi/refresher/common"
	"github.com/infralight/pulumi/refresher/utils"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/rs/zerolog"
	"github.com/thoas/go-funk"
)

const (
	dynamoChunkSize = 25
)

func PulumiMapper(
	ctx context.Context,
	logger *zerolog.Logger,
	consumer *common.Consumer,
	lastUpdate *int64, resourceCount *int) error {

	client, err := refresher.NewClient(context.Background(), consumer.Config.PulumiUrl)
	cfg := consumer.Config
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create new pulumi client")
		return err
	}

	httpBackend, err := client.Login()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to login to pulumi http backend")
		return err
	}

	httpCloudBackend := client.GetHttpBackend(httpBackend, consumer.Config.PulumiUrl)

	stackRef := httpstate.CloudStackSummary{
		Summary: apitype.StackSummary{
			OrgName:       cfg.OrganizationName,
			ProjectName:   cfg.ProjectName,
			StackName:     cfg.StackName,
			LastUpdate:    lastUpdate,
			ResourceCount: resourceCount,
		},
		B: httpCloudBackend,
	}

	stack, err := httpBackend.GetStack(client.Ctx, stackRef.Name())
	if err != nil || stack == nil {
		logger.Err(err).Msg("failed getting stack")
		return consumer.MongoDb.UpdateStateFileDeleted(ctx, consumer.Config.AccountId, consumer.Config.StackId)
	}
	updateOpts := client.GetUpdateOpts()

	dryRunApplierOpts := client.GetDryRunApplierOpts()

	eventsChannel := make(chan engine.Event)
	var events []engine.Event
	go func() {
		// pull the events from the channel and store them locally
		for e := range eventsChannel {
			if e.Type == engine.ResourcePreEvent ||
				e.Type == engine.ResourceOutputsEvent ||
				e.Type == engine.SummaryEvent {

				events = append(events, e)
			}
		}
	}()

	_, _, res := httpCloudBackend.Apply(ctx, apitype.RefreshUpdate, stack, *updateOpts, *dryRunApplierOpts, eventsChannel)
	close(eventsChannel)

	if res != nil && len(events) == 0 {
		logger.Err(res.Error()).Msg("failed running pulumi preview")
		return consumer.MongoDb.UpdateStateFileDeleted(ctx, consumer.Config.AccountId, consumer.Config.StackId)
	}

	//filter out irrelevant events
	events = funk.Filter(events, func(event engine.Event) bool {
		return event.Type != engine.SummaryEvent && getSameMetadata(event).Type.String() != "pulumi:pulumi:Stack"
	}).([]engine.Event)

	if len(events) < 1 {
		logger.Info().Msg("found empty state file")
		return consumer.MongoDb.UpdateEmptyStateFile(ctx, consumer.Config.AccountId, consumer.Config.StackId)
	}
	nodes, atrsToTrigger, err := CreateS3Node(events, logger, consumer.Config, consumer)
	if err != nil {
		logger.Err(err).Msg("failed to create s3 nodes")
		return err
	}
	if len(nodes) == 0 {
		logger.Info().Msg("no nodes found")
		return nil
	}
	jsonlinesNodes, err := utils.ToJsonLines(nodes)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create jsonlines format")
		return err
	}

	s3Path := fmt.Sprintf("%s/pulumi_resources/%s/iac_objects.jsonl", cfg.AccountId, cfg.StackId)

	err = utils.WriteFile(consumer.Config, s3Path, jsonlinesNodes, "jsonl")
	if err != nil {
		logger.Err(err).Str("accountId", cfg.AccountId).Str("pulumiIntegrationId", cfg.PulumiIntegrationId).Str("projectName", cfg.ProjectName).
			Str("stackName", cfg.StackName).Str("OrganizationName", cfg.OrganizationName).Msg("failed to write nodes to s3 bucket")
		return err
	}
	logger.Info().Str("accountId", cfg.AccountId).Str("pulumiIntegrationId", cfg.PulumiIntegrationId).Str("projectName", cfg.ProjectName).
		Str("stackName", cfg.StackName).Int("records", len(nodes)).Str("OrganizationName", cfg.OrganizationName).Msg("Successfully wrote nodes to s3 bucket")

	dynamoClient, err := goKitDynamo.NewClient(consumer.Config.LoadAwsSession())
	if err != nil {
		logger.Err(err).Msg("failed to load aws session")
		return err
	}
	atrsChunks := funk.Chunk(atrsToTrigger, dynamoChunkSize)
	for _, chunk := range atrsChunks.([][]string) {
		items, err := utils.GetAtrsFromDynamo(cfg.AccountId, cfg.EngineAccumulatorDynamo, chunk, dynamoClient)
		if err != nil {
			logger.Err(err).Msg("failed to get batch items from dynamodb")
			continue
		}

		filteredAtrs, err := utils.DiffDynamoItems(items, chunk, cfg.AccountId)
		if err != nil {
			logger.Err(err).Msg("failed to calculate dynamo diff")
			continue
		}

		err = utils.WriteAtrsToDynamo(cfg.AccountId, cfg.EngineAccumulatorDynamo, filteredAtrs, cfg.EngineAccumulatorTTL, dynamoClient)
		logger.Info().Int("Records", len(filteredAtrs)).Msg("Successfully wrote chunk to dynamo")
		if err != nil {
			logger.Err(err).Msg("failed to write batch items to dynamo db")
		}
	}

	logger.Info().Msg("Successfully triggered engine producer from dynamodb")
	return nil
}
