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

package docsrender

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULUMI_HOME", dir)

	prefs := &Preferences{
		Language:   "python",
		OS:         "macos",
		LastPage:   "iac/concepts/stacks",
		BrowseMode: "sections",
	}
	SavePreferences(prefs)

	loaded := LoadPreferences()
	assert.Equal(t, prefs, loaded)
}

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULUMI_HOME", dir)

	prefs := LoadPreferences()
	assert.Equal(t, &Preferences{}, prefs)
}

func TestLoadMalformedFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULUMI_HOME", dir)

	err := os.WriteFile(filepath.Join(dir, preferencesFile), []byte("{not json!"), 0o600)
	require.NoError(t, err)

	prefs := LoadPreferences()
	assert.Equal(t, &Preferences{}, prefs)
}

func TestSaveCreatesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULUMI_HOME", dir)

	SavePreferences(&Preferences{Language: "go"})

	data, err := os.ReadFile(filepath.Join(dir, preferencesFile))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"language": "go"`)
}
