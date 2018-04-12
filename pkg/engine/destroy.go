// Copyright 2018, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func Destroy(u UpdateInfo, manager SnapshotManager,
	events chan<- Event, opts UpdateOptions) (ResourceChanges, error) {
	contract.Require(u != nil, "u")
	contract.Require(manager != nil, "manager")

	defer func() { events <- cancelEvent() }()

	ctx, err := newPlanContext(u, manager)
	if err != nil {
		return nil, err
	}
	defer ctx.Close()

	emitter := makeEventEmitter(events, u)
	return update(ctx, planOptions{
		UpdateOptions: opts,
		SourceFunc:    newDestroySource,
		Events:        emitter,
		Diag:          newEventSink(emitter),
	})
}

func newDestroySource(opts planOptions, proj *workspace.Project, pwd, main string,
	target *deploy.Target, plugctx *plugin.Context) (deploy.Source, error) {
	// For destroy, we consult the manifest for the plugin versions/ required to destroy it.
	if target != nil && target.Snapshot != nil {
		if err := plugctx.Host.EnsurePlugins(target.Snapshot.Manifest.Plugins); err != nil {
			return nil, err
		}
	}

	// Create a nil source.  This simply returns "nothing" as the new state, which will cause the
	// engine to destroy the entire existing state.
	return deploy.NullSource, nil
}
