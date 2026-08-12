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

package cloud

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed testdata/openapi_public.json
var testSpecJSON []byte

// loadTestIndex parses the test-only embedded spec. Production code path goes
// through LoadIndex(ctx, refresh) which fetches from Pulumi Cloud; tests
// short-circuit that by parsing a pinned fixture directly.
func loadTestIndex(t *testing.T) *Index {
	t.Helper()
	idx, err := parseIndex(testSpecJSON)
	require.NoError(t, err)
	return idx
}
