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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
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
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return whocmd.Run(commandContext())
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
	currentBackend func(context.Context, *workspace.Project, display.Options) (backend.Backend, error)
}

func (cmd *whoAmICmd) whoAmI(ctx context.Context) (*WhoAmIJSON, error) {
	if cmd.currentBackend == nil {
		cmd.currentBackend = currentBackend
	}
	currentBackend := cmd.currentBackend // shadow the top-level function

	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	// Try to read the current project
	project, _, err := readProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, err
	}

	b, err := currentBackend(ctx, project, opts)
	if err != nil {
		return nil, err
	}

	name, orgs, tokenInfo, err := b.CurrentUser()
	if err != nil {
		return nil, err
	}

	return &WhoAmIJSON{
		User:             name,
		Organizations:    orgs,
		URL:              b.URL(),
		TokenInformation: tokenInfo,
	}, nil
}

func (cmd *whoAmICmd) Run(ctx context.Context) error {
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}

	who, err := cmd.whoAmI(ctx)
	if err != nil {
		return err
	}

	if cmd.jsonOut {
		return fprintJSON(cmd.Stdout, who)
	}

	name := who.User
	if cmd.verbose {
		orgs, tokenInfo := who.Organizations, who.TokenInformation

		fmt.Fprintf(cmd.Stdout, "User: %s\n", name)
		fmt.Fprintf(cmd.Stdout, "Organizations: %s\n", strings.Join(orgs, ", "))
		fmt.Fprintf(cmd.Stdout, "Backend URL: %s\n", who.URL)
		if tokenInfo != nil {
			tokenType := "unknown"
			if tokenInfo.Team != "" {
				tokenType = fmt.Sprintf("team: %s", tokenInfo.Team)
			} else if tokenInfo.Organization != "" {
				tokenType = fmt.Sprintf("organization: %s", tokenInfo.Organization)
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
