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
	"errors"
	"fmt"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// ByteSourceProvider produces strings containing arbitrary (non-UTF8) bytes. Its resource decodes the
// `base64` input into the `bytes` output, allowing programs to obtain raw byte strings that can't be
// written as program literals.
type ByteSourceProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ByteSourceProvider)(nil)

const byteSourceVersion = "48.0.0"

func (p *ByteSourceProvider) Close() error {
	return nil
}

// Handshake is implemented so the wrapping provider server can negotiate the byte string
// capability with the engine; the capability is only negotiated at handshake time.
func (p *ByteSourceProvider) Handshake(
	_ context.Context, req plugin.ProviderHandshakeRequest,
) (*plugin.ProviderHandshakeResponse, error) {
	if !req.AcceptsByteString {
		return nil, errors.New("bytesource requires an engine that accepts byte strings")
	}
	return &plugin.ProviderHandshakeResponse{}, nil
}

func (p *ByteSourceProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ByteSourceProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "bytesource",
		Version: byteSourceVersion,
		Resources: map[string]schema.ResourceSpec{
			"bytesource:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"base64": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
						"bytes": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"base64", "bytes"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"base64": {
						TypeSpec: schema.TypeSpec{Type: "string"},
					},
				},
				RequiredInputs: []string{"base64"},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ByteSourceProvider) CheckConfig(
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
	if version.StringValue() != byteSourceVersion {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not "+byteSourceVersion),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ByteSourceProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "bytesource:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	value, ok := req.News["base64"]
	if !ok {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("base64", "missing base64"),
		}, nil
	}
	if !value.IsString() && !value.IsComputed() {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("base64", "base64 is not a string"),
		}, nil
	}
	if value.IsString() {
		if _, err := base64.StdEncoding.DecodeString(value.StringValue()); err != nil {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("base64", fmt.Sprintf("base64 is not valid base64: %v", err)),
			}, nil
		}
	}
	if len(req.News) != 1 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ByteSourceProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "bytesource:index:Resource" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	encoded := req.Properties["base64"].StringValue()
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("base64 is not valid base64: %w", err)
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	return plugin.CreateResponse{
		ID: resource.ID(id),
		Properties: resource.PropertyMap{
			"base64": resource.NewProperty(encoded),
			"bytes":  resource.NewProperty(string(decoded)),
		},
		Status: resource.StatusOK,
	}, nil
}

func (p *ByteSourceProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	version := semver.MustParse(byteSourceVersion)
	return plugin.PluginInfo{
		Version: &version,
	}, nil
}

func (p *ByteSourceProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *ByteSourceProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ByteSourceProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *ByteSourceProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ByteSourceProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ByteSourceProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
