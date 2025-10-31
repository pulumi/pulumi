// Copyright 2016-2025, Pulumi Corporation.
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

package plugin

import (
	"context"
	"fmt"

	"github.com/blang/semver"
	pconvert "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/convert"
	"github.com/pulumi/pulumi/sdk/v3/pkg/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/sdk/v3/pkg/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// newInstallPluginFunc creates a function that installs plugins on demand
// Used by the RPC server's mapper to automatically install missing providers
func newInstallPluginFunc(pctx *plugin.Context) func(string) *semver.Version {
	log := func(sev diag.Severity, msg string) {
		pctx.Diag.Logf(sev, diag.RawMessage("", msg))
	}

	return func(pluginName string) *semver.Version {
		// If auto plugin installs are disabled just return nil
		if env.DisableAutomaticPluginAcquisition.Value() {
			return nil
		}

		pluginSpec := workspace.PluginSpec{
			Name:	pluginName,
			Kind:	apitype.ResourcePlugin,
		}
		version, err := pkgWorkspace.InstallPlugin(pctx.Base(), pluginSpec, log)
		if err != nil {
			log(diag.Warning, fmt.Sprintf("failed to install provider %s: %v", pluginName, err))
			return nil
		}
		return version
	}
}

// createPluginRPCServer creates a gRPC server that provides mapper and loader services
// to plugins. This allows plugins to query provider schemas without setting up
// the infrastructure themselves.
// Note: The mapping service is hardcoded to serve terraform mappings only.
func createPluginRPCServer(
	ctx context.Context,
	pctx *plugin.Context,
) (*plugin.GrpcServer, error) {
	installPlugin := newInstallPluginFunc(pctx)

	baseMapper, err := pconvert.NewBasePluginMapper(
		pconvert.DefaultWorkspace(),
		"terraform",
		pconvert.ProviderFactoryFromHost(ctx, pctx.Host),
		installPlugin,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("initializing provider mapper: %w", err)
	}

	// Wrap in a caching mapper for better performance
	mapper := pconvert.NewCachingMapper(baseMapper)

	loader := schema.NewPluginLoader(pctx.Host)

	mapperServer := pconvert.NewMapperServer(mapper)
	loaderServer := schema.NewLoaderServer(loader)

	grpcServer, err := plugin.NewServer(pctx,
		pconvert.MapperRegistration(mapperServer),
		schema.LoaderRegistration(loaderServer))
	if err != nil {
		return nil, fmt.Errorf("starting gRPC server: %w", err)
	}

	return grpcServer, nil
}
