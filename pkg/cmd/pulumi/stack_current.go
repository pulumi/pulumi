// Copyright 2016-2021, Pulumi Corporation.
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

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Returns a command that displays the name of the current stack quickly.
func newStackCurrentNameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "current-name",
		Short:  "Display the name of the current stack",
		Args:   cmdutil.NoArgs,
		Hidden: true,
		Run: cmdutil.RunFunc(func(_ *cobra.Command, _ []string) error {
			w, err := workspace.New()
			if err != nil {
				return err
			}
			s := w.Settings().Stack
			fmt.Print(s)
			return err
		}),
	}
	return cmd
}
