// Copyright 2024, Pulumi Corporation.
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

package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeExecutablePromptChoices(t *testing.T) {
	t.Parallel()

	// Not found executables come after the found ones, and have a [not found] suffix.
	choices := MakeExecutablePromptChoices("executable_that_does_not_exist_in_path", "ls")
	require.Equal(t, 2, len(choices))
	require.Equal(t, choices[0].StringValue, "ls")
	require.Equal(t, choices[0].DisplayName, "ls")
	require.Equal(t, choices[1].StringValue, "executable_that_does_not_exist_in_path")
	require.Equal(t, choices[1].DisplayName, "executable_that_does_not_exist_in_path [not found]")

	// Executables are not reordered within their group.
	choices = MakeExecutablePromptChoices("ls", "cat", "zzz_does_not_exist", "aaa_does_not_exist")
	require.Equal(t, choices[0].StringValue, "ls")
	require.Equal(t, choices[0].DisplayName, "ls")
	require.Equal(t, choices[1].StringValue, "cat")
	require.Equal(t, choices[1].DisplayName, "cat")
	require.Equal(t, choices[2].StringValue, "zzz_does_not_exist")
	require.Equal(t, choices[2].DisplayName, "zzz_does_not_exist [not found]")
	require.Equal(t, choices[3].StringValue, "aaa_does_not_exist")
	require.Equal(t, choices[3].DisplayName, "aaa_does_not_exist [not found]")
}
