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

package cmdutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest // mutates the package-global interactivity flags
func TestStatedInteractiveIsATriState(t *testing.T) {
	prevDisable, prevStated := DisableInteractive, InteractivityStated
	t.Cleanup(func() { DisableInteractive, InteractivityStated = prevDisable, prevStated })

	DisableInteractive, InteractivityStated = false, false
	interactive, stated := StatedInteractive()
	assert.False(t, stated, "saying nothing must not read as a statement")
	assert.True(t, interactive)

	DisableInteractive, InteractivityStated = true, true
	interactive, stated = StatedInteractive()
	assert.True(t, stated)
	assert.False(t, interactive, "--non-interactive states nobody is there")

	DisableInteractive, InteractivityStated = false, true
	interactive, stated = StatedInteractive()
	assert.True(t, stated)
	assert.True(t, interactive, "--non-interactive=false states someone is there")
}
