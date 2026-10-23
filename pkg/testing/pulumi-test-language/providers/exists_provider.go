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

package providers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// ExistsProvider is a provider whose Read reports a resource as missing when the state it is given has a false
// "value", so tests can check that state is passed through to Read.
type ExistsProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ExistsProvider)(nil)

func (p *ExistsProvider) Close() error {
	return nil
}

func (p *ExistsProvider) version() semver.Version {
	return semver.Version{Major: 60}
}

func (p *ExistsProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ExistsProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	resourceProperties := map[string]schema.PropertySpec{
		"value": {
			TypeSpec: schema.TypeSpec{
				Type: "boolean",
			},
		},
	}
	resourceRequired := []string{"value"}

	pkg := schema.PackageSpec{
		Name:    "exists",
		Version: p.version().String(),
		Resources: map[string]schema.ResourceSpec{
			"exists:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: resourceProperties,
					Required:   resourceRequired,
				},
				InputProperties: resourceProperties,
				RequiredInputs:  resourceRequired,
				StateInputs: &schema.ObjectTypeSpec{
					Type:       "object",
					Properties: resourceProperties,
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ExistsProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	version, ok := req.News.GetOk("version")
	if !ok {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "missing version"),
		}, nil
	}
	if !version.IsString() || version.AsString() != p.version().String() {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", fmt.Sprintf("version is not %s", p.version())),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ExistsProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "exists:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	value, ok := req.NewInputs.GetOk("value")
	if !ok {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("value", "missing value"),
		}, nil
	}
	if !value.IsBool() && !value.IsComputed() {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("value", "value is not a boolean"),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.NewInputs}, nil
}

func (p *ExistsProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "exists:index:Resource" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: req.Properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *ExistsProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: new(p.version()),
	}, nil
}

func (p *ExistsProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *ExistsProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ExistsProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *ExistsProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ExistsProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ExistsProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *ExistsProvider) Read(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	if req.URN.Type() != "exists:index:Resource" {
		return plugin.ReadResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	if value, ok := req.State.GetOk("value"); ok && value.IsBool() && !value.AsBool() {
		return plugin.ReadResponse{Status: resource.StatusOK}, nil
	}

	outputs := property.NewMap(map[string]property.Value{
		"value": property.New(true),
	})
	return plugin.ReadResponse{
		ReadResult: plugin.ReadResult{
			ID:      req.ID,
			Inputs:  &outputs,
			Outputs: &outputs,
		},
		Status: resource.StatusOK,
	}, nil
}
