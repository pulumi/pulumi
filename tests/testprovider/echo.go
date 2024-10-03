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
	"os"
	"strconv"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
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
		Methods: map[string]string{
			"doEchoMethod": "testprovider:index:Echo/doEchoMethod",
		},
	}
	providerSchema.Functions["testprovider:index:doEcho"] = pschema.FunctionSpec{
		Description: "A test invoke that echoes its input.",
		Inputs: &pschema.ObjectTypeSpec{
			Properties: map[string]pschema.PropertySpec{
				"echo": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
		Outputs: &pschema.ObjectTypeSpec{
			Properties: map[string]pschema.PropertySpec{
				"echo": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
	}
	if os.Getenv("PULUMI_TEST_MULTI_ARGUMENT_INPUTS") != "" {
		// Conditionally add this if an env flag is set, since it does not work with all langs
		providerSchema.Functions["testprovider:index:doMultiEcho"] = pschema.FunctionSpec{
			Description: "A test invoke that echoes its input, using multiple inputs.",
			MultiArgumentInputs: []string{
				"echoA",
				"echoB",
			},
			Inputs: &pschema.ObjectTypeSpec{
				Properties: map[string]pschema.PropertySpec{
					"echoA": {
						TypeSpec: pschema.TypeSpec{
							Type: "string",
						},
					},
					"echoB": {
						TypeSpec: pschema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
			Outputs: &pschema.ObjectTypeSpec{
				Properties: map[string]pschema.PropertySpec{
					"echoA": {
						TypeSpec: pschema.TypeSpec{
							Type: "string",
						},
					},
					"echoB": {
						TypeSpec: pschema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
		}
	}
	providerSchema.Functions["testprovider:index:Echo/doEchoMethod"] = pschema.FunctionSpec{
		Description: "A test call that echoes its input.",
		Inputs: &pschema.ObjectTypeSpec{
			Properties: map[string]pschema.PropertySpec{
				"__self__": {
					TypeSpec: pschema.TypeSpec{
						Ref: "#/types/testprovider:index:Echo",
					},
				},
				"echo": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
		Outputs: &pschema.ObjectTypeSpec{
			Properties: map[string]pschema.PropertySpec{
				"echo": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
	}
}

type echoProvider struct {
	id int
}

func (p *echoProvider) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return &rpc.CheckResponse{Inputs: req.News, Failures: nil}, nil
}

func (p *echoProvider) Diff(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
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

func (p *echoProvider) Create(ctx context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
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

func (p *echoProvider) Read(ctx context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	return &rpc.ReadResponse{
		Id:         req.Id,
		Properties: req.Properties,
	}, nil
}

func (p *echoProvider) Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	panic("Update not implemented")
}

func (p *echoProvider) Delete(ctx context.Context, req *rpc.DeleteRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *echoProvider) Invoke(ctx context.Context, req *rpc.InvokeRequest) (*rpc.InvokeResponse, error) {
	return &rpc.InvokeResponse{Return: req.Args}, nil
}

func (p *echoProvider) Call(ctx context.Context, req *rpc.CallRequest) (*rpc.CallResponse, error) {
	return &rpc.CallResponse{Return: req.Args}, nil
}
