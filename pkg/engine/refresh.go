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
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func Refresh(u UpdateInfo, ctx *Context, opts UpdateOptions, dryRun bool) (ResourceChanges, error) {
	contract.Require(u != nil, "u")
	contract.Require(ctx != nil, "ctx")

	defer func() { ctx.Events <- cancelEvent() }()

	info, err := newPlanContext(u, "refresh", ctx.ParentSpan)
	if err != nil {
		return nil, err
	}
	defer info.Close()

	emitter := makeEventEmitter(ctx.Events, u)
	return update(ctx, info, planOptions{
		UpdateOptions: opts,
		SkipOutputs:   true, // refresh is exclusively about outputs
		SourceFunc:    newRefreshSource,
		Events:        emitter,
		Diag:          newEventSink(emitter),
	}, dryRun)
}

func newRefreshSource(opts planOptions, proj *workspace.Project, pwd, main string,
	target *deploy.Target, plugctx *plugin.Context, dryRun bool) (deploy.Source, error) {

	// First, consult the manifest for the plugins we will need to ask to refresh the state.
	if target != nil && target.Snapshot != nil {
		// We don't need the language plugin, since refresh doesn't run code, so we will leave that out.
		kinds := plugin.AnalyzerPlugins
		if err := plugctx.Host.EnsurePlugins(target.Snapshot.Manifest.Plugins, kinds); err != nil {
			return nil, err
		}
	}

	// Now create a refresh source.  This source simply loads up the current checkpoint state, enumerates it,
	// and refreshes each state with the current cloud provider's view of it.
	return deploy.NewRefreshSource(plugctx, proj, target, dryRun), nil
}
