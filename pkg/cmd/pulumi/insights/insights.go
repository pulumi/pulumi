// Copyright 2016-2025, Pulumi Corporation.
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

package insights

import (
	"errors"

	"github.com/spf13/cobra"

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// outputFormat represents the output format for commands.
type outputFormat string

const (
	outputFormatTable outputFormat = "table"
	outputFormatJSON  outputFormat = "json"
	outputFormatYAML  outputFormat = "yaml"
)

// String is used both by fmt.Print and by Cobra in help text.
func (o *outputFormat) String() string {
	return string(*o)
}

// Set must have pointer receiver so it doesn't change the value of a copy.
func (o *outputFormat) Set(v string) error {
	switch v {
	case "table", "json", "yaml":
		*o = outputFormat(v)
		return nil
	default:
		return errors.New(`must be one of "table", "json", or "yaml"`)
	}
}

// Type is only used in help text.
func (o *outputFormat) Type() string {
	return "outputFormat"
}

// NewInsightsCmd creates the `pulumi insights` command.
func NewInsightsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "insights",
		Short: "Manage Pulumi Insights discovery",
		Long: "Manage Pulumi Insights discovery.\n" +
			"\n" +
			"Use this command to manage Insights discovery accounts and scans for your cloud infrastructure.",
		Args: cobra.NoArgs,
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newAccountsCmd())
	cmd.AddCommand(newScansCmd())

	return cmd
}

// getDisplayOptions returns the display options for the current command.
func getDisplayOptions() display.Options {
	return display.Options{
		Color:         cmdutil.GetGlobalColorization(),
		IsInteractive: cmdutil.Interactive(),
	}
}

// ensureCloudBackend gets the cloud backend and determines the organization name.
func ensureCloudBackend(cmd *cobra.Command, ws pkgWorkspace.Context) (httpstate.Backend, string, error) {
	ctx := cmd.Context()
	displayOpts := getDisplayOptions()

	// Try to read the current project
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, "", err
	}

	currentBe, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, displayOpts)
	if err != nil {
		return nil, "", err
	}

	cloudBackend, isCloud := currentBe.(httpstate.Backend)
	if !isCloud {
		return nil, "", errors.New("Insights discovery requires Pulumi Cloud. " +
			"Please log in to Pulumi Cloud with 'pulumi login'")
	}

	// Determine organization name
	defaultOrg, err := pkgBackend.GetDefaultOrg(ctx, cloudBackend, project)
	if err != nil {
		return nil, "", err
	}

	if defaultOrg == "" {
		// Fall back to the current user's login name
		userName, _, _, userErr := cloudBackend.CurrentUser()
		if userErr != nil {
			return nil, "", errors.New("unable to determine organization. " +
				"Please set a default organization with 'pulumi org set-default'")
		}
		defaultOrg = userName
	}

	return cloudBackend, defaultOrg, nil
}
