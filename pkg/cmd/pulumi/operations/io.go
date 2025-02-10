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

package operations

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/newcmd"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// parseAndSaveConfigArray parses the config array and saves it as a config for
// the provided stack.
func parseAndSaveConfigArray(ws pkgWorkspace.Context, s backend.Stack, configArray []string, path bool) error {
	if len(configArray) == 0 {
		return nil
	}
	commandLineConfig, err := newcmd.ParseConfig(configArray, path)
	if err != nil {
		return err
	}

	if err = newcmd.SaveConfig(ws, s, commandLineConfig); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	return nil
}

// readProjectForUpdate attempts to detect and read a Pulumi project for the current workspace. If
// the project is successfully detected and read, it is returned along with the path to its
// containing directory, which will be used as the root of the project's Pulumi program. If a
// client address is present, the returned project will always have the runtime set to "client"
// with the address option set to the client address.
func readProjectForUpdate(ws pkgWorkspace.Context, clientAddress string) (*workspace.Project, string, error) {
	proj, root, err := ws.ReadProject()
	if err != nil {
		return nil, "", err
	}
	if clientAddress != "" {
		proj.Runtime = workspace.NewProjectRuntimeInfo("client", map[string]interface{}{
			"address": clientAddress,
		})
	}
	return proj, root, nil
}
