// Copyright 2017 Pulumi, Inc. All rights reserved.

package apigateway

import (
	"crypto/sha1"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	awsapigateway "github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/mapper"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/apigateway"
)

const DeploymentToken = apigateway.DeploymentToken

// constants for the various deployment limits.
const (
	maxDeploymentName = 255
)

// NewDeploymentProvider creates a provider that handles APIGateway Deployment operations.
func NewDeploymentProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &deploymentProvider{ctx}
	return apigateway.NewDeploymentProvider(ops)
}

type deploymentProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *deploymentProvider) Check(ctx context.Context, obj *apigateway.Deployment) ([]mapper.FieldError, error) {
	var failures []mapper.FieldError

	return failures, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *deploymentProvider) Create(ctx context.Context, obj *apigateway.Deployment) (resource.ID, error) {
	// If an explicit name is given, use it.  Otherwise, auto-generate a name in part based on the resource name.
	var stageName string
	if obj.StageName != nil {
		stageName = *obj.StageName
	} else {
		stageName = resource.NewUniqueHex(*obj.Name+"_", maxDeploymentName, sha1.Size)
	}
	fmt.Printf("Creating APIGateway Deployment '%v'\n", obj.Name)
	create := &awsapigateway.CreateDeploymentInput{
		RestApiId:   aws.String(string(obj.RestAPI)),
		Description: obj.Description,
		StageName:   aws.String(stageName),
	}
	deployment, err := p.ctx.APIGateway().CreateDeployment(create)
	if err != nil {
		return "", err
	}
	id := resource.ID(string(obj.RestAPI) + ":" + *deployment.Id)
	return id, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *deploymentProvider) Get(ctx context.Context, id resource.ID) (*apigateway.Deployment, error) {
	parts := strings.Split(id.String(), ":")
	contract.Assertf(len(parts) == 2, "expected deployment ID to be of the form <restAPIID>:<deploymentId>")
	restAPIID := parts[0]
	deploymentID := parts[1]

	resp, err := p.ctx.APIGateway().GetDeployment(&awsapigateway.GetDeploymentInput{
		RestApiId:    aws.String(restAPIID),
		DeploymentId: aws.String(deploymentID),
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Id == nil {
		return nil, nil
	}

	return &apigateway.Deployment{
		RestAPI:     resource.ID(restAPIID),
		ID:          aws.StringValue(resp.Id),
		Description: resp.Description,
		CreatedDate: resp.CreatedDate.String(),
	}, nil
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *deploymentProvider) InspectChange(ctx context.Context, id resource.ID,
	new *apigateway.Deployment, old *apigateway.Deployment, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *deploymentProvider) Update(ctx context.Context, id resource.ID,
	old *apigateway.Deployment, new *apigateway.Deployment, diff *resource.ObjectDiff) error {
	ops, err := patchOperations(diff, apigateway.Deployment_StageName)
	if err != nil {
		return err
	}
	if len(ops) > 0 {
		parts := strings.Split(id.String(), ":")
		contract.Assertf(len(parts) == 2, "expected deployment ID to be of the form <restAPIID>:<deploymentId>")
		deploymentID := parts[1]
		update := &awsapigateway.UpdateDeploymentInput{
			RestApiId:       aws.String(string(new.RestAPI)),
			DeploymentId:    aws.String(deploymentID),
			PatchOperations: ops,
		}
		_, err := p.ctx.APIGateway().UpdateDeployment(update)
		if err != nil {
			return err
		}
	}
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *deploymentProvider) Delete(ctx context.Context, id resource.ID) error {
	fmt.Printf("Deleting APIGateway Deployment '%v'\n", id)
	parts := strings.Split(id.String(), ":")
	contract.Assertf(len(parts) == 2, "expected deployment ID to be of the form <restAPIID>:<deploymentId>")
	restAPIID := parts[0]
	deploymentID := parts[1]
	resp, err := p.ctx.APIGateway().GetStages(&awsapigateway.GetStagesInput{
		RestApiId:    aws.String(restAPIID),
		DeploymentId: aws.String(deploymentID),
	})
	if err != nil || resp == nil {
		return err
	}
	if len(resp.Item) == 1 {
		// Assume that the single stage associated with this deployment
		// is the stage that was automatically created along with the deployment.
		_, err := p.ctx.APIGateway().DeleteStage(&awsapigateway.DeleteStageInput{
			RestApiId: aws.String(restAPIID),
			StageName: resp.Item[0].StageName,
		})
		if err != nil {
			return err
		}
	}
	_, err = p.ctx.APIGateway().DeleteDeployment(&awsapigateway.DeleteDeploymentInput{
		RestApiId:    aws.String(restAPIID),
		DeploymentId: aws.String(deploymentID),
	})
	if err != nil {
		return err
	}
	return nil
}
