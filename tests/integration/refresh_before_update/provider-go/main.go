// Copyright 2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type Provider struct {
	plugin.UnimplementedProvider

	refreshBeforeUpdate bool
}

func (p *Provider) Configure(ctx context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *Provider) Delete(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{Status: resource.StatusOK}, nil
}

func (p *Provider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("0.0.1")
	return workspace.PluginInfo{
		Name:    "provider-go",
		Version: &ver,
	}, nil
}

func (p *Provider) Handshake(
	_ context.Context,
	req plugin.ProviderHandshakeRequest,
) (*plugin.ProviderHandshakeResponse, error) {
	p.refreshBeforeUpdate = req.SupportsRefreshBeforeUpdate

	return &plugin.ProviderHandshakeResponse{
		AcceptSecrets:   true,
		AcceptResources: true,
		AcceptOutputs:   true,
	}, nil
}

func (p *Provider) GetSchema(_ context.Context, req plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
	s := schema.PackageSpec{
		Name:    "provider-go",
		Version: "0.0.1",
		Resources: map[string]schema.ResourceSpec{
			"provider-go:index:MyResource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"result": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
				},
				InputProperties: map[string]schema.PropertySpec{
					"input": {
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
		},
	}

	schemaBytes, err := json.Marshal(s)
	if err != nil {
		return plugin.GetSchemaResponse{}, err
	}

	return plugin.GetSchemaResponse{
		Schema: schemaBytes,
	}, nil
}

func (p *Provider) Check(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{
		Properties: req.News,
	}, nil
}

func (p *Provider) Create(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
	props := req.Properties.Copy()
	props["result"] = props["input"]
	return plugin.CreateResponse{
		Properties:          props,
		ID:                  "new-id",
		RefreshBeforeUpdate: p.refreshBeforeUpdate,
	}, nil
}

func (p *Provider) Diff(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
	if req.NewInputs.DeepEquals(req.OldInputs) {
		return plugin.DiffResponse{Changes: plugin.DiffNone}, nil
	}
	return plugin.DiffResponse{Changes: plugin.DiffSome}, nil
}

func (p *Provider) Update(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
	if req.OldInputs["input"].StringValue() != "<FRESH>" {
		return plugin.UpdateResponse{}, errors.New("Expected input=<FRESH> in the state")
	}
	props := req.NewInputs.Copy()
	props["result"] = props["input"]
	return plugin.UpdateResponse{
		Properties:          props,
		RefreshBeforeUpdate: p.refreshBeforeUpdate,
	}, nil
}

func (p *Provider) Read(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	inputs := req.Inputs.Copy()
	inputs["input"] = resource.NewStringProperty("<FRESH>")
	props := req.State.Copy()
	props["result"] = resource.NewStringProperty("<FRESH>")
	return plugin.ReadResponse{
		Status: resource.StatusOK,
		ReadResult: plugin.ReadResult{
			ID:                  "new-id",
			Inputs:              inputs,
			Outputs:             props,
			RefreshBeforeUpdate: p.refreshBeforeUpdate,
		},
	}, nil
}

func serve(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
	return plugin.NewProviderServer(&Provider{}), nil
}

func main() {
	if err := provider.Main("provider-go", serve); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
