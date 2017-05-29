// Copyright 2017 Pulumi, Inc. All rights reserved.

package apigateway

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awsapigateway "github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/mapper"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/apigateway"
)

const RestAPIToken = apigateway.RestAPIToken

// constants for the various restAPI limits.
const (
	maxRestAPIName = 255
)

// NewRestAPIProvider creates a provider that handles APIGateway RestAPI operations.
func NewRestAPIProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &restAPIProvider{ctx}
	return apigateway.NewRestAPIProvider(ops)
}

type restAPIProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *restAPIProvider) Check(ctx context.Context, obj *apigateway.RestAPI) ([]mapper.FieldError, error) {
	var failures []mapper.FieldError

	return failures, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *restAPIProvider) Create(ctx context.Context, obj *apigateway.RestAPI) (resource.ID, error) {
	var apiName string
	if obj.APIName != nil {
		apiName = *obj.APIName
	} else {
		apiName = resource.NewUniqueHex(obj.Name+"-", maxRestAPIName, sha1.Size)
	}

	// First create the API Gateway
	fmt.Printf("Creating APIGateway RestAPI '%v' with name '%v'\n", obj.Name, apiName)
	create := &awsapigateway.CreateRestApiInput{
		Name:        aws.String(apiName),
		Description: obj.Description,
		CloneFrom:   obj.CloneFrom.StringPtr(),
	}
	restAPI, err := p.ctx.APIGateway().CreateRestApi(create)
	if err != nil {
		return "", err
	}
	id := resource.ID(*restAPI.Id)

	// Next, if a body is specified, put the rest api contents
	if obj.Body != nil {
		body := *obj.Body
		bodyJSON, _ := json.Marshal(body)
		fmt.Printf("APIGateway RestAPI created: %v; putting API contents from OpenAPI specification\n", id)
		put := &awsapigateway.PutRestApiInput{
			RestApiId: id.StringPtr(),
			Body:      bodyJSON,
			Mode:      aws.String("overwrite"),
		}
		_, err := p.ctx.APIGateway().PutRestApi(put)
		if err != nil {
			return "", err
		}
	}

	return id, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *restAPIProvider) Get(ctx context.Context, id resource.ID) (*apigateway.RestAPI, error) {
	return nil, errors.New("Not yet implemented - Get")
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *restAPIProvider) InspectChange(ctx context.Context, id resource.ID,
	new *apigateway.RestAPI, old *apigateway.RestAPI, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *restAPIProvider) Update(ctx context.Context, id resource.ID,
	old *apigateway.RestAPI, new *apigateway.RestAPI, diff *resource.ObjectDiff) error {

	if diff.Updated(apigateway.RestAPI_Body) {
		if new.Body != nil {
			body := *new.Body
			bodyJSON, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("Could not convert Swagger defintion object to JSON: %v", err)
			}
			fmt.Printf("Updating API definition for %v from OpenAPI specification\n", id)
			put := &awsapigateway.PutRestApiInput{
				RestApiId: id.StringPtr(),
				Body:      bodyJSON,
				Mode:      aws.String("overwrite"),
			}
			newAPI, err := p.ctx.APIGateway().PutRestApi(put)
			if err != nil {
				return err
			}
			fmt.Printf("Updated to: %v\n", newAPI)
		} else {
			return errors.New("Cannot remove Body from Rest API which previously had one")
		}
	}
	ops, err := patchOperations(diff, apigateway.RestAPI_Body)
	if err != nil {
		return err
	}
	if len(ops) > 0 {
		update := &awsapigateway.UpdateRestApiInput{
			RestApiId:       id.StringPtr(),
			PatchOperations: ops,
		}
		_, err := p.ctx.APIGateway().UpdateRestApi(update)
		if err != nil {
			return err
		}
	}
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *restAPIProvider) Delete(ctx context.Context, id resource.ID) error {
	fmt.Printf("Deleting APIGateway RestAPI '%v'\n", id)
	_, err := p.ctx.APIGateway().DeleteRestApi(&awsapigateway.DeleteRestApiInput{
		RestApiId: id.StringPtr(),
	})
	if err != nil {
		return err
	}
	return nil
}
