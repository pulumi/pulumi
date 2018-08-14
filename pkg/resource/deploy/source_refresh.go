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

package deploy

import (
	"context"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// NewRefreshSource returns a new source that generates events based on reading an existing checkpoint state,
// combined with refreshing its associated resource state from the cloud provider.
func NewRefreshSource(plugctx *plugin.Context, proj *workspace.Project, target *Target, dryRun bool) Source {
	return &refreshSource{
		plugctx: plugctx,
		proj:    proj,
		target:  target,
		dryRun:  dryRun,
	}
}

// A refreshSource refreshes resource state from the cloud provider.
type refreshSource struct {
	plugctx *plugin.Context
	proj    *workspace.Project
	target  *Target
	dryRun  bool
}

func (src *refreshSource) Close() error                { return nil }
func (src *refreshSource) Project() tokens.PackageName { return src.proj.Name }
func (src *refreshSource) Info() interface{}           { return nil }
func (src *refreshSource) IsRefresh() bool             { return true }

func (src *refreshSource) Iterate(ctx context.Context, opts Options, provs ProviderSource) (SourceIterator, error) {
	contract.Require(ctx != nil, "opts.Context != nil")
	var states []*resource.State
	if snap := src.target.Snapshot; snap != nil {
		states = snap.Resources
	}

	return &refreshSourceIterator{
		ctx:       ctx,
		plugctx:   src.plugctx,
		target:    src.target,
		providers: provs,
		states:    states,
		current:   -1,
	}, nil
}

// refreshSourceIterator returns state from an existing snapshot, augmented by consulting the resource provider.
type refreshSourceIterator struct {
	ctx           context.Context // cancellation context for this source.
	plugctx       *plugin.Context
	target        *Target
	providers     ProviderSource
	states        []*resource.State
	current       int
	lastEventDone chan struct{} // completion channel for the last event that we sent, or nil if we haven't emitted any
}

func (iter *refreshSourceIterator) Close() error {
	return nil // nothing to do.
}

func (iter *refreshSourceIterator) Next() (SourceEvent, error) {
	for {
		// The engine requires that all SourceEvents returned by Next() are ready to execute.
		// The strict definition of "ready" is that the list of resources that the current goal resource depends on
		// must have completed execution before the SourceEvent corresponding to the current goal resource is
		// returned from this function.
		//
		// The simplest way to guarantee this property is to serialize every event such that the next event isn't
		// sent until the previous event retires. This isn't fast, but it works. We should come up with a more
		// performant method at some point.
		if iter.lastEventDone != nil {
			logging.V(7).Infof("refreshSourceIterator.Next(): waiting for previous event to retire")

			select {
			case <-iter.lastEventDone:
			case <-iter.ctx.Done():
				logging.V(7).Infof("refreshSourceIterator.Next(): cancelled, exiting")
				return nil, nil
			}
		}

		logging.V(7).Infof("refreshSourceIterator.Next(): sending next goal state to engine")
		iter.current++
		if iter.current >= len(iter.states) {
			logging.V(7).Infof("refreshSourceIterator.Next(): no more goal states")
			return nil, nil
		}

		current := iter.states[iter.current]
		if current.External {
			event := &refreshReadEvent{
				id:           current.ID,
				name:         current.URN.Name(),
				baseType:     current.Type,
				provider:     current.Provider,
				parent:       current.Parent,
				props:        current.Inputs,
				dependencies: current.Dependencies,
				done:         make(chan struct{}),
			}
			iter.lastEventDone = event.done
			return event, nil
		}
		goal, err := iter.newRefreshGoal(current)
		if err != nil {
			logging.V(7).Infof("refreshSourceIterator.Next(): error: %s", err.Error())
			return nil, err
		} else if goal != nil {
			event := &refreshSourceEvent{goal: goal, done: make(chan struct{})}
			iter.lastEventDone = event.done
			return event, nil
		}
		// If the goal was nil, it means the resource was deleted, and we should keep going
		// without waiting.
		iter.lastEventDone = nil
	}
}

// newRefreshGoal refreshes the state, if appropriate, and returns a new goal state.
func (iter *refreshSourceIterator) newRefreshGoal(s *resource.State) (*resource.Goal, error) {
	// If this is a custom resource, go ahead and load up its plugin, and ask it to refresh the state.
	if s.Custom && !providers.IsProviderType(s.Type) {
		providerRef, err := providers.ParseReference(s.Provider)
		if err != nil {
			return nil, err
		}
		provider, ok := iter.providers.GetProvider(providerRef)
		if !ok {
			return nil, errors.Errorf("unknown provider '%v' for resource '%v'", s.Provider, s.URN)
		}

		initErrorReasons := []string{}
		refreshed, resourceStatus, err := provider.Read(s.URN, s.ID, s.Outputs)
		if err != nil {
			if resourceStatus != resource.StatusPartialFailure {
				return nil, errors.Wrapf(err, "refreshing %s's state", s.URN)
			}

			// Else it's a `StatusPartialError`.
			if initErr, isInitErr := err.(*plugin.InitError); isInitErr {
				initErrorReasons = initErr.Reasons
			}
		} else if refreshed == nil {
			return nil, nil // the resource was deleted.
		}
		s = resource.NewState(
			s.Type, s.URN, s.Custom, s.Delete, s.ID, s.Inputs, refreshed,
			s.Parent, s.Protect, s.External, s.Dependencies, initErrorReasons, s.Provider)
	}

	// Now just return the actual state as the goal state.
	return resource.NewGoal(s.Type, s.URN.Name(), s.Custom, s.Outputs, s.Parent, s.Protect,
		s.Dependencies, s.Provider, s.InitErrors), nil
}

type refreshSourceEvent struct {
	goal *resource.Goal
	done chan struct{}
}

func (rse *refreshSourceEvent) event()               {}
func (rse *refreshSourceEvent) Goal() *resource.Goal { return rse.goal }
func (rse *refreshSourceEvent) Done(result *RegisterResult) {
	rse.done <- struct{}{}
}

type refreshReadEvent struct {
	id           resource.ID
	name         tokens.QName
	baseType     tokens.Type
	provider     string
	parent       resource.URN
	props        resource.PropertyMap
	dependencies []resource.URN
	done         chan struct{}
}

var _ ReadResourceEvent = (*refreshReadEvent)(nil)

func (g *refreshReadEvent) event()                           {}
func (g *refreshReadEvent) ID() resource.ID                  { return g.id }
func (g *refreshReadEvent) Name() tokens.QName               { return g.name }
func (g *refreshReadEvent) Type() tokens.Type                { return g.baseType }
func (g *refreshReadEvent) Provider() string                 { return g.provider }
func (g *refreshReadEvent) Parent() resource.URN             { return g.parent }
func (g *refreshReadEvent) Properties() resource.PropertyMap { return g.props }
func (g *refreshReadEvent) Dependencies() []resource.URN     { return g.dependencies }
func (g *refreshReadEvent) Done(_ *ReadResult) {
	g.done <- struct{}{}
}
