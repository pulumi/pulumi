// Copyright 2025, Pulumi Corporation.
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

type ConformanceComponentProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ConformanceComponentProvider)(nil)

func (p *ConformanceComponentProvider) Close() error {
	return nil
}

func (p *ConformanceComponentProvider) Pkg() tokens.Package {
	return "conformance-component"
}

func (p *ConformanceComponentProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: &semver.Version{Major: 22},
	}, nil
}

func (p *ConformanceComponentProvider) GetSchema(
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
		Name:    "conformance-component",
		Version: "22.0.0",
		Resources: map[string]schema.ResourceSpec{
			"conformance-component:index:Simple": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: resourceProperties,
					Required:   resourceRequired,
				},
				InputProperties: resourceProperties,
				RequiredInputs:  resourceRequired,
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}
