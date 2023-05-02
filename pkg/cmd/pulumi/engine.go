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
	"context"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/pulumi"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

func newRunEngineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "engine",
		Short: "Starts the engine",
		Long: "Starts the engine\n" +
			"\n" +
			"The engine will serve on localhost and will print it's address to stdout.",
		Hidden: true,
		Args:   cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return runEngine(commandContext())
		}),
	}
	return cmd
}

func runEngine(ctx context.Context) error {
	engine, err := pulumi.Start(ctx)
	if err != nil {
		return err
	}

	os.Stdout.WriteString(engine.Address())
	os.Stdout.Close()

	return engine.Done()
}
