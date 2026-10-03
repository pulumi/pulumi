// Copyright 2026, Pulumi Corporation.
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

package main

import (
	"context"
	"encoding/json"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/emptypb"
)

func main() {
	defer logging.Flush()
	if err := provider.Main("testlogging", func(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		return &testloggingProvider{}, nil
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}

type testloggingProvider struct {
	pulumirpc.UnimplementedResourceProviderServer
}

var schema = func() string {
	s := map[string]any{
		"name":    "testlogging",
		"version": "0.0.1",
		"resources": map[string]any{
			"testlogging:index:Resource": map[string]any{
				"inputProperties": map[string]any{
					"value": map[string]any{"type": "string"},
				},
				"requiredInputs": []string{"value"},
				"properties": map[string]any{
					"value": map[string]any{"type": "string"},
				},
			},
		},
	}
	b, _ := json.Marshal(s)
	return string(b)
}()

func (p *testloggingProvider) GetSchema(_ context.Context,
	_ *pulumirpc.GetSchemaRequest,
) (*pulumirpc.GetSchemaResponse, error) {
	return &pulumirpc.GetSchemaResponse{Schema: schema}, nil
}

func (p *testloggingProvider) CheckConfig(_ context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	return &pulumirpc.CheckResponse{Inputs: req.GetNews()}, nil
}

func (p *testloggingProvider) Configure(_ context.Context,
	_ *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{AcceptSecrets: true}, nil
}

func (p *testloggingProvider) Check(_ context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	return &pulumirpc.CheckResponse{Inputs: req.GetNews()}, nil
}

func (p *testloggingProvider) Create(_ context.Context,
	req *pulumirpc.CreateRequest,
) (*pulumirpc.CreateResponse, error) {
	props := req.GetProperties()
	logging.Infof("plugin-log-test-marker: creating resource with inputs %v", props)
	logging.Infof("plugin-log-inline-marker: inline property %v",
		resource.NewPropertyMapFromMap(map[string]any{"foo": "bar"}))
	logging.Infof("plugin-log-scalar-marker: scalar value %v",
		resource.NewProperty("secret-val"))
	return &pulumirpc.CreateResponse{
		Id:         "test-id-1",
		Properties: props,
	}, nil
}

func (p *testloggingProvider) Diff(_ context.Context, _ *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return &pulumirpc.DiffResponse{}, nil
}

func (p *testloggingProvider) Read(_ context.Context, req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
	return &pulumirpc.ReadResponse{Id: req.GetId(), Properties: req.GetProperties()}, nil
}

func (p *testloggingProvider) Delete(_ context.Context, _ *pulumirpc.DeleteRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *testloggingProvider) GetPluginInfo(_ context.Context, _ *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: "0.0.1"}, nil
}

func (p *testloggingProvider) Attach(_ context.Context, _ *pulumirpc.PluginAttach) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
