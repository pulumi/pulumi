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

// Project is a Pulumi project manifest..
//
// We explicitly add yaml tags (instead of using the default behavior from https://github.com/ghodss/yaml which works
// in terms of the JSON tags) so we can directly marshall and unmarshall this struct using go-yaml an have the fields
// in the serialized object match the order they are defined in this struct.
//
// TODO[pulumi/pulumi#423]: use DOM based marshalling so we can roundtrip the seralized structure perfectly.
// nolint: lll
type Project struct {
	Name    tokens.PackageName `json:"name" yaml:"name"`                     // a required fully qualified name.
	Runtime string             `json:"runtime" yaml:"runtime"`               // a required runtime that executes code.
	Main    string             `json:"main,omitempty" yaml:"main,omitempty"` // an optional override for the main program location.

	Description *string `json:"description,omitempty" yaml:"description,omitempty"` // an optional informational description.
	Author      *string `json:"author,omitempty" yaml:"author,omitempty"`           // an optional author.
	Website     *string `json:"website,omitempty" yaml:"website,omitempty"`         // an optional website for additional info.
	License     *string `json:"license,omitempty" yaml:"license,omitempty"`         // an optional license governing this project's usage.

	Analyzers *Analyzers `json:"analyzers,omitempty" yaml:"analyzers,omitempty"` // any analyzers enabled for this project.

	EncryptionSaltDeprecated string `json:"encryptionsalt,omitempty" yaml:"encryptionsalt,omitempty"`     // base64 encoded encryption salt.
	Context                  string `json:"context,omitempty" yaml:"context,omitempty"`                   // an optional path (combined with the on disk location of Pulumi.yaml) to control the data uploaded to the service.
	NoDefaultIgnores         *bool  `json:"nodefaultignores,omitempty" yaml:"nodefaultignores,omitempty"` // true if we should only respect .pulumiignore when archiving

	ConfigDeprecated map[config.Key]config.Value `json:"config,omitempty" yaml:"config,omitempty"` // optional config (applies to all stacks).

	StacksDeprecated map[tokens.QName]ProjectStack `json:"stacks,omitempty" yaml:"stacks,omitempty"` // optional stack specific information.
}

func (proj *Project) Validate() error {
	if proj.Name == "" {
		return errors.New("project is missing a 'name' attribute")
	}
	if proj.Runtime == "" {
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

// Save writes a project definition to a file.
func (proj *Project) Save(path string) error {
	contract.Require(path != "", "path")
	contract.Require(proj != nil, "proj")
	contract.Requiref(proj.Validate() == nil, "proj", "Validate()")

	for name, info := range proj.StacksDeprecated {
		if info.isEmpty() {
			delete(proj.StacksDeprecated, name)
		}
	}

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

// isEmpty returns True if this object contains no information (i.e. all members have their zero values)
func (ps *ProjectStack) isEmpty() bool {
	return len(ps.Config) == 0 && ps.EncryptionSalt == ""
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

	return ioutil.WriteFile(path, b, 0644)
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
