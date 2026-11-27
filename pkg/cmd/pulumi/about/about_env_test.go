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

package about

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	cmdEnv "github.com/pulumi/pulumi/sdk/v3/go/common/env"
	declared "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

func runAboutEnv(t *testing.T, args ...string) string {
	t.Helper()
	cmd := newAboutEnvCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())
	return stdout.String()
}

//nolint:paralleltest // sets environment variables
func TestAboutEnvJSON(t *testing.T) {
	t.Setenv(cmdEnv.AccessToken.Var().Name(), "pul-not-a-real-token")
	t.Setenv(cmdEnv.BackendURL.Var().Name(), "file://~")

	for _, args := range [][]string{{"--json"}, {"--output=json"}} {
		t.Run(args[0], func(t *testing.T) {
			var vars []envVarAbout
			require.NoError(t, json.Unmarshal([]byte(runAboutEnv(t, args...)), &vars))

			byName := map[string]envVarAbout{}
			for _, v := range vars {
				byName[v.Name] = v
			}

			backendURL, ok := byName["PULUMI_BACKEND_URL"]
			require.True(t, ok)
			assert.Equal(t, cmdEnv.BackendURL.Var().Description, backendURL.Description)
			assert.Equal(t, `"file://~"`, backendURL.Value)

			accessToken, ok := byName["PULUMI_ACCESS_TOKEN"]
			require.True(t, ok)
			assert.Equal(t, "[secret]", accessToken.Value)

			skipUpdateCheck, ok := byName["PULUMI_SKIP_UPDATE_CHECK"]
			require.True(t, ok)
			assert.Equal(t, "unset", skipUpdateCheck.Value)
		})
	}
}

func TestAboutEnvTable(t *testing.T) {
	t.Parallel()

	out := runAboutEnv(t)
	assert.Contains(t, out, "Variable")
	assert.Contains(t, out, "PULUMI_BACKEND_URL")
	assert.False(t, json.Valid([]byte(out)))
}

func TestAboutEnvReportsEveryInvalidVariable(t *testing.T) {
	t.Setenv(cmdEnv.Parallel.Var().Name(), "abc")
	t.Setenv(cmdEnv.DIYBackendParallel.Var().Name(), "xyz")

	var stderr bytes.Buffer
	sink := diag.DefaultSink(&stderr, &stderr, diag.FormatOptions{Color: colors.Never})

	assert.True(t, reportInvalidEnvVars(sink, declared.Variables()))
	assert.Contains(t, stderr.String(), "PULUMI_PARALLEL:")
	assert.Contains(t, stderr.String(), "PULUMI_DIY_BACKEND_PARALLEL:")
}
