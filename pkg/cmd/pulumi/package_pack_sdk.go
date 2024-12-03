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

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newPackagePackCmd() *cobra.Command {
	var packCmd packCmd
	cmd := &cobra.Command{
		Use:    "pack-sdk <language> <path>",
		Args:   cobra.ExactArgs(2),
		Short:  "Pack a package SDK to a language specific artifact.",
		Hidden: !env.Dev.Value(),
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return packCmd.Run(ctx, args)
		}),
	}
	return cmd
}

type packCmd struct{}

func (cmd *packCmd) Run(ctx context.Context, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current working directory: %w", err)
	}

	pCtx, err := newPluginContext(cwd)
	if err != nil {
		return fmt.Errorf("create plugin context: %w", err)
	}
	defer contract.IgnoreClose(pCtx.Host)

	language := args[0]
	path := args[1]

	programInfo := plugin.NewProgramInfo(pCtx.Root, cwd, ".", nil)
	languagePlugin, err := pCtx.Host.LanguageRuntime(language, programInfo)
	if err != nil {
		return err
	}

	artifact, err := languagePlugin.Pack(path, cwd)
	if err != nil {
		return err
	}

	fmt.Printf("%s", artifact)

	return nil
}
