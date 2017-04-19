// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"errors"
	"fmt"

	"github.com/fission/fission"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	pbstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/pulumi/coconut/pkg/resource"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
	"github.com/pulumi/coconut/pkg/util/mapper"
	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
	"golang.org/x/net/context"
)

const Environment = tokens.Type("kube-fission:environment:Environment")

// NewEnvironmentProvider creates a provider that handles Fission environment operations.
func NewEnvironmentProvider(ctx *Context) cocorpc.ResourceProviderServer {
	return &envProvider{ctx}
}

type envProvider struct {
	ctx *Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *envProvider) Check(ctx context.Context, req *cocorpc.CheckRequest) (*cocorpc.CheckResponse, error) {
	// Read in the properties, create and validate a new group, and return the failures (if any).
	contract.Assert(req.GetType() == string(Environment))
	_, _, result := unmarshalEnvironment(req.GetProperties())
	return resource.NewCheckResponse(result), nil
}

// Name names a given resource.  Sometimes this will be assigned by a developer, and so the provider
// simply fetches it from the property bag; other times, the provider will assign this based on its own algorithm.
// In any case, resources with the same name must be safe to use interchangeably with one another.
func (p *envProvider) Name(ctx context.Context, req *cocorpc.NameRequest) (*cocorpc.NameResponse, error) {
	contract.Failf("Kube-Fission top-level dispatcher should have handled this RPC")
	return nil, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *envProvider) Create(ctx context.Context, req *cocorpc.CreateRequest) (*cocorpc.CreateResponse, error) {
	contract.Assert(req.GetType() == string(Environment))

	// Read in the properties given by the request, validating as we go; if any fail, reject the request.
	env, _, decerr := unmarshalEnvironment(req.GetProperties())
	if decerr != nil {
		return nil, decerr
	}

	// Perform the operation by contacting the controller.
	fmt.Printf("Creating Fission environment '%v'\n", env.Metadata.Name)
	meta, err := p.ctx.Fission.EnvironmentCreate(&env)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Fission Environment '%v' created: version=%v\n", meta.Name, meta.Uid)
	return &cocorpc.CreateResponse{Id: meta.Name}, nil
}

// Read reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *envProvider) Read(ctx context.Context, req *cocorpc.ReadRequest) (*cocorpc.ReadResponse, error) {
	contract.Assert(req.GetType() == string(Environment))
	return nil, errors.New("Not yet implemented")
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *envProvider) Update(ctx context.Context, req *cocorpc.UpdateRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(Environment))
	return nil, errors.New("Not yet implemented")
}

// UpdateImpact checks what impacts a hypothetical update will have on the resource's properties.
func (p *envProvider) UpdateImpact(
	ctx context.Context, req *cocorpc.UpdateRequest) (*cocorpc.UpdateImpactResponse, error) {
	contract.Assert(req.GetType() == string(Environment))
	return nil, errors.New("Not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *envProvider) Delete(ctx context.Context, req *cocorpc.DeleteRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(Environment))

	id := req.GetId()
	fmt.Printf("Deleting Fission Environment '%v'\n", id)
	meta := &fission.Metadata{Name: id}
	if err := p.ctx.Fission.EnvironmentDelete(meta); err != nil {
		return nil, err
	}
	fmt.Printf("Fission Environment '%v' deleted\n", id)

	return &pbempty.Empty{}, nil
}

// unmarshalEnvironment decodes and validates an environment property bag.
func unmarshalEnvironment(v *pbstruct.Struct) (fission.Environment, resource.PropertyMap, mapper.DecodeError) {
	var env fission.Environment
	props := resource.UnmarshalProperties(v)
	err := mapper.MapIU(props.Mappable(), &env)
	return env, props, err
}
