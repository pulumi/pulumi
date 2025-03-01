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
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func Destroy(
	u UpdateInfo,
	ctx *Context,
	opts UpdateOptions,
	dryRun bool,
) (*deploy.Plan, display.ResourceChanges, error) {
	contract.Requiref(u != nil, "u", "cannot be nil")
	contract.Requiref(ctx != nil, "ctx", "cannot be nil")

	defer func() { ctx.Events <- NewCancelEvent() }()

	info, err := newDeploymentContext(u, "destroy", ctx.ParentSpan)
	if err != nil {
		return nil, nil, err
	}
	defer info.Close()

	emitter, err := makeEventEmitter(ctx.Events, u)
	if err != nil {
		return nil, nil, err
	}
	defer emitter.Close()

	logging.V(7).Infof("*** Starting Destroy(preview=%v) ***", dryRun)
	defer logging.V(7).Infof("*** Destroy(preview=%v) complete ***", dryRun)

	if err := checkTargets(opts.Targets, u.GetTarget().Snapshot); err != nil {
		return nil, nil, err
	}

	return update(ctx, info, &deploymentOptions{
		UpdateOptions: opts,
		SourceFunc:    newDestroySource,
		Events:        emitter,
		Diag:          newEventSink(emitter, false),
		StatusDiag:    newEventSink(emitter, true),
		DryRun:        dryRun,
	})
}

func newDestroySource(
	ctx context.Context,
	client deploy.BackendClient, opts *deploymentOptions, proj *workspace.Project, pwd, main, projectRoot string,
	target *deploy.Target, plugctx *plugin.Context,
) (deploy.Source, error) {
	// Like update, we need to gather the set of plugins necessary to delete everything in the snapshot. While we don't
	// run the program like update does, we still grab the plugins from the program in order to inform the user if their
	// program has updates to plugins that will not be used as part of the destroy operation. In the event that there is
	// no root directory/Pulumi.yaml (perhaps as the result of a command to which an explicit stack name has been passed),
	// we'll populate an empty set of program plugins.

	var programPackages PackageSet
	if plugctx.Root != "" {
		runtime := proj.Runtime.Name()
		programInfo := plugin.NewProgramInfo(
			/* rootDirectory */ plugctx.Root,
			/* programDirectory */ pwd,
			/* entryPoint */ main,
			/* options */ proj.Runtime.Options(),
		)

		var err error
		programPackages, err = gatherPackagesFromProgram(plugctx, runtime, programInfo)
		if err != nil {
			programPackages = NewPackageSet()
		}
	} else {
		programPackages = NewPackageSet()
	}

	snapshotPackages, err := gatherPackagesFromSnapshot(plugctx, target)
	if err != nil {
		return nil, err
	}

	packageUpdates := programPackages.UpdatesTo(snapshotPackages)
	if len(packageUpdates) > 0 {
		for _, update := range packageUpdates {
			plugctx.Diag.Warningf(diag.Message("", fmt.Sprintf(
				"destroy operation is using an older version of package '%s' than the specified program version: %s < %s",
				update.New.PackageName(), update.Old.PackageVersion(), update.New.PackageVersion(),
			)))
		}
	}

	// Like Update, if we're missing plugins, attempt to download the missing plugins.
	allPlugins := snapshotPackages.ToPluginSet().Deduplicate()

	if err := EnsurePluginsAreInstalled(ctx, opts, plugctx.Diag, allPlugins,
		plugctx.Host.GetProjectPlugins(), false /*reinstall*/, false /*explicitInstall*/); err != nil {
		logging.V(7).Infof("newDestroySource(): failed to install missing plugins: %v", err)
	}

	// We don't need the language plugin, since destroy doesn't run code, so we will leave that out.
	if err := ensurePluginsAreLoaded(plugctx, allPlugins, plugin.AnalyzerPlugins); err != nil {
		return nil, err
	}

	// Create a nil source.  This simply returns "nothing" as the new state, which will cause the
	// engine to destroy the entire existing state.
	return deploy.NewNullSource(proj.Name), nil
}

func DestroyV2(
	u UpdateInfo,
	ctx *Context,
	opts UpdateOptions,
	dryRun bool,
) (*deploy.Plan, display.ResourceChanges, error) {
	contract.Requiref(u != nil, "u", "cannot be nil")
	contract.Requiref(ctx != nil, "ctx", "cannot be nil")

	defer func() { ctx.Events <- NewCancelEvent() }()

	info, err := newDeploymentContext(u, "destroy", ctx.ParentSpan)
	if err != nil {
		return nil, nil, err
	}
	defer info.Close()

	emitter, err := makeEventEmitter(ctx.Events, u)
	if err != nil {
		return nil, nil, err
	}
	defer emitter.Close()

	// Force opt.DestroyProgram to true
	opts.DestroyProgram = true

	logging.V(7).Infof("*** Starting Destroy(preview=%v) ***", dryRun)
	defer logging.V(7).Infof("*** Destroy(preview=%v) complete ***", dryRun)

	if err := checkTargets(opts.Targets, u.GetTarget().Snapshot); err != nil {
		return nil, nil, err
	}

	return update(ctx, info, &deploymentOptions{
		UpdateOptions: opts,
		SourceFunc:    newUpdateSource,
		Events:        emitter,
		Diag:          newEventSink(emitter, false),
		StatusDiag:    newEventSink(emitter, true),
		DryRun:        dryRun,
	})
}
