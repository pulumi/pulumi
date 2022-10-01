// Copyright 2016-2018, Pulumi Corporation.
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

package engine

import (
	"github.com/opentracing/opentracing-go"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

type ProvidersOptions struct {
	Diag          diag.Sink               // the sink to use for diag'ing.
	StatusDiag    diag.Sink               // the sink to use for diag'ing status messages.
	Host          plugin.Host             // the plugin host to use for this query.
	BackendClient deploy.BackendClient    // the backend client to use.
	ParentSpan    opentracing.SpanContext // the parent span for tracing, if any,
}

type Providers struct {
	registry *providers.Registry
}

func LoadProviders(u UpdateInfo, opts ProvidersOptions) (*Providers, error) {
	// Create a root span for the operation
	spanOpts := []opentracing.StartSpanOption{}
	if opts.ParentSpan != nil {
		spanOpts = append(spanOpts, opentracing.ChildOf(opts.ParentSpan))
	}
	tracingSpan := opentracing.StartSpan("pulumi-providers", spanOpts...)

	proj, target := u.GetProject(), u.GetTarget()
	projinfo := &Projinfo{Proj: proj, Root: u.GetRoot()}
	_, _, plugctx, err := ProjectInfoContext(projinfo, opts.Host, target,
		opts.Diag, opts.StatusDiag, false, tracingSpan)

	// Like Update, we need to gather the set of plugins necessary to refresh everything in the snapshot.
	// Unlike Update, we don't actually run the user's program so we only need the set of plugins described
	// in the snapshot.
	plugins, err := gatherPluginsFromSnapshot(plugctx, target)
	if err != nil {
		return nil, err
	}

	// Like Update, if we're missing plugins, attempt to download the missing plugins.
	if err := ensurePluginsAreInstalled(plugctx.Request(), plugins.Deduplicate()); err != nil {
		logging.V(7).Infof("LoadProviders(): failed to install missing plugins: %v", err)
	}

	builtins := deploy.LoadBuiltinProvider(opts.BackendClient)

	registry, err := providers.NewRegistry(plugctx.Host, target.Snapshot.Resources, false, builtins)
	if err != nil {
		return nil, err
	}

	return &Providers{registry: registry}, nil
}

func (p *Providers) GetProvider(ref providers.Reference) (plugin.Provider, bool) {
	return p.registry.GetProvider(ref)
}
