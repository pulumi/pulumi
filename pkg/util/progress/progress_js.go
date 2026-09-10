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

//go:build js

// GOOS=js has no interactive terminal and mpb's terminal writer does not
// compile for it, so this variant always prints the plain fallback message
// instead of rendering progress bars.

package progress

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

var (
	stderrOnce  sync.Once
	stderrGroup *Group
)

// Stderr returns the process-wide Group for progress reported directly to
// standard error.
func Stderr() *Group {
	stderrOnce.Do(func() {
		stderrGroup = NewGroup(os.Stderr)
	})
	return stderrGroup
}

// Group reports the progress of concurrent operations.
type Group struct {
	out io.Writer
}

func NewGroup(out io.Writer) *Group {
	return &Group{out: out}
}

// Wrap prints the given message and returns the stream unchanged.
func (g *Group) Wrap(
	closer io.ReadCloser, size int64, message string, colorization colors.Colorization,
) io.ReadCloser {
	fmt.Fprintln(g.out, colorization.Colorize(colors.SpecUnimportant+message+colors.Reset))
	return closer
}
