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

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactorMultilineSecrets(t *testing.T) {
	t.Parallel()

	const (
		key   = "-----BEGIN TEST KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEF\n-----END TEST KEY-----\n"
		cert  = "-----BEGIN CERTIFICATE-----\nMIIDdzCCAl+gAwIBAgIEAgAAuTAN\n-----END CERTIFICATE-----\n"
		chain = "-----BEGIN CERTIFICATE-----\nMIIDrzCCApegAwIBAgIQCDvgVpBC\n-----END CERTIFICATE-----\n"
	)

	tests := []struct {
		name     string
		secrets  []string
		output   string
		expected string
	}{
		{
			name:     "multi-line secret",
			secrets:  []string{key, "hunter2"},
			output:   "key:\n" + key + "password: hunter2\n",
			expected: "key:\n[secret]password: [secret]\n",
		},
		{
			name:     "no trailing newline",
			secrets:  []string{strings.TrimSuffix(key, "\n")},
			output:   "key: " + key + "done",
			expected: "key: [secret]\ndone",
		},
		{
			// The last lines of the first secret begin the second; holding those back must not split the first.
			name:     "overlapping secrets",
			secrets:  []string{key + cert, cert + chain},
			output:   key + cert + "done\n",
			expected: "[secret]done\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			multiline := multilineSecrets(tt.secrets)
			replacer := newReplacer(tt.secrets)
			for _, size := range []int{len(tt.output), 1} {
				var out bytes.Buffer
				r := newRedactor(&out, replacer, multiline)
				for b := []byte(tt.output); len(b) > 0; b = b[min(size, len(b)):] {
					_, err := r.Write(b[:min(size, len(b))])
					require.NoError(t, err)
				}
				require.NoError(t, r.Close())
				assert.Equal(t, tt.expected, out.String(), "writes of %d bytes", size)
			}
		})
	}

	t.Run("unrelated lines are not held back", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer
		r := newRedactor(&out, newReplacer([]string{key}), multilineSecrets([]string{key}))
		_, err := r.Write([]byte("plain\n-----BEGIN TEST KEY-----\n"))
		require.NoError(t, err)
		assert.Equal(t, "plain\n", out.String())

		_, err = r.Write([]byte("other\n"))
		require.NoError(t, err)
		assert.Equal(t, "plain\n-----BEGIN TEST KEY-----\nother\n", out.String())
	})
}
