// Copyright 2016-2023, Pulumi Corporation.
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
	"errors"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func newWhoAmICmd() *cobra.Command {
	var jsonOut bool
	var verbose bool

	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Display the current logged-in user",
		Long: "Display the current logged-in user\n" +
			"\n" +
			"Displays the username of the currently logged in user.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Try to read the current project
			project, _, err := readProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			b, err := currentBackend(ctx, project, opts)
			if err != nil {
				return err
			}

			name, orgs, err := b.CurrentUser()
			if err != nil {
				return err
			}

			if jsonOut {
				return printJSON(WhoAmIJSON{
					User:          name,
					Organizations: orgs,
					URL:           b.URL(),
				})
			}

			if verbose {
				fmt.Printf("User: %s\n", name)
				fmt.Printf("Organizations: %s\n", strings.Join(orgs, ", "))
				fmt.Printf("Backend URL: %s\n", b.URL())
			} else {
				fmt.Println(name)
			}

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&jsonOut, "json", "j", false, "Emit output as JSON")

	cmd.PersistentFlags().BoolVarP(
		&verbose, "verbose", "v", false,
		"Print detailed whoami information")

	return cmd
}

// WhoAmIJSON is the shape of the --json output of this command.
type WhoAmIJSON struct {
	User          string   `json:"user"`
	Organizations []string `json:"organizations,omitempty"`
	URL           string   `json:"url"`
}
