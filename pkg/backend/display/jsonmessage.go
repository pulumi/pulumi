// Copyright 2016-2018, Pulumi Corporation.
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
	"fmt"
	"io"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type messageRenderer struct {
	opts Options

	out io.Writer

	// A spinner to use to show that we're still doing work even when no output has been
	// printed to the console in a while.
	spinner cmdutil.Spinner
}

func newMessageRenderer(out io.Writer, op string, opts Options) *messageRenderer {
	spinner, ticker := cmdutil.NewSpinnerAndTicker(
		fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), op),
		nil, opts.Color, 1 /*timesPerSecond*/)
	ticker.Stop()

	return &messageRenderer{
		opts:    opts,
		out:     out,
		spinner: spinner,
	}
}

func (r *messageRenderer) Close() error {
	return nil
}

func (r *messageRenderer) println(display *ProgressDisplay, line string) {
	// We're about to display something. Reset our spinner so that it will go on the next line.
	r.spinner.Reset()

	_, err := fmt.Fprintln(r.out, r.opts.Color.Colorize(line))
	contract.IgnoreError(err)
}

func (r *messageRenderer) tick(display *ProgressDisplay) {
	// Update the spinner to let the user know that that work is still happening.
	r.spinner.Tick()
}

func (r *messageRenderer) rowUpdated(display *ProgressDisplay, row Row) {
	// otherwise, just print out this single row.
	columns := row.ColorizedColumns()
	columns[display.suffixColumn] += row.ColorizedSuffix()
	if row := renderRow(columns, nil); row != "" {
		r.println(display, row)
	}
}

func (r *messageRenderer) systemMessage(display *ProgressDisplay, payload engine.StdoutEventPayload) {
	r.println(display, renderStdoutColorEvent(payload, display.opts))
}

func (r *messageRenderer) done(display *ProgressDisplay) {
}
