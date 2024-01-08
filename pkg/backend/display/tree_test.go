// Copyright 2016-2023, Pulumi Corporation.
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

package display

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/display/internal/terminal"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

// Table Test using different terminal widths and heights to ensure that the display does not panic.
func TestTreeFrameSize(t *testing.T) {
	t.Parallel()

	// Table Test using different terminal widths and heights
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"narrow", 1, 100},
		{"short", 100, 1},
		{"small", 1, 1},
		{"normal", 100, 100},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			term := terminal.NewMockTerminal(&buf, tt.width, tt.height, true)
			treeRenderer := newInteractiveRenderer(term, "this-is-a-fake-permalink", Options{
				Color: colors.Always,
			}).(*treeRenderer)

			// Fill the renderer with too many rows of strings to fit in the terminal.
			for i := 0; i < 1000; i++ {
				// Fill the renderer with strings that are too long to fit in the terminal.
				treeRenderer.systemMessages = append(treeRenderer.systemMessages, strings.Repeat("a", 1000))
			}
			treeRenderer.treeTableRows = treeRenderer.systemMessages

			// Required to get the frame to render.
			treeRenderer.markDirty()

			// This should not panic.
			treeRenderer.frame(false /* locked */, false /* done */)
		})
	}
}
