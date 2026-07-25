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
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// ByteSinkProvider consumes strings containing arbitrary (non-UTF8) bytes. Its resource verifies at
// Create time that the `bytes` input is byte-for-byte equal to the base64-decoded `expectBase64` input,
// proving the raw bytes arrived at the provider without corruption, then echoes the inputs as outputs.
type ByteSinkProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ByteSinkProvider)(nil)

const byteSinkVersion = "47.0.0"

func (p *ByteSinkProvider) Close() error {
	return nil
}

// Handshake is implemented so the wrapping provider server can negotiate the byte string
// capability with the engine; the capability is only negotiated at handshake time.
func (p *ByteSinkProvider) Handshake(
	context.Context, plugin.ProviderHandshakeRequest,
) (*plugin.ProviderHandshakeResponse, error) {
	return &plugin.ProviderHandshakeResponse{}, nil
}

func (p *ByteSinkProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ByteSinkProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	properties := map[string]schema.PropertySpec{
		"bytes": {
			TypeSpec: schema.TypeSpec{Type: "string"},
		},
		"expectBase64": {
			TypeSpec: schema.TypeSpec{Type: "string"},
		},
	}
	required := []string{"bytes", "expectBase64"}

	pkg := schema.PackageSpec{
		Name:    "bytesink",
		Version: byteSinkVersion,
		Resources: map[string]schema.ResourceSpec{
			"bytesink:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: properties,
					Required:   required,
				},
				InputProperties: properties,
				RequiredInputs:  required,
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ByteSinkProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	// Expect just the version
	version, ok := req.News["version"]
	if !ok {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "missing version"),
		}, nil
	}
	if !version.IsString() {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not a string"),
		}, nil
	}
	if version.StringValue() != byteSinkVersion {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not "+byteSinkVersion),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ByteSinkProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "bytesink:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	for _, key := range []resource.PropertyKey{"bytes", "expectBase64"} {
		value, ok := req.News[key]
		if !ok {
			return plugin.CheckResponse{
				Failures: makeCheckFailure(key, fmt.Sprintf("missing %s", key)),
			}, nil
		}
		if !value.IsString() && !value.IsComputed() {
			return plugin.CheckResponse{
				Failures: makeCheckFailure(key, fmt.Sprintf("%s is not a string", key)),
			}, nil
		}
	}
	if len(req.News) != 2 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ByteSinkProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "bytesink:index:Resource" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	expected, err := base64.StdEncoding.DecodeString(req.Properties["expectBase64"].StringValue())
	if err != nil {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("expectBase64 is not valid base64: %w", err)
	}
	actual := req.Properties["bytes"].StringValue()
	if actual != string(expected) {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("bytes does not match expectBase64: got %x, want %x", actual, expected)
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

func (p *ByteSinkProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	version := semver.MustParse(byteSinkVersion)
	return plugin.PluginInfo{
		Version: &version,
	}, nil
}

func (p *ByteSinkProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *ByteSinkProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ByteSinkProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *ByteSinkProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ByteSinkProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ByteSinkProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
