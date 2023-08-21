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

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

type searchAICmd struct {
	orgName     string
	queryString string

	Stdout io.Writer // defaults to os.Stdout

	// currentBackend is a reference to the top-level currentBackend function.
	// This is used to override the default implementation for testing purposes.
	currentBackend func(context.Context, *workspace.Project, display.Options) (backend.Backend, error)
}

func (cmd *searchAICmd) Run(ctx context.Context, args []string) error {
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
	var filterName string
	if cmd.orgName == "" {
		filterName = userName
	} else {
		filterName = cmd.orgName
	}
	if !sliceContains(orgs, cmd.orgName) && cmd.orgName != "" {
		return fmt.Errorf("user %s is not a member of org %s", userName, cmd.orgName)
	}

	res, err := cloudBackend.NaturalLanguageSearch(ctx, filterName, cmd.queryString)
	if err != nil {
		return err
	}
	err = cmd.RenderTable(res.Resources)
	if err != nil {
		fmt.Fprintf(os.Stderr, "table rendering error: %s\n", err)
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
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
			return scmd.Run(ctx, args)
		},
		),
	}
	cmd.PersistentFlags().StringVarP(
		&scmd.orgName, "org", "o", "",
		"Organization name to search within",
	)
	cmd.PersistentFlags().StringVarP(
		&scmd.queryString, "query", "q", "",
		"Plaintext natural language query",
	)

	return cmd
}

func (cmd *searchAICmd) RenderTable(results []apitype.ResourceResult) error {
	return renderSearchTable(cmd.Stdout, results)
}
