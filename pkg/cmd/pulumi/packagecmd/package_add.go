// Copyright 2025, Pulumi Corporation.
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

package packagecmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdDiag "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/diag"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/agentdetect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

// Constructs the `pulumi package add` command.
func newPackageAddCmd() *cobra.Command {
	var language string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a package to your Pulumi project, plugin, or current directory.",
		Long: `Add a package to your Pulumi project, plugin, or current directory.

This command locally generates an SDK in the selected Pulumi language and
prints instructions on how to use it. The SDK is based on a Pulumi package
schema extracted from a given resource plugin or provided directly.

When run inside a Pulumi project or plugin, the package is also recorded in
Pulumi.yaml or PulumiPlugin.yaml. To run outside one (e.g. for a
single-language component library or with the Automation API), pass
'--language LANG' to select the SDK language explicitly.

The <provider> argument can be specified in one of the following ways:

- When <provider> is specified as a PLUGIN[@VERSION] reference, Pulumi attempts to
  resolve a resource plugin first, installing it on-demand, similarly to:

    pulumi plugin install resource PLUGIN [VERSION]

- When <provider> is specified as a local path, Pulumi executes the provider
  binary to extract its package schema:

    pulumi package add ./my-provider

- When <provider> is a path to a local file with a '.json', '.yml' or '.yaml'
  extension, Pulumi package schema is read from it directly:

    pulumi package add ./my/schema.json

- When <provider> is a reference to a Git repo, Pulumi clones the repo and
  executes the source. Optionally a version can be specified.  It can either
  be a tag (in semver format), or a Git commit hash.  By default the latest
  tag (by semver version), or if not available the latest commit on the
  default branch is used. Paths can be disambiguated from the repo name by
  appending '.git' to the repo URL, followed by the path to the package:

    pulumi package add example.org/org/repo.git/path[@<version>]

For parameterized providers, parameters may be specified as additional
arguments. The exact format of parameters is provider-specific; consult the
provider's documentation for more information. If the parameters include flags
that begin with dashes, you may need to use '--' to separate the provider name
from the parameters, as in:

  pulumi package add <provider> -- --provider-parameter-flag value
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			agent := agentdetect.Detect(os.Getenv)

			wd, err := os.Getwd()
			if err != nil {
				return err
			}

			target, err := loadEnclosingTarget(cmd.Context(), wd)
			if err != nil && !errors.Is(err, workspace.ErrBaseProjectNotFound) {
				return err
			}
			foundProject := err == nil

			if foundProject && language != "" {
				return fmt.Errorf("--language is for use outside a Pulumi project or "+
					"plugin, but %s was found; remove the flag, or run from a "+
					"directory outside the project", *target.projectFilePath)
			}
			if !foundProject {
				if language == "" {
					return fmt.Errorf("%w; pass --language LANG to run "+
						"outside a Pulumi project or plugin", err)
				}
				target = addTarget{
					installRoot: wd,
					proj: &workspace.PluginProject{
						Runtime: workspace.NewProjectRuntimeInfo(cmdCmd.NormalizeRuntimeName(language), nil),
					},
					reg: cmdCmd.NewDefaultRegistry(
						cmd.Context(), cmdBackend.DefaultLoginManager, pkgWorkspace.Instance, nil, cmdutil.Diag(), env.Global()),
				}
			}

			sink := cmdutil.Diag()
			pluginHost, err := pkghost.New(context.WithoutCancel(cmd.Context()), sink, sink, nil,
				pkgWorkspace.EnsureLanguageInstalled, schema.NewLoaderServerFromContext, convert.NewMapperServerFromContext,
				packageworkspace.NewResolverServer(target.reg))
			if err != nil {
				return err
			}
			// host is owned here, closed after the context
			defer contract.IgnoreClose(pluginHost)
			pctx, err := plugin.NewContext(cmd.Context(),
				sink, sink, pluginHost, nil, target.installRoot, nil, false, nil)
			if err != nil {
				return err
			}
			defer contract.IgnoreClose(pctx)

			if target.proj.RuntimeInfo().Name() == "" {
				return errors.New("cannot add a package to a project without a runtime")
			}

			pluginSource := args[0]
			parameters := &plugin.ParameterizeArgs{Args: args[1:]}

			pkg, packageSpec, diags, err := packages.InstallPackage(
				cmd.OutOrStdout(),
				pkgWorkspace.Instance,
				target.proj,
				pctx,
				target.proj.RuntimeInfo().Name(),
				target.installRoot,
				pluginSource,
				parameters,
				target.reg,
				env.Global(),
				0, /* unbounded concurrency */
			)
			cmdDiag.PrintDiagnostics(pctx.Diag, diags)
			if err != nil {
				if errors.Is(err, registry.ErrNotFound) && agent != "" {
					return fmt.Errorf("%w\nSearch: pulumi api '/api/registry/packages?search=<term>'", err)
				}
				return err
			}

			// Build and add the package spec to the project
			pluginSplit := strings.Split(pluginSource, "@")
			source := pluginSplit[0]

			if ext := filepath.Ext(source); ext == ".yaml" || ext == ".yml" || ext == ".json" {
				// We don't add file based schemas to the project's packages, since there is no actual underlying
				// provider for them.
				return nil
			}

			// TODO[#21349]:  We can't bake  a path into  Pulumi.yaml until we  use [packageresolution.Resolve]
			// when loading a new context, so condense local paths to the name of the package.
			//
			// This is wrong, but its less wrong then producing a Pulumi.yaml that `pulumi` can't process
			// (#21348).
			//
			if plugin.IsLocalPluginPath(cmd.Context(), packageSpec.Source) {
				f, err := os.Stat(packageSpec.Source)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
				// A source that does not exist on disk leaves f nil above (ErrNotExist is
				// tolerated), so there is nothing to condense.
				if f != nil && !f.IsDir() {
					if pkg.Parameterization == nil {
						packageSpec.Source = pkg.Name
						if pkg.Version != nil {
							packageSpec.Version = pkg.Version.String()
						}
					} else {
						packageSpec.Source = pkg.Parameterization.BasePlugin.Name
						packageSpec.Version = pkg.Parameterization.BasePlugin.Version.String()
					}
				}
			}

			contract.Assertf(packageSpec != nil, "packageSpec should be nil if & only if source is file based")
			packageSpec.Parameters = parameters.Args

			if target.projectFilePath != nil {
				target.proj.AddPackage(pkg.Name, *packageSpec)

				fileName := filepath.Base(*target.projectFilePath)
				// Save the updated project
				if err := target.proj.Save(*target.projectFilePath); err != nil {
					return fmt.Errorf("failed to update %s: %w", fileName, err)
				}
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Added package %s\n", schemaDisplayName(pkg))
			printRegistryDocsHint(cmd.ErrOrStderr(), agent, cmd.Context(), target.reg, pkg)
			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "provider", Usage: "<provider|schema|path>"},
			{Name: "provider-parameter"},
		},
		Required: 1,
		Variadic: true,
	})

	// It's worth mentioning the `--`, as it means that Cobra will stop parsing flags.
	// In other words, a provider parameter can be `--foo` as long as it's after `--`.
	cmd.Use = "add <provider|schema|path> [flags] [--] [provider-parameter]..."

	cmd.Flags().StringVar(&language, "language", "",
		"Run outside a Pulumi project or plugin: [nodejs|python|go|dotnet|java]")

	return cmd
}

type addTarget struct {
	installRoot     string
	projectFilePath *string
	reg             registry.Registry
	proj            workspace.BaseProject
}

func printRegistryDocsHint(
	w io.Writer, agent string, ctx context.Context, reg registry.Registry, pkg *schema.Package,
) {
	if agent == "" || pkg == nil || pkg.Name == "" || pkg.Version == nil || reg == nil {
		return
	}
	meta, err := registry.ResolvePackageFromName(ctx, reg, pkg.Name, pkg.Version)
	if err != nil {
		return
	}
	base := fmt.Sprintf("/api/registry/packages/%s/%s/%s/versions/%s",
		meta.Source, meta.Publisher, meta.Name, pkg.Version.String())
	hints := []struct{ suffix, comment string }{
		{"/readme", "                    # package readme"},
		{"/nav", "                       # doc tree (modules)"},
		{"/nav?q=<term>&depth=full", "   # search for resources/functions"},
		{"/docs/<type-token>", "         # one resource or function (type token from /nav)"},
	}
	fmt.Fprintln(w, "Documentation:")
	for _, h := range hints {
		fmt.Fprintf(w, "  pulumi api --output=markdown '%s%s'%s\n", base, h.suffix, h.comment)
	}
}

func schemaDisplayName(schema *schema.Package) string {
	name := schema.DisplayName
	if name == "" {
		name = schema.Name
	}
	if schema.Namespace != "" {
		name = schema.Namespace + "/" + name
	}
	return name
}

// Detect the nearest enclosing Pulumi Project or Pulumi Plugin root directory.
func loadEnclosingTarget(ctx context.Context, wd string) (addTarget, error) {
	baseProject, filePath, err := workspace.LoadBaseProjectFrom(wd)
	if err != nil {
		return addTarget{}, err
	}

	switch baseProject := baseProject.(type) {
	case *workspace.Project:
		return addTarget{
			installRoot:     filepath.Dir(filePath),
			projectFilePath: &filePath,
			reg: cmdCmd.NewDefaultRegistry(
				ctx, cmdBackend.DefaultLoginManager, pkgWorkspace.Instance, baseProject, cmdutil.Diag(), env.Global()),
			proj: baseProject,
		}, nil
	case *workspace.PluginProject:
		return addTarget{
			installRoot:     filepath.Dir(filePath),
			projectFilePath: &filePath,
			proj:            baseProject,
			// Cloud registry is linked to a backend, but we don't have one
			// available in a plugin. Use the default backend.
			reg: cmdCmd.NewDefaultRegistry(
				ctx, cmdBackend.DefaultLoginManager, pkgWorkspace.Instance, nil, cmdutil.Diag(), env.Global(),
			),
		}, nil
	default:
		panic(fmt.Sprintf("workspace.LoadBaseProjectFrom promises that it will return "+
			"either *workspace.Project or *workspace.PluginProject, found %T", baseProject))
	}
}
