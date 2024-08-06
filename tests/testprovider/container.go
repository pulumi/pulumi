// Copyright 2016-2021, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//go:build !all
// +build !all

package main

import (
	"context"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/protobuf/types/known/emptypb"
)

func init() {
	providerSchema.Resources["testprovider:index:Container"] = pschema.ResourceSpec{
		ObjectTypeSpec: pschema.ObjectTypeSpec{
			Description: "A test resource that generates a random string of a given length and with an optional prefix.",
			Properties: map[string]pschema.PropertySpec{
				"child": {
					TypeSpec: pschema.TypeSpec{
						Ref: "#/types/testprovider:index:Random",
					},
					Description: "The child of the container",
				},
			},
			Type: "object",
		},
		InputProperties: map[string]pschema.PropertySpec{
			"child": {
				TypeSpec: pschema.TypeSpec{
					Ref: "#/types/testprovider:index:Random",
				},
				Description: "The child of the container",
			},
		},
	}
}

type containerResourceProvider struct{}

func (p *containerResourceProvider) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return &rpc.CheckResponse{Inputs: req.News, Failures: nil}, nil
}

func (p *containerResourceProvider) Diff(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	return &rpc.DiffResponse{
		Changes:  rpc.DiffResponse_DIFF_NONE,
		Replaces: []string{},
	}, nil
}

func (p *containerResourceProvider) Create(ctx context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	inputs, err := plugin.UnmarshalProperties(req.GetProperties(), plugin.MarshalOptions{
		KeepUnknowns:     true,
		SkipNulls:        true,
		KeepResources:    true,
		KeepOutputValues: true,
	})
	if err != nil {
		return nil, err
	}
	outputs := resource.PropertyMap{}
	outputs["child"] = inputs["child"]
	outputProperties, err := plugin.MarshalProperties(
		outputs,
		plugin.MarshalOptions{KeepUnknowns: true, KeepResources: true, KeepOutputValues: true},
	)
	if err != nil {
		return nil, err
	}
	return &rpc.CreateResponse{
		Id:         "test",
		Properties: outputProperties,
	}, nil
}

func (p *containerResourceProvider) Read(ctx context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	// Just return back the input state.
	return &rpc.ReadResponse{
		Id:         req.Id,
		Properties: req.Properties,
	}, nil
}

func (p *containerResourceProvider) Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	panic("Update not implemented")
}

func (p *containerResourceProvider) Delete(ctx context.Context, req *rpc.DeleteRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *containerResourceProvider) Invoke(ctx context.Context, req *rpc.InvokeRequest) (*rpc.InvokeResponse, error) {
	panic("Invoke not implemented")
}
