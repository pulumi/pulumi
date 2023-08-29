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

	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	auto_table "go.pennock.tech/tabular/auto"
)

type searchCmd struct {
	orgName     string
	queryParams []string

	Stdout io.Writer // defaults to os.Stdout

	// currentBackend is a reference to the top-level currentBackend function.
	// This is used to override the default implementation for testing purposes.
	currentBackend func(context.Context, *workspace.Project, display.Options) (backend.Backend, error)
}

func (cmd *searchCmd) Run(ctx context.Context, args []string) error {
	interactive := cmdutil.Interactive()

	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
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
	userName, orgs, err := cloudBackend.CurrentUser()
	if err != nil {
		return err
	}
	filterName := userName
	if cmd.orgName != "" {
		if !sliceContains(orgs, cmd.orgName) {
			return fmt.Errorf("user %s is not a member of organization %s", userName, cmd.orgName)
		}
		filterName = cmd.orgName
	}

	parsedQueryParams := apitype.ParseQueryParams(cmd.queryParams)
	res, err := cloudBackend.Search(ctx, filterName, parsedQueryParams)
	if err != nil {
		return err
	}
	err = cmd.RenderTable(res.Resources)
	if err != nil {
		return fmt.Errorf("table rendering error: %s", err)
	}
	return nil
}

func newSearchCmd() *cobra.Command {
	var scmd searchCmd
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search for resources in Pulumi Cloud",
		Long:  "Search for resources in Pulumi Cloud.",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
			return scmd.Run(ctx, args)
		},
		),
	}

	// TODO: Remove this branch once we release this feature fully.
	if env.Dev.Value() {
		cmd.AddCommand(newSearchAICmd())
	}

	cmd.PersistentFlags().StringVar(
		&scmd.orgName, "org", "",
		"Name of the organization to search. Defaults to the current user's organization.",
	)
	cmd.PersistentFlags().StringArrayVarP(
		&scmd.queryParams, "query", "q", nil,
		"Key-value pairs to use as query parameters. "+
			"Must be formatted like: -q key1=value1 -q key2=value2. "+
			"Alternately, each parameter provided here can be in raw Pulumi query syntax form.",
	)

	return cmd
}

func sliceContains(slice []string, search string) bool {
	for _, s := range slice {
		if s == search {
			return true
		}
	}
	return false
}

func renderSearchTable(w io.Writer, results []apitype.ResourceResult) error {
	table := auto_table.New("utf8-heavy")
	table.AddHeaders("Project", "Stack", "Name", "Type", "Package", "Module", "Modified")
	for _, r := range results {
		table.AddRowItems(*r.Program, *r.Stack, *r.Name, *r.Type, *r.Package, *r.Module, *r.Modified)
	}
	var err error
	if errs := table.Errors(); errs != nil {
		for _, tableErr := range errs {
			err = multierror.Append(err, tableErr)
		}
		return err
	}
	return table.RenderTo(w)
}

func (cmd *searchCmd) RenderTable(results []apitype.ResourceResult) error {
	return renderSearchTable(cmd.Stdout, results)
}
