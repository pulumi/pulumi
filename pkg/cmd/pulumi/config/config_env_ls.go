// Copyright 2016-2024, Pulumi Corporation.
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

package config

import (
	"context"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

func newConfigEnvLsCmd(parent *configEnvCmd) *cobra.Command {
	var jsonOut bool

	impl := configEnvLsCmd{parent: parent, jsonOut: &jsonOut}

	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lists imported environments.",
		Long:  "Lists the environments imported into a stack's configuration.",
		Args:  cmdutil.NoArgs,
		RunE: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			parent.initArgs()
			return impl.run(cmd.Context(), args)
		}),
	}

	cmd.Flags().BoolVarP(
		&jsonOut, "json", "j", false,
		"Emit output as JSON")

	return cmd
}

type configEnvLsCmd struct {
	parent *configEnvCmd

	jsonOut *bool
}

func (cmd *configEnvLsCmd) run(ctx context.Context, _ []string) error {
	return cmd.parent.listStackEnvironments(ctx, *cmd.jsonOut)
}
