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

package plugins

import (
	"context"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	installer "github.com/pulumi/pulumi/pkg/v3/plugins/install"
	"github.com/pulumi/pulumi/pkg/v3/plugins/load"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// GetSchemaFromSchemaSource gets a schema from a schema source, downloading and
// installing packages as necessary to drive the schema acquisition.
func GetSchemaFromSchemaSource(
	ctx context.Context,
	source string, parameters plugin.ParameterizeParameters,
	host plugin.Host, registry registry.Registry,
	projectRoot string,
	sink diag.Sink,
) (*schema.PackageSpec, workspace.PackageSpec, error) {
	pluginSpec, err := workspace.NewPluginSpec(ctx, source, apitype.ResourcePlugin, nil, "", nil)
	if err != nil {
		return nil, workspace.PackageSpec{}, err
	}
	_, err = installer.New(host, registry).EnsureSpec(ctx, pluginSpec)
	if err != nil {
		return nil, workspace.PackageSpec{}, err
	}

	return load.SchemaFromSchemaSource(ctx, source, parameters, registry, projectRoot, host, sink)
}
