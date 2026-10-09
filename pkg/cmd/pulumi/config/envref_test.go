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

package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
)

func TestEnvRefVersion(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"proj/env":         "",
		"proj/env@7":       "7",
		"proj/env@stable":  "stable",
		"proj/env:7":       "7",
		"proj/env:stable":  "stable",
		"proj/env@7:extra": "7:extra", // "@" wins; everything after it is the version
	}
	for ref, want := range cases {
		require.Equal(t, want, envRefVersion(ref), ref)
	}
}

func TestStripEnvVersion(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"proj/env":        "proj/env",
		"proj/env@7":      "proj/env",
		"proj/env:7":      "proj/env",
		"proj/env:stable": "proj/env",
	}
	for ref, want := range cases {
		require.Equal(t, want, stripEnvVersion(ref), ref)
	}
}

func TestSplitEnvRef(t *testing.T) {
	t.Parallel()

	for _, ref := range []string{"proj/env", "proj/env@7", "proj/env:7"} {
		project, name, err := splitEnvRef(ref)
		require.NoError(t, err, ref)
		require.Equal(t, "proj", project, ref)
		require.Equal(t, "env", name, ref)
	}

	for _, ref := range []string{"env", "env@7", "/env", "proj/", ""} {
		_, _, err := splitEnvRef(ref)
		require.Error(t, err, ref)
	}
}

func TestRejectIfPinned(t *testing.T) {
	t.Parallel()

	remote := func(ref string) backend.Stack {
		return &backend.MockStack{
			ConfigLocationF: func() backend.StackConfigLocation {
				return backend.StackConfigLocation{IsRemote: true, EscEnv: &ref}
			},
		}
	}

	// A pinned ref (via either separator) is rejected with an actionable message.
	require.ErrorContains(t, rejectIfPinned(remote("proj/env@5"), ""), "unpin")
	require.ErrorContains(t, rejectIfPinned(remote("proj/env:5"), ""), "unpin")

	// An unpinned ref is allowed.
	require.NoError(t, rejectIfPinned(remote("proj/env"), ""))

	// An explicit --config-file routes the write to a local file, so the pin guard must not fire even
	// for a pinned remote stack.
	require.NoError(t, rejectIfPinned(remote("proj/env@5"), "Pulumi.local.yaml"))

	// A local stack is never pinned.
	local := &backend.MockStack{
		ConfigLocationF: func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: false}
		},
	}
	require.NoError(t, rejectIfPinned(local, ""))
}
