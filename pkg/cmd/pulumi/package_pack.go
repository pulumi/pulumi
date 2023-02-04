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
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newPackagePackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pack <language> <package> <out>",
		Args:  cobra.ExactArgs(3),
		Short: "Pack a package into a packaged artifact.",
		Long: `Pack a package into a packaged artifact.

This publish`,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return packagePack(args[0], args[1], args[2])
		}),
	}
	return cmd
}

func packagePack(language, packagePath, outPath string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	ctx := context.Background()
	pctx, err := plugin.NewContextWithRoot(cmdutil.Diag(), cmdutil.Diag(), nil, cwd, cwd, nil, false, nil, nil)
	if err != nil {
		return err
	}

	lang, err := pctx.Host.LanguageRuntime(cwd, cwd, language, nil)
	if err != nil {
		return fmt.Errorf("failed to load language plugin %s: %w", language, err)
	} else {
		artifact, err := lang.PackPackage(ctx, packagePath, outPath, os.Stderr)
		if err != nil {
			return fmt.Errorf("failed to pack package: %w", err)
		}
		fmt.Printf("%s", artifact)
	}
	return nil
}
