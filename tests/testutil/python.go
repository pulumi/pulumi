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

package testutil

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// SkipPythonDynamicProviders skips tests requiring dill on Python versions it does not yet support.
func SkipPythonDynamicProviders(t *testing.T) {
	t.Helper()

	cmd := exec.Command("python", "-c", "import sys; print(sys.version_info >= (3, 15))")
	output, err := cmd.Output()
	require.NoError(t, err)
	if strings.TrimSpace(string(output)) == "True" {
		t.Skip("Dynamic providers require a dill release including https://github.com/uqfoundation/dill/pull/755")
	}
}
