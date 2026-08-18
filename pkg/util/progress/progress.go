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

//go:build !js

package progress

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

var (
	stderrOnce  sync.Once
	stderrGroup *Group
)

// Stderr returns the process-wide Group for progress bars rendered directly to
// standard error. Bars from independent groups writing to the same terminal
// would garble each other, so all callers rendering to stderr must share this
// one.
func Stderr() *Group {
	stderrOnce.Do(func() {
		stderrGroup = NewGroup(os.Stderr)
	})
	return stderrGroup
}

// Group renders progress bars for concurrent operations without garbling the
// terminal: each active bar gets its own line, and the whole block is redrawn
// in place. Finished bars are printed once more in their final state and then
// scroll away with the regular output.
type Group struct {
	out io.Writer

	mu sync.Mutex
	p  *mpb.Progress
}

func NewGroup(out io.Writer) *Group {
	return &Group{out: out}
}

// Wrap attaches a progress bar to the given stream, coordinating with the
// other bars in the group. When the size is unknown or the output is not
// interactive, a plain message is printed instead. Closing the returned reader
// finishes the bar; closing it more than once is safe.
func (g *Group) Wrap(
	closer io.ReadCloser, size int64, message string, colorization colors.Colorization,
) io.ReadCloser {
	if size == -1 || !cmdutil.Interactive() {
		fmt.Fprintln(g.out, colorization.Colorize(colors.SpecUnimportant+message+colors.Reset))
		return closer
	}

	g.mu.Lock()
	if g.p == nil {
		g.p = mpb.New(
			mpb.WithOutput(g.out),
			mpb.WithRefreshRate(150*time.Millisecond),
			mpb.PopCompletedMode(),
		)
	}
	p := g.p
	g.mu.Unlock()

	bar := p.New(size,
		mpb.BarStyle(),
		mpb.PrependDecorators(
			decor.Name(message+": "),
			decor.CountersKibiByte("% .2f / % .2f"),
		),
		mpb.AppendDecorators(
			decor.NewPercentage("%.2f"),
			decor.Name(" "),
			decor.Elapsed(decor.ET_STYLE_GO),
		),
	)
	return &barCloser{bar: bar, readCloser: bar.ProxyReader(closer)}
}

type barCloser struct {
	bar        *mpb.Bar
	readCloser io.ReadCloser
	closeOnce  sync.Once
}

func (bc *barCloser) Read(dest []byte) (int, error) {
	return bc.readCloser.Read(dest)
}

func (bc *barCloser) Close() error {
	var err error
	bc.closeOnce.Do(func() {
		err = bc.readCloser.Close()
		bc.bar.SetTotal(-1, true)
	})
	return err
}
