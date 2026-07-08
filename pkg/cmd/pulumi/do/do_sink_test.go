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

package do

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

func TestForwardingSink(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	base := diag.DefaultSink(&buf, &buf, diag.FormatOptions{Color: colors.Never})
	s := &forwardingSink{base: base}

	var forwarded []string
	s.set(func(sev diag.Severity, d *diag.Diag, args ...any) {
		forwarded = append(forwarded, string(sev)+": "+d.Message)
	})
	s.Errorf(diag.RawMessage("", "boom"))
	assert.Empty(t, buf.String())
	assert.Equal(t, []string{"error: boom"}, forwarded)

	s.clear()
	s.Warningf(diag.RawMessage("", "careful"))
	assert.Contains(t, buf.String(), "careful")
	require.Len(t, forwarded, 1)
}
