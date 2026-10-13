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

package executable

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToolchainNotFoundError(t *testing.T) {
	t.Parallel()

	err := &ToolchainNotFoundError{
		Runtime:    "go",
		Toolchain:  "Go",
		Programs:   []string{"go"},
		InstallURL: "https://example.com/go",
	}
	assert.Equal(t, "the go runtime requires Go, but 'go' was not found on your PATH. "+
		"See https://example.com/go for installation instructions", err.Error())

	err = &ToolchainNotFoundError{
		Runtime:    "python",
		Toolchain:  "Python 3.10 or later",
		Programs:   []string{"python3", "python"},
		InstallURL: "https://example.com/python",
	}
	assert.Equal(t, "the python runtime requires Python 3.10 or later, but none of 'python3', 'python' was found "+
		"on your PATH. See https://example.com/python for installation instructions", err.Error())
}

func TestToolchainNotFoundErrorIsFailedPrecondition(t *testing.T) {
	t.Parallel()

	err := &ToolchainNotFoundError{Runtime: "go", Toolchain: "Go", Programs: []string{"go"}, InstallURL: "url"}

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
	assert.Equal(t, err.Error(), st.Message())

	st, ok = status.FromError(fmt.Errorf("wrapped: %w", err))
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
	assert.Equal(t, "wrapped: "+err.Error(), st.Message())
}
