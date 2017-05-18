// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"errors"
	"fmt"

	"github.com/fission/fission"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/mapper"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	idl "github.com/pulumi/lumi/lib/kubefission/rpc"
)

const EnvironmentToken = idl.EnvironmentToken

// NewEnvironmentProvider creates a provider that handles Fission environment operations.
func NewEnvironmentProvider(ctx *Context) lumirpc.ResourceProviderServer {
	ops := &envProvider{ctx}
	return idl.NewEnvironmentProvider(ops)
}

type envProvider struct {
	ctx *Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *envProvider) Check(ctx context.Context, obj *idl.Environment) ([]mapper.FieldError, error) {
	return nil, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *envProvider) Create(ctx context.Context, obj *idl.Environment) (resource.ID, error) {
	// Perform the operation by contacting the controller.
	fmt.Printf("Creating Fission environment '%v'\n", obj.Name)
	if meta, err := p.ctx.Fission.EnvironmentCreate(&fission.Environment{
		Metadata:             fission.Metadata{Name: obj.Name},
		RunContainerImageUrl: obj.RunContainerImageURL,
	}); err != nil {
		return "", err
	} else {
		fmt.Printf("Fission Environment '%v' created: version=%v\n", meta.Name, meta.Uid)
	}
	return resource.ID(obj.Name), nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *envProvider) Get(ctx context.Context, id resource.ID) (*idl.Environment, error) {
	return nil, errors.New("Not yet implemented")
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *envProvider) InspectChange(ctx context.Context, id resource.ID,
	old *idl.Environment, new *idl.Environment, diff *resource.ObjectDiff) ([]string, error) {
	return nil, errors.New("Not yet implemented")
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *envProvider) Update(ctx context.Context, id resource.ID,
	old *idl.Environment, new *idl.Environment, diff *resource.ObjectDiff) error {
	return errors.New("Not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *envProvider) Delete(ctx context.Context, id resource.ID) error {
	fmt.Printf("Deleting Fission Environment '%v'\n", id)
	if err := p.ctx.Fission.EnvironmentDelete(&fission.Metadata{Name: id.String()}); err != nil {
		return err
	}
	fmt.Printf("Fission Environment '%v' deleted\n", id)
	return nil
}
