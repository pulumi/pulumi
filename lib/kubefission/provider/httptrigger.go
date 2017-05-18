// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

const HTTPTriggerToken = idl.HTTPTriggerToken

// NewHTTPTriggerProvider creates a provider that handles Fission httptrigger operations.
func NewHTTPTriggerProvider(ctx *Context) lumirpc.ResourceProviderServer {
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
		Function:   fission.Metadata{Name: string(obj.Function)},
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
