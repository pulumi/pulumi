package engine

import (
	"github.com/infralight/go-kit/flywheel/arn"
	"github.com/infralight/pulumi/refresher"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/rs/zerolog"
)


func CreatePulumiNodes(events []engine.Event, accountId, stackId, integrationId, stackName, projectName, organizationName string, logger *zerolog.Logger,) (result []map[string]interface{}, err error) {

	var s3Nodes = make([]map[string]interface{}, 0, len(events))

	for _, event := range events {
		var metadata = getSameMetadata(event)

		var s3Node = make(map[string]interface{})
		s3Node["iac"] = "pulumi"
		s3Node["accountId"] = accountId
		s3Node["integrationId"] = integrationId
		s3Node["stackId"] = stackId
		s3Node["stackName"] = stackName
		s3Node["projectName"] = projectName
		s3Node["organizationName"] = organizationName
		s3Node["isOrchestrator"] = false

		switch metadata.Op {
		case deploy.OpSame:
			newState := *metadata.New
			if len(newState.Outputs) > 0 {
				 s3Node["pulumiState"] = "managed"
				s3Node["arn"] = newState.Outputs["arn"].V
				region, err := getRegionFromArn(s3Node["arn"].(string))
				if err != nil {
					logger.Err(err).Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).Str("projectName", projectName).
						Str("stackName", stackName).Str("OrganizationName", organizationName).Str("arn",s3Node["arn"].(string) ).Msg("failed to parse arn")
					continue
				}
				s3Node["region"] = region
				s3Nodes = append(s3Nodes, s3Node)
			}
			case deploy.OpDelete:
			oldState := *metadata.Old
			if len(oldState.Outputs) > 0 {
				s3Node["pulumiState"] = "ghost"
				s3Node["arn"] = oldState.Outputs["arn"].V
				region, err := getRegionFromArn(s3Node["arn"].(string))
				if err != nil {
					logger.Err(err).Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).Str("projectName", projectName).
						Str("stackName", stackName).Str("OrganizationName", organizationName).Str("arn",s3Node["arn"].(string) ).Msg("failed to parse arn")
					continue
				}
				s3Node["region"] = region
				s3Nodes = append(s3Nodes, s3Node)

			}
			case deploy.OpUpdate:
				newState := *metadata.New
				s3Node["pulumiState"] = "modified"
				s3Node["arn"] = newState.Outputs["arn"].V
				region, err := getRegionFromArn(s3Node["arn"].(string))
				if err != nil {
					logger.Err(err).Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).Str("projectName", projectName).
						Str("stackName", stackName).Str("OrganizationName", organizationName).Str("arn",s3Node["arn"].(string) ).Msg("failed to parse arn")
					continue
				}
				s3Node["region"] = region
				s3Nodes = append(s3Nodes, s3Node)

				drifts := refresher.CalcDrift(metadata)
				s3Node["pulumiDrifts"] = drifts
		}

	}
	return s3Nodes, nil
}

func getSameMetadata(event engine.Event)  engine.StepEventMetadata {
	var metadata engine.StepEventMetadata
	if event.Type == engine.ResourcePreEvent {
		metadata = event.Payload().(engine.ResourcePreEventPayload).Metadata

	} else if event.Type == engine.ResourceOutputsEvent {
		metadata = event.Payload().(engine.ResourceOutputsEventPayload).Metadata
	}
	return metadata
}

func getRegionFromArn(assetArn string) (region string, err error) {
	parsedArn,err := arn.Parse(assetArn)
	if err != nil {
		return "", err
	}
	region = parsedArn.Location
	if region == "" {
		region = "global"
	}
	return region, nil
}
