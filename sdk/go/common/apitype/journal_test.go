// Copyright 2026, Pulumi Corporation.
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

package apitype

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJournalEntryStringStateMigration(t *testing.T) {
	t.Parallel()
	baseIndex, stateIndex := int64(3), int64(0)
	state := ResourceV3{
		URN:     "urn:pulumi:test::project::pkg:m:Resource::resource",
		Inputs:  map[string]any{"password": "sensitive-input"},
		Outputs: map[string]any{"password": "sensitive-output"},
	}
	entry := JournalEntry{
		SequenceID: 12, Kind: JournalEntryKindStateMigration,
		Layout:           []JournalLayoutItem{{BaseIndex: &baseIndex}, {StateIndex: &stateIndex}},
		States:           []ResourceV3{state},
		BaseStatePatches: []JournalBaseStatePatch{{Index: 3, State: state}},
		NewStatePatches:  []JournalNewStatePatch{{OperationID: 7, State: state}},
	}
	actual := entry.String()
	assert.Equal(t, "(12, 0): state-migration, layout([{base:3} {state:0}]), "+
		"states([urn:pulumi:test::project::pkg:m:Resource::resource]), "+
		"baseStatePatches([3:urn:pulumi:test::project::pkg:m:Resource::resource]), "+
		"newStatePatches([7:urn:pulumi:test::project::pkg:m:Resource::resource])", actual)
	assert.NotContains(t, actual, "sensitive-input")
	assert.NotContains(t, actual, "sensitive-output")
}
