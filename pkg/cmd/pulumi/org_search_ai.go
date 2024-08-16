// Copyright 2016-2023, Pulumi Corporation.
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

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/pkg/browser"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

//nolint:lll
type OrgSearchAIArgs struct {
	Organization   string       `args:"org" argsUsage:"Organization name to search within"`
	CSVDelimiter   Delimiter    `args:"delimiter" argsUsage:"Delimiter to use when rendering CSV output."`
	OutputFormat   outputFormat `args:"output" argsType:"var" argsShort:"o" argsUsage:"Output format. Supported formats are 'table', 'json', 'csv' and 'yaml'."`
	QueryString    string       `args:"query" argsShort:"q" argsUsage:"Plaintext natural language query"`
	OpenWebBrowser bool         `args:"web" argsUsage:"Open the search results in a web browser."`
}

type searchAICmd struct {
	Args OrgSearchAIArgs

	Stdout io.Writer // defaults to os.Stdout

	// currentBackend is a reference to the top-level currentBackend function.
	// This is used to override the default implementation for testing purposes.
	currentBackend func(context.Context, *workspace.Project, display.Options) (backend.Backend, error)
}

func newSearchAICmd(
	v *viper.Viper,
	parentOrgSearchCmd *cobra.Command,
) *cobra.Command {
	var scmd searchAICmd
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "Search for resources in Pulumi Cloud using Pulumi AI",
		Long:  "Search for resources in Pulumi Cloud using Pulumi AI",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			scmd.Args = UnmarshalArgs[OrgSearchAIArgs](v, cmd)
			ctx := cmd.Context()
			return scmd.Run(ctx, args)
		},
		),
	}

	parentOrgSearchCmd.AddCommand(cmd)
	BindFlags[OrgSearchAIArgs](v, cmd)

	return cmd
}

func (cmd *searchAICmd) Run(ctx context.Context, cliArgs []string) error {
	interactive := cmdutil.Interactive()

	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}

	if cmd.Args.OutputFormat == "" {
		cmd.Args.OutputFormat = outputFormatTable
	}

	if cmd.currentBackend == nil {
		cmd.currentBackend = currentBackend
	}
	currentBackend := cmd.currentBackend // shadow the top-level function

	opts := backend.QueryOptions{}
	opts.Display = display.Options{
		Color:         cmdutil.GetGlobalColorization(),
		IsInteractive: interactive,
		Type:          display.DisplayQuery,
	}
	// Try to read the current project
	project, _, err := readProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	backend, err := currentBackend(ctx, project, opts.Display)
	if err != nil {
		return err
	}
	cloudBackend, isCloud := backend.(httpstate.Backend)
	if !isCloud {
		return errors.New("Pulumi AI search is only supported for the Pulumi Cloud")
	}
	defaultOrg, err := workspace.GetBackendConfigDefaultOrg(project)
	if err != nil {
		return err
	}
	userName, orgs, _, err := cloudBackend.CurrentUser()
	if err != nil {
		return err
	}
	if defaultOrg != "" && cmd.Args.Organization == "" {
		cmd.Args.Organization = defaultOrg
	}
	if cmd.Args.Organization == "" {
		cmd.Args.Organization = userName
	}
	if cmd.Args.Organization == userName {
		return fmt.Errorf(
			"%s is an individual account, not an organization."+
				"Organization search is not supported for individual accounts",
			userName,
		)
	}
	if !sliceContains(orgs, cmd.Args.Organization) && cmd.Args.Organization != "" {
		return fmt.Errorf("user %s is not a member of org %s", userName, cmd.Args.Organization)
	}

	res, err := cloudBackend.NaturalLanguageSearch(ctx, cmd.Args.Organization, cmd.Args.QueryString)
	if err != nil {
		return err
	}
	err = cmd.Args.OutputFormat.Render(cmd.Stdout, rune(cmd.Args.CSVDelimiter), res)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rendering error: %s\n", err)
	}
	if cmd.Args.OpenWebBrowser {
		err = browser.OpenURL(res.URL)
		if err != nil {
			return fmt.Errorf("failed to open URL: %w", err)
		}
	}
	return nil
}
