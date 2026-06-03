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
// +build !all

package main

import (
	"context"
	"encoding/json"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/emptypb"
)

func main() {
	defer logging.Flush()
	if err := provider.Main("testlogging", func(host *provider.HostClient) (rpc.ResourceProviderServer, error) {
		return &testloggingProvider{}, nil
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}

type testloggingProvider struct {
	rpc.UnimplementedResourceProviderServer
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

func (p *testloggingProvider) GetSchema(_ context.Context, _ *rpc.GetSchemaRequest) (*rpc.GetSchemaResponse, error) {
	return &rpc.GetSchemaResponse{Schema: schema}, nil
}

func (p *testloggingProvider) CheckConfig(_ context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return &rpc.CheckResponse{Inputs: req.GetNews()}, nil
}

func (p *testloggingProvider) Configure(_ context.Context, _ *rpc.ConfigureRequest) (*rpc.ConfigureResponse, error) {
	return &rpc.ConfigureResponse{AcceptSecrets: true}, nil
}

func (p *testloggingProvider) Check(_ context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return &rpc.CheckResponse{Inputs: req.GetNews()}, nil
}

func (p *testloggingProvider) Create(_ context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	props := req.GetProperties()
	logging.Infof("plugin-log-test-marker: creating resource with inputs %v",
		logging.NewPropertyValue("inputs", props))
	return &rpc.CreateResponse{
		Id:         "test-id-1",
		Properties: props,
	}, nil
}

func (p *testloggingProvider) Diff(_ context.Context, _ *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	return &rpc.DiffResponse{}, nil
}

func (p *testloggingProvider) Read(_ context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	return &rpc.ReadResponse{Id: req.GetId(), Properties: req.GetProperties()}, nil
}

func (p *testloggingProvider) Delete(_ context.Context, _ *rpc.DeleteRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *testloggingProvider) GetPluginInfo(_ context.Context, _ *emptypb.Empty) (*rpc.PluginInfo, error) {
	return &rpc.PluginInfo{Version: "0.0.1"}, nil
}

func (p *testloggingProvider) Attach(_ context.Context, _ *rpc.PluginAttach) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

