// Copyright 2025, Pulumi Corporation.
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

package project

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// NewProjectCmd creates a new command that manages Pulumi projects.
func NewProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage Pulumi projects",
		Long: "Manage Pulumi projects.\n" +
			"\n" +
			"This command can be used to manage Pulumi projects. Projects are the " +
			"unit of organization in Pulumi, and contain multiple stacks.",
		Args: cmdutil.NoArgs,
	}

	// Add subcommands
	cmd.AddCommand(newProjectLsCmd())

	return cmd
}

// GetOrgFromStackName extracts the organization name from a stack reference.
// The stack reference can be in the format "org/project/stack" or "project/stack".
// Returns an empty string if no organization is specified.
func GetOrgFromStackName(stackRef string) string {
	parts := strings.Split(stackRef, "/")
	if len(parts) < 3 {
		return ""
	}
	return parts[0]
}
