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
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStartedURLTracksCLISource(t *testing.T) {
	t.Parallel()

	guideURL, err := url.Parse(getStartedURL)
	require.NoError(t, err)
	assert.Equal(t, "cli", guideURL.Query().Get("utm_source"))
	require.Len(t, guideURL.Query(), 1)
}

func TestOfferFirstStepSaysNothingToAUserWithStacks(t *testing.T) {
	t.Parallel()

	// Both backend kinds take the same path: a user with stacks is a returning user either way.
	for _, url := range []string{"https://api.pulumi.com", "file:///tmp/state"} {
		t.Run(url, func(t *testing.T) {
			t.Parallel()

			be := &pkgBackend.MockBackend{
				URLF: func() string { return url },
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
		})
	}
}

func TestOfferFirstStepNeverCallsBackendWhenNotInteractive(t *testing.T) {
	t.Parallel()

	be := &pkgBackend.MockBackend{
		URLF: func() string { return "https://api.pulumi.com" },
		ListStackNamesF: func(
			context.Context, pkgBackend.ListStackNamesFilter, pkgBackend.ContinuationToken,
		) ([]pkgBackend.StackReference, pkgBackend.ContinuationToken, error) {
			t.Fatal("ListStackNames must not be called on a non-interactive login")
			return nil, nil, nil
		},
	}

	out := &strings.Builder{}
	err := offerFirstStep(t.Context(), be, t.TempDir(), out, display.Options{}, false)
	require.NoError(t, err)
	assert.Empty(t, out.String())
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

func TestValidateProjectDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	empty := filepath.Join(root, "empty")
	require.NoError(t, os.Mkdir(empty, 0o700))

	occupied := filepath.Join(root, "occupied")
	require.NoError(t, os.Mkdir(occupied, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(occupied, "main.go"), []byte("package main"), 0o600))

	tests := []struct {
		name    string
		answer  any
		wantErr string
	}{
		{name: "a directory that does not exist yet", answer: filepath.Join(root, "new")},
		{name: "an existing empty directory", answer: empty},
		{
			name:    "an existing directory with files in it",
			answer:  occupied,
			wantErr: "is not empty, please enter an empty or new directory",
		},
		{name: "an empty answer", answer: "  ", wantErr: "please enter a directory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateProjectDirectory(tt.answer)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestSuggestDirectories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "infra"), 0o700))
	require.NoError(t, os.Mkdir(filepath.Join(root, "infra-staging"), 0o700))
	require.NoError(t, os.Mkdir(filepath.Join(root, "other"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "infra.txt"), []byte("x"), 0o600))

	suggestions := suggestDirectories(filepath.Join(root, "inf"))

	assert.Equal(t, []string{
		filepath.Join(root, "infra"),
		filepath.Join(root, "infra-staging"),
	}, suggestions, "only directories matching the prefix are suggested, never files")
}
