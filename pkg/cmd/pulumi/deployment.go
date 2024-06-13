// Copyright 2016-2024, Pulumi Corporation.
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

package main

import (
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var stackDeploymentConfigFile string

func loadProjectStackDeployment(stack backend.Stack) (*workspace.ProjectStackDeployment, error) {
	if stackDeploymentConfigFile == "" {
		return workspace.DetectProjectStackDeployment(stack.Ref().Name().Q())
	}
	return workspace.LoadProjectStackDeployment(stackDeploymentConfigFile)
}

func saveProjectStackDeployment(psd *workspace.ProjectStackDeployment, stack backend.Stack) error {
	if stackDeploymentConfigFile == "" {
		return workspace.SaveProjectStackDeployment(stack.Ref().Name().Q(), psd)
	}
	return psd.Save(stackDeploymentConfigFile)
}
