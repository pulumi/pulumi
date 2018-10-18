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

func Destroy(u UpdateInfo, ctx *Context, opts UpdateOptions, dryRun bool) (ResourceChanges, error) {
	contract.Require(u != nil, "u")
	contract.Require(ctx != nil, "ctx")

	defer func() { ctx.Events <- cancelEvent() }()

	info, err := newPlanContext(u, "destroy", ctx.ParentSpan)
	if err != nil {
		return nil, err
	}
	defer info.Close()

	emitter, err := makeEventEmitter(ctx.Events, u)
	if err != nil {
		return nil, err
	}
	return update(ctx, info, planOptions{
		UpdateOptions: opts,
		SourceFunc:    newDestroySource,
		Events:        emitter,
		Diag:          newEventSink(emitter, false),
		StatusDiag:    newEventSink(emitter, true),
	}, dryRun)
}

func newDestroySource(
	opts planOptions, proj *workspace.Project, pwd, main string,
	target *deploy.Target, plugctx *plugin.Context, dryRun bool) (deploy.Source, error) {

	// For destroy, we consult the manifest for the plugin versions/ required to destroy it.
	if target != nil && target.Snapshot != nil {
		// We don't need the language plugin, since destroy doesn't run code, so we will leave that out.
		kinds := plugin.AnalyzerPlugins
		if err := plugctx.Host.EnsurePlugins(target.Snapshot.Manifest.Plugins, kinds); err != nil {
			return nil, err
		}
	}

	// Create a nil source.  This simply returns "nothing" as the new state, which will cause the
	// engine to destroy the entire existing state.
	return deploy.NullSource, nil
}
