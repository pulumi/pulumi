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

const HTTPTrigger = tokens.Type("kube-fission:httptrigger:HTTPTrigger")

// NewHTTPTriggerProvider creates a provider that handles Fission httptrigger operations.
func NewHTTPTriggerProvider(ctx *Context) cocorpc.ResourceProviderServer {
	return &httProvider{ctx}
}

type httProvider struct {
	ctx *Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *httProvider) Check(ctx context.Context, req *cocorpc.CheckRequest) (*cocorpc.CheckResponse, error) {
	// Get in the properties, create and validate a new group, and return the failures (if any).
	contract.Assert(req.GetType() == string(HTTPTrigger))
	_, _, result := unmarshalHTTPTrigger(req.GetProperties())
	return resource.NewCheckResponse(result), nil
}

// Name names a given resource.  Sometimes this will be assigned by a developer, and so the provider
// simply fetches it from the property bag; other times, the provider will assign this based on its own algorithm.
// In any case, resources with the same name must be safe to use interchangeably with one another.
func (p *httProvider) Name(ctx context.Context, req *cocorpc.NameRequest) (*cocorpc.NameResponse, error) {
	contract.Failf("Kube-Fission top-level dispatcher should have handled this RPC")
	return nil, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *httProvider) Create(ctx context.Context, req *cocorpc.CreateRequest) (*cocorpc.CreateResponse, error) {
	contract.Assert(req.GetType() == string(HTTPTrigger))

	// Get in the properties given by the request, validating as we go; if any fail, reject the request.
	htt, _, decerr := unmarshalHTTPTrigger(req.GetProperties())
	if decerr != nil {
		return nil, decerr
	}

	// Perform the operation by contacting the controller.
	fmt.Printf("Creating Fission HTTP trigger '%v'\n", htt.Metadata.Name)
	meta, err := p.ctx.Fission.HTTPTriggerCreate(&htt)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Fission HTTP trigger '%v' created: version=%v\n", meta.Name, meta.Uid)
	return &cocorpc.CreateResponse{Id: meta.Name}, nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *httProvider) Get(ctx context.Context, req *cocorpc.GetRequest) (*cocorpc.GetResponse, error) {
	contract.Assert(req.GetType() == string(HTTPTrigger))
	return nil, errors.New("Not yet implemented")
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *httProvider) InspectChange(
	ctx context.Context, req *cocorpc.ChangeRequest) (*cocorpc.InspectChangeResponse, error) {
	contract.Assert(req.GetType() == string(HTTPTrigger))
	return nil, errors.New("Not yet implemented")
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *httProvider) Update(ctx context.Context, req *cocorpc.ChangeRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(HTTPTrigger))
	return nil, errors.New("Not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *httProvider) Delete(ctx context.Context, req *cocorpc.DeleteRequest) (*pbempty.Empty, error) {
	contract.Assert(req.GetType() == string(HTTPTrigger))

	id := req.GetId()
	fmt.Printf("Deleting Fission HTTP trigger '%v'\n", id)
	meta := &fission.Metadata{Name: id}
	if err := p.ctx.Fission.HTTPTriggerDelete(meta); err != nil {
		return nil, err
	}
	fmt.Printf("Fission HTTP trigger '%v' deleted\n", id)

	return &pbempty.Empty{}, nil
}

// httpTrigger is the shape of the resource on the wire when communicating with the Coconut host.
type httpTrigger struct {
	Name       string      `json:"name"`
	URLPattern string      `json:"urlPattern"`
	Method     string      `json:"method"`
	Function   resource.ID `json:"function"`
}

// unmarshalHTTPTrigger decodes and validates a HTTP trigger property bag.
func unmarshalHTTPTrigger(v *pbstruct.Struct) (fission.HTTPTrigger, resource.PropertyMap, mapper.DecodeError) {
	var htt httpTrigger
	props := resource.UnmarshalProperties(v)
	err := mapper.MapIU(props.Mappable(), &htt)
	return fission.HTTPTrigger{
		Metadata:   fission.Metadata{Name: htt.Name},
		UrlPattern: htt.URLPattern,
		Method:     htt.Method,
		Function:   fission.Metadata{Name: string(htt.Function)},
	}, props, err
}
