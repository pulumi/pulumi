// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"errors"
	"fmt"

	"github.com/fission/fission"
	"github.com/pulumi/coconut/pkg/resource"
	"github.com/pulumi/coconut/pkg/util/mapper"
	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
	"golang.org/x/net/context"

	idl "github.com/pulumi/coconut/lib/kubefission/rpc"
)

const HTTPTriggerToken = idl.HTTPTriggerToken

// NewHTTPTriggerProvider creates a provider that handles Fission httptrigger operations.
func NewHTTPTriggerProvider(ctx *Context) cocorpc.ResourceProviderServer {
	ops := &httProvider{ctx}
	return idl.NewHTTPTriggerProvider(ops)
}

type httProvider struct {
	ctx *Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *httProvider) Check(ctx context.Context, obj *idl.HTTPTrigger) ([]mapper.FieldError, error) {
	return nil, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *httProvider) Create(ctx context.Context, obj *idl.HTTPTrigger) (resource.ID, error) {
	// Perform the operation by contacting the controller.
	fmt.Printf("Creating Fission HTTP trigger '%v'\n", obj.Name)
	if meta, err := p.ctx.Fission.HTTPTriggerCreate(&fission.HTTPTrigger{
		Metadata:   fission.Metadata{Name: obj.Name},
		UrlPattern: obj.URLPattern,
		Method:     obj.Method,
		Function:   fission.Metadata{Name: string(*obj.Function)},
	}); err != nil {
		return "", err
	} else {
		fmt.Printf("Fission HTTP trigger '%v' created: version=%v\n", meta.Name, meta.Uid)
	}
	return resource.ID(obj.Name), nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *httProvider) Get(ctx context.Context, id resource.ID) (*idl.HTTPTrigger, error) {
	return nil, errors.New("Not yet implemented")
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *httProvider) InspectChange(ctx context.Context, id resource.ID,
	old *idl.HTTPTrigger, new *idl.HTTPTrigger, diff *resource.ObjectDiff) ([]string, error) {
	return nil, errors.New("Not yet implemented")
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *httProvider) Update(ctx context.Context, id resource.ID,
	old *idl.HTTPTrigger, new *idl.HTTPTrigger, diff *resource.ObjectDiff) error {
	return errors.New("Not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *httProvider) Delete(ctx context.Context, id resource.ID) error {
	fmt.Printf("Deleting Fission HTTP trigger '%v'\n", id)
	if err := p.ctx.Fission.HTTPTriggerDelete(&fission.Metadata{Name: string(id)}); err != nil {
		return err
	}
	fmt.Printf("Fission HTTP trigger '%v' deleted\n", id)
	return nil
}
