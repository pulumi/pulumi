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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	cmdDiag "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/diag"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/pkg/v3/registry"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/util/cmdutil"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	utilCmdutil "github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// InstallDependencies will install dependencies for the project, e.g. by running `npm install` for nodejs projects.
func InstallDependencies(ctx *plugin.Context, runtime *workspace.ProjectRuntimeInfo, main string) error {
	if runtime.Name() == "" {
		return nil
	}

	// First make sure the language plugin is present.  We need this to load the required resource plugins.
	// TODO: we need to think about how best to version this.  For now, it always picks the latest.
	lang, err := ctx.Host.LanguageRuntime(ctx, runtime.Name())
	if err != nil {
		return fmt.Errorf("failed to load language plugin %s: %w", runtime.Name(), err)
	}

	programInfo := plugin.NewProgramInfo(ctx.Root, ctx.Pwd, main, runtime.Options())
	// Helper used by multiple commands; output goes to the process streams
	// directly when not given a writer.
	err = cmdutil.InstallDependencies(ctx.Request(), lang, plugin.InstallDependenciesRequest{
		Info:     programInfo,
		IsPlugin: false,
	}, os.Stdout, os.Stderr) //nolint:forbidigo
	if err != nil {
		//revive:disable-next-line:error-strings // This error message is user facing.
		return fmt.Errorf("installing dependencies failed: %w\nRun `pulumi install` to complete the installation.", err)
	}

	return nil
}

// InstallRequiredPackages asks the language runtime which packages the program requires,
// resolves them, and then installs them.
func InstallRequiredPackages(
	ctx context.Context, pctx *plugin.Context, proj *workspace.Project, root, main string,
	prior packageinstallation.State, parallelism int, useLanguageVersionTools bool,
	reg registry.Registry, stdout, stderr io.Writer,
) error {
	lang, err := pctx.Host.LanguageRuntime(pctx, proj.Runtime.Name())
	if err != nil {
		return fmt.Errorf("failed to load language plugin %s: %w", proj.Runtime.Name(), err)
	}

	programInfo := plugin.NewProgramInfo(pctx.Root, pctx.Pwd, main, proj.Runtime.Options())
	packages, specs, err := lang.GetRequiredPackages(ctx, programInfo)
	if err != nil {
		return err
	}

	projPath, err := workspace.DetectProjectPathFrom(root)
	if err != nil {
		return fmt.Errorf("locating Pulumi.yaml: %w", err)
	}

	ws := packageworkspace.New(pluginstorage.Instance, pkgWorkspace.Instance, pctx, stdout, stderr, nil,
		packageworkspace.Options{
			UseLanguageVersionTools: useLanguageVersionTools,
		})

	_, err = packageinstallation.InstallPluginSet(ctx, packages, specs, proj, filepath.Dir(projPath),
		packageinstallation.Options{
			Concurrency: parallelism,
			PriorState:  prior,
			Options: packageresolution.Options{
				ResolveVersionWithLocalWorkspace:           true,
				ResolveWithRegistry:                        !env.DisableRegistryResolve.Value(),
				AllowNonInvertableLocalWorkspaceResolution: true,
			},
		}, reg, ws)
	if err != nil {
		return fmt.Errorf("installing packages: %w", err)
	}
	return nil
}

// InstallPackagesFromProject processes packages specified in the Pulumi.yaml file
// and installs them using similar logic to the 'pulumi package add' command
func InstallPackagesFromProject(
	ctx context.Context, proj workspace.BaseProject, root string, registry registry.Registry,
	parallelism int, useLanguageVersionTools bool,
	stdout, stderr io.Writer, e env.Env,
) (packageinstallation.State, error) {
	d := diag.DefaultSink(stdout, stderr, diag.FormatOptions{
		Color: utilCmdutil.GetGlobalColorization(),
	})
	pluginHost, err := pkghost.New(context.WithoutCancel(ctx), d, d, nil, pkgWorkspace.EnsureLanguageInstalled,
		schema.NewLoaderServerFromContext, convert.NewMapperServerFromContext,
		packageworkspace.NewResolverServer(registry))
	if err != nil {
		return packageinstallation.State{}, err
	}
	pctx, err := plugin.NewContext(ctx, d, d, pluginHost, nil, root, nil, false, nil)
	if err != nil {
		return packageinstallation.State{}, errors.Join(err, pluginHost.Close())
	}
	ws := packageworkspace.New(pluginstorage.Instance, pkgWorkspace.Instance, pctx, stdout, stderr, nil,
		packageworkspace.Options{
			UseLanguageVersionTools: useLanguageVersionTools,
		})
	opts := packageinstallation.Options{
		Options: packageresolution.Options{
			ResolveWithRegistry:                        !env.DisableRegistryResolve.Value(),
			ResolveVersionWithLocalWorkspace:           true,
			AllowNonInvertableLocalWorkspaceResolution: true,
		},
		Concurrency: parallelism,
	}
	continuation, err := packageinstallation.InstallProjectPlugins(ctx, proj, root, opts, registry, ws)
	if e := (packageinstallation.ErrorCyclicDependencies{}); errors.As(err, &e) {
		err = cmdDiag.FormatCyclicInstallError(ctx, e, root)
	}

	return continuation, errors.Join(err, pctx.Close(), pluginHost.Close())
}
