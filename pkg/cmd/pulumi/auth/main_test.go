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

package auth

import (
	"fmt"
	"os"
	"testing"
)

// TestMain also runs this test executable as a credential helper subprocess.
// Tests use os.Executable() with -credential-helper-test, a request log path, and a JSON response.
// It records the request from stdin and writes the response to stdout. Other invocations run tests normally.
func TestMain(m *testing.M) {
	for i, arg := range os.Args {
		if arg == "-credential-helper-test" {
			os.Exit(runCredentialsLoginHelper(os.Args[i+1:]))
		}
	}
	// The OS credential store holds one key for all credentials files. With a mode that the
	// developer set, `pulumi logout --all` in a test deletes the key that protects the real
	// credentials of the developer.
	//
	//nolint:forbidigo // TestMain has no t
	if err := os.Unsetenv("PULUMI_CREDENTIAL_STORE"); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
