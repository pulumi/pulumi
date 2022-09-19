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
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/engineInterface"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newHostEngineCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "host-engine",
		Short: "Starts the pulumi engine gRPC interface",
		Long: "Starts the pulumi engine gRPC interface.\n" +
			"\n",
		Args:   cmdutil.NoArgs,
		Hidden: !hasDebugCommands(),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {

			ctx := commandContext()
			engine, err := engineInterface.Start(ctx)
			if err != nil {
				return err
			}

			fmt.Println(engine.Address())

			return engine.Done()
		}),
	}

	return cmd
}
