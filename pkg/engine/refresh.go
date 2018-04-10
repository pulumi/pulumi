// Copyright 2018, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func Refresh(u UpdateInfo, events chan<- Event, opts UpdateOptions, dryRun bool) (ResourceChanges, error) {
	contract.Require(u != nil, "u")

	defer func() { events <- cancelEvent() }()

	ctx, err := newPlanContext(u)
	if err != nil {
		return nil, err
	}
	defer ctx.Close()

	emitter := makeEventEmitter(events, u)
	return update(ctx, planOptions{
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
		if err := plugctx.Host.EnsurePlugins(target.Snapshot.Manifest.Plugins); err != nil {
			return nil, err
		}
	}

	// Now create a refresh source.  This source simply loads up the current checkpoint state, enumerates it,
	// and refreshes each state with the current cloud provider's view of it.
	return deploy.NewRefreshSource(plugctx, proj, target, dryRun), nil
}
