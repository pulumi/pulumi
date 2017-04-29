// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/fission/fission"
	"github.com/pulumi/coconut/pkg/resource"
	"github.com/pulumi/coconut/pkg/util/mapper"
	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
	"golang.org/x/net/context"

	idl "github.com/pulumi/coconut/lib/kubefission/rpc"
)

const FunctionToken = idl.FunctionToken

// NewFunctionProvider creates a provider that handles Fission function operations.
func NewFunctionProvider(ctx *Context) cocorpc.ResourceProviderServer {
	ops := &funcProvider{ctx}
	return idl.NewFunctionProvider(ops)
}

type funcProvider struct {
	ctx *Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *funcProvider) Check(ctx context.Context, obj *idl.Function) ([]mapper.FieldError, error) {
	return nil, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *funcProvider) Create(ctx context.Context, obj *idl.Function) (resource.ID, error) {
	// Generate the code string.
	var code bytes.Buffer
	crd, err := obj.Code.Read()
	if err != nil {
		return "", err
	} else {
		code.ReadFrom(crd)
	}

	// Now manufacture a real fission function data type that will get marshaled to the controller.
	fissfun := &fission.Function{
		Metadata: fission.Metadata{
			Name: obj.Name,
		},
		Environment: fission.Metadata{
			Name: string(obj.Environment),
		},
		Code: code.String(),
	}

	// Perform the operation by contacting the controller.
	fmt.Printf("Creating Fission function '%v'\n", obj.Name)
	if meta, err := p.ctx.Fission.FunctionCreate(fissfun); err != nil {
		return "", err
	} else {
		fmt.Printf("Fission Function '%v' created: version=%v\n", meta.Name, meta.Uid)
	}
	return resource.ID(obj.Name), nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *funcProvider) Get(ctx context.Context, id resource.ID) (*idl.Function, error) {
	return nil, errors.New("Not yet implemented")
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *funcProvider) InspectChange(ctx context.Context, id resource.ID,
	old *idl.Function, new *idl.Function, diff *resource.ObjectDiff) ([]string, error) {
	return nil, errors.New("Not yet implemented")
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *funcProvider) Update(ctx context.Context, id resource.ID,
	old *idl.Function, new *idl.Function, diff *resource.ObjectDiff) error {
	return errors.New("Not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *funcProvider) Delete(ctx context.Context, id resource.ID) error {
	fmt.Printf("Deleting Fission Function '%v'\n", id)
	if err := p.ctx.Fission.FunctionDelete(&fission.Metadata{Name: id.String()}); err != nil {
		return err
	}
	fmt.Printf("Fission Function '%v' deleted\n", id)
	return nil
}
