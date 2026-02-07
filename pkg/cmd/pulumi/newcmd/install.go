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

	cmdDiag "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/diag"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/pkg/v3/util/cmdutil"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	utilCmdutil "github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// InstallDependencies will install dependencies for the project, e.g. by running `npm install` for nodejs projects.
func InstallDependencies(ctx *plugin.Context, runtime *workspace.ProjectRuntimeInfo, main string) error {
	// First make sure the language plugin is present.  We need this to load the required resource plugins.
	// TODO: we need to think about how best to version this.  For now, it always picks the latest.
	lang, err := ctx.Host.LanguageRuntime(runtime.Name())
	if err != nil {
		return fmt.Errorf("failed to load language plugin %s: %w", runtime.Name(), err)
	}

	programInfo := plugin.NewProgramInfo(ctx.Root, ctx.Pwd, main, runtime.Options())
	err = cmdutil.InstallDependencies(lang, plugin.InstallDependenciesRequest{
		Info:     programInfo,
		IsPlugin: false,
	}, os.Stdout, os.Stderr)
	if err != nil {
		//revive:disable-next-line:error-strings // This error message is user facing.
		return fmt.Errorf("installing dependencies failed: %w\nRun `pulumi install` to complete the installation.", err)
	}

	return nil
}

// InstallPackagesFromProject processes packages specified in the Pulumi.yaml file
// and installs them using similar logic to the 'pulumi package add' command
func InstallPackagesFromProject(
	ctx context.Context, proj workspace.BaseProject, root string, registry registry.Registry,
	parallelism int, useLanguageVersionTools bool,
	stdout, stderr io.Writer, e env.Env,
) error {
	d := diag.DefaultSink(stdout, stderr, diag.FormatOptions{
		Color: utilCmdutil.GetGlobalColorization(),
	})
	pctx, err := plugin.NewContext(ctx, d, d, nil, nil, root, nil, false, nil)
	if err != nil {
		return err
	}
	ws := packageworkspace.New(pluginstorage.Instance, pkgWorkspace.Instance, pctx.Host, stdout, stderr, nil,
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
	err = packageinstallation.InstallProjectPlugins(ctx, proj, root, opts, registry, ws)
	if e := (packageinstallation.ErrorCyclicDependencies{}); errors.As(err, &e) {
		err = cmdDiag.FormatCyclicInstallError(ctx, e, root)
	}

	return errors.Join(err, pctx.Close())
}
