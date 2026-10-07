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

package sandbox

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

func TestSandboxEnv(t *testing.T) {
	srv, _ := fakeFloci(t, true)
	t.Setenv("FLOCI_ENDPOINT", srv.URL)
	t.Setenv("FLOCI_GCP_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("AWS_REGION", "eu-west-1")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("CLOUDSDK_CORE_PROJECT", "")

	var warnings bytes.Buffer
	vars, err := sandboxEnv(t.Context(), Emulators, nil, &warnings)
	require.NoError(t, err)

	assert.Equal(t, srv.URL, vars["AWS_ENDPOINT_URL"])
	assert.Equal(t, "test", vars["AWS_ACCESS_KEY_ID"])
	assert.Equal(t, "", vars["AWS_PROFILE"], "real credentials are unset")
	assert.Equal(t, "eu-west-1", vars["AWS_REGION"], "the region falls back to the environment")
	assert.Equal(t, "eu-west-1", vars["AWS_DEFAULT_REGION"], "both region variables agree")
	assert.Equal(t, "http://127.0.0.1:1", vars["STORAGE_EMULATOR_HOST"])
	assert.Equal(t, "127.0.0.1:1", vars["PUBSUB_EMULATOR_HOST"])
	assert.Equal(t, "floci-local", vars["GOOGLE_CLOUD_PROJECT"], "the project falls back to floci-gcp's default")
	assert.Contains(t, warnings.String(), "floci-gcp (Google Cloud) isn't running")
	assert.NotContains(t, warnings.String(), "floci (AWS)")

	cfg := config.Map{
		config.MustParseKey("aws:region"):  config.NewValue("us-west-2"),
		config.MustParseKey("gcp:project"): config.NewSecureValue("c2VjcmV0"),
	}
	vars, err = sandboxEnv(t.Context(), Emulators[:1], cfg, &warnings)
	require.NoError(t, err)
	assert.Equal(t, "us-west-2", vars["AWS_REGION"], "the stack's region wins")
	assert.NotContains(t, vars, "GOOGLE_CLOUD_PROJECT", "only the requested emulator is printed")
}

func TestShellFormats(t *testing.T) {
	t.Parallel()

	vars := map[string]string{"AWS_PROFILE": "", "AWS_ENDPOINT_URL": "http://localhost:4566", "Q": "it's"}
	for shell, want := range map[string]string{
		"bash": "export AWS_ENDPOINT_URL='http://localhost:4566'\nunset AWS_PROFILE\nexport Q='it'\\''s'\n\n" +
			"# Run: eval $(pulumi sandbox env)\n",
		"fish": "set -gx AWS_ENDPOINT_URL 'http://localhost:4566'\nset -e AWS_PROFILE\nset -gx Q 'it\\'s'\n\n" +
			"# Run: pulumi sandbox env --shell fish | source\n",
		"powershell": "$env:AWS_ENDPOINT_URL = 'http://localhost:4566'\n" +
			"Remove-Item Env:AWS_PROFILE -ErrorAction SilentlyContinue\n$env:Q = 'it''s'\n\n" +
			"# Run: pulumi sandbox env --shell powershell | Invoke-Expression\n",
	} {
		var out bytes.Buffer
		shellFormats[shell].write(&out, vars)
		assert.Equal(t, want, out.String(), shell)
	}
}

// The command loads the current stack's config through the real CLI machinery; outside a project it must quietly
// fall back rather than fail.
func TestEnvCmdOutsideProject(t *testing.T) {
	srv, _ := fakeFloci(t, true)
	t.Setenv("FLOCI_ENDPOINT", srv.URL)
	t.Chdir(t.TempDir())

	cmd := newSandboxEnvCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"aws"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "export AWS_ENDPOINT_URL='"+srv.URL+"'")
	assert.Contains(t, out.String(), "unset AWS_PROFILE")
}
