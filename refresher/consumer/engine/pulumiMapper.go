package engine

import (
	"context"
	"fmt"
	"github.com/infralight/pulumi/refresher"
	"github.com/infralight/pulumi/refresher/common"
	"github.com/infralight/pulumi/refresher/utils"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/rs/zerolog"
)

func PulumiMapper(
	ctx context.Context,
	logger *zerolog.Logger,
	consumer *common.Consumer,
	accountId, integrationId, stackName, projectName, organizationName, stackId string, lastUpdate *int64, resourceCount *int) error {

	client, err := refresher.NewClient(context.Background(), consumer.Config.PulumiUrl)
	if err != nil {
		logger.Fatal().Err(err).Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).Str("projectName", projectName).
			Str("stackName", stackName).Str("OrganizationName", organizationName).Msg("failed to create new pulumi client")
		return err
	}

	httpBackend, err := client.Login()
	if err != nil {
		logger.Fatal().Err(err).Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).Str("projectName", projectName).
			Str("stackName", stackName).Str("OrganizationName", organizationName).Msg("failed to login to pulumi http backend")
		return err
	}

	httpCloudBackend := client.GetHttpBackend(httpBackend, consumer.Config.PulumiUrl)

	stackRef := httpstate.CloudStackSummary{
		Summary: apitype.StackSummary{
			OrgName:       organizationName,
			ProjectName:   projectName,
			StackName:     stackName,
			LastUpdate:    lastUpdate,
			ResourceCount: resourceCount,
		},
		B: httpCloudBackend,
	}

	stack, err := httpBackend.GetStack(client.Ctx, stackRef.Name())

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

	httpCloudBackend.Apply(ctx, apitype.RefreshUpdate, stack, *updateOpts, *dryRunApplierOpts, eventsChannel)
	close(eventsChannel)
	nodes, assetTypes, err := CreatePulumiNodes(events, accountId, stackId, integrationId, stackName, projectName, organizationName, logger, consumer.Config)

	jsonlinesNodes, err := utils.ToJsonLines(nodes)
	if err != nil {
		logger.Fatal().Err(err).Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).Str("projectName", projectName).
			Str("stackName", stackName).Str("OrganizationName", organizationName).Msg("failed to create jsonlines format")
		return err
	}

	s3Path := fmt.Sprintf("%s/pulumi_resources/%s/iac_objects.jsonl", accountId, stackId)

	err = utils.WriteFile(consumer.Config, s3Path, jsonlinesNodes, "jsonl")
	if err != nil {
		logger.Err(err).Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).Str("projectName", projectName).
			Str("stackName", stackName).Str("OrganizationName", organizationName).Msg("failed to write nodes to s3 bucket")
		return err
	}
	logger.Info().Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).Str("projectName", projectName).
		Str("stackName", stackName).Str("OrganizationName", organizationName).Msg("Successfully wrote nodes to s3 bucket")


	err = utils.InvokeEngineLambda(consumer.Config, assetTypes, logger)
	if err != nil {
		logger.Err(err).Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).Str("projectName", projectName).
			Str("stackName", stackName).Str("OrganizationName", organizationName).Msg("failed to trigger engine producer")
		return err
	}
	logger.Info().Str("accountId", accountId).Str("awsIntegrationId", consumer.Config.ClientAWSIntegrationId).Str("pulumiIntegrationId", integrationId).Str("projectName", projectName).
		Str("stackName", stackName).Str("OrganizationName", organizationName).Msg("Successfully triggered engine producer")
	return nil

}
