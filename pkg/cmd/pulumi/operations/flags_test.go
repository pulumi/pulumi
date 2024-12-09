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
