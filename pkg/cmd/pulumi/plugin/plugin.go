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

package plugin

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func NewPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage language and resource provider plugins",
		Long: "Manage language and resource provider plugins.\n" +
			"\n" +
			"Pulumi uses dynamically loaded plugins as an extensibility mechanism for\n" +
			"supporting any number of languages and resource providers.  These plugins are\n" +
			"distributed out of band and must be installed manually.  Attempting to use a\n" +
			"package that provisions resources without the corresponding plugin will fail.\n" +
			"\n" +
			"You may write your own plugins, for example to implement custom languages or\n" +
			"resources, although most people will never need to do this.  To understand how to\n" +
			"write and distribute your own plugins, please consult the relevant documentation.\n" +
			"\n" +
			"The plugin family of commands provides a way of explicitly managing plugins.\n" +
			"\n" +
			"For a list of available resource plugins, please see https://www.pulumi.com/registry/.",
		Args: cmdutil.NoArgs,
	}

	cmd.AddCommand(newPluginInstallCmd())
	cmd.AddCommand(newPluginLsCmd())
	cmd.AddCommand(newPluginRmCmd())
	cmd.AddCommand(newPluginRunCmd())

	return cmd
}

// getProjectPlugins fetches a list of plugins used by this project.
func getProjectPlugins() ([]workspace.PluginSpec, error) {
	proj, root, err := pkgWorkspace.Instance.ReadProject()
	if err != nil {
		return nil, err
	}

	projinfo := &engine.Projinfo{Proj: proj, Root: root}
	pwd, main, ctx, err := engine.ProjectInfoContext(projinfo, nil, cmdutil.Diag(), cmdutil.Diag(), nil, false, nil, nil)
	if err != nil {
		return nil, err
	}

	defer ctx.Close()
	runtimeOptions := proj.Runtime.Options()
	programInfo := plugin.NewProgramInfo(root, pwd, main, runtimeOptions)
	// Get the required plugins and then ensure they have metadata populated about them.  Because it's possible
	// a plugin required by the project hasn't yet been installed, we will simply skip any errors we encounter.
	plugins, err := engine.GetRequiredPlugins(
		ctx.Host,
		proj.Runtime.Name(),
		programInfo)
	if err != nil {
		return nil, err
	}
	return plugins, nil
}

func resolvePlugins(plugins []workspace.PluginSpec) ([]workspace.PluginInfo, error) {
	proj, root, err := pkgWorkspace.Instance.ReadProject()
	if err != nil {
		return nil, err
	}

	d := cmdutil.Diag()

	projinfo := &engine.Projinfo{Proj: proj, Root: root}
	_, _, ctx, err := engine.ProjectInfoContext(projinfo, nil, d, d, nil, false, nil, nil)
	if err != nil {
		return nil, err
	}

	defer ctx.Close()

	// Get the required plugins and then ensure they have metadata populated about them.  Because it's possible
	// a plugin required by the project hasn't yet been installed, we will simply skip any errors we encounter.
	var results []workspace.PluginInfo
	for _, plugin := range plugins {
		info, err := workspace.GetPluginInfo(ctx.Base(), d, plugin, ctx.Host.GetProjectPlugins())
		if err != nil {
			contract.IgnoreError(err)
		}
		if info != nil {
			results = append(results, *info)
		}
	}
	return results, nil
}
