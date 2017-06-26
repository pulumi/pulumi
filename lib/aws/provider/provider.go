// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"fmt"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pulumi/lumi/pkg/resource/provider"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/apigateway"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/provider/dynamodb"
	"github.com/pulumi/lumi/lib/aws/provider/ec2"
	"github.com/pulumi/lumi/lib/aws/provider/elasticbeanstalk"
	"github.com/pulumi/lumi/lib/aws/provider/iam"
	"github.com/pulumi/lumi/lib/aws/provider/lambda"
	"github.com/pulumi/lumi/lib/aws/provider/s3"
)

// Provider implements the AWS resource provider's operations for all known AWS types.
type Provider struct {
	impls map[tokens.Type]lumirpc.ResourceProviderServer
}

// NewProvider creates a new provider instance with server objects registered for every resource type.
func NewProvider(host *provider.HostClient) (*Provider, error) {
	ctx, err := awsctx.New(host)
	if err != nil {
		return nil, err
	}
	return &Provider{
		impls: map[tokens.Type]lumirpc.ResourceProviderServer{
			apigateway.DeploymentToken:               apigateway.NewDeploymentProvider(ctx),
			apigateway.RestAPIToken:                  apigateway.NewRestAPIProvider(ctx),
			apigateway.StageToken:                    apigateway.NewStageProvider(ctx),
			dynamodb.TableToken:                      dynamodb.NewTableProvider(ctx),
			ec2.InstanceToken:                        ec2.NewInstanceProvider(ctx),
			ec2.SecurityGroupToken:                   ec2.NewSecurityGroupProvider(ctx),
			elasticbeanstalk.ApplicationToken:        elasticbeanstalk.NewApplicationProvider(ctx),
			elasticbeanstalk.ApplicationVersionToken: elasticbeanstalk.NewApplicationVersionProvider(ctx),
			elasticbeanstalk.EnvironmentToken:        elasticbeanstalk.NewEnvironmentProvider(ctx),
			lambda.FunctionToken:                     lambda.NewFunctionProvider(ctx),
			lambda.PermissionToken:                   lambda.NewPermissionProvider(ctx),
			iam.InstanceProfileToken:                 iam.NewInstanceProfileProvider(ctx),
			iam.RoleToken:                            iam.NewRoleProvider(ctx),
			s3.BucketToken:                           s3.NewBucketProvider(ctx),
			s3.ObjectToken:                           s3.NewObjectProvider(ctx),
		},
	}, nil
}

var _ lumirpc.ResourceProviderServer = (*Provider)(nil)

// Check validates that the given property bag is valid for a resource of the given type.
func (p *Provider) Check(ctx context.Context, req *lumirpc.CheckRequest) (*lumirpc.CheckResponse, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Check(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Check): %v", t)
}

// Name names a given resource.  Sometimes this will be assigned by a developer, and so the provider
// simply fetches it from the property bag; other times, the provider will assign this based on its own algorithm.
// In any case, resources with the same name must be safe to use interchangeably with one another.
func (p *Provider) Name(ctx context.Context, req *lumirpc.NameRequest) (*lumirpc.NameResponse, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Name(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Name): %v", t)
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *Provider) Create(ctx context.Context, req *lumirpc.CreateRequest) (*lumirpc.CreateResponse, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Create(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Create): %v", t)
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *Provider) Get(ctx context.Context, req *lumirpc.GetRequest) (*lumirpc.GetResponse, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Get(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Get): %v", t)
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *Provider) InspectChange(
	ctx context.Context, req *lumirpc.InspectChangeRequest) (*lumirpc.InspectChangeResponse, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.InspectChange(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (InspectChange): %v", t)
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *Provider) Update(ctx context.Context, req *lumirpc.UpdateRequest) (*pbempty.Empty, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Update(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Update): %v", t)
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *Provider) Delete(ctx context.Context, req *lumirpc.DeleteRequest) (*pbempty.Empty, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Delete(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Delete): %v", t)
}
