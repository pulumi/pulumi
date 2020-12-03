// Copyright 2016-2020, Pulumi Corporation.
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

package python

import (
	"encoding/json"
	"io/ioutil"
)

// PulumiPlugin represents an optional pulumiplugin.json file that can be included inside a Python package to include
// additional information about the package's associated Pulumi plugin.
//nolint:lll
type PulumiPlugin struct {
	Resource bool   `json:"resource"` // Indicates whether the package has an associated resource plugin. Set to false to indicate no plugin.
	Name     string `json:"name"`     // Optional plugin name. If not set, the plugin name is derived from the package name.
	Version  string `json:"version"`  // Optional plugin version. If not set, the version is derived from the package version (if possible).
	Server   string `json:"server"`   // Optional plugin server. If not set, the default server is used when installing the plugin.
}

func (plugin *PulumiPlugin) MarshalJSON() ([]byte, error) {
	json, err := json.MarshalIndent(plugin, "", "  ")
	if err != nil {
		return nil, err
	}
	return json, nil
}

func LoadPulumiPluginFile(path string) (*PulumiPlugin, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		// Deliberately not wrapping the error here so that os.IsNotExist checks can be used to determine
		// if the file could not be opened due to it not existing.
		return nil, err
	}

	var plugin *PulumiPlugin
	if err := json.Unmarshal(b, plugin); err != nil {
		return nil, err
	}

	return plugin, nil
}
