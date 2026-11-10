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

package terminalstatus

import (
	"bytes"
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedirectedOutput(t *testing.T) {
	t.Parallel()

	t.Run("buffer", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		clearStatus := Working(&out, "Updating")
		defer clearStatus()
		NeedsInput(&out)()
		assert.Empty(t, out.String())
	})
	t.Run("file", func(t *testing.T) {
		t.Parallel()
		out, err := os.CreateTemp(t.TempDir(), "output")
		require.NoError(t, err)
		defer out.Close()
		Working(out, "Updating")()
		NeedsInput(out)()
		info, err := out.Stat()
		require.NoError(t, err)
		assert.Zero(t, info.Size())
	})
}

func TestEncodeMessage(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, message, want string
	}{
		{name: "empty"},
		{name: "unicode", message: "Updating π", want: "Updating π"},
		{name: "controls", message: "Up\n\r\t\x1b\x7f\u0085dating", want: "Updating"},
		{name: "invalid UTF-8", message: "Updating \xff", want: "Updating �"},
		{name: "limit", message: strings.Repeat("a", 2049), want: strings.Repeat("a", 2048)},
		{name: "whole runes", message: strings.Repeat("a", 2047) + "π", want: strings.Repeat("a", 2047)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			decoded, err := base64.StdEncoding.DecodeString(encodeMessage(tt.message))
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(decoded))
		})
	}
}
