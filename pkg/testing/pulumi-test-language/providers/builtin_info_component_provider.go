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

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type BuiltinInfoComponentProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*BuiltinInfoComponentProvider)(nil)

func (p *BuiltinInfoComponentProvider) Close() error {
	return nil
}

func (p *BuiltinInfoComponentProvider) Pkg() tokens.Package {
	return "builtin-info-component"
}

func (p *BuiltinInfoComponentProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: &semver.Version{Major: 37},
	}, nil
}

func (p *BuiltinInfoComponentProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "builtin-info-component",
		Version: "37.0.0",
		Resources: map[string]schema.ResourceSpec{
			"builtin-info-component:index:BuiltinInfo": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"organization": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
						"project": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
						"stack": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

// N.B This provider should not implement any runtime code. It is just used for its schema for program binding.
