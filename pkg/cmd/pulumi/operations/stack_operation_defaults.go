// Copyright 2026, Pulumi Corporation.
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

package operations

import (
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func stackOperationOverrides(cmd *cobra.Command, proj *workspace.Project,
	opts engine.UpdateOptions,
) *apitype.StackOperationDefaults {
	overrides := &apitype.StackOperationDefaults{}
	if cmd.Flags().Changed("refresh") || (proj.Options != nil && proj.Options.Refresh != "") {
		overrides.Refresh = new(opts.Refresh)
	}
	if _, set := env.ContinueOnError.Underlying(); cmd.Flags().Changed("continue-on-error") || set {
		overrides.ContinueOnError = new(opts.ContinueOnError)
	}
	if _, set := env.RunProgram.Underlying(); cmd.Flags().Changed("run-program") || set {
		overrides.RunProgram = new(opts.RefreshProgram || opts.DestroyProgram)
	}
	if _, set := env.Parallel.Underlying(); cmd.Flags().Changed("parallel") || set {
		overrides.Parallel = new(int(opts.Parallel))
	}
	return overrides
}
