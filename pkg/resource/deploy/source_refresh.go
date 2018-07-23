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
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
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

func (src *refreshSource) Iterate(opts Options) (SourceIterator, error) {
	var states []*resource.State
	if snap := src.target.Snapshot; snap != nil {
		states = snap.Resources
	}
	return &refreshSourceIterator{
		plugctx: src.plugctx,
		states:  states,
		current: -1,
	}, nil
}

// refreshSourceIterator returns state from an existing snapshot, augmented by consulting the resource provider.
type refreshSourceIterator struct {
	plugctx *plugin.Context
	states  []*resource.State
	current int
}

func (iter *refreshSourceIterator) Close() error {
	return nil // nothing to do.
}

func (iter *refreshSourceIterator) Next() (SourceEvent, error) {
	for {
		iter.current++
		if iter.current >= len(iter.states) {
			return nil, nil
		}
		goal, err := iter.newRefreshGoal(iter.states[iter.current])
		if err != nil {
			return nil, err
		} else if goal != nil {
			return &refreshSourceEvent{goal: goal}, nil
		}
		// If the goal was nil, it means the resource was deleted, and we should keep going.
	}
}

// newRefreshGoal refreshes the state, if appropriate, and returns a new goal state.
func (iter *refreshSourceIterator) newRefreshGoal(s *resource.State) (*resource.Goal, error) {
	// If this is a custom resource, go ahead and load up its plugin, and ask it to refresh the state.
	if s.Custom {
		provider, err := iter.plugctx.Host.Provider(s.Type.Package(), nil)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching provider to refresh %s", s.URN)
		}
		refreshed, err := provider.Read(s.URN, s.ID, s.Outputs)
		if err != nil {
			return nil, errors.Wrapf(err, "refreshing %s's state", s.URN)
		} else if refreshed == nil {
			return nil, nil // the resource was deleted.
		}
		s = resource.NewState(
			s.Type, s.URN, s.Custom, s.Delete, s.ID, s.Inputs, refreshed,
			s.Parent, s.Protect, s.External, s.Dependencies, s.InitErrors)
	}

	// Now just return the actual state as the goal state.
	return resource.NewGoal(s.Type, s.URN.Name(), s.Custom, s.Outputs, s.Parent, s.Protect, s.Dependencies), nil
}

type refreshSourceEvent struct {
	goal *resource.Goal
}

func (rse *refreshSourceEvent) event()                      {}
func (rse *refreshSourceEvent) Goal() *resource.Goal        { return rse.goal }
func (rse *refreshSourceEvent) Done(result *RegisterResult) {}
