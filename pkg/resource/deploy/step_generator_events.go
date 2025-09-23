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

// ContinueResourceRefreshEvent is a step that asks the engine to continue provisioning a resource after a
// refresh, it is always created from a base RegisterResourceEvent.
type ContinueResourceRefreshEvent interface {
	RegisterResourceEvent

	URN() resource.URN
	Old() *resource.State
	New() *resource.State
	Invalid() bool
}

type continueResourceRefreshEvent struct {
	RegisterResourceEvent
	urn     resource.URN    // the URN of the resource being processed.
	old     *resource.State // the old state of the resource being processed.
	new     *resource.State // the new state of the resource being processed.
	invalid bool            // whether the resource is invalid.
}

var _ ContinueResourceRefreshEvent = (*continueResourceRefreshEvent)(nil)

func (g *continueResourceRefreshEvent) event() {}

func (g *continueResourceRefreshEvent) URN() resource.URN {
	return g.urn
}

func (g *continueResourceRefreshEvent) Old() *resource.State {
	return g.old
}

func (g *continueResourceRefreshEvent) New() *resource.State {
	return g.new
}

func (g *continueResourceRefreshEvent) Invalid() bool {
	return g.invalid
}

// ContinueResourceImportEvent is a step that asks the engine to continue provisioning a resource after an import, it is
// always created from a base RegisterResourceEvent.
type ContinueResourceImportEvent interface {
	RegisterResourceEvent

	Error() error
	URN() resource.URN
	New() *resource.State
	Old() *resource.State
	Provider() plugin.Provider
	Invalid() bool
	Recreating() bool
	RandomSeed() []byte
	IsImported() bool
}

type continueResourceImportEvent struct {
	RegisterResourceEvent
	err        error
	urn        resource.URN // the URN of the resource being processed.
	old        *resource.State
	new        *resource.State
	provider   plugin.Provider
	invalid    bool
	recreating bool
	randomSeed []byte
	// whether the resource is actually imported, or if we're just continuing the step generation for a
	// normal resource.
	isImported bool
}

var _ ContinueResourceImportEvent = (*continueResourceImportEvent)(nil)

func (g *continueResourceImportEvent) event() {}

func (g *continueResourceImportEvent) Error() error {
	return g.err
}

func (g *continueResourceImportEvent) URN() resource.URN {
	return g.urn
}

func (g *continueResourceImportEvent) Old() *resource.State {
	return g.old
}

func (g *continueResourceImportEvent) New() *resource.State {
	return g.new
}

func (g *continueResourceImportEvent) Provider() plugin.Provider {
	return g.provider
}

func (g *continueResourceImportEvent) Invalid() bool {
	return g.invalid
}

func (g *continueResourceImportEvent) Recreating() bool {
	return g.recreating
}

func (g *continueResourceImportEvent) RandomSeed() []byte {
	return g.randomSeed
}

func (g *continueResourceImportEvent) IsImported() bool {
	return g.isImported
}

// ContinueResourceCheckEvent is a step that asks the engine to continue from a Check
// event.
type ContinueResourceCheckEvent interface {
	RegisterResourceEvent
	Invalid() bool
	Recreating() bool
	WasExternal() bool
	Imported() bool
	OldInputs() resource.PropertyMap
	OldOutputs() resource.PropertyMap
	Inputs() resource.PropertyMap
	URN() resource.URN
	New() *resource.State
	Old() *resource.State
	RandomSeed() []byte
	Goal() *resource.Goal
	Provider() plugin.Provider
	IsTargeted() bool
	Autonaming() *plugin.AutonamingOptions

	Error() error

	isContinueCheckResourceEvent()
}

type continueCheckResourceEvent struct {
	RegisterResourceEvent
	invalid     bool
	recreating  bool
	wasExternal bool
	imported    bool
	oldInputs   resource.PropertyMap
	oldOutputs  resource.PropertyMap
	inputs      resource.PropertyMap
	urn         resource.URN
	new         *resource.State
	old         *resource.State
	randomSeed  []byte
	goal        *resource.Goal
	provider    plugin.Provider
	isTargeted  bool
	autonaming  *plugin.AutonamingOptions

	err error
}

func (e continueCheckResourceEvent) isContinueCheckResourceEvent()         {}
func (e continueCheckResourceEvent) Invalid() bool                         { return e.invalid }
func (e continueCheckResourceEvent) Recreating() bool                      { return e.recreating }
func (e continueCheckResourceEvent) WasExternal() bool                     { return e.wasExternal }
func (e continueCheckResourceEvent) Imported() bool                        { return e.imported }
func (e continueCheckResourceEvent) OldInputs() resource.PropertyMap       { return e.oldInputs }
func (e continueCheckResourceEvent) OldOutputs() resource.PropertyMap      { return e.oldOutputs }
func (e continueCheckResourceEvent) Inputs() resource.PropertyMap          { return e.inputs }
func (e continueCheckResourceEvent) URN() resource.URN                     { return e.urn }
func (e continueCheckResourceEvent) New() *resource.State                  { return e.new }
func (e continueCheckResourceEvent) Old() *resource.State                  { return e.old }
func (e continueCheckResourceEvent) RandomSeed() []byte                    { return e.randomSeed }
func (e continueCheckResourceEvent) Goal() *resource.Goal                  { return e.goal }
func (e continueCheckResourceEvent) Provider() plugin.Provider             { return e.provider }
func (e continueCheckResourceEvent) IsTargeted() bool                      { return e.isTargeted }
func (e continueCheckResourceEvent) Autonaming() *plugin.AutonamingOptions { return e.autonaming }
func (e continueCheckResourceEvent) Error() error                          { return e.err }
