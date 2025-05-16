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

package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packagecmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/policy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkgCmdUtil "github.com/pulumi/pulumi/pkg/v3/util/cmdutil"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func NewInstallCmd() *cobra.Command {
	var reinstall bool
	var noPlugins, noDependencies bool
	var useLanguageVersionTools bool

	cmd := &cobra.Command{
		Use:   "install",
		Args:  cmdutil.NoArgs,
		Short: "Install packages and plugins for the current program or policy pack.",
		Long: "Install packages and plugins for the current program or policy pack.\n" +
			"\n" +
			"This command is used to manually install packages and plugins required by your program or policy pack.\n" +
			"If your Pulumi.yaml file contains a 'packages' section, this command will automatically install\n" +
			"SDKs for all packages declared in that section, similar to the 'pulumi package add' command.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			installPolicyPackDeps, err := shouldInstallPolicyPackDependencies()
			if err != nil {
				return err
			}
			if installPolicyPackDeps {
				// No project found, check if we are in a policy pack project and install the policy
				// pack dependencies if so.
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getting the working directory: %w", err)
				}
				policyPackPath, err := workspace.DetectPolicyPackPathFrom(cwd)
				if err == nil && policyPackPath != "" {
					proj, _, root, err := policy.ReadPolicyProject(policyPackPath)
					if err != nil {
						return err
					}
					return policy.InstallPluginDependencies(ctx, root, proj.Runtime)
				}
			}

			installPluginDeps, err := shouldInstallPluginDependencies()
			if err != nil {
				return err
			}
			if installPluginDeps {
				// No project found, check if we are in a plugin project and install the plugin dependencies if so.
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getting the working directory: %w", err)
				}
				pluginPath, err := workspace.DetectPluginPathFrom(cwd)
				if err == nil && pluginPath != "" {
					proj, err := workspace.LoadPluginProject(pluginPath)
					if err != nil {
						return err
					}
					return policy.InstallPluginDependencies(ctx, filepath.Dir(pluginPath), proj.Runtime)
				}
			}

			// Load the project
			proj, root, err := pkgWorkspace.Instance.ReadProject()
			if err != nil {
				return err
			}

			span := opentracing.SpanFromContext(ctx)
			projinfo := &engine.Projinfo{Proj: proj, Root: root}
			pwd, main, pctx, err := engine.ProjectInfoContext(
				projinfo,
				nil,
				cmdutil.Diag(),
				cmdutil.Diag(),
				nil,
				false,
				span,
				nil,
			)
			if err != nil {
				return err
			}

			defer pctx.Close()

			// Process packages section from Pulumi.yaml. Do so before installing language-specific dependencies,
			// so that the SDKs folder is present and references to it from package.json etc are valid.
			if err := installPackagesFromProject(pctx, proj, root); err != nil {
				return fmt.Errorf("installing `packages` from Pulumi.yaml: %w", err)
			}

			// First make sure the language plugin is present.  We need this to load the required resource plugins.
			// TODO: we need to think about how best to version this.  For now, it always picks the latest.
			runtime := proj.Runtime
			programInfo := plugin.NewProgramInfo(pctx.Root, pwd, main, runtime.Options())
			lang, err := pctx.Host.LanguageRuntime(runtime.Name(), programInfo)
			if err != nil {
				return fmt.Errorf("load language plugin %s: %w", runtime.Name(), err)
			}

			if !noDependencies {
				err = pkgCmdUtil.InstallDependencies(lang, plugin.InstallDependenciesRequest{
					Info:                    programInfo,
					UseLanguageVersionTools: useLanguageVersionTools,
					IsPlugin:                false,
				})
				if err != nil {
					return fmt.Errorf("installing dependencies: %w", err)
				}
			}

			if !noPlugins {
				// Compute the set of plugins the current project needs.
				packages, err := lang.GetRequiredPackages(programInfo)
				if err != nil {
					return err
				}

				pluginSet := engine.NewPluginSet()
				for _, pkg := range packages {
					pluginSet.Add(pkg.PluginSpec)
				}

				if err = engine.EnsurePluginsAreInstalled(ctx, nil, pctx.Diag, pluginSet,
					pctx.Host.GetProjectPlugins(), reinstall, true); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.PersistentFlags().BoolVar(&reinstall,
		"reinstall", false, "Reinstall a plugin even if it already exists")
	cmd.PersistentFlags().BoolVar(&noPlugins,
		"no-plugins", false, "Skip installing plugins")
	cmd.PersistentFlags().BoolVar(&noDependencies,
		"no-dependencies", false, "Skip installing dependencies")
	cmd.PersistentFlags().BoolVar(&useLanguageVersionTools,
		"use-language-version-tools", false, "Use language version tools to setup and install the language runtime")

	return cmd
}

