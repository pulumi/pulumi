package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/infralight/go-kit/db/mongo"
	"github.com/infralight/go-kit/flywheel/arn"
	"github.com/infralight/go-kit/helpers"
	k8sUtils "github.com/infralight/go-kit/k8s"
	goKit "github.com/infralight/go-kit/pulumi"
	goKitTypes "github.com/infralight/go-kit/types"
	k8sApiUtils "github.com/infralight/k8s-api/pkg/utils"
	"github.com/infralight/pulumi/refresher"
	"github.com/infralight/pulumi/refresher/common"
	"github.com/infralight/pulumi/refresher/config"
	"github.com/infralight/pulumi/refresher/utils"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/rs/zerolog"
	"github.com/thoas/go-funk"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strings"
	"time"
)

func CreatePulumiNodes(events []engine.Event, logger *zerolog.Logger, config *config.Config, consumer *common.Consumer) (result []PulumiNode, assetTypes []string, err error) {

	var nodes []PulumiNode
	var k8sNodes []PulumiNode
	var uids []string
	var kinds []string
	awsCommonProviders := make(map[string]int)
	k8sCommonProviders := make(map[string]int)
	ctx := context.Background()

	awsIntegrations, err := consumer.MongoDb.ListAWSIntegrations(ctx, config.AccountId)
	if err != nil {
		logger.Err(err).Msg("failed to list aws integrations")
		return nil, nil, err
	}
	k8sIntegrations, err := consumer.MongoDb.ListK8SIntegrations(ctx, config.AccountId)
	if err != nil {
		logger.Err(err).Msg("failed to list k8s integrations")
		return nil, nil, err
	}

	stack, err := consumer.MongoDb.GetStack(ctx, config.AccountId, config.StackId, nil)
	if err != nil {
		logger.Err(err).Msg("failed to get stack")
		return nil, nil, err
	}

	for _, event := range events {
		var metadata = getSameMetadata(event)
		var state engine.StepEventStateMetadata

		var node PulumiNode
		node.Metadata.StackId = config.StackId
		node.Metadata.StackName = config.StackName
		node.Metadata.ProjectName = config.ProjectName
		node.Metadata.OrganizationName = config.OrganizationName
		node.Metadata.PulumiType = metadata.Type.String()
		node.StackId = config.StackId
		node.Iac = "pulumi"
		node.AccountId = config.AccountId
		node.PulumiIntegrationId = config.PulumiIntegrationId
		node.IsOrchestrator = false
		node.UpdatedAt = time.Now().Unix()

		if strings.HasPrefix(metadata.Type.String(), "aws:") {
			node.Type = "aws"
			terraformType, err := goKit.GetTerraformTypeByPulumi(metadata.Type.String())
			if err != nil {
				logger.Err(err).Str("pulumiAssetType", metadata.Type.String()).Msg("missing pulumi to terraform type mapping")
			} else {
				node.ObjectType = terraformType
				if !helpers.StringSliceContains(assetTypes, terraformType) {
					assetTypes = append(assetTypes, terraformType)
				}
			}

			switch metadata.Op {
			case deploy.OpSame:
				state = *metadata.New
				node.Metadata.PulumiState = "managed"
			case deploy.OpDelete:
				state = *metadata.Old
				node.Metadata.PulumiState = "ghost"
			case deploy.OpUpdate:
				state = *metadata.New
				drifts, err := refresher.CalcDrift(metadata)
				if err != nil {
					logger.Warn().Err(err).Msg("failed to calc some of the drifts")
				}
				if drifts == nil {
					node.Metadata.PulumiState = "managed"

				} else {
					node.Metadata.PulumiState = "modified"
					node.Metadata.PulumiDrifts = drifts
				}
			case deploy.OpRefresh:
				if consumer.Config.ClientAWSIntegrationId == "" {
					state = *metadata.New
				} else {
					continue
				}

			default:
				continue
			}
			if len(state.Outputs) == 0 {
				continue
			}
			node.Attributes = getIacAttributes(state.Outputs)
			if ARN := state.Outputs["arn"].V; ARN != nil {
				node.Arn = goKitTypes.ToString(ARN)
				awsAccount, region, err := getAccountAndRegionFromArn(fmt.Sprintf("%v", ARN))
				if err != nil {
					logger.Err(err).Interface("arn", ARN).Msg("failed to parse arn")
					continue
				}
				node.Region = region
				node.ProviderAccountId = awsAccount
				node.AssetId = node.Arn
				if awsIntegrationId := getAwsIntegrationId(awsIntegrations, awsAccount); awsIntegrationId != "" {
					node.AwsIntegration = awsIntegrationId
				}
				if count, ok := awsCommonProviders[awsAccount]; ok {
					awsCommonProviders[awsAccount] = count + 1
				} else {
					awsCommonProviders[awsAccount] = 1
				}

			} else {
				logger.Warn().Str("type", metadata.Type.String()).Msg("no arn for resource")
				continue
			}
			// if we don't have the aws integration id - we use the only calculating the most common provider.
			if consumer.Config.ClientAWSIntegrationId != "" {
				nodes = append(nodes, node)

			}
		} else if strings.HasPrefix(metadata.Type.String(), "kubernetes:") {
			node.Type = "k8s"
			// k8s flow currently supports only managed state
			var uid string
			var state engine.StepEventStateMetadata
			switch metadata.Op {
			case deploy.OpSame:
				state = *metadata.New
			case deploy.OpUpdate:
				state = *metadata.New
			case deploy.OpDelete:
				state = *metadata.Old
			default:
				continue

			}
			node.Attributes, err = getK8sIacAttributes(state.Outputs, []string{"status", "__inputs", "__initialApiVersion"})
			if err != nil {
				logger.Err(err).Msg("failed to get iac attributes")
				continue
			}
			if resourceMetadata := state.Outputs["metadata"].Mappable(); resourceMetadata != nil {
				namespace := funk.Get(resourceMetadata, "namespace")
				name := funk.Get(resourceMetadata, "name")
				interfaceUid := funk.Get(resourceMetadata, "uid")
				uid = goKitTypes.ToString(interfaceUid)

				if name == nil || interfaceUid == nil {
					logger.Warn().Err(errors.New("found resource with empty name/uid"))
					continue
				}
				if namespace != nil {
					node.Location = goKitTypes.ToString(namespace)
				} else {
					node.Location = ""
				}
				node.Name = goKitTypes.ToString(name)
				node.ResourceId = goKitTypes.ToString(interfaceUid)

			} else {
				logger.Warn().Msg("found k8s resource without metadata")
				continue
			}
			var resourceKind interface{}
			if resourceKind = state.Outputs["kind"].V; resourceKind == nil {
				logger.Warn().Msg("found k8s resource without kind")
				continue
			}
			kind := goKitTypes.ToString(resourceKind)
			node.ObjectType = k8sUtils.GetKubernetesResourceType(kind, goKitTypes.ToString(node.Name))
			node.Kind = kind
			if !helpers.StringSliceContains(uids, uid) {
				uids = append(uids, uid)
			}

			if !helpers.StringSliceContains(kinds, kind) {
				kinds = append(kinds, kind)
			}
			assetTypes = append(assetTypes, goKitTypes.ToString(node.ObjectType))
			k8sNodes = append(k8sNodes, node)
		}

	}
	if len(k8sNodes) > 0 {
		k8sNodes, clusterId, err := buildK8sArns(k8sNodes, uids, kinds , logger, consumer, k8sIntegrations)
		if err != nil {
			logger.Err(err).Msg("failed to build k8s arns")
		} else {
			nodes = append(nodes, k8sNodes...)
			k8sCommonProviders[clusterId] = 1

		}
	}

	err = handleCommonProviders(ctx, awsCommonProviders, stack, awsIntegrations, consumer, "aws")
	if err != nil {
		logger.Err(err).Msg("failed to update aws common provider")
	}

	err = handleCommonProviders(ctx, k8sCommonProviders, stack, k8sIntegrations, consumer, "k8s")
	if err != nil {
		logger.Err(err).Msg("failed to update aws common provider")
	}

	return nodes, assetTypes, nil
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

func getK8sIacAttributes(outputs resource.PropertyMap, blackList []string) (string, error) {
	// in case we want k8s attributes we use blacklist for redundant attributes
	iacAttributes := make(map[string]interface{})
	for key, val := range outputs {
		stringKey := fmt.Sprintf("%v", key)
		if !helpers.StringSliceContains(blackList, stringKey) {
			iacAttributes[stringKey] = val.Mappable()
		}
	}
	if metadata, err := k8sApiUtils.GetMapFromMap(iacAttributes, "metadata"); err == nil {
		_ = k8sApiUtils.ConvertItemToYaml(metadata, "managedFields")

	} else {
		return "", errors.New("failed to get metadata")
	}

	if data, err := k8sApiUtils.GetMapFromMap(iacAttributes, "data"); err == nil {
		if dataSpec, err := k8sApiUtils.GetMapFromMap(data, "spec"); err == nil {
			if err = k8sApiUtils.ConvertItemToYaml(dataSpec, "template"); err != nil {
				return "", errors.New("failed to find or to yaml template")
			}
		}
	}
	attributesBytes, err := json.Marshal(&iacAttributes)
	if err != nil {
		return "", errors.New("failed to find or to marshal attributes")
	}
	return string(attributesBytes), nil
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

func buildK8sArns(k8sNodes []PulumiNode,  uids, kinds []string,  logger *zerolog.Logger,consumer *common.Consumer, k8sIntegrations []mongo.K8sIntegration) ([]PulumiNode, string, error) {
	var clusterId string
	integrationIds, err := utils.GetK8sIntegrationIds(consumer.Config.AccountId, uids, kinds, logger)
	if err != nil || len(integrationIds) == 0 {
		logger.Err(err).Msg("failed to get k8s integration")
		clusterId = "K8sCluster"
	}
	if len(integrationIds) > 1 {
		return nil, "", errors.New("found more than one k8s integrations")
	}
	if clusterId != "K8sCluster" {
		k8sIntegration := getK8sIntegrationObjectById(k8sIntegrations, integrationIds[0])
		if k8sIntegration == nil {
			logger.Err(err).Msg("failed to get k8s integration")
			return nil, "", err
		}
		clusterId = k8sIntegration.ClusterId
	}

	k8sNodes = funk.Map(k8sNodes, func(node PulumiNode) PulumiNode {
		node.Arn = k8sUtils.BuildArn(goKitTypes.ToString(node.Location), clusterId, goKitTypes.ToString(node.Kind), goKitTypes.ToString(node.Name))
		node.AssetId = k8sUtils.BuildArn(goKitTypes.ToString(node.Location), clusterId, goKitTypes.ToString(node.Kind), goKitTypes.ToString(node.Name))
		if len(integrationIds) > 0 {
			node.K8sIntegration = integrationIds[0]
		}
		return node
	}).([]PulumiNode)
	return k8sNodes, clusterId, nil
}

func getAwsIntegrationId(awsIntegrations []mongo.AwsIntegration, providerAccountId string) string {
	if awsIntegrations != nil {
		for _, integration := range awsIntegrations {
			if integration.AccountNumber == providerAccountId {
				return integration.ID
			}
		}
	}
	return ""
}

func getK8sIntegrationId(k8sIntegrations []mongo.K8sIntegration, clusterId string) string {
	if k8sIntegrations != nil {
		for _, integration := range k8sIntegrations {
			if integration.ClusterId == clusterId {
				return integration.ID
			}
		}
	}
	return ""
}

func getK8sIntegrationObjectById(k8sIntegrations []mongo.K8sIntegration, integrationId string) *mongo.K8sIntegration {
	if k8sIntegrations != nil {
		for _, integration := range k8sIntegrations {
			if integration.ID == integrationId {
				return &integration
			}
		}
	}
	return nil
}

func handleCommonProviders(ctx context.Context, commonProviderMap map[string]int, stack *mongo.GlobalStack, integrationsArray interface{}, consumer *common.Consumer, provider string) error {

	if len(commonProviderMap) != 0 {
		max := 0
		var mostCommonProvider string
		var err error
		updateDict := make(bson.M)
		for providerId, count := range commonProviderMap {
			if count > max {
				mostCommonProvider = providerId
			}
		}
		var integrationId string
		if provider == "aws" {
			integrationArray := integrationsArray.([]mongo.AwsIntegration)
			integrationId = getAwsIntegrationId(integrationArray, mostCommonProvider)

		} else if provider == "k8s" {
			integrationArray := integrationsArray.([]mongo.K8sIntegration)
			integrationId = getK8sIntegrationId(integrationArray, mostCommonProvider)

		}

		if mongoIntegrationObject, ok := stack.Integrations[provider]; ok {
			if externalId, ok := mongoIntegrationObject["externalId"]; ok {
				if externalId != mostCommonProvider {
					updateDict["integrations.aws.externalId"] = externalId
				}
			}
			if IntegrationId, ok := mongoIntegrationObject["id"]; ok {
				if IntegrationId != integrationId && integrationId != "" {
					updateDict[fmt.Sprintf("integrations.%s.id", provider)], err = primitive.ObjectIDFromHex(integrationId)
					if err != nil {
						return err
					}
				}
			}
		} else {

			updateDict[fmt.Sprintf("integrations.%s.externalId", provider)] = mostCommonProvider
			if integrationId != "" {
				updateDict[fmt.Sprintf("integrations.%s.id", provider)], err = primitive.ObjectIDFromHex(integrationId)
				if err != nil {
					return err
				}
			}
		}

		if len(updateDict) != 0 {
			updateDict["updatedAt"] = time.Now().Format(time.RFC3339)
			_, err = consumer.MongoDb.UpdateStack(ctx, consumer.Config.AccountId, consumer.Config.StackId, nil, bson.M{
				"$set": updateDict,
			})
			if err != nil {
				return err
			}
		}

	}
	return nil
}