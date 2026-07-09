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

package workspace

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func policyPackWithBinaries(t *testing.T, binaries map[string]any) *PolicyPackProject {
	proj := &PolicyPackProject{Runtime: NewProjectRuntimeInfo("executable", map[string]any{
		"binaries": binaries,
	})}
	require.NoError(t, proj.Validate())
	return proj
}

func TestExecutableBinaries(t *testing.T) {
	t.Parallel()

	t.Run("valid map", func(t *testing.T) {
		t.Parallel()
		proj := policyPackWithBinaries(t, map[string]any{
			"linux-amd64":   "bin/pack-linux-amd64",
			"darwin-arm64":  "bin/pack-darwin-arm64",
			"windows-amd64": "bin/pack-windows-amd64.exe",
		})
		binaries, err := proj.ExecutableBinaries()
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"linux-amd64":   "bin/pack-linux-amd64",
			"darwin-arm64":  "bin/pack-darwin-arm64",
			"windows-amd64": "bin/pack-windows-amd64.exe",
		}, binaries)
	})

	t.Run("missing binaries option", func(t *testing.T) {
		t.Parallel()
		proj := &PolicyPackProject{Runtime: NewProjectRuntimeInfo("executable", nil)}
		_, err := proj.ExecutableBinaries()
		assert.ErrorContains(t, err, "'binaries'")
	})

	t.Run("empty map", func(t *testing.T) {
		t.Parallel()
		proj := policyPackWithBinaries(t, map[string]any{})
		_, err := proj.ExecutableBinaries()
		assert.ErrorContains(t, err, "'binaries'")
	})

	t.Run("unknown platform key", func(t *testing.T) {
		t.Parallel()
		proj := policyPackWithBinaries(t, map[string]any{"linux-mips": "bin/p"})
		_, err := proj.ExecutableBinaries()
		assert.ErrorContains(t, err, "linux-mips")
	})

	t.Run("absolute path", func(t *testing.T) {
		t.Parallel()
		abs := "/usr/bin/pack"
		if runtime.GOOS == "windows" {
			abs = `C:\bin\pack.exe`
		}
		proj := policyPackWithBinaries(t, map[string]any{"linux-amd64": abs})
		_, err := proj.ExecutableBinaries()
		assert.ErrorContains(t, err, "relative")
	})

	t.Run("path escaping pack directory", func(t *testing.T) {
		t.Parallel()
		proj := policyPackWithBinaries(t, map[string]any{"linux-amd64": "../outside/pack"})
		_, err := proj.ExecutableBinaries()
		assert.ErrorContains(t, err, "escape")
	})

	t.Run("non-string value", func(t *testing.T) {
		t.Parallel()
		proj := policyPackWithBinaries(t, map[string]any{"linux-amd64": 42})
		_, err := proj.ExecutableBinaries()
		assert.ErrorContains(t, err, "linux-amd64")
	})
}

func TestCurrentPlatform(t *testing.T) {
	t.Parallel()
	assert.Equal(t, fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH), CurrentPlatform())
}
