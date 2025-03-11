// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package org

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/pkg/browser"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

type searchAICmd struct {
	searchCmd
	queryString string
	openWeb     bool
}

func (cmd *searchAICmd) Run(ctx context.Context, args []string) error {
	interactive := cmdutil.Interactive()

	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}

	if cmd.outputFormat == "" {
		cmd.outputFormat = outputFormatTable
	}

	if cmd.currentBackend == nil {
		cmd.currentBackend = cmdBackend.CurrentBackend
	}
	currentBackend := cmd.currentBackend // shadow the top-level function

	opts := backend.QueryOptions{}
	opts.Display = display.Options{
		Color:         cmdutil.GetGlobalColorization(),
		IsInteractive: interactive,
		Type:          display.DisplayQuery,
	}
	// Try to read the current project
	ws := pkgWorkspace.Instance
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	backend, err := currentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, opts.Display)
	if err != nil {
		return err
	}
	cloudBackend, isCloud := backend.(httpstate.Backend)
	if !isCloud {
		return errors.New("Pulumi AI search is only supported for the Pulumi Cloud")
	}
	defaultOrg, err := pkgWorkspace.GetBackendConfigDefaultOrg(project)
	if err != nil {
		return err
	}
	userName, orgs, _, err := cloudBackend.CurrentUser()
	if err != nil {
		return err
	}
	if defaultOrg != "" && cmd.orgName == "" {
		cmd.orgName = defaultOrg
	}
	if cmd.orgName == "" {
		cmd.orgName = userName
	}
	if cmd.orgName == userName {
		return fmt.Errorf(
			"%s is an individual account, not an organization."+
				"Organization search is not supported for individual accounts",
			userName,
		)
	}
	if !sliceContains(orgs, cmd.orgName) && cmd.orgName != "" {
		return fmt.Errorf("user %s is not a member of org %s", userName, cmd.orgName)
	}

	res, err := cloudBackend.NaturalLanguageSearch(ctx, cmd.orgName, cmd.queryString)
	if err != nil {
		return err
	}
	err = cmd.outputFormat.Render(&cmd.searchCmd, res)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rendering error: %s\n", err)
	}
	if cmd.openWeb {
		err = browser.OpenURL(res.URL)
		if err != nil {
			return fmt.Errorf("failed to open URL: %w", err)
		}
	}
	return nil
}

func newSearchAICmd() *cobra.Command {
	var scmd searchAICmd
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "Search for resources in Pulumi Cloud using Pulumi AI",
		Long:  "Search for resources in Pulumi Cloud using Pulumi AI",
		Args:  cmdutil.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return scmd.Run(ctx, args)
		},
	}
	cmd.PersistentFlags().StringVar(
		&scmd.orgName, "org", "",
		"Organization name to search within",
	)
	cmd.PersistentFlags().StringVarP(
		&scmd.queryString, "query", "q", "",
		"Plaintext natural language query",
	)
	cmd.PersistentFlags().VarP(
		&scmd.outputFormat, "output", "o",
		"Output format. Supported formats are 'table', 'json', 'csv' and 'yaml'.",
	)
	cmd.PersistentFlags().Var(
		&scmd.csvDelimiter, "delimiter",
		"Delimiter to use when rendering CSV output.",
	)
	cmd.PersistentFlags().BoolVar(
		&scmd.openWeb, "web", false,
		"Open the search results in a web browser.",
	)
	return cmd
}
