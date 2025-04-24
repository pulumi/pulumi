// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package newcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorIfNotEmptyDirectory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc  string
		files []string
		dirs  []string
		ok    bool
	}{
		{
			desc: "empty",
			ok:   true,
		},
		{
			desc:  "non-empty",
			files: []string{"foo"},
			dirs:  []string{"bar"},
			ok:    false,
		},
		{
			desc: "empty git repository",
			dirs: []string{".git"},
			ok:   true,
		},
		{
			desc:  "non-empty git repository",
			dirs:  []string{".git"},
			files: []string{".gitignore"},
			ok:    false,
		},
		{
			desc: "every VCS",
			dirs: []string{".git", ".hg", ".bzr"},
			ok:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			path := t.TempDir()

			// Fill test directory with files and directories
			// requested by the test case.
			for _, name := range tt.dirs {
				err := os.MkdirAll(filepath.Join(path, name), 0o1700)
				require.NoError(t, err)
			}
			for _, name := range tt.files {
				err := os.WriteFile(filepath.Join(path, name), nil /* body */, 0o600)
				require.NoError(t, err)
			}

			err := ErrorIfNotEmptyDirectory(path)
			if tt.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
