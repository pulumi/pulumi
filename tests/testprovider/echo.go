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
	"strconv"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/protobuf/types/known/emptypb"
)

func init() {
	providerSchema.Resources["testprovider:index:Echo"] = pschema.ResourceSpec{
		ObjectTypeSpec: pschema.ObjectTypeSpec{
			Description: "A test resource that echoes its input.",
			Properties: map[string]pschema.PropertySpec{
				"echo": {
					TypeSpec: pschema.TypeSpec{
						Ref: "pulumi.json#/Any",
					},
					Description: "Input to echo.",
				},
			},
			Type: "object",
		},
		InputProperties: map[string]pschema.PropertySpec{
			"echo": {
				TypeSpec: pschema.TypeSpec{
					Ref: "pulumi.json#/Any",
				},
				Description: "An echoed input.",
			},
		},
	}
}

type echoResourceProvider struct {
	id int
}

func (p *echoResourceProvider) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return &rpc.CheckResponse{Inputs: req.News, Failures: nil}, nil
}

func (p *echoResourceProvider) Diff(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	olds, err := plugin.UnmarshalProperties(req.GetOlds(), plugin.MarshalOptions{KeepUnknowns: true, SkipNulls: true})
	if err != nil {
		return nil, err
	}

	news, err := plugin.UnmarshalProperties(req.GetNews(), plugin.MarshalOptions{KeepUnknowns: true, SkipNulls: true})
	if err != nil {
		return nil, err
	}

	d := olds.Diff(news)
	changes := rpc.DiffResponse_DIFF_NONE
	if d.Changed("echo") {
		changes = rpc.DiffResponse_DIFF_SOME
	}

	return &rpc.DiffResponse{
		Changes:  changes,
		Replaces: []string{"echo"},
	}, nil
}

func (p *echoResourceProvider) Create(ctx context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	inputs, err := plugin.UnmarshalProperties(req.GetProperties(), plugin.MarshalOptions{
		KeepUnknowns: true,
		SkipNulls:    true,
	})
	if err != nil {
		return nil, err
	}

	outputs := map[string]interface{}{
		"echo": inputs["echo"],
	}

	outputProperties, err := plugin.MarshalProperties(
		resource.NewPropertyMapFromMap(outputs),
		plugin.MarshalOptions{KeepUnknowns: true, SkipNulls: true},
	)
	if err != nil {
		return nil, err
	}

	p.id++
	return &rpc.CreateResponse{
		Id:         strconv.Itoa(p.id),
		Properties: outputProperties,
	}, nil
}

func (p *echoResourceProvider) Read(ctx context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	return &rpc.ReadResponse{
		Id:         req.Id,
		Properties: req.Properties,
	}, nil
}

func (p *echoResourceProvider) Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	panic("Update not implemented")
}

func (p *echoResourceProvider) Delete(ctx context.Context, req *rpc.DeleteRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
