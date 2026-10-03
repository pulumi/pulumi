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

package testing

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/agentdetect"
)

// IsolatedCredentials holds the temporary directories created by IsolateCredentials.
type IsolatedCredentials struct {
	// Home is the temporary PULUMI_HOME directory that holds default credentials.
	Home string
	// AgentDir is the temporary directory that holds shared agent credentials.
	AgentDir string
}

// IsolateCredentials clears inherited authentication and redirects default and agent credentials
// to separate temporary directories.
func IsolateCredentials(t testing.TB) IsolatedCredentials {
	t.Helper()

	dirs := IsolatedCredentials{Home: t.TempDir(), AgentDir: t.TempDir()}
	t.Setenv("PULUMI_CREDENTIALS_PATH", "")
	t.Setenv("PULUMI_HOME", dirs.Home)
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", dirs.AgentDir)

	// Explicitly setting PULUMI_HOME, as done above, disables agent fallback. Tests can re-enable it by setting
	// `PULUMI_TEST_ALLOW_AGENT_FALLBACK=true`.
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "")

	// Temporary files still share the OS credential-store key with real credentials.
	// Encryption tests must opt in with a fake store.
	t.Setenv("PULUMI_CREDENTIAL_STORE", "plaintext")

	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	t.Setenv("PULUMI_BACKEND_URL", "")
	t.Setenv("PULUMI_API", "")

	for _, name := range agentdetect.DetectionEnvVars() {
		t.Setenv(name, "")
	}
	return dirs
}
