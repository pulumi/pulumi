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

package backend

import (
	"os"
	"os/exec"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultLoginManagerDiagnosticColor(t *testing.T) {
	t.Parallel()

	const colorEnv = "PULUMI_TEST_LOGIN_MANAGER_DIAGNOSTIC_COLOR"
	if color := os.Getenv(colorEnv); color != "" {
		// Stack and policy commands construct their secrets providers before flags are parsed.
		require.NotNil(t, SecretsProvider(DefaultLoginManager))
		require.NoError(t, cmdutil.SetGlobalColorization(color))
		require.NotNil(t, DefaultLoginManager.ClientSource())
		cmdutil.Diag().Warningf(diag.Message("", "diagnostic color test"))
		return
	}

	// os.Executable returns this test binary. Run just this test in a fresh process
	// so package initialization happens before we set the requested color mode,
	// as it does when the CLI parses --color.
	executable, err := os.Executable()
	require.NoError(t, err)
	for _, color := range []string{"always", "never"} {
		t.Run(color, func(t *testing.T) {
			t.Parallel()
			cmd := exec.CommandContext(t.Context(), executable, "-test.run=^TestDefaultLoginManagerDiagnosticColor$")
			cmd.Env = append(os.Environ(), colorEnv+"="+color)
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s", output)
			assert.Contains(t, string(output), "diagnostic color test")
			if color == "always" {
				assert.Contains(t, string(output), "\x1b[")
			} else {
				assert.NotContains(t, string(output), "\x1b[")
			}
		})
	}
}
