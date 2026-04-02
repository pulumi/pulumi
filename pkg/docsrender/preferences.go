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
	"encoding/json"
	"errors"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const preferencesFile = "docs-preferences.json"

// Preferences stores user preferences for docs rendering.
type Preferences struct {
	Language   string `json:"language,omitempty"`
	OS         string `json:"os,omitempty"`
	LastPage   string `json:"lastPage,omitempty"`
	BrowseMode string `json:"browseMode,omitempty"`
}

// preferencesPath returns the path to the preferences file.
func preferencesPath() (string, error) {
	return workspace.GetPulumiPath(preferencesFile)
}

// LoadPreferences reads preferences from disk. Returns empty preferences
// if the file is missing or malformed.
func LoadPreferences() *Preferences {
	path, err := preferencesPath()
	if err != nil {
		logging.V(7).Infof("docs preferences: could not resolve path: %v", err)
		return &Preferences{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logging.V(7).Infof("docs preferences: could not read %s: %v", path, err)
		}
		return &Preferences{}
	}

	var prefs Preferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		logging.V(7).Infof("docs preferences: could not parse %s: %v", path, err)
		return &Preferences{}
	}

	return &prefs
}

// SavePreferences writes preferences to disk. Errors are logged at V(7)
// and not returned; a failed save must never abort a render.
func SavePreferences(prefs *Preferences) {
	path, err := preferencesPath()
	if err != nil {
		logging.V(7).Infof("docs preferences: could not resolve path: %v", err)
		return
	}

	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		logging.V(7).Infof("docs preferences: could not marshal: %v", err)
		return
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		logging.V(7).Infof("docs preferences: could not write %s: %v", path, err)
	}
}
