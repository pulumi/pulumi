// Copyright 2016-2024, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// ContinueResourceDiffEvent is a step that asks the engine to continue provisioning a resource after completing its
// diff, it is always created from a base RegisterResourceEvent.
type ContinueResourceDiffEvent interface {
	// The base event that triggered this continue event.
	Event() RegisterResourceEvent
	// Any error that occurred calling provider.Diff.
	Error() error
	// The diff result from the provider.Diff call.
	Diff() plugin.DiffResult

	// The URN of the resource being continued.
	URN() resource.URN
	// The old state of the resource being continued.
	Old() *resource.State
	// The new state of the resource being continued.
	New() *resource.State
	// The provider that should be used to continue the resource.
	Provider() plugin.Provider
	// The autonaming options that should be used to continue the resource.
	Autonaming() *plugin.AutonamingOptions
	// The random seed that should be used to continue the resource.
	RandomSeed() []byte
}

type continueDiffResourceEvent struct {
	evt        RegisterResourceEvent
	err        error
	diff       plugin.DiffResult
	urn        resource.URN
	old        *resource.State
	new        *resource.State
	provider   plugin.Provider
	autonaming *plugin.AutonamingOptions
	randomSeed []byte
}

var _ ContinueResourceDiffEvent = (*continueDiffResourceEvent)(nil)

func (g *continueDiffResourceEvent) event() {}

func (g *continueDiffResourceEvent) Event() RegisterResourceEvent {
	return g.evt
}

func (g *continueDiffResourceEvent) URN() resource.URN {
	return g.urn
}

func (g *continueDiffResourceEvent) Error() error {
	return g.err
}

func (g *continueDiffResourceEvent) Diff() plugin.DiffResult {
	return g.diff
}

func (g *continueDiffResourceEvent) Old() *resource.State {
	return g.old
}

func (g *continueDiffResourceEvent) New() *resource.State {
	return g.new
}

func (g *continueDiffResourceEvent) Provider() plugin.Provider {
	return g.provider
}

func (g *continueDiffResourceEvent) Autonaming() *plugin.AutonamingOptions {
	return g.autonaming
}

func (g *continueDiffResourceEvent) RandomSeed() []byte {
	return g.randomSeed
}
