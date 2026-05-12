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

package templatecmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// TODO[https://github.com/pulumi/pulumi/issues/22961]: Not yet implemented.
func newTemplateListCmd() *cobra.Command {
	var (
		org    string
		search string
		token  string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "List registry-backed templates",
		Long: "List registry-backed templates.\n" +
			"\n" +
			"Results can be filtered by template name and owning organization,\n" +
			"and searched by name, description, metadata, and runtime.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&org, "org", "", "Filter templates by owning organization")
	cmd.Flags().StringVar(&search, "search", "", "Case-insensitive partial match against template metadata")
	cmd.Flags().StringVar(&token, "continuation-token", "", "The continuation token for paginated retrieval")

	return cmd
}
