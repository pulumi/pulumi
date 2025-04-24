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

package whoami

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func NewWhoAmICmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	var jsonOut bool
	var verbose bool

	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Display the current logged-in user",
		Long: "Display the current logged-in user\n" +
			"\n" +
			"Displays the username of the currently logged in user.",
		Args: cmdutil.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			stdout := cmd.OutOrStdout()

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Try to read the current project
			project, _, err := ws.ReadProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			b, err := cmdBackend.CurrentBackend(ctx, ws, lm, project, opts)
			if err != nil {
				return err
			}

			name, orgs, tokenInfo, err := b.CurrentUser()
			if err != nil {
				return err
			}

			if jsonOut {
				return ui.FprintJSON(stdout, whoAmIJSON{
					User:             name,
					Organizations:    orgs,
					URL:              b.URL(),
					TokenInformation: tokenInfo,
				})
			}

			if verbose {
				fmt.Fprintf(stdout, "User: %s\n", name)
				fmt.Fprintf(stdout, "Organizations: %s\n", strings.Join(orgs, ", "))
				fmt.Fprintf(stdout, "Backend URL: %s\n", b.URL())
				if tokenInfo != nil {
					tokenType := "unknown"
					if tokenInfo.Team != "" {
						tokenType = "team: " + tokenInfo.Team
					} else if tokenInfo.Organization != "" {
						tokenType = "organization: " + tokenInfo.Organization
					}
					fmt.Fprintf(stdout, "Token type: %s\n", tokenType)
					fmt.Fprintf(stdout, "Token name: %s\n", tokenInfo.Name)
				} else {
					fmt.Fprintf(stdout, "Token type: personal\n")
				}
			} else {
				fmt.Fprintf(stdout, "%s\n", name)
			}

			return nil
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&jsonOut, "json", "j", false, "Emit output as JSON")

	cmd.PersistentFlags().BoolVarP(
		&verbose, "verbose", "v", false,
		"Print detailed whoami information")

	return cmd
}

// whoAmIJSON is the shape of the --json output of this command.
type whoAmIJSON struct {
	User             string                      `json:"user"`
	Organizations    []string                    `json:"organizations,omitempty"`
	URL              string                      `json:"url"`
	TokenInformation *workspace.TokenInformation `json:"tokenInformation,omitempty"`
}
