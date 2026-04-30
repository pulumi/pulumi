// Copyright 2016, Pulumi Corporation.
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

package ai

import (
	"context"
	"io"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/spf13/cobra"
)

type aiCmd struct {
	Stdout io.Writer // defaults to os.Stdout
}

func (cmd *aiCmd) Run(ctx context.Context, args []string) error {
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}

	return nil
}

func NewAICommand() *cobra.Command {
	var aiCommand aiCmd
	cmd := &cobra.Command{
		Use:    "ai",
		Short:  "Basic Pulumi AI CLI commands.",
		Long:   "Contains the current set of supported CLI functionality for the Pulumi AI service.",
		Hidden: !env.Experimental.Value(),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if len(args) == 0 {
				return cmd.Help()
			}
			return aiCommand.Run(ctx, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newAIWebCommand())
	return cmd
}
