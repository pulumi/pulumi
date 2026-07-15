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

package noderesolver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveName(t *testing.T) {
	t.Parallel()

	name, err := archiveName("22.12.0", "linux", "amd64")
	require.NoError(t, err)
	assert.Equal(t, "node-v22.12.0-linux-x64.tar.gz", name)

	name, err = archiveName("22.12.0", "darwin", "arm64")
	require.NoError(t, err)
	assert.Equal(t, "node-v22.12.0-darwin-arm64.tar.gz", name)

	name, err = archiveName("22.12.0", "windows", "amd64")
	require.NoError(t, err)
	assert.Equal(t, "node-v22.12.0-win-x64.zip", name)

	_, err = archiveName("22.12.0", "plan9", "amd64")
	assert.ErrorContains(t, err, "plan9")
	assert.ErrorContains(t, err, "supported")

	_, err = archiveName("22.12.0", "linux", "mips")
	assert.ErrorContains(t, err, "mips")
}

func TestLayoutBinDir(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		filepath.Join("root", "node-v22.12.0-linux-x64", "bin"),
		layoutBinDir("root", "node-v22.12.0-linux-x64", "linux"))
	assert.Equal(t,
		filepath.Join("root", "node-v22.12.0-win-x64"),
		layoutBinDir("root", "node-v22.12.0-win-x64", "windows"))
}

func TestBaseURL(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https://nodejs.org/dist", Spec{}.baseURL())
	assert.Equal(t, "https://mirror.example.com", Spec{BaseURL: "https://mirror.example.com/"}.baseURL())
}
