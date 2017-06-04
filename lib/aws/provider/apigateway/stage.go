// Copyright 2017 Pulumi, Inc. All rights reserved.

package apigateway

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awsapigateway "github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/mapper"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"strings"

	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/apigateway"
)

const StageToken = apigateway.StageToken

// constants for the various stage limits.
const (
	maxStageName = 255
)

// NewStageProvider creates a provider that handles APIGateway Stage operations.
func NewStageProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &stageProvider{ctx}
	return apigateway.NewStageProvider(ops)
}

type stageProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *stageProvider) Check(ctx context.Context, obj *apigateway.Stage) ([]mapper.FieldError, error) {
	var failures []mapper.FieldError

	return failures, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *stageProvider) Create(ctx context.Context, obj *apigateway.Stage) (resource.ID, error) {
	if obj.MethodSettings != nil || obj.ClientCertificate != nil {
		return "", fmt.Errorf("Not yet supported - MethodSettings or ClientCertificate")
	}
	fmt.Printf("Creating APIGateway Stage '%v' with stage name '%v'\n", obj.Name, obj.StageName)
	parts := strings.Split(string(obj.Deployment), ":")
	contract.Assertf(len(parts) == 2, "expected deployment ID to be of the form <restAPIID>:<deploymentid>")
	deploymentID := parts[1]
	create := &awsapigateway.CreateStageInput{
		StageName:           aws.String(obj.StageName),
		RestApiId:           aws.String(string(obj.RestAPI)),
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
	id := resource.ID(string(obj.RestAPI) + ":" + *stage.StageName)
	return id, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *stageProvider) Get(ctx context.Context, id resource.ID) (*apigateway.Stage, error) {
	parts := strings.Split(id.String(), ":")
	contract.Assertf(len(parts) == 2, "expected stage ID to be of the form <restAPIID>:<stagename>")
	restAPIID := parts[0]
	stageName := parts[1]

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
	deploymentID := resource.ID(restAPIID + ":" + aws.StringValue(resp.DeploymentId))
	variables := aws.StringValueMap(resp.Variables)

	return &apigateway.Stage{
		RestAPI:             resource.ID(restAPIID),
		Deployment:          deploymentID,
		CacheClusterEnabled: resp.CacheClusterEnabled,
		CacheClusterSize:    resp.CacheClusterSize,
		StageName:           aws.StringValue(resp.StageName),
		Variables:           &variables,
		Description:         resp.Description,
		CreatedDate:         resp.CreatedDate.String(),
		LastUpdatedDate:     resp.LastUpdatedDate.String(),
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
		parts := strings.Split(string(new.Deployment), ":")
		contract.Assertf(len(parts) == 2, "expected deployment ID to be of the form <restAPIID>:<deploymentid>")
		deploymentID := parts[1]
		ops = append(ops, &awsapigateway.PatchOperation{
			Op:    aws.String("replace"),
			Path:  aws.String("/deploymentId"),
			Value: aws.String(deploymentID),
		})
	}

	parts := strings.Split(id.String(), ":")
	contract.Assertf(len(parts) == 2, "expected stage ID to be of the form <restAPIID>:<stagename>")
	stageName := parts[1]
	if len(ops) > 0 {
		update := &awsapigateway.UpdateStageInput{
			StageName:       aws.String(stageName),
			RestApiId:       aws.String(string(new.RestAPI)),
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
	parts := strings.Split(id.String(), ":")
	contract.Assertf(len(parts) == 2, "expected stage ID to be of the form <restAPIID>:<stagename>")
	restAPIID := parts[0]
	stageName := parts[1]
	_, err := p.ctx.APIGateway().DeleteStage(&awsapigateway.DeleteStageInput{
		RestApiId: aws.String(restAPIID),
		StageName: aws.String(stageName),
	})
	if err != nil {
		return err
	}
	return nil
}
