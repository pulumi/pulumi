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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy/unauthenticatedregistry"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdDiag "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/diag"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/policy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkgCmdUtil "github.com/pulumi/pulumi/pkg/v3/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/v3/util/pdag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func NewInstallCmd(ws pkgWorkspace.Context) *cobra.Command {
	var reinstall bool
	var noPlugins, noDependencies bool
	var useLanguageVersionTools bool
	var parallel int

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
					// We're in a plugin. First install the packages specified
					// in the plugin project file, and then install the plugin's
					// dependencies.

					proj, err := workspace.LoadPluginProject(pluginPath)
					if err != nil {
						return err
					}

					pctx, err := plugin.NewContextWithRoot(ctx,
						cmdutil.Diag(),
						cmdutil.Diag(),
						nil, // host
						cwd, // pwd
						cwd, // rot
						proj.Runtime.Options(),
						false, // disableProviderPreview
						nil,   // tracingSpan
						nil,   // Plugins
						proj.GetPackageSpecs(),
						nil, // config
						nil, // debugging
					)
					if err != nil {
						return err
					}

					// Cloud registry is linked to a backend, but we don't have
					// one available in a plugin. Use the unauthenticated
					// registry.
					reg := unauthenticatedregistry.New(cmdutil.Diag(), env.Global())

					if err := installPackagesFromProject(pctx.Base(), proj, cwd, reg, parallel,
						cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
						return fmt.Errorf("installing `packages` from PulumiPlugin.yaml: %w", err)
					}

					return policy.InstallPluginDependencies(ctx, filepath.Dir(pluginPath), proj.Runtime)
				}
			}

			// Load the project
			proj, root, err := ws.ReadProject()
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
			if err := installPackagesFromProject(cmd.Context(), proj, root,
				cmdCmd.NewDefaultRegistry(cmd.Context(), pkgWorkspace.Instance, proj, cmdutil.Diag(), env.Global()),
				parallel, cmd.OutOrStdout(), cmd.ErrOrStderr(),
			); err != nil {
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
				}, cmd.OutOrStdout(), cmd.ErrOrStderr())
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

	cmd.Flags().IntVar(&parallel,
		"parallel", 4, "The max number of concurrent installs to perform. "+
			"Parallelism of less then 1 implies unbounded parallelism")
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
func installPackagesFromProject(
	ctx context.Context, proj workspace.BaseProject, root string, registry registry.Registry,
	parallelism int,
	stdout, stderr io.Writer,
) error {
	pkgs := proj.GetPackageSpecs()
	if len(pkgs) == 0 {
		return nil
	}

	fmt.Println("Installing packages...")

	type node struct {
		name        string
		packageSpec func(context.Context) error
	}

	installPackage := func(
		cwd, name string, proj workspace.BaseProject, packageSpec workspace.PackageSpec,
	) func(context.Context) error {
		return func(ctx context.Context) error {
			fmt.Printf("Installing package '%s'...\n", name)

			pctx, err := plugin.NewContextWithRoot(ctx,
				cmdutil.Diag(),
				cmdutil.Diag(),
				nil, // host
				cwd, // pwd
				cwd, // root
				proj.RuntimeInfo().Options(),
				false, // disableProviderPreview
				nil,   // tracingSpan
				nil,   // Plugins
				proj.GetPackageSpecs(),
				nil, // config
				nil, // debugging
			)
			if err != nil {
				return err
			}

			installSource := packageSpec.Source
			if !plugin.IsLocalPluginPath(ctx, installSource) && packageSpec.Version != "" {
				installSource = fmt.Sprintf("%s@%s", installSource, packageSpec.Version)
			}

			parameters := &plugin.ParameterizeArgs{Args: packageSpec.Parameters}
			_, _, diags, err := packages.InstallPackage(
				proj, pctx, proj.RuntimeInfo().Name(), pctx.Root, installSource, parameters, registry)
			cmdDiag.PrintDiagnostics(pctx.Diag, diags)
			if err != nil {
				return errors.Join(
					fmt.Errorf("failed to install package '%s': %w", name, err),
					pctx.Close(),
				)
			}

			fmt.Printf("Package '%s' installed successfully\n", name)
			return pctx.Close()
		}
	}

	installPlugin := func(path string, proj *workspace.PluginProject) func(context.Context) error {
		return func(ctx context.Context) error {
			pctx, err := plugin.NewContextWithRoot(ctx,
				cmdutil.Diag(),
				cmdutil.Diag(),
				nil,  // host
				path, // pwd
				path, // root
				proj.RuntimeInfo().Options(),
				false, // disableProviderPreview
				nil,   // tracingSpan
				nil,   // Plugins
				proj.GetPackageSpecs(),
				nil, // config
				nil, // debugging
			)
			if err != nil {
				return err
			}

			if err := pkgWorkspace.InstallPluginAtPath(pctx, proj, stdout, stderr); err != nil {
				return errors.Join(fmt.Errorf("installing at '%s': %w", pctx.Pwd, err), pctx.Close())
			}
			return pctx.Close()
		}
	}

	var wg pdag.DAG[node]
	seen := map[string]pdag.Node{}
	var findPlugins func(root pdag.Node, cwd string, proj workspace.BaseProject) error
	findPlugins = func(root pdag.Node, cwd string, proj workspace.BaseProject) error {
		for name, packageSpec := range proj.GetPackageSpecs() {
			var pluginInstall *pdag.Node
			if plugin.IsLocalPluginPath(ctx, packageSpec.Source) {
				// If the package is a local spec, then we need to install it and the
				// packages that it depends on.
				pluginYaml := filepath.Join(packageSpec.Source, "PulumiPlugin.yaml")
				pluginProject, err := workspace.LoadPluginProject(pluginYaml)
				if err != nil {
					return fmt.Errorf("Failed to load plugin project '%s': %w", name, err)
				}
				absPluginSource, err := filepath.Abs(packageSpec.Source)
				if err != nil {
					return err
				}

				if n, ok := seen[absPluginSource]; ok {
					pluginInstall = &n
				} else {
					pkg := wg.NewNode(node{name, installPlugin(absPluginSource, pluginProject)})
					if err := wg.NewEdge(pkg, root); err != nil {
						return err
					}
					pluginInstall = &pkg
					seen[absPluginSource] = pkg
					if err := findPlugins(pkg, packageSpec.Source, pluginProject); err != nil {
						return err
					}
				}
			}

			installPkg := wg.NewNode(node{name, installPackage(cwd, name, proj, packageSpec)})
			if pluginInstall != nil {
				if err := wg.NewEdge(*pluginInstall, installPkg); err != nil {
					return err
				}
			}
			// Ensure that we install this package before we install the plugin.
			if err := wg.NewEdge(installPkg, root); err != nil {
				return err
			}
		}
		return nil
	}

	// Search for plugins
	if err := findPlugins(
		wg.NewNode(node{name: "root", packageSpec: func(context.Context) error { return nil }}),
		root,
		proj,
	); err != nil {
		var cycle pdag.ErrorCycle[node]
		if errors.As(err, &cycle) {
			cyclePath := make([]string, len(cycle.Cycle))
			for i, n := range cycle.Cycle {
				cyclePath[i] = n.name
			}
			return fmt.Errorf("Cycle found: %s", strings.Join(cyclePath, " -> "))
		}
		return err
	}

	return wg.Walk(ctx, func(ctx context.Context, f node) error {
		return f.packageSpec(ctx)
	}, pdag.MaxProcs(parallelism))
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
		return false, fmt.Errorf("detecting plugin path: %w", err)
	}
	if pluginPath != "" {
		// There's a PulumiPlugin.yaml in cwd or a parent folder. The plugin might be nested
		// within a project, or vice-vera, so we need to check if there's a Pulumi.yaml in a parent
		// folder.
		projectPath, err := workspace.DetectProjectPathFrom(cwd)
		if err != nil {
			if errors.Is(err, workspace.ErrProjectNotFound) {
				// No project found, we should install the dependencies for the plugin.
				return true, nil
			}
			return false, fmt.Errorf("detecting project path: %w", err)
		}
		// We have both a project and a plugin. If the project path is a parent of the plugin
		// path, we should install dependencies for the plugin, otherwise we should
		// install dependencies for the project.
		baseProjectPath := filepath.Dir(projectPath)
		basePluginPath := filepath.Dir(pluginPath)
		return strings.Contains(basePluginPath, baseProjectPath), nil
	}
	return false, nil
}