// installPackagesFromProject processes packages specified in the Pulumi.yaml file
// and installs them using similar logic to the 'pulumi package add' command
func installPackagesFromProject(pctx *plugin.Context, proj *workspace.Project, root string) error {
	packages := proj.GetPackageSpecs()
	if len(packages) == 0 {
		return nil
	}

	fmt.Println("Installing packages defined in Pulumi.yaml...")

	for name, packageSpec := range packages {
		fmt.Printf("Installing package '%s'...\n", name)

		installSource := packageSpec.Source
		if !plugin.IsLocalPluginPath(installSource) && packageSpec.Version != "" {
			installSource = fmt.Sprintf("%s@%s", installSource, packageSpec.Version)
		}

		_, err := packagecmd.InstallPackage(
			pkgWorkspace.Instance, pctx, proj.Runtime.Name(), root, installSource, packageSpec.Parameters)
		if err != nil {
			return fmt.Errorf("failed to install package '%s': %w", name, err)
		}

		fmt.Printf("Package '%s' installed successfully\n", name)
	}

	return nil
}

func shouldInstallPolicyPackDependencies() (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("getting the working directory: %w", err)
	}
	policyPackPath, err := workspace.DetectPolicyPackPathFrom(cwd)
	if err != nil {
		return false, fmt.Errorf("detecting policy pack path: %w", err)
	}
	if policyPackPath != "" {
		// There's a PulumiPolicy.yaml in cwd or a parent folder. The policy pack might be nested
		// within a project, or vice-vera, so we need to check if there's a Pulumi.yaml in a parent
		// folder.
		projectPath, err := workspace.DetectProjectPathFrom(cwd)
		if err != nil {
			if errors.Is(err, workspace.ErrProjectNotFound) {
				// No project found, we should install the dependencies for the policy pack.
				return true, nil
			}
			return false, fmt.Errorf("detecting project path: %w", err)
		}
		// We have both a project and a policy pack. If the project path is a parent of the policy
		// pack path, we should install dependencies for the policy pack, otherwise we should
		// install dependencies for the project.
		baseProjectPath := filepath.Dir(projectPath)
		basePolicyPackPath := filepath.Dir(policyPackPath)
		return strings.Contains(basePolicyPackPath, baseProjectPath), nil
	}
	return false, nil
}

func shouldInstallPluginDependencies() (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("getting the working directory: %w", err)
	}
	pluginPath, err := workspace.DetectPluginPathFrom(cwd)
	if err != nil {
		return false, fmt.Errorf("detecting policy pack path: %w", err)
	}
	if pluginPath != "" {
		// There's a PulumiPlugin.yaml in cwd or a parent folder. The plugin might be nested
		// within a project, or vice-vera, so we need to check if there's a Pulumi.yaml in a parent
		// folder.
		projectPath, err := workspace.DetectProjectPathFrom(cwd)
		if err != nil {
			if errors.Is(err, workspace.ErrProjectNotFound) {
				// No project found, we should install the dependencies for the policy pack.
				return true, nil
			}
			return false, fmt.Errorf("detecting project path: %w", err)
		}
		// We have both a project and a policy pack. If the project path is a parent of the policy
		// pack path, we should install dependencies for the policy pack, otherwise we should
		// install dependencies for the project.
		baseProjectPath := filepath.Dir(projectPath)
		basePluginPath := filepath.Dir(pluginPath)
		return strings.Contains(basePluginPath, baseProjectPath), nil
	}
	return false, nil
}
