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

package packagecmd

import (
	"fmt"
	"os"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newPackagePackSdkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "pack-sdk",
		Short:  "Pack a package SDK to a language specific artifact.",
		Hidden: !env.Dev.Value(),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current working directory: %w", err)
			}

			reg := cmdCmd.NewDefaultRegistry(
				cmd.Context(), cmdBackend.DefaultLoginManager, pkgWorkspace.Instance, nil, cmdutil.Diag(), env.Global())
			pCtx, err := packages.NewPluginContext(cwd, reg)
			if err != nil {
				return fmt.Errorf("create plugin context: %w", err)
			}
			// The context owns its loader/mapper servers; the host is caller-owned. Close the
			// context first, then the host.
			defer contract.IgnoreClose(pCtx.Host)
			defer contract.IgnoreClose(pCtx)

			language := args[0]
			path := args[1]

			languagePlugin, err := pCtx.Host.LanguageRuntime(pCtx, language)
			if err != nil {
				return err
			}

			artifact, err := languagePlugin.Pack(pCtx.Request(), path, cwd)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s", artifact)

			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "language"},
			{Name: "path"},
		},
		Required: 2,
	})

	return cmd
}
