// Copyright 2020-2024, Pulumi Corporation.
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

package pulumi

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPrintf(t *testing.T, ins ...interface{}) {
	const f = "%v %v %v"
	expected := fmt.Sprintf(f, "foo", 42, true)

	// Fprintf
	buf := &bytes.Buffer{}
	out := Output(Fprintf(buf, f, ins...))
	_, known, secret, deps, err := await(out)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Nil(t, deps)
	require.NoError(t, err)
	assert.Equal(t, expected, buf.String())

	// Sprintf
	out = Sprintf(f, ins...)
	v, known, secret, deps, err := await(out)
	assert.False(t, secret)
	assert.True(t, known)
	assert.Nil(t, deps)
	require.NoError(t, err)
	assert.Equal(t, expected, v)
}

func TestSprintfPrompt(t *testing.T) {
	t.Parallel()

	testPrintf(t, "foo", 42, true)
}

func TestSprintfInputs(t *testing.T) {
	t.Parallel()

	testPrintf(t, String("foo"), Int(42), Bool(true))
}

func TestSprintfOutputs(t *testing.T) {
	t.Parallel()

	testPrintf(t, ToOutput("foo"), ToOutput(42), ToOutput(true))
}
