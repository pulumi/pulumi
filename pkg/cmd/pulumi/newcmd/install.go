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

package newcmd

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// InstallDependencies will install dependencies for the project, e.g. by running `npm install` for nodejs projects.
func InstallDependencies(ctx *plugin.Context, runtime *workspace.ProjectRuntimeInfo, main string) error {
	// First make sure the language plugin is present.  We need this to load the required resource plugins.
	// TODO: we need to think about how best to version this.  For now, it always picks the latest.
	programInfo := plugin.NewProgramInfo(ctx.Root, ctx.Pwd, main, runtime.Options())
	lang, err := ctx.Host.LanguageRuntime(runtime.Name(), programInfo)
	if err != nil {
		return fmt.Errorf("failed to load language plugin %s: %w", runtime.Name(), err)
	}

	err = cmdutil.InstallDependencies(lang, plugin.InstallDependenciesRequest{
		Info:     programInfo,
		IsPlugin: false,
	})
	if err != nil {
		//revive:disable-next-line:error-strings // This error message is user facing.
		return fmt.Errorf("installing dependencies failed: %w\nRun `pulumi install` to complete the installation.", err)
	}

	return nil
}
