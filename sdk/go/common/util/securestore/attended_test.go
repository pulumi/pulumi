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

func statedInteractivity(t *testing.T, disable, stated bool) {
	t.Helper()
	prevDisable, prevStated := cmdutil.DisableInteractive, cmdutil.InteractivityStated
	cmdutil.DisableInteractive, cmdutil.InteractivityStated = disable, stated
	t.Cleanup(func() {
		cmdutil.DisableInteractive, cmdutil.InteractivityStated = prevDisable, prevStated
	})
}

//nolint:paralleltest // mutates interactivity globals and the environment
func TestStatedInteractivityWinsOverDetection(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	statedInteractivity(t, false, true)
	assert.True(t, someoneCanAnswerAPasswordDialog(), "--non-interactive=false asserts a user is present")

	statedInteractivity(t, true, true)
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("DISPLAY", ":0")
	assert.False(t, someoneCanAnswerAPasswordDialog(), "--non-interactive forbids prompting")
}

//nolint:paralleltest // mutates interactivity globals and the environment
func TestDetectionWithoutAStatedPreference(t *testing.T) {
	statedInteractivity(t, false, false)

	t.Setenv("CI", "")
	t.Setenv("PULUMI_DISABLE_CI_DETECTION", "")
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("DISPLAY", ":0")
	assert.False(t, someoneCanAnswerAPasswordDialog(), "CI has nobody to answer a dialog")

	// The test host may itself be CI, so neutralise every detector before
	// asserting on the display branch.
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("CI", "")
	t.Setenv("PULUMI_DISABLE_CI_DETECTION", "1")
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	if runtime.GOOS == "linux" {
		assert.False(t, someoneCanAnswerAPasswordDialog(), "no display means no dialog can be shown")
		t.Setenv("WAYLAND_DISPLAY", "wayland-0")
		assert.True(t, someoneCanAnswerAPasswordDialog(), "a Wayland session can show one")
		t.Setenv("WAYLAND_DISPLAY", "")
		t.Setenv("DISPLAY", ":0")
		assert.True(t, someoneCanAnswerAPasswordDialog(), "an X session can show one")
	} else {
		assert.True(t, someoneCanAnswerAPasswordDialog(),
			"macOS and Windows refuse UI themselves rather than reading a display variable")
	}
}
