// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"bytes"
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

const Function = tokens.Type("kube-fission:function:Function")

// NewFunctionProvider creates a provider that handles Fission function operations.
func NewFunctionProvider(ctx *Context) cocorpc.ResourceProviderServer {
	return &funcProvider{ctx}
}

type funcProvider struct {
	ctx *Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *funcProvider) Check(ctx context.Context, req *cocorpc.CheckRequest) (*cocorpc.CheckResponse, error) {
	// Get in the properties, create and validate a new group, and return the failures (if any).
	contract.Assert(req.GetType() == string(Function))
	_, _, result := unmarshalFunction(req.GetProperties())
	return resource.NewCheckResponse(result), nil
}

// Name names a given resource.  Sometimes this will be assigned by a developer, and so the provider
// simply fetches it from the property bag; other times, the provider will assign this based on its own algorithm.
// In any case, resources with the same name must be safe to use interchangeably with one another.
func (p *funcProvider) Name(ctx context.Context, req *cocorpc.NameRequest) (*cocorpc.NameResponse, error) {
	contract.Failf("Kube-Fission top-level dispatcher should have handled this RPC")
	return nil, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *funcProvider) Create(ctx context.Context, req *cocorpc.CreateRequest) (*cocorpc.CreateResponse, error) {
	contract.Assert(req.GetType() == string(Function))

	// Get in the properties given by the request, validating as we go; if any fail, reject the request.
	fun, _, decerr := unmarshalFunction(req.GetProperties())
	if decerr != nil {
		return nil, decerr
	}

	// Generate the code string.
	crd, err := fun.Code.Read()
	if err != nil {
		return nil, err
	}
	var code bytes.Buffer
	code.ReadFrom(crd)

	// Now manufacture a real fission function data type that will get marshaled to the controller.
	fissfun := &fission.Function{
		Metadata: fission.Metadata{
			Name: fun.Name,
		},
		Environment: fission.Metadata{
			Name: string(fun.Environment),
		},
		Code: code.String(),
	}

	// Perform the operation by contacting the controller.
	fmt.Printf("Creating Fission function '%v'\n", fun.Name)
	meta, err := p.ctx.Fission.FunctionCreate(fissfun)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Fission Function '%v' created: version=%v\n", meta.Name, meta.Uid)
	return &cocorpc.CreateResponse{Id: meta.Name}, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *funcProvider) Get(ctx context.Context, req *cocorpc.GetRequest) (*cocorpc.GetResponse, error) {
	contract.Assert(req.GetType() == string(Function))
	return nil, errors.New("Not yet implemented")
}

// PreviewUpdate checks what impacts a hypothetical update will have on the resource's properties.
func (p *funcProvider) PreviewUpdate(
	ctx context.Context, req *cocorpc.UpdateRequest) (*cocorpc.PreviewUpdateResponse, error) {
	contract.Assert(req.GetType() == string(Function))
	return nil, errors.New("Not yet implemented")
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *funcProvider) Update(ctx context.Context, req *cocorpc.UpdateRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(Function))
	return nil, errors.New("Not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *funcProvider) Delete(ctx context.Context, req *cocorpc.DeleteRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(Function))

	id := req.GetId()
	fmt.Printf("Deleting Fission Function '%v'\n", id)
	meta := &fission.Metadata{Name: id}
	if err := p.ctx.Fission.FunctionDelete(meta); err != nil {
		return nil, err
	}
	fmt.Printf("Fission Function '%v' deleted\n", id)

	return &pbempty.Empty{}, nil
}

// function is a lot like Fission's internal Function type, except that it uses Coconut inter-resource references
// instead of the metadata payload, and assets for code.  It is used to marshal gRPC property bags in and out.
// TODO: eventually we want a way to reference specific versions (using the URI).
type function struct {
	Name        string         `json:"name"`
	Environment resource.ID    `json:"environment"`
	Code        resource.Asset `json:"code"`
}

// unmarshalFunction decodes and validates an function property bag.
func unmarshalFunction(v *pbstruct.Struct) (function, resource.PropertyMap, mapper.DecodeError) {
	var fun function
	props := resource.UnmarshalProperties(v)
	err := mapper.MapIU(props.Mappable(), &fun)
	return fun, props, err
}
