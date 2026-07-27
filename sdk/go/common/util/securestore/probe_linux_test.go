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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecretServicePrecheckNoSessionBus pins the cheap environment fast-fail:
// with no bus address and no user-runtime bus socket the probe must fail with
// ErrUnavailable without attempting any connection.
func TestSecretServicePrecheckNoSessionBus(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // exists, but has no "bus" socket

	err := secretServicePrecheck()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.Contains(t, err.Error(), "no D-Bus session bus")
}

func TestSecretServicePrecheckNoRuntimeDir(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("XDG_RUNTIME_DIR", "")

	err := secretServicePrecheck()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnavailable)
}
