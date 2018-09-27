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

	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Analyzers is a list of analyzers to run on this project.
type Analyzers []tokens.QName

// ProjectTemplate is a Pulumi project template manifest.
// nolint: lll
type ProjectTemplate struct {
	Description string                                `json:"description,omitempty" yaml:"description,omitempty"` // an optional description of the template.
	Quickstart  string                                `json:"quickstart,omitempty" yaml:"quickstart,omitempty"`   // optional text to be displayed after template creation.
	Config      map[string]ProjectTemplateConfigValue `json:"config,omitempty" yaml:"config,omitempty"`           // optional template config.
}

// ProjectTemplateConfigValue is a config value included in the project template manifest.
// nolint: lll
type ProjectTemplateConfigValue struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"` // an optional description for the config value.
	Default     string `json:"default,omitempty" yaml:"default,omitempty"`         // an optional default value for the config value.
	Secret      bool   `json:"secret,omitempty" yaml:"secret,omitempty"`           // an optional value indicating whether the config value should be encrypted.
}

// Project is a Pulumi project manifest..
//
// We explicitly add yaml tags (instead of using the default behavior from https://github.com/ghodss/yaml which works
// in terms of the JSON tags) so we can directly marshall and unmarshall this struct using go-yaml an have the fields
// in the serialized object match the order they are defined in this struct.
//
// TODO[pulumi/pulumi#423]: use DOM based marshalling so we can roundtrip the seralized structure perfectly.
// nolint: lll
type Project struct {
	Name        tokens.PackageName `json:"name" yaml:"name"`                     // a required fully qualified name.
	RuntimeInfo ProjectRuntimeInfo `json:"runtime" yaml:"runtime"`               // a required runtime that executes code.
	Main        string             `json:"main,omitempty" yaml:"main,omitempty"` // an optional override for the main program location.

	Description *string `json:"description,omitempty" yaml:"description,omitempty"` // an optional informational description.
	Author      *string `json:"author,omitempty" yaml:"author,omitempty"`           // an optional author.
	Website     *string `json:"website,omitempty" yaml:"website,omitempty"`         // an optional website for additional info.
	License     *string `json:"license,omitempty" yaml:"license,omitempty"`         // an optional license governing this project's usage.

	Analyzers *Analyzers `json:"analyzers,omitempty" yaml:"analyzers,omitempty"` // any analyzers enabled for this project.

	Context          string `json:"context,omitempty" yaml:"context,omitempty"`                   // an optional path (combined with the on disk location of Pulumi.yaml) to control the data uploaded to the service.
	NoDefaultIgnores *bool  `json:"nodefaultignores,omitempty" yaml:"nodefaultignores,omitempty"` // true if we should only respect .pulumiignore when archiving

	Config string `json:"config,omitempty" yaml:"config,omitempty"` // where to store Pulumi.<stack-name>.yaml files, this is combined with the folder Pulumi.yaml is in.

	Template *ProjectTemplate `json:"template,omitempty" yaml:"template,omitempty"` // optional template manifest.
}

func (proj *Project) Validate() error {
	if proj.Name == "" {
		return errors.New("project is missing a 'name' attribute")
	}
	if proj.RuntimeInfo.Name() == "" {
		return errors.New("project is missing a 'runtime' attribute")
	}

	return nil
}

func (proj *Project) UseDefaultIgnores() bool {
	if proj.NoDefaultIgnores == nil {
		return true
	}

	return !(*proj.NoDefaultIgnores)
}

// TrustResourceDependencies returns whether or not this project's runtime can be trusted to accurately report
// dependencies. Not all languages supported by Pulumi do this correctly.
func (proj *Project) TrustResourceDependencies() bool {
	return proj.RuntimeInfo.Name() != "python"
}

// Save writes a project definition to a file.
func (proj *Project) Save(path string) error {
	contract.Require(path != "", "path")
	contract.Require(proj != nil, "proj")
	contract.Requiref(proj.Validate() == nil, "proj", "Validate()")

	m, err := marshallerForPath(path)
	if err != nil {
		return err
	}

	b, err := m.Marshal(proj)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, b, 0644)
}

// ProjectStack holds stack specific information about a project.
// nolint: lll
type ProjectStack struct {
	EncryptionSalt string     `json:"encryptionsalt,omitempty" yaml:"encryptionsalt,omitempty"` // base64 encoded encryption salt.
	Config         config.Map `json:"config,omitempty" yaml:"config,omitempty"`                 // optional config.
}

// Save writes a project definition to a file.
func (ps *ProjectStack) Save(path string) error {
	contract.Require(path != "", "path")
	contract.Require(ps != nil, "ps")

	m, err := marshallerForPath(path)
	if err != nil {
		return err
	}

	b, err := m.Marshal(ps)
	if err != nil {
		return err
	}

	// nolint: gas, gas prefers 0700 for a directory, but 0755 (so group and world can read it) is what we prefer
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return ioutil.WriteFile(path, b, 0644)
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

// LoadProject reads a project definition from a file.
func LoadProject(path string) (*Project, error) {
	contract.Require(path != "", "path")

	m, err := marshallerForPath(path)
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var proj Project
	err = m.Unmarshal(b, &proj)
	if err != nil {
		return nil, err
	}

	err = proj.Validate()
	if err != nil {
		return nil, err
	}

	return &proj, err
}

// LoadProjectStack reads a stack definition from a file.
func LoadProjectStack(path string) (*ProjectStack, error) {
	contract.Require(path != "", "path")

	m, err := marshallerForPath(path)
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return &ProjectStack{
			Config: make(config.Map),
		}, nil
	} else if err != nil {
		return nil, err
	}

	var ps ProjectStack
	err = m.Unmarshal(b, &ps)
	if err != nil {
		return nil, err
	}

	if ps.Config == nil {
		ps.Config = make(config.Map)
	}

	return &ps, err
}

func marshallerForPath(path string) (encoding.Marshaler, error) {
	ext := filepath.Ext(path)
	m, has := encoding.Marshalers[ext]
	if !has {
		return nil, errors.Errorf("no marshaler found for file format '%v'", ext)
	}

	return m, nil
}
