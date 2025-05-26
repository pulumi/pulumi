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

package operations

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/newcmd"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// parseAndSaveConfigArray parses the config array and saves it as a config for
// the provided stack.
func parseAndSaveConfigArray(ctx context.Context, ws pkgWorkspace.Context, s backend.Stack,
	configArray []string, path bool,
) error {
	if len(configArray) == 0 {
		return nil
	}
	commandLineConfig, err := newcmd.ParseConfig(configArray, path)
	if err != nil {
		return err
	}

	if err = newcmd.SaveConfig(ctx, ws, s, commandLineConfig); err != nil {
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

// configureCopilotOptions configures display options related to Copilot features based on the command line flags and
// environment variables.
func configureCopilotOptions(copilotEnabledFlag bool, cmd *cobra.Command, displayOpts *display.Options,
	isDIYBackend bool,
) {
	// Handle copilot-summary flag and environment variable If flag is explicitly set (via command line), use that value
	// Otherwise fall back to environment variable, then default to false
	var showCopilotFeatures bool
	if cmd.Flags().Changed("copilot") {
		showCopilotFeatures = copilotEnabledFlag
	} else {
		showCopilotFeatures = env.CopilotEnabled.Value()
	}
	logging.V(7).Infof("copilot flag=%v, PULUMI_COPILOT=%v, using value=%v",
		copilotEnabledFlag, env.CopilotEnabled.Value(), showCopilotFeatures)

	// Do not enable any copilot features if we are using a DIY backend
	if showCopilotFeatures && isDIYBackend {
		logging.Warningf("Copilot features are not available with DIY backends.")
		return
	}

	displayOpts.ShowCopilotFeatures = showCopilotFeatures
	displayOpts.CopilotSummaryModel = env.CopilotSummaryModel.Value()
	displayOpts.CopilotSummaryMaxLen = env.CopilotSummaryMaxLen.Value()
}
