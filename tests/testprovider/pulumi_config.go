// Copyright 2024, Pulumi Corporation.
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
	"encoding/json"
	"os"
	"strconv"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/protobuf/types/known/emptypb"
)

func init() {
	providerSchema.Resources["testprovider:index:PulumiConfig"] = pschema.ResourceSpec{
		ObjectTypeSpec: pschema.ObjectTypeSpec{
			Description: "A test resource that references the PULUMI_CONFIG environment variable.",
			Properties: map[string]pschema.PropertySpec{
				"value": {
					TypeSpec: pschema.TypeSpec{
						Type: "string",
					},
					Description: "The value of PULUMI_CONFIG['value'], or an empty string if not set.",
				},
			},
			Type: "object",
		},
		InputProperties: map[string]pschema.PropertySpec{},
	}
}

// A provider for testing how configuration is passed to providers through the
// `PULUMI_CONFIG` environment variable.
type pulumiConfigProvider struct {
	id int
}

func (p *pulumiConfigProvider) Check(_ context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return &rpc.CheckResponse{Inputs: req.News, Failures: nil}, nil
}

func (p *pulumiConfigProvider) Diff(context.Context, *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	return &rpc.DiffResponse{}, nil
}

func (p *pulumiConfigProvider) Create(context.Context, *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	value := ""

	pulumiConfigStr := os.Getenv("PULUMI_CONFIG")
	if pulumiConfigStr != "" {
		pulumiConfig := map[string]interface{}{}
		err := json.Unmarshal([]byte(pulumiConfigStr), &pulumiConfig)
		if err == nil {
			if v, ok := pulumiConfig["value"]; ok {
				if s, ok := v.(string); ok {
					value = s
				}
			}
		}
	}

	outputs := resource.NewPropertyMapFromMap(map[string]interface{}{
		"value": value,
	})

	outputProperties, err := plugin.MarshalProperties(
		outputs,
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

func (p *pulumiConfigProvider) Read(_ context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	return &rpc.ReadResponse{
		Id:         req.Id,
		Properties: req.Properties,
	}, nil
}

func (p *pulumiConfigProvider) Update(context.Context, *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	panic("Update not implemented")
}

func (p *pulumiConfigProvider) Delete(context.Context, *rpc.DeleteRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *pulumiConfigProvider) Invoke(context.Context, *rpc.InvokeRequest) (*rpc.InvokeResponse, error) {
	panic("Invoke not implemented")
}

func (p *pulumiConfigProvider) Call(context.Context, *rpc.CallRequest) (*rpc.CallResponse, error) {
	panic("Call not implemented")
}
