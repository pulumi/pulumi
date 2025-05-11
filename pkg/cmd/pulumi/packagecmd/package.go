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

package packagecmd

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

func NewPackageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Work with Pulumi packages",
		Long: `Work with Pulumi packages

Install and configure Pulumi packages and their plugins and SDKs.`,
		Args: cmdutil.NoArgs,
	}
	cmd.AddCommand(
		newExtractSchemaCommand(),
		newExtractMappingCommand(),
		newGenSdkCommand(),
		newPackagePublishSdkCmd(),
		newPackagePackSdkCmd(),
		newPackageAddCmd(),
		newPackagePublishCmd(),
		newPackageInfoCmd(),
	)
	return cmd
}
