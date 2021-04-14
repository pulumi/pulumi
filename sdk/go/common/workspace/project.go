// Copyright 2016-2018, Pulumi Corporation.
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

package workspace

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Analyzers is a list of analyzers to run on this project.
type Analyzers []tokens.QName

// ProjectTemplate is a Pulumi project template manifest.
type ProjectTemplate struct {
	// Description is an optional description of the template.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Quickstart contains optional text to be displayed after template creation.
	Quickstart string `json:"quickstart,omitempty" yaml:"quickstart,omitempty"`
	// Config is an optional template config.
	Config map[string]ProjectTemplateConfigValue `json:"config,omitempty" yaml:"config,omitempty"`
	// Important indicates the template is important and should be listed by default.
	Important bool `json:"important,omitempty" yaml:"important,omitempty"`
}

// ProjectTemplateConfigValue is a config value included in the project template manifest.
type ProjectTemplateConfigValue struct {
	// Description is an optional description for the config value.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// Default is an optional default value for the config value.
	Default string `json:"default,omitempty" yaml:"default,omitempty"`
	// Secret may be set to true to indicate that the config value should be encrypted.
	Secret bool `json:"secret,omitempty" yaml:"secret,omitempty"`
}

// ProjectBackend is a configuration for backend used by project
type ProjectBackend struct {
	// URL is optional field to explicitly set backend url
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
}

