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

package plugin

import (
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Project wraps a [Context] & [Host] with project specific information.
type Project struct {
	Context
	Host Host   // the host that can be used to fetch providers.
	Pwd  string // the working directory to spawn all plugins in.
	Root string // the root directory of the context.

	// the runtime options for the project, passed to resource providers to support dynamic providers.
	runtimeOptions         map[string]any
	projectName            tokens.PackageName    // name of the project
	disableProviderPreview bool                  // true if provider plugins should disable provider preview
	config                 map[config.Key]string // the configuration map for the stack, if any.
}

// Initialize a new [Project].
//
// NewProject attempts to read the project at root on disk.
func NewProject(ctx Context, host Host, pwd, root string, runtimeOptions map[string]any, disableProviderPreview bool) Project {
	var projectName tokens.PackageName
	projPath, err := workspace.DetectProjectPathFrom(root)
	if err == nil && filepath.Dir(projPath) == root {
		project, err := workspace.LoadProject(projPath)
		if err == nil {
			projectName = project.Name
		}
	}
	return Project{
		Context:                ctx,
		Host:                   host,
		Pwd:                    pwd,
		Root:                   root,
		runtimeOptions:         runtimeOptions,
		projectName:            projectName,
		disableProviderPreview: disableProviderPreview,
		config:                 map[config.Key]string{},
	}
}
