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
	"sync"
	"sync/atomic"

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
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func NewInstallCmd(ws pkgWorkspace.Context) *cobra.Command {
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

					if err := walkLocalPackagesFromProject[workspace.BaseProject](pctx, proj, installPackageDependency(reg), nil,
						func(path string) (workspace.BaseProject, error) {
							p, err := workspace.LoadProject(path)
							if err != nil {
								return nil, err
							}
							return p, nil
						}); err != nil {
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
			_, _, pctx, err := engine.ProjectInfoContext(
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

			registry := cmdCmd.NewDefaultRegistry(cmd.Context(), pkgWorkspace.Instance, proj, cmdutil.Diag(), env.Global())

			var hasDisplayedInstallingPackages atomic.Bool
			return walkLocalPackagesFromProject(pctx, proj, func(
				pctx *plugin.Context, proj workspace.BaseProject, pkgName string, packageSpec workspace.PackageSpec,
			) error {
				// Print that we are installing packages, if we have not printed it before.
				if !hasDisplayedInstallingPackages.Swap(true) {
					fmt.Println("Installing packages...")
				}
				fmt.Printf("Installing package '%s'...\n", pkgName)
				err := installPackageDependency(registry)(pctx, proj, pkgName, packageSpec)
				if err != nil {
					return err
				}
				fmt.Printf("Package '%s' installed successfully\n", pkgName)
				return nil
			}, func(pctx *plugin.Context, proj *workspace.Project) error {
				return installForProject(pctx, proj, noDependencies, useLanguageVersionTools, noPlugins, reinstall)
			}, workspace.LoadProject)
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

func installForProject(
	pctx *plugin.Context, proj *workspace.Project,
	noDependencies, useLanguageVersionTools, noPlugins, reinstall bool,
) error {
	// First make sure the language plugin is present.  We need this to load the required resource plugins.
	// TODO: we need to think about how best to version this.  For now, it always picks the latest.
	runtime := proj.RuntimeInfo()
	projinfo := &engine.Projinfo{Proj: proj, Root: pctx.Root}
	pwd, main, err := projinfo.GetPwdMain()
	if err != nil {
		return err
	}

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

		if err = engine.EnsurePluginsAreInstalled(pctx.Request(), nil, pctx.Diag, pluginSet,
			pctx.Host.GetProjectPlugins(), reinstall, true); err != nil {
			return err
		}
	}

	return nil
}

func installPackageDependency(
	registry registry.Registry,
) func(pctx *plugin.Context, proj workspace.BaseProject, pkgName string, pkgSpec workspace.PackageSpec) error {
	return func(pctx *plugin.Context, proj workspace.BaseProject, pkgName string, pkgSpec workspace.PackageSpec) error {
		// Process packages section from Pulumi.yaml. Do so before installing language-specific dependencies,
		// so that the SDKs folder is present and references to it from package.json etc are valid.
		parameters := &plugin.ParameterizeArgs{Args: pkgSpec.Parameters}
		_, _, diags, err := packages.InstallPackage(
			proj, pctx, proj.RuntimeInfo().Name(),
			pctx.Root, pkgSpec.Source, parameters, registry)
		cmdDiag.PrintDiagnostics(pctx.Diag, diags)
		return err
	}
}

// Recursively walk packages and plugin dependencies from a project.
//
// WARNING: walk *may* be called in parallel, and will be called in parallel when
// possible.
//
// walkLocalPackagesFromProject holds as invariant that:
//   - walk will only be called once per package.
//   - walk will only be called on a package X after all packages that X depends on have
//     been walked.
func walkLocalPackagesFromProject[P workspace.BaseProject](
	pctx *plugin.Context, proj P,
	walkPackage func(
		pctx *plugin.Context, proj workspace.BaseProject, pkgName string, pkgSpec workspace.PackageSpec,
	) error,
	walkProject func(pctx *plugin.Context, proj P) error,
	loadProject func(path string) (P, error), // workspace.LoadProject
) error {
	// walkGroup is used to bound invocations of walkProject and walkPackage.
	walkGroup, ctx := newDepWorkPool(pctx.Base(), 4) // TODO: What parallelism limit do we want to use
	seenProjects := map[string]*sync.WaitGroup{}
	seenPackages := map[string]*sync.WaitGroup{}
	pctx = pctx.WithCancelChannel(ctx.Done())

	var doWalk func(proj P, root string) (*sync.WaitGroup, error)
	doWalk = func(proj P, root string) (*sync.WaitGroup, error) {
		_, ok := seenProjects[filepath.Clean(root)]
		if ok {
			return nil, fmt.Errorf("cyclic dependency detected on %s", root)
		}

		project := new(sync.WaitGroup)
		project.Add(1)
		seenProjects[filepath.Clean(root)] = project

		dependenciesWg := new(sync.WaitGroup)

		pctx := func(v *plugin.Context) *plugin.Context {
			pctx, err := plugin.NewContextWithRoot(
				pctx.Base(), pctx.Diag, pctx.StatusDiag,
				pctx.Host, "", root, nil, false, nil, nil, nil, nil, nil)
			contract.AssertNoErrorf(err, "plugin.NewContextWithRoot can only error with a non-nil host")
			return pctx
		}(pctx)

		var didError atomic.Bool

		for name, packageSpec := range proj.GetPackageSpecs() {
			installSource := packageSpec.Source
			isLocal := plugin.IsLocalPluginPath(pctx.Base(), installSource)
			if !isLocal && packageSpec.Version != "" {
				installSource = fmt.Sprintf("%s@%s", installSource, packageSpec.Version)
			}
			pkwgWg, ok := seenPackages[filepath.Clean(installSource)]
			if ok {
				dependenciesWg.Add(1)
				go func() {
					pkwgWg.Wait()
					dependenciesWg.Done()
				}()
				continue
			}
			pkgWg := new(sync.WaitGroup)
			pkgWg.Add(1)
			seenPackages[filepath.Clean(installSource)] = pkgWg

			pkgDepsWg := new(sync.WaitGroup)
			if isLocal {
				// Make sure we don't install the same dependency twice
				pulumiYamlPath, err := workspace.DetectProjectPathFrom(installSource)
				if err != nil {
					return nil, fmt.Errorf("unable to find project at '%s': %w", installSource, err)
				}
				localProj, err := loadProject(pulumiYamlPath)
				if err != nil {
					return nil, fmt.Errorf("unable to load %s: %w", installSource, err)
				}
				// DetectProjectPathFrom returns the path to the Pulumi.yaml file,
				// but doWalk expects the directory containing it.
				pkgDepsWg, err = doWalk(localProj, filepath.Dir(pulumiYamlPath))
				if err != nil {
					return nil, err
				}
			}

			dependenciesWg.Add(1)
			walkGroup.Go(func() error {
				defer dependenciesWg.Done()
				defer pkgWg.Done()
				err := walkPackage(pctx, proj, name, packageSpec)
				didError.CompareAndSwap(false, err != nil)
				return err
			}, pkgDepsWg)
		}

		walkGroup.Go(func() error {
			defer project.Done()
			if didError.Load() || walkProject == nil {
				return nil
			}
			return walkProject(pctx, proj)
		}, dependenciesWg)

		return project, nil
	}

	_, err := doWalk(proj, pctx.Root)
	return errors.Join(err, walkGroup.Wait())
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
