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

func newPackagePublishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish <language> <packaged artifact>",
		Args:  cobra.ExactArgs(2),
		Short: "Publish a packaged artifact to the languages package repository.",
		Long: `Publish a packaged artifact to the languages package repository.

This publish`,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return packagePublish(args[0], args[1])
		}),
	}
	return cmd
}

func packagePublish(language, artifact string) error {
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
		err := lang.PublishPackage(ctx, artifact, os.Stderr)
		if err != nil {
			return fmt.Errorf("failed to publish package: %w", err)
		}
	}
	return nil
}
