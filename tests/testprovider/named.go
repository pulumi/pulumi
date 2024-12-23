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
	"strings"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/protobuf/types/known/emptypb"
)

func init() {
	providerSchema.Resources["testprovider:index:Named"] = pschema.ResourceSpec{
		ObjectTypeSpec: pschema.ObjectTypeSpec{
			Description: "A test resource that has an auto-generated name.",
			Properties: map[string]pschema.PropertySpec{
				"name": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
					Description: "The name of the resource.",
				},
			},
			Type: "object",
		},
		InputProperties: map[string]pschema.PropertySpec{
			"name": {
				TypeSpec: pschema.TypeSpec{
					Type: "string",
				},
				Description: "Optional explicit name.",
			},
		},
	}
}

type namedProvider struct {
	id int
}

func (p *namedProvider) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	news, err := plugin.UnmarshalProperties(req.GetNews(), plugin.MarshalOptions{KeepUnknowns: true, SkipNulls: true})
	if err != nil {
		return nil, err
	}
	_, ok := news["name"]
	if !ok {
		generatedName := "default-name"
		if req.Autonaming != nil {
			switch req.Autonaming.Mode {
			case rpc.CheckRequest_AutonamingOptions_DISABLE:
				generatedName = ""
			case rpc.CheckRequest_AutonamingOptions_ENFORCE:
				generatedName = req.Autonaming.GetProposedName()
			case rpc.CheckRequest_AutonamingOptions_PROPOSE:
				generatedName = strings.ToLower(req.Autonaming.GetProposedName())
			}
		}
		if generatedName != "" {
			news["name"] = resource.NewStringProperty(generatedName)
		}
	}

	inputs, err := plugin.MarshalProperties(
		news,
		plugin.MarshalOptions{KeepUnknowns: true, SkipNulls: true},
	)
	if err != nil {
		return nil, err
	}

	return &rpc.CheckResponse{Inputs: inputs, Failures: nil}, nil
}

func (p *namedProvider) Diff(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
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
	var replaces []string
	if d != nil && d.Changed("echo") {
		changes = rpc.DiffResponse_DIFF_SOME
		replaces = append(replaces, "echo")
	}

	return &rpc.DiffResponse{
		Changes:  changes,
		Replaces: replaces,
	}, nil
}

func (p *namedProvider) Create(ctx context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	inputs, err := plugin.UnmarshalProperties(req.GetProperties(), plugin.MarshalOptions{
		KeepUnknowns: true,
		SkipNulls:    true,
	})
	if err != nil {
		return nil, err
	}

	outputProperties, err := plugin.MarshalProperties(
		inputs,
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

func (p *namedProvider) Read(ctx context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	return &rpc.ReadResponse{
		Id:         req.Id,
		Properties: req.Properties,
	}, nil
}

func (p *namedProvider) Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	panic("Update not implemented")
}

func (p *namedProvider) Delete(ctx context.Context, req *rpc.DeleteRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *namedProvider) Invoke(ctx context.Context, req *rpc.InvokeRequest) (*rpc.InvokeResponse, error) {
	panic("Invoke not implemented")
}

func (p *namedProvider) Call(ctx context.Context, req *rpc.CallRequest) (*rpc.CallResponse, error) {
	panic("Call not implemented")
}
