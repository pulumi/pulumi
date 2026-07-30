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

package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOfferFirstStepSaysNothingToAUserWithStacks(t *testing.T) {
	t.Parallel()

	be := &pkgBackend.MockBackend{
		URLF: func() string { return "https://api.pulumi.com" },
		ListStackNamesF: func(
			context.Context, pkgBackend.ListStackNamesFilter, pkgBackend.ContinuationToken,
		) ([]pkgBackend.StackReference, pkgBackend.ContinuationToken, error) {
			return []pkgBackend.StackReference{&pkgBackend.MockStackReference{}}, nil, nil
		},
	}

	out := &strings.Builder{}
	err := offerFirstStep(t.Context(), be, t.TempDir(), out, display.Options{}, true)
	require.NoError(t, err)
	assert.Empty(t, out.String(), "a returning user's login output must be unchanged")
}

func TestOfferFirstStepPrintsHintInNonEmptyDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0o600))

	be := &pkgBackend.MockBackend{
		URLF: func() string { return "https://api.pulumi.com" },
		ListStackNamesF: func(
			context.Context, pkgBackend.ListStackNamesFilter, pkgBackend.ContinuationToken,
		) ([]pkgBackend.StackReference, pkgBackend.ContinuationToken, error) {
			return nil, nil, nil
		},
	}

	out := &strings.Builder{}
	err := offerFirstStep(t.Context(), be, dir, out, display.Options{}, true)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "To get started, run `pulumi new` in an empty directory")
}

func TestOfferFirstStepNeverCallsBackendWhenGuardSaysNo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		url         string
		interactive bool
	}{
		{name: "non-interactive", url: "https://api.pulumi.com", interactive: false},
		{name: "diy backend", url: "file:///tmp/state", interactive: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			be := &pkgBackend.MockBackend{
				URLF: func() string { return tt.url },
				ListStackNamesF: func(
					context.Context, pkgBackend.ListStackNamesFilter, pkgBackend.ContinuationToken,
				) ([]pkgBackend.StackReference, pkgBackend.ContinuationToken, error) {
					t.Fatal("ListStackNames must not be called when the guard says no")
					return nil, nil, nil
				},
			}

			out := &strings.Builder{}
			err := offerFirstStep(t.Context(), be, t.TempDir(), out, display.Options{}, tt.interactive)
			require.NoError(t, err)
			assert.Empty(t, out.String())
		})
	}
}

func TestOfferFirstStepIgnoresListStacksFailure(t *testing.T) {
	t.Parallel()

	be := &pkgBackend.MockBackend{
		URLF: func() string { return "https://api.pulumi.com" },
		ListStackNamesF: func(
			context.Context, pkgBackend.ListStackNamesFilter, pkgBackend.ContinuationToken,
		) ([]pkgBackend.StackReference, pkgBackend.ContinuationToken, error) {
			return nil, nil, errors.New("network is unreachable")
		},
	}

	out := &strings.Builder{}
	err := offerFirstStep(t.Context(), be, t.TempDir(), out, display.Options{}, true)
	require.NoError(t, err, "a failed emptiness check must not fail a successful login")
	assert.Empty(t, out.String())
}
