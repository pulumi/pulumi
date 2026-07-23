// Copyright 2020, Pulumi Corporation.
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
	"testing"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestTrySendEvent(t *testing.T) {
	t.Parallel()
	e := Event{}
	c := make(chan Event, 100)
	assert.Equal(t, true, trySendEvent(c, e))
	close(c)
	assert.Equal(t, false, trySendEvent(c, e))
}

func TestTryCloseEventChan(t *testing.T) {
	t.Parallel()
	c := make(chan Event, 100)
	assert.Equal(t, true, tryCloseEventChan(c))
	assert.Equal(t, false, tryCloseEventChan(c))
}

// TestStateMigrationEventEphemeral guards that state-migration events are ephemeral, so they are shown in the
// display and the live event stream but never persisted or uploaded to the Pulumi Cloud service, which older
// service versions do not recognize.
func TestStateMigrationEventEphemeral(t *testing.T) {
	t.Parallel()
	e := NewEvent(StateMigrationEventPayload{URN: "urn:pulumi:test::test::my:module:Comp::comp"})
	assert.True(t, e.Ephemeral(), "state-migration events must be ephemeral so they are not uploaded")
	assert.False(t, e.Internal())
}

func TestSummarizeStateMigration(t *testing.T) {
	t.Parallel()

	state := func(urn resource.URN) *pkgresource.State {
		return &pkgresource.State{URN: urn}
	}
	const root resource.URN = "urn:root"

	t.Run("rename", func(t *testing.T) {
		t.Parallel()

		prior := []*pkgresource.State{state("urn:a")}
		migrated := []*pkgresource.State{state("urn:b")}
		successors := map[resource.URN]resource.URN{"urn:a": "urn:b"}

		assert.Equal(t, StateMigrationEventPayload{
			URN:        root,
			Migrated:   1,
			Added:      []resource.URN{"urn:b"},
			Removed:    []resource.URN{"urn:a"},
			Successors: successors,
		}, summarizeStateMigration(root, prior, migrated, successors))
	})

	t.Run("fold", func(t *testing.T) {
		t.Parallel()

		prior := []*pkgresource.State{state("urn:a"), state("urn:b")}
		migrated := []*pkgresource.State{state("urn:c")}
		successors := map[resource.URN]resource.URN{"urn:a": "urn:c", "urn:b": "urn:c"}

		assert.Equal(t, StateMigrationEventPayload{
			URN:        root,
			Migrated:   2,
			Added:      []resource.URN{"urn:c"},
			Removed:    []resource.URN{"urn:a", "urn:b"},
			Successors: successors,
		}, summarizeStateMigration(root, prior, migrated, successors))
	})
}
