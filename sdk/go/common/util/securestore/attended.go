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
	"os"
	"runtime"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/ciutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// Attended reports whether a user is present for this run. It decides both
// whether an unlock prompt may be completed and whether stderr warnings have
// an audience, so the CLI holds one opinion about presence.
//
// The question is whether a dialog the credential provider draws on the
// desktop session can reach someone, which is why this deliberately ignores
// whether stdio is a TTY: `pulumi up | tee log` is attended, and `ssh host
// pulumi whoami` is not. --non-interactive forces unattended; there is no way
// to force the opposite, because a session with no display cannot show a
// dialog however present the user is.
func Attended() bool { return someoneCanAnswerAPasswordDialog() }

func someoneCanAnswerAPasswordDialog() bool {
	if cmdutil.DisableInteractive {
		return false
	}
	// ciutil knows vendor variables; the bare CI convention is checked too so
	// this agrees with the rest of the CLI about what counts as headless.
	if ciutil.IsCI() || os.Getenv("CI") != "" {
		return false
	}
	return desktopSessionCanDrawDialogs()
}

func desktopSessionCanDrawDialogs() bool {
	if runtime.GOOS != "linux" {
		return true
	}
	return os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("DISPLAY") != ""
}
