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
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/policy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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
			"This command is used to manually install packages and plugins required by your program or policy pack.",
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
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
					return policy.InstallPolicyPackDependencies(ctx, root, proj)
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

			// First make sure the language plugin is present.  We need this to load the required resource plugins.
			// TODO: we need to think about how best to version this.  For now, it always picks the latest.
			runtime := proj.Runtime
			programInfo := plugin.NewProgramInfo(pctx.Root, pwd, main, runtime.Options())
			lang, err := pctx.Host.LanguageRuntime(runtime.Name(), programInfo)
			if err != nil {
				return fmt.Errorf("load language plugin %s: %w", runtime.Name(), err)
			}

			if !noDependencies {
				if err = lang.InstallDependencies(plugin.InstallDependenciesRequest{
					Info:                    programInfo,
					UseLanguageVersionTools: useLanguageVersionTools,
				}); err != nil {
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
		}),
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
		projectPath, err := workspace.DetectProjectPath()
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
