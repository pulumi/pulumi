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
	"fmt"
	"os"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newPackagePackCmd() *cobra.Command {
	var packCmd packCmd
	cmd := &cobra.Command{
		Use:    "pack-sdk <language> <version> <path>",
		Args:   cobra.ExactArgs(3),
		Short:  "Pack a package SDK to a language specific artifact.",
		Hidden: !env.Dev.Value(),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
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
	version := args[1]
	path := args[2]

	var v semver.Version
	if v, err = semver.ParseTolerant(version); err != nil {
		return fmt.Errorf("invalid version %q: %w", version, err)
	}

	languagePlugin, err := pCtx.Host.LanguageRuntime(cwd, cwd, language, nil)
	if err != nil {
		return err
	}

	artifact, err := languagePlugin.Pack(path, v, cwd)
	if err != nil {
		return err
	}

	fmt.Printf("%s", artifact)

	return nil
}
