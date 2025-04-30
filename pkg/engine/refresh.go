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
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/constant"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
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
	contract.Requiref(ctx != nil, "ctx", "cannot be nil")

	defer func() { ctx.Events <- NewCancelEvent() }()

	info, err := newDeploymentContext(u, "refresh", ctx.ParentSpan)
	if err != nil {
		return nil, nil, err
	}
	defer info.Close()

	emitter, err := makeEventEmitter(ctx.Events, []UpdateInfo{u})
	if err != nil {
		return nil, nil, err
	}
	defer emitter.Close()

	// Force opts.Refresh to true.
	opts.Refresh = true

	logging.V(7).Infof("*** Starting Refresh(preview=%v) ***", dryRun)
	defer logging.V(7).Infof("*** Refresh(preview=%v) complete ***", dryRun)

	if err := checkTargets(opts.Targets, opts.Excludes, u.Target.Snapshot); err != nil {
		return nil, nil, err
	}

	return update(ctx, info, &deploymentOptions{
		UpdateOptions:   opts,
		SourceFunc:      newRefreshSource,
		Events:          emitter,
		Diag:            newEventSink(emitter, false),
		StatusDiag:      newEventSink(emitter, true),
		debugTraceMutex: &sync.Mutex{},
		isRefresh:       true,
		DryRun:          dryRun,
	})
}

func newRefreshSource(
	ctx context.Context, client deploy.BackendClient, opts *deploymentOptions, proj *workspace.Project, pwd, main,
	projectRoot string, target *deploy.Target, plugctx *plugin.Context,
) (deploy.Source, error) {
	// Like update, we need to gather the set of plugins necessary to refresh everything in the snapshot. While we don't
	// run the program like update does, we still grab the plugins from the program in order to inform the user if their
	// program has updates to plugins that will not be used as part of the refresh operation. In the event that there is
	// no root directory/Pulumi.yaml (perhaps as the result of a command to which an explicit stack name has been passed),
	// we'll populate an empty set of program plugins.

	var programPackages PackageSet
	if plugctx.Root != "" && opts.ExecKind != constant.ExecKindAutoInline {
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
				"refresh operation is using an older version of package '%s' than the specified program version: %s < %s",
				update.New.PackageName(), update.Old.PackageVersion(), update.New.PackageVersion(),
			)))
		}
	}

	// Like Update, if we're missing plugins, attempt to download the missing plugins.
	if err := EnsurePluginsAreInstalled(ctx, opts, plugctx.Diag, snapshotPackages.ToPluginSet().Deduplicate(),
		plugctx.Host.GetProjectPlugins(), false /*reinstall*/, false /*explicitInstall*/); err != nil {
		logging.V(7).Infof("newRefreshSource(): failed to install missing plugins: %v", err)
	}

	// Just return an error source. Refresh doesn't use its source.
	return deploy.NewErrorSource(), nil
}

// RefreshV2 is a version of Refresh that uses the normal update source (i.e. it runs the user program) and
// runs the step generator in "refresh" mode. This allows it to get up-to-date configuration for provider
// resources.
func RefreshV2(
	u UpdateInfo,
	ctx *Context,
	opts UpdateOptions,
	dryRun bool,
) (*deploy.Plan, display.ResourceChanges, error) {
	contract.Requiref(ctx != nil, "ctx", "cannot be nil")

	defer func() { ctx.Events <- NewCancelEvent() }()

	info, err := newDeploymentContext(u, "refresh", ctx.ParentSpan)
	if err != nil {
		return nil, nil, err
	}
	defer info.Close()

	emitter, err := makeEventEmitter(ctx.Events, []UpdateInfo{u})
	if err != nil {
		return nil, nil, err
	}
	defer emitter.Close()

	// Force opts.Refresh and RefreshProgram to true.
	opts.Refresh = true
	opts.RefreshProgram = true

	logging.V(7).Infof("*** Starting Refresh(preview=%v) ***", dryRun)
	defer logging.V(7).Infof("*** Refresh(preview=%v) complete ***", dryRun)

	if err := checkTargets(opts.Targets, opts.Excludes, u.Target.Snapshot); err != nil {
		return nil, nil, err
	}

	return update(ctx, info, &deploymentOptions{
		UpdateOptions:   opts,
		SourceFunc:      newUpdateSource,
		Events:          emitter,
		Diag:            newEventSink(emitter, false),
		StatusDiag:      newEventSink(emitter, true),
		debugTraceMutex: &sync.Mutex{},
		isRefresh:       true,
		DryRun:          dryRun,
	})
}
