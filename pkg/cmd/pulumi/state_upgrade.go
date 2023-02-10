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
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/filestate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"

	"github.com/spf13/cobra"
)

func newStateUpgradeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Migrates the current backend to the latest supported version",
		Long: `Migrates the current backend to the latest supported version

This only has an effect on the filestate backend.
`,
		Args: cmdutil.ExactArgs(1),
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			ctx := commandContext()

			b, err := currentBackend(ctx, nil, display.Options{Color: cmdutil.GetGlobalColorization()})
			if err != nil {
				return result.FromError(err)
			}

			if lb, is := b.(filestate.Backend); is {
				err = lb.Upgrade(ctx)
				if err != nil {
					return result.FromError(err)
				}
			}

			return nil
		}),
	}
	return cmd
}
