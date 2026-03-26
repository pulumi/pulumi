// Copyright 2024, Pulumi Corporation.
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
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const preferencesFile = "docs-preferences.json"

// Preferences stores the user's last chooser selections and last viewed page.
type Preferences struct {
	Language   string `json:"language,omitempty"`
	OS         string `json:"os,omitempty"`
	Cloud      string `json:"cloud,omitempty"`
	LastPage   string `json:"lastPage,omitempty"`
	BrowseMode string `json:"browseMode,omitempty"` // "sections" (default) or "full"
}

// Get returns the stored preference for the given chooser type.
func (p *Preferences) Get(chooserType string) string {
	switch chooserType {
	case "language":
		return p.Language
	case "os":
		return p.OS
	case "cloud":
		return p.Cloud
	default:
		return ""
	}
}

// Set updates the stored preference for the given chooser type.
func (p *Preferences) Set(chooserType, value string) {
	switch chooserType {
	case "language":
		p.Language = value
	case "os":
		p.OS = value
	case "cloud":
		p.Cloud = value
	}
}

func preferencesPath() (string, error) {
	home, err := workspace.GetPulumiHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, preferencesFile), nil
}

// LoadPreferences reads preferences from disk. Returns empty preferences if the file doesn't exist.
func LoadPreferences() (*Preferences, error) {
	path, err := preferencesPath()
	if err != nil {
		return &Preferences{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Preferences{}, nil
		}
		return nil, err
	}

	var prefs Preferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		return &Preferences{}, nil
	}
	return &prefs, nil
}

// Save writes preferences to disk.
func (p *Preferences) Save() error {
	path, err := preferencesPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
