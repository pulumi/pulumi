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

package securestore

import (
	"runtime"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/stretchr/testify/assert"
)

func disableInteractive(t *testing.T, disable bool) {
	t.Helper()
	prev := cmdutil.DisableInteractive
	cmdutil.DisableInteractive = disable
	t.Cleanup(func() { cmdutil.DisableInteractive = prev })
}

func TestNonInteractiveForbidsPrompting(t *testing.T) {
	disableInteractive(t, true)
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("CI", "")
	t.Setenv("PULUMI_DISABLE_CI_DETECTION", "1")
	t.Setenv("DISPLAY", ":0")
	assert.False(t, someoneCanAnswerAPasswordDialog())
}

func TestDetectionWithoutAStatedPreference(t *testing.T) {
	disableInteractive(t, false)

	t.Setenv("CI", "")
	t.Setenv("PULUMI_DISABLE_CI_DETECTION", "")
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("DISPLAY", ":0")
	assert.False(t, someoneCanAnswerAPasswordDialog(), "CI has nobody to answer a dialog")

	// The test host may itself be CI or an SSH session, so neutralise every
	// detector before asserting on the session branch.
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("CI", "")
	t.Setenv("PULUMI_DISABLE_CI_DETECTION", "1")
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("SSH_TTY", "")
	switch runtime.GOOS {
	case "linux":
		assert.False(t, someoneCanAnswerAPasswordDialog(), "no display means no dialog can be shown")
		t.Setenv("WAYLAND_DISPLAY", "wayland-0")
		assert.True(t, someoneCanAnswerAPasswordDialog(), "a Wayland session can show one")
		t.Setenv("WAYLAND_DISPLAY", "")
		t.Setenv("DISPLAY", ":0")
		assert.True(t, someoneCanAnswerAPasswordDialog(), "an X session can show one")
	case "darwin":
		assert.True(t, someoneCanAnswerAPasswordDialog(), "the console session can show a keychain dialog")
		t.Setenv("SSH_CONNECTION", "10.0.0.1 50022 10.0.0.2 22")
		assert.False(t, someoneCanAnswerAPasswordDialog(), "an SSH session cannot see a keychain dialog")
	default:
		assert.True(t, someoneCanAnswerAPasswordDialog(),
			"Windows never prompts, so any session may proceed")
	}
}

func TestWarningsHaveAnAudienceWhenNoDialogCanBeShown(t *testing.T) {
	disableInteractive(t, false)
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("CI", "")
	t.Setenv("PULUMI_DISABLE_CI_DETECTION", "1")
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("SSH_CONNECTION", "10.0.0.1 50022 10.0.0.2 22")
	t.Setenv("SSH_TTY", "/dev/ttys000")

	assert.True(t, Attended(), "an SSH user reads stderr")
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		assert.False(t, someoneCanAnswerAPasswordDialog())
	}
}
