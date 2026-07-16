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

package adder

import (
	"errors"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

type projectInfo struct {
	project *workspace.Project
	root    string
}

// Project resolves the project in the current working directory. It returns a
// nil project (and no error) when there isn't one.
func (e Environment) Project(cmd *cobra.Command) (*workspace.Project, string, error) {
	p, err := bagFrom(cmd).project.get(func() (projectInfo, error) {
		e := e.defaults(cmd)
		cwd, err := os.Getwd()
		if err != nil {
			return projectInfo{}, err
		}
		project, root, err := e.WS.ReadProject(cwd)
		if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
			return projectInfo{}, err
		}
		return projectInfo{project: project, root: root}, nil
	})
	return p.project, p.root, err
}
