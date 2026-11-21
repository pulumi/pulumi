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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
)

// NestedFailureProvider supplies the schema for the language providers in the nested Construct test.
// This provider should not implement any runtime code. It is just used for its schema for program binding.
type NestedFailureProvider struct {
	plugin.UnimplementedProvider
}

func (*NestedFailureProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{Version: &semver.Version{Major: 63}}, nil
}

func (*NestedFailureProvider) GetSchema(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
	spec := schema.PackageSpec{
		Name:    "nested-failure",
		Version: "63.0.0",
		Resources: map[string]schema.ResourceSpec{
			"nested-failure:index:Wrapper": {
				IsComponent: true,
			},
		},
	}
	bytes, err := json.Marshal(spec)
	return plugin.GetSchemaResponse{Schema: bytes}, err
}
