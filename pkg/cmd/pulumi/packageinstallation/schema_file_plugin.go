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

package packageinstallation

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func isSchemaPath(path string) bool {
	return slices.Contains(encoding.Exts, filepath.Ext(path))
}

func newProviderFromSchemaPath(path string, file io.Reader) (plugin.Provider, error) {
	var b bytes.Buffer
	_, err := b.ReadFrom(file)
	if err != nil {
		return nil, err
	}
	m, ext := encoding.Detect(path)
	if m == nil {
		return nil, fmt.Errorf("unknown schema extension %q", ext)
	}
	s := schemaProvider{
		schemaBytes: b.Bytes(),
	}
	if err := m.Unmarshal(s.schemaBytes, &s.schema); err != nil {
		return nil, err
	}
	return s, nil
}

var _ plugin.Provider = schemaProvider{}

type schemaProvider struct {
	plugin.NotForwardCompatibleProvider

	schema      schema.PartialPackageSpec
	schemaBytes []byte
}

func (s schemaProvider) Close() error { return nil }

func (s schemaProvider) Pkg() tokens.Package { return tokens.Package(s.schema.Name) }

func (s schemaProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	if s.schema.Version == "" {
		return plugin.PluginInfo{}, nil
	}
	version, err := semver.ParseTolerant(s.schema.Version)
	return plugin.PluginInfo{Version: &version}, err
}

func (s schemaProvider) GetSchema(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
	return plugin.GetSchemaResponse{Schema: s.schemaBytes}, nil
}

func (s schemaProvider) errSchemaBased(method string) error {
	return fmt.Errorf("method %q not supported for schema based providers", method)
}

func (s schemaProvider) Handshake(
	context.Context, plugin.ProviderHandshakeRequest,
) (*plugin.ProviderHandshakeResponse, error) {
	return nil, s.errSchemaBased("Handshake")
}

func (s schemaProvider) Parameterize(context.Context, plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
	return plugin.ParameterizeResponse{}, s.errSchemaBased("Parameterize")
}

func (s schemaProvider) CheckConfig(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{}, s.errSchemaBased("CheckConfig")
}

func (s schemaProvider) DiffConfig(context.Context, plugin.DiffConfigRequest) (plugin.DiffConfigResponse, error) {
	return plugin.DiffConfigResponse{}, s.errSchemaBased("DiffConfig")
}

func (s schemaProvider) Configure(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, s.errSchemaBased("Configure")
}

func (s schemaProvider) Check(context.Context, plugin.CheckRequest) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{}, s.errSchemaBased("Check")
}

func (s schemaProvider) Diff(context.Context, plugin.DiffRequest) (plugin.DiffResponse, error) {
	return plugin.DiffResponse{}, s.errSchemaBased("Diff")
}

func (s schemaProvider) Create(context.Context, plugin.CreateRequest) (plugin.CreateResponse, error) {
	return plugin.CreateResponse{}, s.errSchemaBased("Create")
}

func (s schemaProvider) Read(context.Context, plugin.ReadRequest) (plugin.ReadResponse, error) {
	return plugin.ReadResponse{}, s.errSchemaBased("Read")
}

func (s schemaProvider) Update(context.Context, plugin.UpdateRequest) (plugin.UpdateResponse, error) {
	return plugin.UpdateResponse{}, s.errSchemaBased("Update")
}

func (s schemaProvider) Delete(context.Context, plugin.DeleteRequest) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, s.errSchemaBased("Delete")
}

func (s schemaProvider) Construct(context.Context, plugin.ConstructRequest) (plugin.ConstructResponse, error) {
	return plugin.ConstructResponse{}, s.errSchemaBased("Construct")
}

func (s schemaProvider) Invoke(context.Context, plugin.InvokeRequest) (plugin.InvokeResponse, error) {
	return plugin.InvokeResponse{}, s.errSchemaBased("Invoke")
}

func (s schemaProvider) Call(context.Context, plugin.CallRequest) (plugin.CallResponse, error) {
	return plugin.CallResponse{}, s.errSchemaBased("Call")
}

func (s schemaProvider) SignalCancellation(context.Context) error {
	return nil
}

func (s schemaProvider) GetMapping(context.Context, plugin.GetMappingRequest) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, s.errSchemaBased("GetMapping")
}

func (s schemaProvider) GetMappings(context.Context, plugin.GetMappingsRequest) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, s.errSchemaBased("GetMappings")
}
