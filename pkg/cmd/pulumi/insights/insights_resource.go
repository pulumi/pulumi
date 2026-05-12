// Copyright 2026, Pulumi Corporation.
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

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// TODO[https://github.com/pulumi/pulumi/issues/22973]: Not yet implemented.
func newInsightsResourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "resource",
		Short:  "Inspect resources discovered by Pulumi Insights",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newInsightsResourceGetCmd())
	cmd.AddCommand(newInsightsResourceSearchCmd())

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/22974]: Not yet implemented.
func newInsightsResourceGetCmd() *cobra.Command {
	var org string
	var account string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "get",
		Short:  "Get a discovered resource with its current version details",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "resource-type"},
			{Name: "resource-id"},
		},
		Required: 2,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization that owns the resource")
	cmd.Flags().StringVar(&account, "account", "", "The Insights account that owns the resource")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/22975]: Not yet implemented.
func newInsightsResourceSearchCmd() *cobra.Command {
	var (
		org        string
		query      string
		sort       string
		page       int
		size       int
		cursor     string
		properties bool
		collapse   bool
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "search",
		Short:  "Search for resources within an organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&org, "org", "", "The organization to search within")
	cmd.Flags().StringVarP(&query, "query", "q", "", "The query string to filter resources by")
	cmd.Flags().StringVar(&sort, "sort", "", "The field to sort results by")
	cmd.Flags().IntVar(&page, "page", 0, "The page of results to return (max 10,000 total)")
	cmd.Flags().IntVar(&size, "page-size", 0, "The number of results per page")
	cmd.Flags().StringVar(&cursor, "cursor", "", "The cursor to continue pagination from (Enterprise only)")
	cmd.Flags().BoolVar(&properties, "properties", false, "Include resource input/output values in results")
	cmd.Flags().BoolVar(&collapse, "collapse", false, "Consolidate resources that exist in multiple sources")

	return cmd
}
