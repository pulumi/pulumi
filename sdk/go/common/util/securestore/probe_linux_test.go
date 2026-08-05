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

//go:build linux

package securestore

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretServicePrecheckNoSessionBus(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // exists, but has no "bus" socket

	outcome, err := secretServicePrecheck(false)
	assert.Equal(t, Absent, outcome)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.Contains(t, err.Error(), "no D-Bus session bus")
}

func statePrompting(t *testing.T, attended bool) *[]bool {
	t.Helper()
	var granted []bool
	restore := platformCandidatesHook
	platformCandidatesHook = func(allowPrompt bool, _ string) []backendImpl {
		granted = append(granted, allowPrompt)
		return nil
	}
	t.Cleanup(func() { platformCandidatesHook = restore })

	prevDisable := cmdutil.DisableInteractive
	cmdutil.DisableInteractive = !attended
	t.Cleanup(func() { cmdutil.DisableInteractive = prevDisable })
	if attended {
		// Neutralise the detectors so an attended run stays attended on a CI
		// host or a headless machine.
		t.Setenv("CI", "")
		t.Setenv("PULUMI_DISABLE_CI_DETECTION", "1")
		t.Setenv("DISPLAY", ":0")
	}
	return &granted
}

//nolint:paralleltest // swaps package-global resolution and interactivity state
func TestPromptPolicy(t *testing.T) {
	granted := statePrompting(t, true)

	_, _ = Resolve(ModeAuto, "")
	assert.True(t, (*granted)[0], "auto opted in, so it may ask rather than downgrade")

	*granted = nil
	_, _ = Resolve(ModeOS, "")
	assert.True(t, (*granted)[0], "os may ask")

	*granted = nil
	_, _ = ForBackend(BackendLinuxSecretService, "")
	assert.True(t, (*granted)[0], "reading an encrypted file may ask")

	*granted = nil
	_, _ = Resolve(ModeDefault, "")
	assert.Empty(t, *granted, "the default mode never reaches a backend")
}

//nolint:paralleltest // swaps package-global resolution and interactivity state
func TestPromptPolicyUnattended(t *testing.T) {
	granted := statePrompting(t, false)

	_, _ = Resolve(ModeOS, "")
	assert.False(t, (*granted)[0], "--non-interactive forbids prompting even in os mode")

	*granted = nil
	_, _ = ForBackend(BackendLinuxSecretService, "")
	assert.False(t, (*granted)[0], "reads must not ask when nobody can answer")
}

func TestSecretServicePrecheckNoRuntimeDir(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("XDG_RUNTIME_DIR", "")

	outcome, err := secretServicePrecheck(false)
	assert.Equal(t, Absent, outcome)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnavailable)
}
