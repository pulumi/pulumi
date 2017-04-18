// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"fmt"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pulumi/coconut/pkg/resource"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
	"golang.org/x/net/context"

	"github.com/pulumi/coconut/lib/aws/provider/awsctx"
	"github.com/pulumi/coconut/lib/aws/provider/ec2"
	"github.com/pulumi/coconut/lib/aws/provider/iam"
	"github.com/pulumi/coconut/lib/aws/provider/lambda"
	"github.com/pulumi/coconut/lib/aws/provider/s3"
)

// provider implements the AWS resource provider's operations for all known AWS types.
type Provider struct {
	impls map[tokens.Type]cocorpc.ResourceProviderServer
}

// NewProvider creates a new provider instance with server objects registered for every resource type.
func NewProvider() (*Provider, error) {
	ctx, err := awsctx.New()
	if err != nil {
		return nil, err
	}
	return &Provider{
		impls: map[tokens.Type]cocorpc.ResourceProviderServer{
			ec2.Instance:      ec2.NewInstanceProvider(ctx),
			ec2.SecurityGroup: ec2.NewSecurityGroupProvider(ctx),
			lambda.Function:   lambda.NewFunctionProvider(ctx),
			iam.Role:          iam.NewRoleProvider(ctx),
			s3.Bucket:         s3.NewBucketProvider(ctx),
			s3.Object:         s3.NewObjectProvider(ctx),
		},
	}, nil
}

var _ cocorpc.ResourceProviderServer = (*Provider)(nil)

const nameProperty string = "name" // the property used for naming AWS resources.

// Check validates that the given property bag is valid for a resource of the given type.
func (p *Provider) Check(ctx context.Context, req *cocorpc.CheckRequest) (*cocorpc.CheckResponse, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Check(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Check): %v", t)
}

// Name names a given resource.  Sometimes this will be assigned by a developer, and so the provider
// simply fetches it from the property bag; other times, the provider will assign this based on its own algorithm.
// In any case, resources with the same name must be safe to use interchangeably with one another.
func (p *Provider) Name(ctx context.Context, req *cocorpc.NameRequest) (*cocorpc.NameResponse, error) {
	// First, see if the provider overrides the naming.
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		if res, err := prov.Name(ctx, req); res != nil || err != nil {
			return res, err
		}
	} else {
		return nil, fmt.Errorf("Unrecognized resource type (Name): %v", t)
	}

	// If the provider didn't override, we can go ahead and default to the name property.
	// TODO: eventually, we want to specialize some resources, like SecurityGroups, since they already have names.
	if nameprop, has := req.GetProperties().Fields[nameProperty]; has {
		name := resource.UnmarshalPropertyValue(nameprop)
		if name.IsString() {
			return &cocorpc.NameResponse{Name: name.StringValue()}, nil
		} else {
			return nil, fmt.Errorf(
				"Resource '%v' had a name property '%v', but it wasn't a string", t, nameProperty)
		}
	}
	return nil, fmt.Errorf("Resource '%v' was missing a name property '%v'", t, nameProperty)
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *Provider) Create(ctx context.Context, req *cocorpc.CreateRequest) (*cocorpc.CreateResponse, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Create(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Create): %v", t)
}

// Read reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *Provider) Read(ctx context.Context, req *cocorpc.ReadRequest) (*cocorpc.ReadResponse, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Read(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Read): %v", t)
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *Provider) Update(ctx context.Context, req *cocorpc.UpdateRequest) (*pbempty.Empty, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Update(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Update): %v", t)
}

// UpdateImpact checks what impacts a hypothetical update will have on the resource's properties.
func (p *Provider) UpdateImpact(
	ctx context.Context, req *cocorpc.UpdateRequest) (*cocorpc.UpdateImpactResponse, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.UpdateImpact(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (UpdateImpact): %v", t)
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *Provider) Delete(ctx context.Context, req *cocorpc.DeleteRequest) (*pbempty.Empty, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Delete(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Delete): %v", t)
}
