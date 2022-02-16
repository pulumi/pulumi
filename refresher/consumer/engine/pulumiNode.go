package engine

import (
	"encoding/json"
	"fmt"
	"github.com/infralight/go-kit/flywheel/arn"
	"github.com/infralight/go-kit/helpers"
	goKit "github.com/infralight/go-kit/pulumi"
	"github.com/infralight/pulumi/refresher"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/rs/zerolog"
	"strings"
	"time"
)

func CreatePulumiNodes(events []engine.Event, accountId, stackId, integrationId, stackName, projectName, organizationName string, logger *zerolog.Logger) (result []map[string]interface{}, assetTypes []string, err error) {

	var s3Nodes = make([]map[string]interface{}, 0, len(events))

	for _, event := range events {
		var metadata = getSameMetadata(event)

		var s3Node = make(map[string]interface{})
		iacMetadata := make(map[string]interface{})
		iacMetadata["stackId"] = stackId
		iacMetadata["stackName"] = stackName
		iacMetadata["projectName"] = projectName
		iacMetadata["organizationName"] = organizationName
		iacMetadata["pulumiType"] = metadata.Type.String()

		if !strings.HasPrefix(metadata.Type.String(), "aws:") {
			continue
		}

		s3Node["stackId"] = stackId
		s3Node["iac"] = "pulumi"
		s3Node["accountId"] = accountId
		s3Node["integrationId"] = integrationId
		s3Node["isOrchestrator"] = false
		s3Node["updatedAt"] = time.Now().Unix()

		if terraformType, ok := goKit.TypesMapping[metadata.Type.String()]; ok {
			s3Node["objectType"] = terraformType
			if !helpers.StringSliceContains(assetTypes, terraformType) {
				assetTypes = append(assetTypes, terraformType)
			}
		} else {
			logger.Warn().Str("pulumiAssetType", metadata.Type.String()).Msg("missing pulumi to terraform type mapping")
		}

		switch metadata.Op {
		case deploy.OpSame:
			newState := *metadata.New
			if len(newState.Outputs) > 0 {
				iacMetadata["pulumiState"] = "managed"
				s3Node["metadata"] = iacMetadata
				if ARN := newState.Outputs["arn"].V; ARN != nil {
					s3Node["arn"] = ARN
					awsAccount, region, err := getAccountAndRegionFromArn(fmt.Sprintf("%v", ARN))
					if err != nil {
						logger.Err(err).Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).
							Str("projectName", projectName).Str("stackName", stackName).
							Str("OrganizationName", organizationName).Interface("arn", ARN).
							Msg("failed to parse arn")
						continue
					}
					s3Node["region"] = region
					s3Node["providerAccountId"] = awsAccount
				} else {
					logger.Warn().Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).
						Str("projectName", projectName).Str("stackName", stackName).
						Str("OrganizationName", organizationName).Str("type", metadata.Type.String()).
						Msg("no arn for resource")
					continue
				}
				s3Node["attributes"] = getIacAttributes(newState.Outputs)
				s3Nodes = append(s3Nodes, s3Node)
			}
		case deploy.OpDelete:
			oldState := *metadata.Old
			if len(oldState.Outputs) > 0 {
				iacMetadata["pulumiState"] = "ghost"
				s3Node["metadata"] = iacMetadata
				if ARN := oldState.Outputs["arn"].V; ARN != nil {
					s3Node["arn"] = ARN
					awsAccount, region, err := getAccountAndRegionFromArn(fmt.Sprintf("%v", ARN))
					if err != nil {
						logger.Err(err).Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).
							Str("projectName", projectName).Str("stackName", stackName).
							Str("OrganizationName", organizationName).Interface("arn", ARN).
							Msg("failed to parse arn")
						continue
					}
					s3Node["region"] = region
					s3Node["providerAccountId"] = awsAccount
				} else {
					logger.Warn().Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).
						Str("projectName", projectName).Str("stackName", stackName).
						Str("OrganizationName", organizationName).Str("type", metadata.Type.String()).
						Msg("no arn for resource")
					continue
				}
				s3Node["attributes"] = getIacAttributes(oldState.Outputs)
				s3Nodes = append(s3Nodes, s3Node)

			}
		case deploy.OpUpdate:
			newState := *metadata.New
			if len(newState.Outputs) > 0 {
				drifts, err := refresher.CalcDrift(metadata)
				if err != nil {
					logger.Warn().Err(err).Msg("failed to calc some of the drifts")
				}
				if drifts == nil {
					iacMetadata["pulumiState"] = "managed"

				} else {
					iacMetadata["pulumiState"] = "modified"
					iacMetadata["pulumiDrifts"] = drifts
				}

				s3Node["metadata"] = iacMetadata
				if ARN := newState.Outputs["arn"].V; ARN != nil {
					s3Node["arn"] = ARN
					awsAccount, region, err := getAccountAndRegionFromArn(fmt.Sprintf("%v", ARN))
					if err != nil {
						logger.Err(err).Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).
							Str("projectName", projectName).Str("stackName", stackName).
							Str("OrganizationName", organizationName).Interface("arn", ARN).
							Msg("failed to parse arn")
						continue
					}
					s3Node["region"] = region
					s3Node["providerAccountId"] = awsAccount
				} else {
					logger.Warn().Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).
						Str("projectName", projectName).Str("stackName", stackName).
						Str("OrganizationName", organizationName).Str("type", metadata.Type.String()).
						Msg("no arn for resource")
					continue
				}
				s3Node["attributes"] = getIacAttributes(newState.Outputs)

				s3Nodes = append(s3Nodes, s3Node)
			}
		}

	}
	return s3Nodes, assetTypes,nil
}

func getSameMetadata(event engine.Event) engine.StepEventMetadata {
	var metadata engine.StepEventMetadata
	if event.Type == engine.ResourcePreEvent {
		metadata = event.Payload().(engine.ResourcePreEventPayload).Metadata

	} else if event.Type == engine.ResourceOutputsEvent {
		metadata = event.Payload().(engine.ResourceOutputsEventPayload).Metadata
	}
	return metadata
}

func getAccountAndRegionFromArn(assetArn string) (account, region string, err error) {
	parsedArn, err := arn.Parse(assetArn)
	if err != nil {
		return "", "", err
	}
	region = parsedArn.Location
	if region == "" {
		region = "global"
	}
	return parsedArn.AccountID, region, nil
}

func getIacAttributes(outputs resource.PropertyMap) string {
	iacAttributes := make(map[string]interface{})
	for key, val := range outputs {
		stringKey := fmt.Sprintf("%v", key)
		iacAttributes[stringKey] = val.Mappable()
	}

	attributesBytes, err := json.Marshal(&iacAttributes)
	if err != nil {
		return ""
	}
	return string(attributesBytes)
}

func getStringMetadata(metadata map[string]interface{}) string {
	metadataBytes, err := json.Marshal(&metadata)
	if err != nil {
		return ""
	}
	return string(metadataBytes)
}
