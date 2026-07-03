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

package backend

import (
	"context"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
)

// DefaultHostFactory builds the production engine.HostFactory: the standard plugin host with
// language installation and the schema-loader, conversion-mapper, and package-resolver services.
// The engine supplies the diagnostic sinks and debug context so plugin logs surface in the UI as
// events. The package resolver resolves against reg, which the caller derives from the active
// backend so a logged-in user resolves against their authenticated registry.
func DefaultHostFactory(reg registry.Registry) engine.HostFactory {
	resolver := packageworkspace.NewResolverServer(reg)
	return func(
		ctx context.Context, d, statusD diag.Sink, debug plugin.DebugContext,
	) (plugin.Host, error) {
		return pkghost.New(ctx, d, statusD, debug,
			pkgWorkspace.EnsureLanguageInstalled,
			schema.NewLoaderServerFromContext, convert.NewMapperServerFromContext,
			resolver)
	}
}

// Assert DefaultHostFactory builds the engine's factory type.
var _ func(registry.Registry) engine.HostFactory = DefaultHostFactory
