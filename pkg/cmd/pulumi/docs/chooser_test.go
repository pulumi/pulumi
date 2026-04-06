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

package docs

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/docsrender"
	"github.com/stretchr/testify/assert"
)

func TestBuildChooserSelectionsNonInteractive(t *testing.T) {
	// buildChooserSelections calls cmdutil.Interactive() which returns false in tests,
	// so it won't prompt. We can test the flag/session/preference resolution logic.

	t.Run("flag takes priority over preferences", func(t *testing.T) {
		t.Parallel()
		prefs := &docsrender.Preferences{Language: "python"}
		session := map[string]string{}
		selections := buildChooserSelections(nil, prefs, "typescript", "", session)
		assert.Equal(t, "typescript", selections["language"])
	})

	t.Run("preferences used when no flag", func(t *testing.T) {
		t.Parallel()
		prefs := &docsrender.Preferences{Language: "python"}
		session := map[string]string{}
		selections := buildChooserSelections(nil, prefs, "", "", session)
		assert.Equal(t, "python", selections["language"])
	})

	t.Run("session reused over preferences", func(t *testing.T) {
		t.Parallel()
		prefs := &docsrender.Preferences{Language: "python"}
		session := map[string]string{"language": "go"}
		selections := buildChooserSelections(nil, prefs, "", "", session)
		assert.Equal(t, "go", selections["language"])
	})

	t.Run("flag overrides session", func(t *testing.T) {
		t.Parallel()
		prefs := &docsrender.Preferences{}
		session := map[string]string{"language": "go"}
		selections := buildChooserSelections(nil, prefs, "typescript", "", session)
		assert.Equal(t, "typescript", selections["language"])
	})

	t.Run("os flag applied", func(t *testing.T) {
		t.Parallel()
		prefs := &docsrender.Preferences{}
		session := map[string]string{}
		selections := buildChooserSelections(nil, prefs, "", "macos", session)
		assert.Equal(t, "macos", selections["os"])
	})

	t.Run("session updated with selections", func(t *testing.T) {
		t.Parallel()
		prefs := &docsrender.Preferences{}
		session := map[string]string{}
		_ = buildChooserSelections(nil, prefs, "go", "linux", session)
		assert.Equal(t, "go", session["language"])
		assert.Equal(t, "linux", session["os"])
	})
}
