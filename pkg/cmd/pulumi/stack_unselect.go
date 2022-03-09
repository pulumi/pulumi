// Copyright 2016-2022, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

func newStackUnselectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unselect",
		Short: "Resets stack selection from the current workspace",
		Long: "Resets stack selection from the current workspace.\n" +
			"\n" +
			"This way, next time pulumi needs to execute an operation, the user is prompted with one of the stacks to select\n" +
			"from.\n",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			state.UnselectStack()
			return nil
		}),
	}
}
