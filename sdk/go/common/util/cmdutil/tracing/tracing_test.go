// Copyright 2025, Pulumi Corporation.
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

package tracing

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpanToFile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmp := t.TempDir()
	traceFile := tmp + "/test.trace"
	t.Logf("traceFile: %q", traceFile)
	_, closer, err := Init(ctx, "file:"+traceFile)
	require.NoError(t, err)
	require.NoError(t, closer.Close())

	b, err := os.ReadFile(traceFile)
	require.NoError(t, err)
	assert.Contains(t, string(b), "root")
}
