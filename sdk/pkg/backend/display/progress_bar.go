// Copyright 2024, Pulumi Corporation.
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
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

// ProgressBarRenderer is a function that renders a progress bar.
type ProgressBarRenderer func(width int, completed int64, total int64) string

// Renders a progress event as a string that is at most width characters, using
// the supplied progress bar renderer to produce the progress bar.
func renderProgress(
	renderProgressBar ProgressBarRenderer,
	width int,
	payload engine.ProgressEventPayload,
) string {
	// Determine the counts to display based on the type of progress event:
	//
	// * Plugin download -- bytes downloaded/total
	// * Plugin install -- bytes unpacked/total
	var completed, total string
	switch payload.Type {
	case engine.PluginDownload, engine.PluginInstall:
		completed = formatBytes(payload.Completed)
		total = formatBytes(payload.Total)
	}

	totalWidth := len(total)
	sizesWidth := totalWidth*2 + 1
	messageWidth := colors.MeasureColorizedString(payload.Message)

	if messageWidth+sizesWidth+minimumBarWidth <= width {
		// Room for the message, the total, and a progress bar.
		progressBarWidth := width - (messageWidth + sizesWidth + 2)
		progressBar := renderProgressBar(progressBarWidth, payload.Completed, payload.Total)
		return fmt.Sprintf("%s %s %*s/%s", payload.Message, progressBar, totalWidth, completed, total)
	} else if messageWidth+minimumBarWidth <= width {
		// Room for the message and a progress bar.
		progressBarWidth := width - (messageWidth + 1)
		progressBar := renderProgressBar(progressBarWidth, payload.Completed, payload.Total)
		return fmt.Sprintf("%s %s", payload.Message, progressBar)
	} else if messageWidth <= width {
		// Just room for the message.
		return payload.Message
	}

	// Not even room for the message; truncate and display as best we can.
	return colors.TrimColorizedString(payload.Message, width)
}

const minimumBarWidth = 10

// A progress bar renderer that uses only ASCII characters to produce e.g.
// `[--->_____]`
func renderASCIIProgressBar(width int, completed int64, total int64) string {
	innerWidth := width - 2
	if completed <= 0 {
		return "[" + strings.Repeat("_", innerWidth) + "]"
	} else if completed >= total {
		return "[" + strings.Repeat("-", innerWidth) + "]"
	}

	offset := int(completed * int64(innerWidth) / total)
	var b strings.Builder
	b.Grow(width)
	b.WriteRune('[')
	for i := 0; i < innerWidth; i++ {
		if i == offset {
			b.WriteRune('>')
		} else if i < offset {
			b.WriteRune('-')
		} else {
			b.WriteRune('_')
		}
	}
	b.WriteRune(']')
	return b.String()
}

// A progress bar renderer that uses Unicode block characters. These characters
// are generally well supported in most terminal fonts and used heavily by
// libraries like Ncurses, etc. See https://en.wikipedia.org/wiki/Block_Elements
// for more information.
func renderUnicodeProgressBar(width int, completed int64, total int64) string {
	innerWidth := width - 2
	if completed <= 0 {
		return "[" + strings.Repeat(" ", innerWidth) + "]"
	} else if completed >= total {
		return "[" + strings.Repeat("\u2588", innerWidth) + "]"
	}

	// As well as the "full block" character, there are seven other characters
	// that partially fill a block. We use these to try and make the bar as smooth
	// as possible.
	offset := int(completed * int64(innerWidth) / total)
	subOffset := int(completed*int64(innerWidth)*8/total) % 8
	var b strings.Builder
	b.Grow(width)
	b.WriteRune('[')
	for i := 0; i < innerWidth; i++ {
		if i == offset {
			if subOffset == 0 {
				b.WriteRune(' ')
			} else {
				b.WriteRune(rune(0x2590 - subOffset))
			}
		} else if i < offset {
			b.WriteRune('\u2588')
		} else {
			b.WriteRune(' ')
		}
	}
	b.WriteRune(']')
	return b.String()
}

// Formats an integer number of bytes as a human-readable string, using the
// largest possible unit (up to gibibytes).
func formatBytes(n int64) string {
	if n >= GiB {
		return fmt.Sprintf("%.2f GiB", float64(n)/GiB)
	} else if n >= MiB {
		return fmt.Sprintf("%.2f MiB", float64(n)/MiB)
	} else if n >= KiB {
		return fmt.Sprintf("%.2f KiB", float64(n)/KiB)
	}
	return fmt.Sprintf("%d B", n)
}

const (
	KiB = 1024
	MiB = 1024 * KiB
	GiB = 1024 * MiB
)
