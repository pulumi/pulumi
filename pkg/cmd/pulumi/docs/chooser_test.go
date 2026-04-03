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
	"github.com/stretchr/testify/require"
)

func TestScanChoosers(t *testing.T) {
	t.Parallel()

	t.Run("single chooser with two options", func(t *testing.T) {
		t.Parallel()
		md := "Before.\n\n<!-- chooser: language -->\n" +
			"<!-- option: typescript -->\nconsole.log('hi');\n<!-- /option -->\n" +
			"<!-- option: python -->\nprint('hi')\n<!-- /option -->\n" +
			"<!-- /chooser -->\n\nAfter."
		result := scanChoosers(md)
		require.Len(t, result, 1)
		assert.Equal(t, "language", result[0].chooserType)
		require.Len(t, result[0].options, 2)
		assert.Equal(t, "typescript", result[0].options[0])
		assert.Equal(t, "python", result[0].options[1])
	})

	t.Run("plain text only", func(t *testing.T) {
		t.Parallel()
		md := "Just plain text with no choosers."
		result := scanChoosers(md)
		assert.Empty(t, result)
	})

	t.Run("unmatched chooser open tag", func(t *testing.T) {
		t.Parallel()
		md := "Before.\n<!-- chooser: language -->\nNo closing tag."
		result := scanChoosers(md)
		// No closing tag means no complete chooser found
		assert.Empty(t, result)
	})

	t.Run("multiple choosers", func(t *testing.T) {
		t.Parallel()
		md := "<!-- chooser: language -->\n<!-- option: go -->\n" +
			"Go code\n<!-- /option -->\n<!-- /chooser -->\n\nMiddle text.\n\n" +
			"<!-- chooser: os -->\n<!-- option: linux -->\n" +
			"Linux\n<!-- /option -->\n<!-- /chooser -->"
		result := scanChoosers(md)
		require.Len(t, result, 2)
		assert.Equal(t, "language", result[0].chooserType)
		assert.Equal(t, "os", result[1].chooserType)
	})

	t.Run("duplicate chooser types deduplicated", func(t *testing.T) {
		t.Parallel()
		md := "<!-- chooser: language -->\n<!-- option: go -->\n" +
			"Go code\n<!-- /option -->\n<!-- /chooser -->\n\n" +
			"<!-- chooser: language -->\n<!-- option: python -->\n" +
			"Python code\n<!-- /option -->\n<!-- /chooser -->"
		result := scanChoosers(md)
		require.Len(t, result, 1)
		assert.Equal(t, "language", result[0].chooserType)
		// Only the first occurrence's options are captured
		require.Len(t, result[0].options, 1)
		assert.Equal(t, "go", result[0].options[0])
	})

	t.Run("chooser with three options", func(t *testing.T) {
		t.Parallel()
		md := "<!-- chooser: language -->\n" +
			"<!-- option: typescript -->\nTS\n<!-- /option -->\n" +
			"<!-- option: python -->\nPY\n<!-- /option -->\n" +
			"<!-- option: go -->\nGO\n<!-- /option -->\n" +
			"<!-- /chooser -->"
		result := scanChoosers(md)
		require.Len(t, result, 1)
		require.Len(t, result[0].options, 3)
		assert.Equal(t, "typescript", result[0].options[0])
		assert.Equal(t, "python", result[0].options[1])
		assert.Equal(t, "go", result[0].options[2])
	})

	t.Run("os chooser", func(t *testing.T) {
		t.Parallel()
		md := "<!-- chooser: os -->\n" +
			"<!-- option: linux -->\napt-get install\n<!-- /option -->\n" +
			"<!-- option: macos -->\nbrew install\n<!-- /option -->\n" +
			"<!-- /chooser -->"
		result := scanChoosers(md)
		require.Len(t, result, 1)
		assert.Equal(t, "os", result[0].chooserType)
		require.Len(t, result[0].options, 2)
		assert.Equal(t, "linux", result[0].options[0])
		assert.Equal(t, "macos", result[0].options[1])
	})
}

func TestBuildChooserSelectionsNonInteractive(t *testing.T) {
	// buildChooserSelections calls cmdutil.Interactive() which returns false in tests,
	// so it won't prompt. We can test the flag/session/preference resolution logic.

	t.Run("flag takes priority over preferences", func(t *testing.T) {
		t.Parallel()
		prefs := &docsrender.Preferences{Language: "python"}
		session := map[string]string{}
		selections := buildChooserSelections("", prefs, "typescript", "", session)
		assert.Equal(t, "typescript", selections["language"])
	})

	t.Run("preferences used when no flag", func(t *testing.T) {
		t.Parallel()
		prefs := &docsrender.Preferences{Language: "python"}
		session := map[string]string{}
		selections := buildChooserSelections("", prefs, "", "", session)
		assert.Equal(t, "python", selections["language"])
	})

	t.Run("session reused over preferences", func(t *testing.T) {
		t.Parallel()
		prefs := &docsrender.Preferences{Language: "python"}
		session := map[string]string{"language": "go"}
		selections := buildChooserSelections("", prefs, "", "", session)
		assert.Equal(t, "go", selections["language"])
	})

	t.Run("flag overrides session", func(t *testing.T) {
		t.Parallel()
		prefs := &docsrender.Preferences{}
		session := map[string]string{"language": "go"}
		selections := buildChooserSelections("", prefs, "typescript", "", session)
		assert.Equal(t, "typescript", selections["language"])
	})

	t.Run("os flag applied", func(t *testing.T) {
		t.Parallel()
		prefs := &docsrender.Preferences{}
		session := map[string]string{}
		selections := buildChooserSelections("", prefs, "", "macos", session)
		assert.Equal(t, "macos", selections["os"])
	})

	t.Run("session updated with selections", func(t *testing.T) {
		t.Parallel()
		prefs := &docsrender.Preferences{}
		session := map[string]string{}
		_ = buildChooserSelections("", prefs, "go", "linux", session)
		assert.Equal(t, "go", session["language"])
		assert.Equal(t, "linux", session["os"])
	})
}
