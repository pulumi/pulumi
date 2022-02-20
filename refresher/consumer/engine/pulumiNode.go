package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/infralight/go-kit/flywheel/arn"
	"github.com/infralight/go-kit/helpers"
	k8sUtils "github.com/infralight/go-kit/k8s"
	goKit "github.com/infralight/go-kit/pulumi"
	goKitTypes "github.com/infralight/go-kit/types"
	k8sApiUtils "github.com/infralight/k8s-api/pkg/utils"
	"github.com/infralight/pulumi/refresher"
	"github.com/infralight/pulumi/refresher/config"
	"github.com/infralight/pulumi/refresher/utils"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/rs/zerolog"
	"github.com/thoas/go-funk"
	"strings"
	"time"
)

func CreatePulumiNodes(events []engine.Event, accountId, stackId, integrationId, stackName, projectName, organizationName string, logger *zerolog.Logger, config *config.Config) (result []map[string]interface{}, assetTypes []string, err error) {

	var s3Nodes = make([]map[string]interface{}, 0, len(events))
	var k8sNodes = make([]map[string]interface{}, 0, len(events))
	var uids []string
	var kinds []string

	for _, event := range events {
		var metadata = getSameMetadata(event)

		var s3Node = make(map[string]interface{})
		iacMetadata := make(map[string]interface{})
		iacMetadata["stackId"] = stackId
		iacMetadata["stackName"] = stackName
		iacMetadata["projectName"] = projectName
		iacMetadata["organizationName"] = organizationName
		iacMetadata["pulumiType"] = metadata.Type.String()

		s3Node["stackId"] = stackId
		s3Node["iac"] = "pulumi"
		s3Node["accountId"] = accountId
		s3Node["integrationId"] = integrationId
		s3Node["isOrchestrator"] = false
		s3Node["updatedAt"] = time.Now().Unix()

		if strings.HasPrefix(metadata.Type.String(), "aws:") {
			terraformType, err := goKit.GetTerraformTypeByPulumi(metadata.Type.String())
			if err != nil {
				logger.Warn().Str("pulumiAssetType", metadata.Type.String()).Msg("missing pulumi to terraform type mapping")
			} else {
				s3Node["objectType"] = terraformType
				if !helpers.StringSliceContains(assetTypes, terraformType) {
					assetTypes = append(assetTypes, terraformType)
				}
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

		} else if strings.HasPrefix(metadata.Type.String(), "kubernetes:") {
			// k8s flow currently supports only managed state
			var uid string
			newState := *metadata.New
			s3Node["metadata"] = iacMetadata
			s3Node["attributes"], err = getK8sIacAttributes(newState.Outputs,  []string{"status", "__inputs", "__initialApiVersion"})
			if err != nil {
				logger.Err(err).Msg("failed to get iac attributes")
				continue
			}
			if resourceMetadata := newState.Outputs["metadata"].Mappable(); resourceMetadata != nil {
				namespace := funk.Get(resourceMetadata, "namespace")
				name := funk.Get(resourceMetadata, "name")
				interfaceUid := funk.Get(resourceMetadata, "uid")
				uid =  goKitTypes.ToString(interfaceUid)

				if name == nil || interfaceUid == nil {
					logger.Warn().Err(errors.New("found resource with empty name/uid"))
					continue
				}
				if namespace != nil {
					s3Node["location"] = goKitTypes.ToString(namespace)
				} else{
					s3Node["location"] = ""
				}
				s3Node["name"] = goKitTypes.ToString(name)
				s3Node["resourceId"] = goKitTypes.ToString(interfaceUid)

			} else {
				logger.Warn().Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).
					Str("stackId", stackId).Msg("found k8s resource without metadata")
				continue
			}
			var resourceKind interface{}
			if resourceKind = newState.Outputs["kind"].V; resourceKind == nil {
				logger.Warn().Str("accountId", accountId).Str("pulumiIntegrationId", integrationId).
					Str("stackId", stackId).Msg("found k8s resource without kind")
				continue
			}
			kind := goKitTypes.ToString(resourceKind)
			s3Node["objectType"] = k8sUtils.GetKubernetesResourceType(kind, goKitTypes.ToString(s3Node["name"]))
			s3Node["kind"] = kind
			if !helpers.StringSliceContains(uids, uid) {
				uids = append(uids, uid)
			}

			if !helpers.StringSliceContains(kinds,kind) {
				kinds = append(kinds, kind)
			}
			assetTypes = append(assetTypes, goKitTypes.ToString(s3Node["objectType"]))
			k8sNodes = append(k8sNodes, s3Node)
		}

	}
	if len(k8sNodes) > 0 {
		k8sNodes, err = buildK8sArns(k8sNodes, accountId, uids, kinds, config, logger)
		if err != nil {
			logger.Err(err).Msg("failed to build k8s arns")
		} else {
			s3Nodes = append(s3Nodes, k8sNodes...)
		}
	}
	return s3Nodes, assetTypes, nil
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
		if ! helpers.StringSliceContains(blackList, stringKey) {
			iacAttributes[stringKey] = val.Mappable()
		}
	}
	if metadata, err := k8sApiUtils.GetMapFromMap(iacAttributes, "metadata"); err == nil {
		_ = k8sApiUtils.ConvertItemToYaml(metadata, "managedFields")

	} else {
		return "",  errors.New("failed to get metadata")
	}

	if data, err := k8sApiUtils.GetMapFromMap(iacAttributes, "data"); err == nil {
		if dataSpec, err := k8sApiUtils.GetMapFromMap(data, "spec"); err == nil {
			if err = k8sApiUtils.ConvertItemToYaml(dataSpec, "template"); err != nil {
				return "",  errors.New("failed to find or to yaml template")
			}
		}
	}
	attributesBytes, err := json.Marshal(&iacAttributes)
	if err != nil {
		return "",  errors.New("failed to find or to marshal attributes")
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

func buildK8sArns(k8sNodes []map[string]interface{}, accountId string, uids, kinds []string, cfg *config.Config, logger *zerolog.Logger) ([]map[string]interface{}, error) {
	integrationIds, err := utils.GetK8sIntegrationIds(accountId, uids, kinds, logger)
	if err != nil || len(integrationIds) == 0 {
		logger.Err(err).Msg("failed to get k8s integration")
		return nil, errors.New("failed to get k8s integration")
	}
	if len(integrationIds) > 1 {
		return nil, errors.New("found more than one k8s integrations")
	}
	ctx := context.Background()
	clusterId, err := utils.GetClusterId(ctx, cfg, integrationIds[0], accountId, logger)
	if err != nil {
		logger.Err(err).Msg("failed to get cluster id")
		return nil, err
	}
	funk.Map(k8sNodes, func(node map[string]interface{}) map[string]interface{} {
		if namespace, ok := node["location"]; ok && namespace != nil {
			node["arn"] = k8sUtils.BuildArn(goKitTypes.ToString(namespace), clusterId, goKitTypes.ToString(node["kind"]), goKitTypes.ToString(node["name"]))
			node["assetId"] = k8sUtils.BuildArn(goKitTypes.ToString(namespace), clusterId, goKitTypes.ToString(node["kind"]), goKitTypes.ToString(node["name"]))
			node["k8sIntegration"] = integrationIds[0]
		} else {
			node["arn"] = k8sUtils.BuildArn("", clusterId, goKitTypes.ToString(node["kind"]), goKitTypes.ToString(node["name"]))
			node["assetId"] = k8sUtils.BuildArn("", clusterId, goKitTypes.ToString(node["kind"]), goKitTypes.ToString(node["name"]))
			node["k8sIntegration"] = integrationIds[0]
		}
		return node
	})
	return k8sNodes, nil
}
