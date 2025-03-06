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

package operations

import (
	"runtime"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest // Changes environment variables
func TestContinueOnErrorEnvVar(t *testing.T) {
	commands := []func() *cobra.Command{
		NewUpCmd,
		NewDestroyCmd,
	}
	testCases := []struct {
		EnvVarValue string
		Expected    bool
	}{
		{
			EnvVarValue: "true",
			Expected:    true,
		},
		{
			EnvVarValue: "1",
			Expected:    true,
		},
		{
			EnvVarValue: "false",
			Expected:    false,
		},
		{
			EnvVarValue: "0",
			Expected:    false,
		},
		{
			EnvVarValue: "",
			Expected:    false,
		},
	}

	for _, command := range commands {
		for _, test := range testCases {
			t.Setenv("PULUMI_CONTINUE_ON_ERROR", test.EnvVarValue)
			cmd := command()
			f, err := cmd.PersistentFlags().GetBool("continue-on-error")
			assert.Nil(t, err)
			assert.Equal(t, test.Expected, f)
		}
	}
}

//nolint:paralleltest // Changes environment variables
func TestParallelEnvVar(t *testing.T) {
	osDefaultParallel := int32(runtime.GOMAXPROCS(0)) * 4 //nolint:gosec
	commands := []func() *cobra.Command{
		NewUpCmd,
		NewPreviewCmd,
		NewRefreshCmd,
		NewDestroyCmd,
		NewImportCmd,
		NewWatchCmd,
	}
	testCases := []struct {
		EnvVarValue string
		Expected    int32
	}{
		{
			EnvVarValue: "4",
			Expected:    4,
		},
		{
			EnvVarValue: "1",
			Expected:    1,
		},
		{
			EnvVarValue: "0",
			Expected:    osDefaultParallel,
		},
		{
			EnvVarValue: "",
			Expected:    osDefaultParallel,
		},
		{
			EnvVarValue: "16",
			Expected:    16,
		},
	}

	for _, command := range commands {
		for _, test := range testCases {
			t.Setenv("PULUMI_PARALLEL", test.EnvVarValue)
			cmd := command()
			f, err := cmd.PersistentFlags().GetInt32("parallel")
			assert.Nil(t, err)
			assert.Equal(t, test.Expected, f)
		}
	}
}