// Project is a Pulumi project manifest.
//
// We explicitly add yaml tags (instead of using the default behavior from https://github.com/ghodss/yaml which works
// in terms of the JSON tags) so we can directly marshall and unmarshall this struct using go-yaml an have the fields
// in the serialized object match the order they are defined in this struct.
//
// TODO[pulumi/pulumi#423]: use DOM based marshalling so we can roundtrip the seralized structure perfectly.
type Project struct {
	// Name is a required fully qualified name.
	Name tokens.PackageName `json:"name" yaml:"name"`
	// Runtime is a required runtime that executes code.
	Runtime ProjectRuntimeInfo `json:"runtime" yaml:"runtime"`
	// Main is an optional override for the program's main entry-point location.
	Main string `json:"main,omitempty" yaml:"main,omitempty"`

	// Description is an optional informational description.
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`
	// Author is an optional author that created this project.
	Author *string `json:"author,omitempty" yaml:"author,omitempty"`
	// Website is an optional website for additional info about this project.
	Website *string `json:"website,omitempty" yaml:"website,omitempty"`
	// License is the optional license governing this project's usage.
	License *string `json:"license,omitempty" yaml:"license,omitempty"`

	// Config indicates where to store the Pulumi.<stack-name>.yaml files, combined with the folder Pulumi.yaml is in.
	Config string `json:"config,omitempty" yaml:"config,omitempty"`

	// Template is an optional template manifest, if this project is a template.
	Template *ProjectTemplate `json:"template,omitempty" yaml:"template,omitempty"`

	// Backend is an optional backend configuration
	Backend *ProjectBackend `json:"backend,omitempty" yaml:"backend,omitempty"`
}

func (proj *Project) Validate() error {
	if proj.Name == "" {
		return errors.New("project is missing a 'name' attribute")
	}
	if proj.Runtime.Name() == "" {
		return errors.New("project is missing a 'runtime' attribute")
	}

	return nil
}

// TrustResourceDependencies returns whether or not this project's runtime can be trusted to accurately report
// dependencies. All languages supported by Pulumi today do this correctly. This option remains useful when bringing
// up new Pulumi languages.
func (proj *Project) TrustResourceDependencies() bool {
	return true
}

// Save writes a project definition to a file.
func (proj *Project) Save(path string) error {
	contract.Require(path != "", "path")
	contract.Require(proj != nil, "proj")
	contract.Requiref(proj.Validate() == nil, "proj", "Validate()")
	return save(path, proj, false /*mkDirAll*/)
}

type PolicyPackProject struct {
	// Runtime is a required runtime that executes code.
	Runtime ProjectRuntimeInfo `json:"runtime" yaml:"runtime"`
	// Version specifies the version of the policy pack. If set, it will override the
	// version specified in `package.json` for Node.js policy packs.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`

	// Main is an optional override for the program's main entry-point location.
	Main string `json:"main,omitempty" yaml:"main,omitempty"`

	// Description is an optional informational description.
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`
	// Author is an optional author that created this project.
	Author *string `json:"author,omitempty" yaml:"author,omitempty"`
	// Website is an optional website for additional info about this project.
	Website *string `json:"website,omitempty" yaml:"website,omitempty"`
	// License is the optional license governing this project's usage.
	License *string `json:"license,omitempty" yaml:"license,omitempty"`
}

func (proj *PolicyPackProject) Validate() error {
	if proj.Runtime.Name() == "" {
		return errors.New("project is missing a 'runtime' attribute")
	}

	return nil
}

// Save writes a project definition to a file.
func (proj *PolicyPackProject) Save(path string) error {
	contract.Require(path != "", "path")
	contract.Require(proj != nil, "proj")
	contract.Requiref(proj.Validate() == nil, "proj", "Validate()")
	return save(path, proj, false /*mkDirAll*/)
}

type PluginProject struct {
	// Runtime is a required runtime that executes code.
	Runtime ProjectRuntimeInfo `json:"runtime" yaml:"runtime"`
}

func (proj *PluginProject) Validate() error {
	if proj.Runtime.Name() == "" {
		return errors.New("project is missing a 'runtime' attribute")
	}

	return nil
}

// ProjectStack holds stack specific information about a project.
type ProjectStack struct {
	// SecretsProvider is this stack's secrets provider.
	SecretsProvider string `json:"secretsprovider,omitempty" yaml:"secretsprovider,omitempty"`
	// EncryptedKey is the KMS-encrypted ciphertext for the data key used for secrets encryption.
	// Only used for cloud-based secrets providers.
	EncryptedKey string `json:"encryptedkey,omitempty" yaml:"encryptedkey,omitempty"`
	// EncryptionSalt is this stack's base64 encoded encryption salt.  Only used for
	// passphrase-based secrets providers.
	EncryptionSalt string `json:"encryptionsalt,omitempty" yaml:"encryptionsalt,omitempty"`
	// Config is an optional config bag.
	Config config.Map `json:"config,omitempty" yaml:"config,omitempty"`
}

// Save writes a project definition to a file.
func (ps *ProjectStack) Save(path string) error {
	contract.Require(path != "", "path")
	contract.Require(ps != nil, "ps")
	return save(path, ps, true /*mkDirAll*/)
}

type ProjectRuntimeInfo struct {
	name    string
	options map[string]interface{}
}

func NewProjectRuntimeInfo(name string, options map[string]interface{}) ProjectRuntimeInfo {
	return ProjectRuntimeInfo{
		name:    name,
		options: options,
	}
}

func (info *ProjectRuntimeInfo) Name() string {
	return info.name
}

func (info *ProjectRuntimeInfo) Options() map[string]interface{} {
	return info.options
}

func (info *ProjectRuntimeInfo) SetOption(key string, value interface{}) {
	if info.options == nil {
		info.options = make(map[string]interface{})
	}
	info.options[key] = value
}

func (info ProjectRuntimeInfo) MarshalYAML() (interface{}, error) {
	if info.options == nil || len(info.options) == 0 {
		return info.name, nil
	}

	return map[string]interface{}{
		"name":    info.name,
		"options": info.options,
	}, nil
}

func (info ProjectRuntimeInfo) MarshalJSON() ([]byte, error) {
	if info.options == nil || len(info.options) == 0 {
		return json.Marshal(info.name)
	}

	return json.Marshal(map[string]interface{}{
		"name":    info.name,
		"options": info.options,
	})
}

func (info *ProjectRuntimeInfo) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &info.name); err == nil {
		return nil
	}

	var payload struct {
		Name    string                 `json:"name"`
		Options map[string]interface{} `json:"options"`
	}

	if err := json.Unmarshal(data, &payload); err == nil {
		info.name = payload.Name
		info.options = payload.Options
		return nil
	}

	return errors.New("runtime section must be a string or an object with name and options attributes")
}

func (info *ProjectRuntimeInfo) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&info.name); err == nil {
		return nil
	}

	var payload struct {
		Name    string                 `yaml:"name"`
		Options map[string]interface{} `yaml:"options"`
	}

	if err := unmarshal(&payload); err == nil {
		info.name = payload.Name
		info.options = payload.Options
		return nil
	}

	return errors.New("runtime section must be a string or an object with name and options attributes")
}

func marshallerForPath(path string) (encoding.Marshaler, error) {
	ext := filepath.Ext(path)
	m, has := encoding.Marshalers[ext]
	if !has {
		return nil, errors.Errorf("no marshaler found for file format '%v'", ext)
	}

	return m, nil
}

func save(path string, value interface{}, mkDirAll bool) error {
	contract.Require(path != "", "path")
	contract.Require(value != nil, "value")

	m, err := marshallerForPath(path)
	if err != nil {
		return err
	}

	b, err := m.Marshal(value)
	if err != nil {
		return err
	}

	if mkDirAll {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
	}

	// Changing the permissions on these file is ~ a breaking change, so disable golint.
	//nolint: gosec
	return ioutil.WriteFile(path, b, 0644)
}
