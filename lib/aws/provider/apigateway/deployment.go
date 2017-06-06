// Copyright 2017 Pulumi, Inc. All rights reserved.

package apigateway

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	awsapigateway "github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/apigateway"
)

const DeploymentToken = apigateway.DeploymentToken

// constants for the various deployment limits.
const (
	maxDeploymentName = 255
)

// NewDeploymentID returns an AWS APIGateway Deployment ARN ID for the given restAPIID and deploymentID
func NewDeploymentID(region, restAPIID, deploymentID string) resource.ID {
	return arn.NewID("apigateway", region, "", "/restapis/"+restAPIID+"/deployments/"+deploymentID)
}

// ParseDeploymentID parses an AWS APIGateway Deployment ARN ID to extract the restAPIID and deploymentID
func ParseDeploymentID(id resource.ID) (string, string, error) {
	res, err := arn.ParseResourceName(id)
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(res, "/")
	if len(parts) != 4 || parts[0] != "restapis" || parts[2] != "deployments" {
		return "", "", fmt.Errorf("expected Deployment ARN of the form %v: %v",
			"arn:aws:apigateway:region::/restapis/api-id/deployments/deployment-id", id)
	}
	return parts[1], parts[3], nil
}

// NewDeploymentProvider creates a provider that handles APIGateway Deployment operations.
func NewDeploymentProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &deploymentProvider{ctx}
	return apigateway.NewDeploymentProvider(ops)
}

type deploymentProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *deploymentProvider) Check(ctx context.Context, obj *apigateway.Deployment) ([]error, error) {
	return nil, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *deploymentProvider) Create(ctx context.Context, obj *apigateway.Deployment) (resource.ID, error) {
	restAPIID, err := ParseRestAPIID(obj.RestAPI)
	if err != nil {
		return "", err
	}
	fmt.Printf("Creating APIGateway Deployment '%v'\n", obj.Name)
	create := &awsapigateway.CreateDeploymentInput{
		RestApiId:   aws.String(restAPIID),
		Description: obj.Description,
	}
	deployment, err := p.ctx.APIGateway().CreateDeployment(create)
	if err != nil {
		return "", err
	}
	id := NewDeploymentID(p.ctx.Region(), restAPIID, *deployment.Id)
	return id, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *deploymentProvider) Get(ctx context.Context, id resource.ID) (*apigateway.Deployment, error) {
	restAPIID, deploymentID, err := ParseDeploymentID(id)
	if err != nil {
		return nil, err
	}
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
		RestAPI:     NewRestAPIID(p.ctx.Region(), restAPIID),
		Description: resp.Description,
		ID:          aws.StringValue(resp.Id),
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
	ops, err := patchOperations(diff)
	if err != nil {
		return err
	}
	if len(ops) > 0 {
		restAPIID, deploymentID, err := ParseDeploymentID(id)
		if err != nil {
			return err
		}
		update := &awsapigateway.UpdateDeploymentInput{
			RestApiId:       aws.String(restAPIID),
			DeploymentId:    aws.String(deploymentID),
			PatchOperations: ops,
		}
		_, err = p.ctx.APIGateway().UpdateDeployment(update)
		if err != nil {
			return err
		}
	}
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *deploymentProvider) Delete(ctx context.Context, id resource.ID) error {
	fmt.Printf("Deleting APIGateway Deployment '%v'\n", id)
	restAPIID, deploymentID, err := ParseDeploymentID(id)
	if err != nil {
		return err
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
