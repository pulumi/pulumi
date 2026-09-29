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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestStackOperationOverrides(t *testing.T) {
	for _, name := range []string{
		env.RunProgram.Var().Name(), env.ContinueOnError.Var().Name(), env.Parallel.Var().Name(),
	} {
		ptesting.Unsetenv(t, name)
	}
	for _, tt := range []struct {
		name           string
		args           []string
		environment    map[string]string
		projectRefresh string
		engine         engine.UpdateOptions
		want           apitype.StackOperationDefaults
	}{
		{name: "unset"},
		{name: "implicit parallel", engine: engine.UpdateOptions{Parallel: 16}},
		{
			name: "explicit parallel", args: []string{"--parallel=2"},
			engine: engine.UpdateOptions{Parallel: 2}, want: apitype.StackOperationDefaults{Parallel: new(2)},
		},
		{
			name: "explicit unlimited", args: []string{"--parallel=0"},
			want: apitype.StackOperationDefaults{Parallel: new(0)},
		},
		{
			name: "parallel environment", environment: map[string]string{"PULUMI_PARALLEL": "3"},
			engine: engine.UpdateOptions{Parallel: 3}, want: apitype.StackOperationDefaults{Parallel: new(3)},
		},
		{
			name: "parallel flag beats environment", args: []string{"--parallel=1"},
			environment: map[string]string{"PULUMI_PARALLEL": "3"},
			engine:      engine.UpdateOptions{Parallel: 1}, want: apitype.StackOperationDefaults{Parallel: new(1)},
		},
		{
			name: "explicit false", args: []string{"--refresh=false", "--run-program=false", "--continue-on-error=false"},
			want: apitype.StackOperationDefaults{Refresh: new(false), RunProgram: new(false), ContinueOnError: new(false)},
		},
		{
			name: "explicit true", args: []string{"--refresh", "--run-program", "--continue-on-error"},
			engine: engine.UpdateOptions{Refresh: true, RefreshProgram: true, ContinueOnError: true},
			want:   apitype.StackOperationDefaults{Refresh: new(true), RunProgram: new(true), ContinueOnError: new(true)},
		},
		{
			name:        "environment false",
			environment: map[string]string{"PULUMI_RUN_PROGRAM": "false", "PULUMI_CONTINUE_ON_ERROR": "0"},
			want:        apitype.StackOperationDefaults{RunProgram: new(false), ContinueOnError: new(false)},
		},
		{
			name:        "environment true",
			environment: map[string]string{"PULUMI_RUN_PROGRAM": "true", "PULUMI_CONTINUE_ON_ERROR": "1"},
			engine:      engine.UpdateOptions{DestroyProgram: true, ContinueOnError: true},
			want:        apitype.StackOperationDefaults{RunProgram: new(true), ContinueOnError: new(true)},
		},
		{
			name: "flag beats environment", args: []string{"--run-program=false", "--continue-on-error=false"},
			environment: map[string]string{"PULUMI_RUN_PROGRAM": "true", "PULUMI_CONTINUE_ON_ERROR": "true"},
			want:        apitype.StackOperationDefaults{RunProgram: new(false), ContinueOnError: new(false)},
		},
		{
			name: "project always", projectRefresh: "always", engine: engine.UpdateOptions{Refresh: true},
			want: apitype.StackOperationDefaults{Refresh: new(true)},
		},
		{name: "project never", projectRefresh: "never", want: apitype.StackOperationDefaults{Refresh: new(false)}},
		{
			name: "flag beats project", args: []string{"--refresh=false"}, projectRefresh: "always",
			want: apitype.StackOperationDefaults{Refresh: new(false)},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for name, value := range tt.environment {
				t.Setenv(name, value)
			}
			cmd := &cobra.Command{}
			cmd.Flags().String("refresh", "", "")
			cmd.Flags().Lookup("refresh").NoOptDefVal = "true"
			cmd.Flags().Bool("run-program", false, "")
			cmd.Flags().Bool("continue-on-error", false, "")
			cmd.Flags().Int32P("parallel", "p", 16, "")
			require.NoError(t, cmd.ParseFlags(tt.args))
			proj := &workspace.Project{Options: &workspace.ProjectOptions{Refresh: tt.projectRefresh}}
			require.Equal(t, &tt.want, stackOperationOverrides(cmd, proj, tt.engine))
		})
	}
}
