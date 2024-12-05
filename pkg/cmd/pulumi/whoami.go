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
	"io"
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func newWhoAmICmd() *cobra.Command {
	var whocmd whoAmICmd
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Display the current logged-in user",
		Long: "Display the current logged-in user\n" +
			"\n" +
			"Displays the username of the currently logged in user.",
		Args: cmdutil.NoArgs,
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			return whocmd.Run(cmd.Context())
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&whocmd.jsonOut, "json", "j", false, "Emit output as JSON")

	cmd.PersistentFlags().BoolVarP(
		&whocmd.verbose, "verbose", "v", false,
		"Print detailed whoami information")

	return cmd
}

type whoAmICmd struct {
	jsonOut bool
	verbose bool

	Stdout io.Writer // defaults to os.Stdout

	// currentBackend is a reference to the top-level currentBackend function.
	// This is used to override the default implementation for testing purposes.
	currentBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
	) (backend.Backend, error)
}

func (cmd *whoAmICmd) Run(ctx context.Context) error {
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}

	if cmd.currentBackend == nil {
		cmd.currentBackend = cmdBackend.CurrentBackend
	}
	currentBackend := cmd.currentBackend // shadow the top-level function

	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	// Try to read the current project
	ws := pkgWorkspace.Instance
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	b, err := currentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, opts)
	if err != nil {
		return err
	}

	name, orgs, tokenInfo, err := b.CurrentUser()
	if err != nil {
		return err
	}

	if cmd.jsonOut {
		return ui.FprintJSON(cmd.Stdout, WhoAmIJSON{
			User:             name,
			Organizations:    orgs,
			URL:              b.URL(),
			TokenInformation: tokenInfo,
		})
	}

	if cmd.verbose {
		fmt.Fprintf(cmd.Stdout, "User: %s\n", name)
		fmt.Fprintf(cmd.Stdout, "Organizations: %s\n", strings.Join(orgs, ", "))
		fmt.Fprintf(cmd.Stdout, "Backend URL: %s\n", b.URL())
		if tokenInfo != nil {
			tokenType := "unknown"
			if tokenInfo.Team != "" {
				tokenType = "team: " + tokenInfo.Team
			} else if tokenInfo.Organization != "" {
				tokenType = "organization: " + tokenInfo.Organization
			}
			fmt.Fprintf(cmd.Stdout, "Token type: %s\n", tokenType)
			fmt.Fprintf(cmd.Stdout, "Token name: %s\n", tokenInfo.Name)
		} else {
			fmt.Fprintf(cmd.Stdout, "Token type: personal\n")
		}
	} else {
		fmt.Fprintf(cmd.Stdout, "%s\n", name)
	}

	return nil
}

// WhoAmIJSON is the shape of the --json output of this command.
type WhoAmIJSON struct {
	User             string                      `json:"user"`
	Organizations    []string                    `json:"organizations,omitempty"`
	URL              string                      `json:"url"`
	TokenInformation *workspace.TokenInformation `json:"tokenInformation,omitempty"`
}
