// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"fmt"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
	"golang.org/x/net/context"
)

// provider implements the Fission resource package functionality behind a gRPC interface.
type Provider struct {
	impls map[tokens.Type]cocorpc.ResourceProviderServer
}

// NewProvider creates a new provider instance with server objects registered for every resource type.
func NewProvider() (*Provider, error) {
	ctx := NewContext()
	return &Provider{
		impls: map[tokens.Type]cocorpc.ResourceProviderServer{
			EnvironmentToken: NewEnvironmentProvider(ctx),
			FunctionToken:    NewFunctionProvider(ctx),
			HTTPTriggerToken: NewHTTPTriggerProvider(ctx),
		},
	}, nil
}

var _ cocorpc.ResourceProviderServer = (*Provider)(nil)

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
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Name(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Name): %v", t)
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

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *Provider) Get(ctx context.Context, req *cocorpc.GetRequest) (*cocorpc.GetResponse, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Get(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Get): %v", t)
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *Provider) InspectChange(
	ctx context.Context, req *cocorpc.ChangeRequest) (*cocorpc.InspectChangeResponse, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.InspectChange(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (InspectChange): %v", t)
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *Provider) Update(ctx context.Context, req *cocorpc.ChangeRequest) (*pbempty.Empty, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Update(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Update): %v", t)
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *Provider) Delete(ctx context.Context, req *cocorpc.DeleteRequest) (*pbempty.Empty, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Delete(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Delete): %v", t)
}
