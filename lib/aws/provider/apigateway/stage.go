// Copyright 2017 Pulumi, Inc. All rights reserved.

package apigateway

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awsapigateway "github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"strings"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/apigateway"
)

const StageToken = apigateway.StageToken

// NewStageID returns an AWS APIGateway Stage ARN ID for the given restAPIID and stageID
func NewStageID(region, restAPIID, stageID string) resource.ID {
	return arn.NewID("apigateway", region, "", "/restapis/"+restAPIID+"/stages/"+stageID)
}

// ParseStageID parses an AWS APIGateway Stage ARN ID to extract the restAPIID and stageID
func ParseStageID(id resource.ID) (string, string, error) {
	res, err := arn.ParseResourceName(id)
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(res, "/")
	if len(parts) != 4 || parts[0] != "restapis" || parts[2] != "stages" {
		return "", "", fmt.Errorf(
			"expected Stage ARN of the form arn:aws:apigateway:region::/restapis/api-id/stages/stage-id: %v", id)
	}
	return parts[1], parts[3], nil
}

// NewStageProvider creates a provider that handles APIGateway Stage operations.
func NewStageProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &stageProvider{ctx}
	return apigateway.NewStageProvider(ops)
}

type stageProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *stageProvider) Check(ctx context.Context, obj *apigateway.Stage, property string) error {
	return nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *stageProvider) Create(ctx context.Context, obj *apigateway.Stage) (resource.ID, error) {
	if obj.MethodSettings != nil || obj.ClientCertificate != nil {
		return "", fmt.Errorf("Not yet supported - MethodSettings or ClientCertificate")
	}
	fmt.Printf("Creating APIGateway Stage '%v' with stage name '%v'\n", *obj.Name, obj.StageName)
	restAPIID, deploymentID, err := ParseDeploymentID(obj.Deployment)
	if err != nil {
		return "", err
	}
	create := &awsapigateway.CreateStageInput{
		StageName:           aws.String(obj.StageName),
		RestApiId:           aws.String(restAPIID),
		DeploymentId:        aws.String(deploymentID),
		Description:         obj.Description,
		CacheClusterEnabled: obj.CacheClusterEnabled,
		CacheClusterSize:    obj.CacheClusterSize,
	}
	if obj.Variables != nil {
		create.Variables = aws.StringMap(*obj.Variables)
	}
	stage, err := p.ctx.APIGateway().CreateStage(create)
	if err != nil {
		return "", err
	}
	return NewStageID(p.ctx.Region(), restAPIID, *stage.StageName), nil
}

// Query returns an (possibly empty) array of resource objects.
func (p *stageProvider) Query(ctx context.Context) ([]*apigateway.Stage, error) {
	restAPIs := restapi.Query(ctx)
	var stages []*apigateway.Stage
	for _, restAPI := range restAPIs {
		for _, deploys := range p.ctx.APIGateway().GetDeployments(restAPI.Id).Items {
			deploymentStages := p.ctx.APIGateway().GetStages(deploys, restAPI).Item
			for stage := range deploymentStages {
				variables := aws.StringValueMap(stage.Variables)
				url := "https://" + restAPI.Id + ".execute-api." + p.ctx.Region() + ".amazonaws.com/" + stage.StageName
				executionARN := "arn:aws:execute-api:" + p.ctx.Region() + ":" + p.ctx.AccountID() + ":" + restAPI.Id + "/" + stage.StageName

				stages = append(stages, &apigateway.Stage{
					RestAPI:             NewRestAPIID(p.ctx.Region(), restAPI.Id),
					Deployment:          NewDeploymentID(p.ctx.Region(), restAPI.Id, aws.StringValue(stage.DeploymentId)),
					CacheClusterEnabled: stage.CacheClusterEnabled,
					CacheClusterSize:    stage.CacheClusterSize,
					StageName:           aws.StringValue(stage.StageName),
					Variables:           &variables,
					Description:         stage.Description,
					CreatedDate:         stage.CreatedDate.String(),
					LastUpdatedDate:     stage.LastUpdatedDate.String(),
					URL:                 url,
					ExecutionARN:        executionARN,
				})
			}
		}
	}

	return stages, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *stageProvider) Get(ctx context.Context, id resource.ID) (*apigateway.Stage, error) {
	restAPIID, stageName, err := ParseStageID(id)
	if err != nil {
		return nil, err
	}

	resp, err := p.ctx.APIGateway().GetStage(&awsapigateway.GetStageInput{
		RestApiId: aws.String(restAPIID),
		StageName: aws.String(stageName),
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.DeploymentId == nil {
		return nil, nil
	}
	variables := aws.StringValueMap(resp.Variables)

	url := "https://" + restAPIID + ".execute-api." + p.ctx.Region() + ".amazonaws.com/" + stageName
	executionARN := "arn:aws:execute-api:" + p.ctx.Region() + ":" + p.ctx.AccountID() + ":" + restAPIID + "/" + stageName

	return &apigateway.Stage{
		RestAPI:             NewRestAPIID(p.ctx.Region(), restAPIID),
		Deployment:          NewDeploymentID(p.ctx.Region(), restAPIID, aws.StringValue(resp.DeploymentId)),
		CacheClusterEnabled: resp.CacheClusterEnabled,
		CacheClusterSize:    resp.CacheClusterSize,
		StageName:           aws.StringValue(resp.StageName),
		Variables:           &variables,
		Description:         resp.Description,
		CreatedDate:         resp.CreatedDate.String(),
		LastUpdatedDate:     resp.LastUpdatedDate.String(),
		URL:                 url,
		ExecutionARN:        executionARN,
	}, nil
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *stageProvider) InspectChange(ctx context.Context, id resource.ID,
	new *apigateway.Stage, old *apigateway.Stage, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *stageProvider) Update(ctx context.Context, id resource.ID,
	old *apigateway.Stage, new *apigateway.Stage, diff *resource.ObjectDiff) error {
	ops, err := patchOperations(diff, apigateway.Stage_Deployment)
	if err != nil {
		return err
	}

	if diff.Updated(apigateway.Stage_Deployment) {
		_, deploymentID, deperr := ParseDeploymentID(new.Deployment)
		if deperr != nil {
			return deperr
		}
		ops = append(ops, &awsapigateway.PatchOperation{
			Op:    aws.String("replace"),
			Path:  aws.String("/deploymentId"),
			Value: aws.String(deploymentID),
		})
	}

	restAPIId, stageName, err := ParseStageID(id)
	if err != nil {
		return err
	}
	if len(ops) > 0 {
		update := &awsapigateway.UpdateStageInput{
			StageName:       aws.String(stageName),
			RestApiId:       aws.String(restAPIId),
			PatchOperations: ops,
		}
		_, err := p.ctx.APIGateway().UpdateStage(update)
		if err != nil {
			return err
		}
	}
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *stageProvider) Delete(ctx context.Context, id resource.ID) error {
	fmt.Printf("Deleting APIGateway Stage '%v'\n", id)
	restAPIID, stageName, err := ParseStageID(id)
	if err != nil {
		return err
	}
	_, delerr := p.ctx.APIGateway().DeleteStage(&awsapigateway.DeleteStageInput{
		RestApiId: aws.String(restAPIID),
		StageName: aws.String(stageName),
	})
	return delerr
}
