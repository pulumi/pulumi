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
	"sync"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// FlakyCreateProvider is a provider whose resource fails its first Create with a retryable
// (partial failure) error and succeeds on subsequent attempts. It is used to test error hooks,
// which the engine only invokes for retryable failures.
type FlakyCreateProvider struct {
	plugin.UnimplementedProvider

	mu       sync.Mutex
	attempts int
}

var _ plugin.Provider = (*FlakyCreateProvider)(nil)

func (p *FlakyCreateProvider) Close() error {
	return nil
}

func (p *FlakyCreateProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *FlakyCreateProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "flaky",
		Version: "46.0.0",
		Resources: map[string]schema.ResourceSpec{
			"flaky:index:FlakyCreate": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *FlakyCreateProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	// Expect just the version
	version, ok := req.News["version"]
	if !ok {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "missing version")}, nil
	}
	if !version.IsString() {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "version is not a string")}, nil
	}
	if version.StringValue() != "46.0.0" {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "version is not 46.0.0")}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *FlakyCreateProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "flaky:index:FlakyCreate" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	if len(req.News) != 0 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *FlakyCreateProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.attempts++
	if p.attempts == 1 {
		// The engine only runs error hooks for retryable failures, i.e. a partial failure
		// with init errors.
		return plugin.CreateResponse{
			ID:         "id",
			Properties: resource.PropertyMap{},
			Status:     resource.StatusPartialFailure,
		}, &plugin.InitError{Reasons: []string{"first create attempt fails"}}
	}

	return plugin.CreateResponse{
		ID:         "id",
		Properties: resource.PropertyMap{},
		Status:     resource.StatusOK,
	}, nil
}

func (p *FlakyCreateProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("46.0.0")
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *FlakyCreateProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *FlakyCreateProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *FlakyCreateProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *FlakyCreateProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *FlakyCreateProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *FlakyCreateProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
