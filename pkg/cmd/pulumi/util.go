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

package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/opentracing/opentracing-go"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/newcmd"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
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

// readPolicyProject attempts to detect and read a Pulumi PolicyPack project for the current
// workspace. If the project is successfully detected and read, it is returned along with the path
// to its containing directory, which will be used as the root of the project's Pulumi program.
func readPolicyProject(pwd string) (*workspace.PolicyPackProject, string, string, error) {
	// Now that we got here, we have a path, so we will try to load it.
	path, err := workspace.DetectPolicyPackPathFrom(pwd)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to find current Pulumi project because of "+
			"an error when searching for the PulumiPolicy.yaml file (searching upwards from %s)"+": %w", pwd, err)
	} else if path == "" {
		return nil, "", "", fmt.Errorf("no PulumiPolicy.yaml project file found (searching upwards from %s)", pwd)
	}
	proj, err := workspace.LoadPolicyPack(path)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to load Pulumi policy project located at %q: %w", path, err)
	}

	return proj, path, filepath.Dir(path), nil
}

// updateFlagsToOptions ensures that the given update flags represent a valid combination.  If so, an UpdateOptions
// is returned with a nil-error; otherwise, the non-nil error contains information about why the combination is invalid.
func updateFlagsToOptions(interactive, skipPreview, yes, previewOnly bool) (backend.UpdateOptions, error) {
	switch {
	case !interactive && !yes && !skipPreview && !previewOnly:
		return backend.UpdateOptions{},
			errors.New("one of --yes, --skip-preview, or --preview-only must be specified in non-interactive mode")
	case skipPreview && previewOnly:
		return backend.UpdateOptions{},
			errors.New("--skip-preview and --preview-only cannot be used together")
	case yes && previewOnly:
		return backend.UpdateOptions{},
			errors.New("--yes and --preview-only cannot be used together")
	default:
		return backend.UpdateOptions{
			AutoApprove: yes,
			SkipPreview: skipPreview,
			PreviewOnly: previewOnly,
		}, nil
	}
}

func getRefreshOption(proj *workspace.Project, refresh string) (bool, error) {
	// we want to check for an explicit --refresh or a --refresh=true or --refresh=false
	// refresh is assigned the empty string by default to distinguish the difference between
	// when the user actually interacted with the cli argument (`NoOptDefVal`)
	// and the default functionality today
	if refresh != "" {
		refreshDetails, boolErr := strconv.ParseBool(refresh)
		if boolErr != nil {
			// the user has passed a --refresh but with a random value that we don't support
			return false, errors.New("unable to determine value for --refresh")
		}
		return refreshDetails, nil
	}

	// the user has not specifically passed an argument on the cli to refresh but has set a Project option to refresh
	if proj.Options != nil && proj.Options.Refresh == "always" {
		return true, nil
	}

	// the default functionality right now is to always skip a refresh
	return false, nil
}

func installPolicyPackDependencies(ctx context.Context, root string, proj *workspace.PolicyPackProject) error {
	span := opentracing.SpanFromContext(ctx)
	// Bit of a hack here. Creating a plugin context requires a "program project", but we've only got a
	// policy project. Ideally we should be able to make a plugin context without any related project. But
	// fow now this works.
	projinfo := &engine.Projinfo{Proj: &workspace.Project{
		Main:    proj.Main,
		Runtime: proj.Runtime,
	}, Root: root}
	_, main, pluginCtx, err := engine.ProjectInfoContext(
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
	defer pluginCtx.Close()

	programInfo := plugin.NewProgramInfo(pluginCtx.Root, pluginCtx.Pwd, main, proj.Runtime.Options())
	lang, err := pluginCtx.Host.LanguageRuntime(proj.Runtime.Name(), programInfo)
	if err != nil {
		return fmt.Errorf("failed to load language plugin %s: %w", proj.Runtime.Name(), err)
	}

	if err = lang.InstallDependencies(plugin.InstallDependenciesRequest{Info: programInfo}); err != nil {
		return fmt.Errorf("installing dependencies failed: %w", err)
	}

	return nil
}
