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
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func Refresh(
	u UpdateInfo,
	ctx *Context,
	opts UpdateOptions,
	dryRun bool) (*deploy.Plan, display.ResourceChanges, result.Result) {

	contract.Requiref(u != nil, "u", "cannot be nil")
	contract.Requiref(ctx != nil, "ctx", "cannot be nil")

	defer func() { ctx.Events <- cancelEvent() }()

	info, err := newDeploymentContext(u, "refresh", ctx.ParentSpan)
	if err != nil {
		return nil, nil, result.FromError(err)
	}
	defer info.Close()

	emitter, err := makeEventEmitter(ctx.Events, u)
	if err != nil {
		return nil, nil, result.FromError(err)
	}
	defer emitter.Close()

	// Force opts.Refresh to true.
	opts.Refresh = true

	logging.V(7).Infof("*** Starting Refresh(preview=%v) ***", dryRun)
	defer logging.V(7).Infof("*** Refresh(preview=%v) complete ***", dryRun)

	return update(ctx, info, deploymentOptions{
		UpdateOptions: opts,
		SourceFunc:    newRefreshSource,
		Events:        emitter,
		Diag:          newEventSink(emitter, false),
		StatusDiag:    newEventSink(emitter, true),
		isRefresh:     true,
	}, dryRun)
}

func newRefreshSource(client deploy.BackendClient, opts deploymentOptions, proj *workspace.Project, pwd, main string,
	target *deploy.Target, plugctx *plugin.Context, dryRun bool) (deploy.Source, error) {

	snapshotPlugins, err := gatherPluginsFromSnapshot(plugctx, target)
	if err != nil {
		return nil, err
	}
	programPlugins, err := gatherPluginsFromProgram(plugctx, plugin.ProgInfo{
		Proj:    proj,
		Pwd:     pwd,
		Program: main,
	})
	if err != nil {
		return nil, err
	}

	for _, pp := range programPlugins.Values() {
		for _, sp := range snapshotPlugins.Values() {
			if sp.Name == pp.Name && sp.Kind == pp.Kind {
				if pp.Version.GT(*sp.Version) {
					msg := fmt.Sprintf(
						"refresh operation is using an older version of plugin '%s' than the specified program version: %s < %s",
						pp.Name, sp.Version, pp.Version)
					plugctx.Diag.Warningf(diag.Message("", msg))
				}
			}
		}
	}

	// Like Update, if we're missing plugins, attempt to download the missing plugins.
	if err := ensurePluginsAreInstalled(plugctx.Request(), snapshotPlugins.Deduplicate(),
		plugctx.Host.GetProjectPlugins()); err != nil {
		logging.V(7).Infof("newRefreshSource(): failed to install missing plugins: %v", err)
	}

	// Just return an error source. Refresh doesn't use its source.
	return deploy.NewErrorSource(proj.Name), nil
}
