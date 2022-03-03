package engine

import (
	"github.com/infralight/go-kit/helpers"
	"github.com/infralight/pulumi/refresher/common"
	"github.com/infralight/pulumi/refresher/config"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/rs/zerolog"
)

func CreateS3Node(events []engine.Event,  logger *zerolog.Logger, config *config.Config, consumer *common.Consumer) ([]map[string]interface{},[]string, error) {

	nodes, assetTypes, err := CreatePulumiNodes(events, logger, config, consumer)
	if err != nil {
		logger.Err(err).Msg("failed to create pulumi Nodes")
		return nil, nil, err
	}
	var s3Nodes = make([]map[string]interface{}, 0, len(nodes))
	for _, node := range nodes {
		var s3Node = make(map[string]interface{})
		s3Node["stackId"] = node.StackId
		s3Node["accountId"] = node.AccountId
		s3Node["iac"] = node.Iac
		s3Node["integrationId"] = node.PulumiIntegrationId
		s3Node["isOrchestrator"] = node.IsOrchestrator
		s3Node["updatedAt"] = node.UpdatedAt
		s3Node["objectType"] = node.ObjectType
		s3Node["metadata"] = helpers.ConvertToMap(&node.Metadata)
		s3Node["arn"] = node.Arn
		s3Node["region"] = node.Region
		s3Node["attributes"] = node.Attributes
		s3Node["name"] = node.Name
		s3Node["assetId"] = node.AssetId


		if node.ProviderAccountId != "" {
			s3Node["providerAccountId"] = node.ProviderAccountId
		}

		if node.AwsIntegration != "" {
			s3Node["awsIntegrationId"] = node.AwsIntegration
		}
		if node.K8sIntegration != "" {
			s3Node["k8sIntegrationId"] = node.K8sIntegration
		}

		if node.Type == "k8s" {
			s3Node["location"] = node.Location
			s3Node["resourceId"] = node.ResourceId
			s3Node["kind"] = node.Kind
		}

		s3Nodes = append(s3Nodes, s3Node)

	}
	return s3Nodes, assetTypes, nil
}
