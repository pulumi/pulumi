// Copyright 2016-2018, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type PolicyArgs struct{}

func newPolicyCmd(
	v *viper.Viper,
	parentPulumiCmd *cobra.Command,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage resource policies",
		Args:  cmdutil.NoArgs,
	}

	parentPulumiCmd.AddCommand(cmd)
	BindFlags[PolicyArgs](v, cmd)

	newPolicyDisableCmd(v, cmd)
	newPolicyEnableCmd(v, cmd)
	newPolicyGroupCmd(v, cmd)
	newPolicyLsCmd(v, cmd)
	newPolicyNewCmd(v, cmd)
	newPolicyPublishCmd(v, cmd)
	newPolicyRmCmd(v, cmd)
	newPolicyValidateCmd(v, cmd)

	return cmd
}
