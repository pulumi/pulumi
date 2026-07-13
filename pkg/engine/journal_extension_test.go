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

package engine

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/require"
)

// captureJournal collects entries in memory for assertion.
type captureJournal struct {
	entries []JournalEntry
}

func (c *captureJournal) AddJournalEntry(entry JournalEntry) error {
	c.entries = append(c.entries, entry)
	return nil
}

func (c *captureJournal) Close() error { return nil }

func TestJournalExtensionParameterize(t *testing.T) {
	t.Parallel()

	journal := &captureJournal{}
	sm, err := NewJournalSnapshotManager(journal, nil, b64.NewBase64SecretsManager(), apitype.LatestJournalVersion)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sm.Close()) })

	ref := apitype.ExtensionRef("ref-1")
	ext := apitype.Extension{Name: "myext", Version: "1.0.0", Value: []byte("Hello")}
	step := deploy.NewExtensionParameterizeStep(nil, nil, ref, ext, nil)

	mutation, err := sm.BeginMutation(step)
	require.NoError(t, err)
	require.NoError(t, mutation.End(step, true))

	// Find the extension entry the journal recorded.
	var found *JournalEntry
	for i := range journal.entries {
		if journal.entries[i].Kind == JournalEntryExtensionParameterize {
			found = &journal.entries[i]
			break
		}
	}
	require.NotNil(t, found, "expected a JournalEntryExtensionParameterize in the journal")
	require.NotNil(t, found.ExtensionRef)
	require.NotNil(t, found.Extension)
	require.Equal(t, ref, *found.ExtensionRef)
	require.Equal(t, ext, *found.Extension)
}
