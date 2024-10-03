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
	"context"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func Refresh(
	u UpdateInfo,
	ctx *Context,
	opts UpdateOptions,
	dryRun bool,
) (*deploy.Plan, display.ResourceChanges, error) {
	contract.Requiref(u != nil, "u", "cannot be nil")
	contract.Requiref(ctx != nil, "ctx", "cannot be nil")

	defer func() { ctx.Events <- NewCancelEvent() }()

	info, err := newDeploymentContext(u, "refresh", ctx.ParentSpan)
	if err != nil {
		return nil, nil, err
	}
	defer info.Close()

	emitter, err := makeEventEmitter(ctx.Events, u)
	if err != nil {
		return nil, nil, err
	}
	defer emitter.Close()

	// Force opts.Refresh to true.
	opts.Refresh = true

	logging.V(7).Infof("*** Starting Refresh(preview=%v) ***", dryRun)
	defer logging.V(7).Infof("*** Refresh(preview=%v) complete ***", dryRun)

	if err := checkTargets(opts.Targets, u.GetTarget().Snapshot); err != nil {
		return nil, nil, err
	}

	return update(ctx, info, &deploymentOptions{
		UpdateOptions: opts,
		SourceFunc:    newRefreshSource,
		Events:        emitter,
		Diag:          newEventSink(emitter, false),
		StatusDiag:    newEventSink(emitter, true),
		isRefresh:     true,
		DryRun:        dryRun,
	})
}

func newRefreshSource(
	ctx context.Context, client deploy.BackendClient, opts *deploymentOptions, proj *workspace.Project, pwd, main,
	projectRoot string, target *deploy.Target, plugctx *plugin.Context,
) (deploy.Source, error) {
	// Like Update, we need to gather the set of plugins necessary to refresh everything in the snapshot.
	// Unlike Update, we don't actually run the user's program so we only need the set of plugins described
	// in the snapshot.
	plugins, err := gatherPluginsFromSnapshot(plugctx, target)
	if err != nil {
		return nil, err
	}

	// Like Update, if we're missing plugins, attempt to download the missing plugins.
	if err := EnsurePluginsAreInstalled(ctx, opts, plugctx.Diag, plugins.Deduplicate(),
		plugctx.Host.GetProjectPlugins(), false /*reinstall*/, false /*explicitInstall*/); err != nil {
		logging.V(7).Infof("newRefreshSource(): failed to install missing plugins: %v", err)
	}

	// Just return an error source. Refresh doesn't use its source.
	return deploy.NewErrorSource(proj.Name), nil
}
