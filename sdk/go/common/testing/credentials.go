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

// IsolateCredentials clears inherited authentication and redirects default and agent credentials
// to separate temporary directories. It returns the temporary PULUMI_HOME directory.
func IsolateCredentials(t testing.TB) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("PULUMI_CREDENTIALS_PATH", "")
	t.Setenv("PULUMI_HOME", home)
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())

	// An explicit PULUMI_HOME disables agent fallback; tests can opt in with this hook.
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
	return home
}
