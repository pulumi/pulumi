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

package logs

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/require"
)

func newTestEnv(t *testing.T) *ptesting.Environment {
	t.Helper()
	e := ptesting.NewEnvironment(t)
	t.Cleanup(func() { e.DeleteIfNotFailed() })
	return e
}

func writeGzipFile(t *testing.T, e *ptesting.Environment, name, content string) {
	t.Helper()
	path := filepath.Join(e.RootPath, name)
	f, err := os.Create(path)
	require.NoError(t, err)
	gz := gzip.NewWriter(f)
	_, err = gz.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	require.NoError(t, f.Close())
}

func findLogInDir(t *testing.T, homeDir string) string {
	t.Helper()
	logsDir := filepath.Join(homeDir, "logs")
	entries, err := os.ReadDir(logsDir)
	require.NoError(t, err)
	for i := len(entries) - 1; i >= 0; i-- {
		if strings.HasSuffix(entries[i].Name(), ".log") &&
			!strings.Contains(entries[i].Name(), ".shared.") {
			return filepath.Join(logsDir, entries[i].Name())
		}
	}
	t.Fatalf("no log files found in %s", logsDir)
	return ""
}

func assertFileHasPrefix(t *testing.T, path, prefix string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading %s", path)
	require.True(t, strings.HasPrefix(string(data), prefix),
		"expected %s to start with %q, got %q", path, prefix, string(data[:min(len(data), 20)]))
}

func assertFileDoesNotContain(t *testing.T, path, substr string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "reading %s", path)
	require.False(t, strings.Contains(string(data), substr),
		"expected %s to NOT contain %q", path, substr)
}
