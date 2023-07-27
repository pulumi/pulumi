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
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	envutil "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var orgName string
	var queryParams *[]string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search for resources in Pulumi Cloud",
		Long:  "Search for resources in Pulumi Cloud.",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
			interactive := cmdutil.Interactive()

			opts := backend.QueryOptions{}
			opts.Display = display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: interactive,
				Type:          display.DisplayQuery,
			}
			// Try to read the current project
			project, _, err := readProject()
			if err != nil {
				return err
			}

			b, err := currentBackend(ctx, project, opts.Display)
			if err != nil {
				return err
			}
			fmt.Println(queryParams)
			parsedQueryParams := parseQueryParams(queryParams)
			userName, orgs, err := b.CurrentUser()
			if err != nil {
				return err
			}
			var filterName string
			if orgName == "" {
				filterName = userName
			} else {
				filterName = orgName
			}
			if !sliceContains(orgs, orgName) && orgName != "" {
				return fmt.Errorf("user %s is not a member of org %s", userName, orgName)
			}

			res, err := b.Search(ctx, filterName, parsedQueryParams)
			if err != nil {
				return err
			}
			for _, r := range res.Resources {
				fmt.Printf("%s - %s\n", *r.Name, *r.Type)
			}
			return nil
		},
		),
	}

	// TODO: Remove this branch once we release this feature fully.
	if envutil.BoolValue(env.Dev).Value() {
		cmd.AddCommand(newAISearchCmd())
	}

	cmd.PersistentFlags().StringVarP(
		&orgName, "org", "o", "",
		"Allow P resource operations to run in parallel at once (1 for no parallelism). Defaults to unbounded.",
	)
	queryParams = cmd.PersistentFlags().StringArrayP(
		"query", "q", []string{},
		"Key-value pairs to use as query parameters. Must be formatted like: -q key1=value1 -q key2=value2",
	)

	return cmd
}

func newAISearchCmd() *cobra.Command {
	var queryString *string
	var orgName string
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "Search for resources in Pulumi Cloud using Pulumi AI",
		Long:  "Search for resources in Pulumi Cloud using Pulumi AI",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
			interactive := cmdutil.Interactive()

			opts := backend.QueryOptions{}
			opts.Display = display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: interactive,
				Type:          display.DisplayQuery,
			}
			// Try to read the current project
			project, _, err := readProject()
			if err != nil {
				return err
			}

			b, err := currentBackend(ctx, project, opts.Display)
			if err != nil {
				return err
			}
			fmt.Println(queryString)
			userName, orgs, err := b.CurrentUser()
			if err != nil {
				return err
			}
			var filterName string
			if orgName == "" {
				filterName = userName
			} else {
				filterName = orgName
			}
			if !sliceContains(orgs, orgName) && orgName != "" {
				return fmt.Errorf("user %s is not a member of org %s", userName, orgName)
			}

			res, err := b.NaturalLanguageSearch(ctx, filterName, *queryString)
			if err != nil {
				return err
			}
			for _, r := range res.Resources {
				fmt.Printf("%s - %s\n", *r.Name, *r.Type)
			}
			return nil
		},
		),
	}
	cmd.PersistentFlags().StringVarP(
		&orgName, "org", "o", "",
		"Allow P resource operations to run in parallel at once (1 for no parallelism). Defaults to unbounded.",
	)
	queryString = cmd.PersistentFlags().StringP(
		"query", "q", "",
		"Plaintext natural language query",
	)

	return cmd
}

func parseQueryParams(rawParams *[]string) interface{} {
	return apitype.PulumiQueryRequest{Query: strings.Join(*rawParams, "&")}
}

func sliceContains(slice []string, search string) bool {
	for _, s := range slice {
		if s == search {
			return true
		}
	}
	return false
}
