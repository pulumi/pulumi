// Copyright 2016-2021, Pulumi Corporation.
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
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// projectSingleton is a singleton instance of projectLoader, which controls a global map of instances of Project
// configs (one per path).
var projectSingleton *projectLoader = &projectLoader{
	internal: map[string]*Project{},
}

// projectStackSingleton is a singleton instance of projectStackLoader, which controls a global map of instances of
// ProjectStack configs (one per path).
var projectStackSingleton *projectStackLoader = &projectStackLoader{
	internal: map[string]*ProjectStack{},
}

// pluginProjectSingleton is a singleton instance of pluginProjectLoader, which controls a global map of instances of
// PluginProject configs (one per path).
var pluginProjectSingleton *pluginProjectLoader = &pluginProjectLoader{
	internal: map[string]*PluginProject{},
}

// policyPackProjectSingleton is a singleton instance of policyPackProjectLoader, which controls a global map of
// instances of PolicyPackProject configs (one per path).
var policyPackProjectSingleton *policyPackProjectLoader = &policyPackProjectLoader{
	internal: map[string]*PolicyPackProject{},
}

// readFileStripUTF8BOM wraps ioutil.ReadFile and also strips the UTF-8 Byte-order Mark (BOM) if present.
func readFileStripUTF8BOM(path string) ([]byte, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Strip UTF-8 BOM bytes if present to avoid problems with downstream parsing.
	// References:
	//   https://github.com/spkg/bom
	//   https://en.wikipedia.org/wiki/Byte_order_mark
	if len(b) >= 3 &&
		b[0] == 0xef &&
		b[1] == 0xbb &&
		b[2] == 0xbf {
		b = b[3:]
	}

	return b, nil
}

// projectLoader is used to load a single global instance of a Project config.
type projectLoader struct {
	sync.RWMutex
	internal map[string]*Project
}

// Load a Project config file from the specified path. The configuration will be cached for subsequent loads.
func (singleton *projectLoader) load(path string) (*Project, error) {
	singleton.Lock()
	defer singleton.Unlock()

	if v, ok := singleton.internal[path]; ok {
		return v, nil
	}

	marshaller, err := marshallerForPath(path)
	if err != nil {
		return nil, err
	}

	b, err := readFileStripUTF8BOM(path)
	if err != nil {
		return nil, err
	}

	var project Project
	err = marshaller.Unmarshal(b, &project)
	if err != nil {
		return nil, err
	}

	err = project.Validate()
	if err != nil {
		return nil, err
	}

	singleton.internal[path] = &project
	return &project, nil
}

// projectStackLoader is used to load a single global instance of a ProjectStack config.
type projectStackLoader struct {
	sync.RWMutex
	internal map[string]*ProjectStack
}

// Load a ProjectStack config file from the specified path. The configuration will be cached for subsequent loads.
func (singleton *projectStackLoader) load(path string) (*ProjectStack, error) {
	singleton.Lock()
	defer singleton.Unlock()

	if v, ok := singleton.internal[path]; ok {
		return v, nil
	}

	marshaler, err := marshallerForPath(path)
	if err != nil {
		return nil, err
	}

	var projectStack ProjectStack
	b, err := readFileStripUTF8BOM(path)
	if os.IsNotExist(err) {
		projectStack = ProjectStack{
			Config: make(config.Map),
		}
		singleton.internal[path] = &projectStack
		return &projectStack, nil
	} else if err != nil {
		return nil, err
	}

	err = marshaler.Unmarshal(b, &projectStack)
	if err != nil {
		return nil, err
	}

	if projectStack.Config == nil {
		projectStack.Config = make(config.Map)
	}

	singleton.internal[path] = &projectStack
	return &projectStack, nil
}

// pluginProjectLoader is used to load a single global instance of a PluginProject config.
type pluginProjectLoader struct {
	sync.RWMutex
	internal map[string]*PluginProject
}

// Load a PluginProject config file from the specified path. The configuration will be cached for subsequent loads.
func (singleton *pluginProjectLoader) load(path string) (*PluginProject, error) {
	singleton.Lock()
	defer singleton.Unlock()

	if result, ok := singleton.internal[path]; ok {
		return result, nil
	}

	marshaller, err := marshallerForPath(path)
	if err != nil {
		return nil, err
	}

	b, err := readFileStripUTF8BOM(path)
	if err != nil {
		return nil, err
	}

	var pluginProject PluginProject
	err = marshaller.Unmarshal(b, &pluginProject)
	if err != nil {
		return nil, err
	}

	err = pluginProject.Validate()
	if err != nil {
		return nil, err
	}

	singleton.internal[path] = &pluginProject
	return &pluginProject, nil
}

// policyPackProjectLoader is used to load a single global instance of a PolicyPackProject config.
type policyPackProjectLoader struct {
	sync.RWMutex
	internal map[string]*PolicyPackProject
}

// Load a PolicyPackProject config file from the specified path. The configuration will be cached for subsequent loads.
func (singleton *policyPackProjectLoader) load(path string) (*PolicyPackProject, error) {
	singleton.Lock()
	defer singleton.Unlock()

	if result, ok := singleton.internal[path]; ok {
		return result, nil
	}

	marshaller, err := marshallerForPath(path)
	if err != nil {
		return nil, err
	}

	b, err := readFileStripUTF8BOM(path)
	if err != nil {
		return nil, err
	}

	var policyPackProject PolicyPackProject
	err = marshaller.Unmarshal(b, &policyPackProject)
	if err != nil {
		return nil, err
	}

	err = policyPackProject.Validate()
	if err != nil {
		return nil, err
	}

	singleton.internal[path] = &policyPackProject
	return &policyPackProject, nil
}

// LoadProject reads a project definition from a file.
func LoadProject(path string) (*Project, error) {
	contract.Require(path != "", "path")

	return projectSingleton.load(path)
}

// LoadProjectStack reads a stack definition from a file.
func LoadProjectStack(path string) (*ProjectStack, error) {
	contract.Require(path != "", "path")

	return projectStackSingleton.load(path)
}

// LoadPluginProject reads a plugin project definition from a file.
func LoadPluginProject(path string) (*PluginProject, error) {
	contract.Require(path != "", "path")

	return pluginProjectSingleton.load(path)
}

// LoadPolicyPack reads a policy pack definition from a file.
func LoadPolicyPack(path string) (*PolicyPackProject, error) {
	contract.Require(path != "", "path")

	return policyPackProjectSingleton.load(path)
}
