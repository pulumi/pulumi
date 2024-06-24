// Copyright 2016-2024, Pulumi Corporation.
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

const (
	KiB = 1024
	MiB = 1024 * 1024
	GiB = 1024 * 1024 * 1024
)

func formatBytes(value int64) string {
	if value >= GiB {
		return fmt.Sprintf("%.02f GiB", float64(value)/GiB)
	} else if value >= MiB {
		return fmt.Sprintf("%.02f MiB", float64(value)/MiB)
	} else if value >= KiB {
		return fmt.Sprintf("%.02f KiB", float64(value)/KiB)
	}
	return fmt.Sprintf("%d B", value)
}

// Render the progress bar. This uses only ascii characters
func renderBarASCII(received, total int64, width int) string {
	innerWidth := width - 2
	if received <= 0 {
		return "[" + strings.Repeat("_", innerWidth) + "]"
	} else if received >= total {
		return "[" + strings.Repeat("-", innerWidth) + "]"
	}

	offset := int(received * int64(innerWidth) / total)
	output := make([]byte, innerWidth)
	for i := 0; i != innerWidth; i++ {
		if i == offset {
			output[i] = '>'
		} else if i < offset {
			output[i] = '-'
		} else {
			output[i] = '_'
		}
	}
	return "[" + string(output) + "]"
}

// Render the progress bar. This uses the unicode block characters, see
// https://en.wikipedia.org/wiki/Block_Elements, to draw the progress.
// These characters are well supported in terminal fonts and used extensively by
// libraries like ncurses
func renderBar(received, total int64, width int) string {
	innerWidth := width - 2
	if received <= 0 {
		return "[" + strings.Repeat(" ", innerWidth) + "]"
	} else if received >= total {
		return "[" + strings.Repeat("\u2588", innerWidth) + "]"
	}

	offset := int(received * int64(innerWidth) / total)
	subchar := int(received*int64(innerWidth)*8/total) % 8
	output := make([]rune, innerWidth)
	for i := 0; i != innerWidth; i++ {
		if i == offset {
			if subchar == 0 {
				output[i] = ' '
			} else {
				output[i] = rune(0x2590 - subchar)
			}
		} else if i < offset {
			output[i] = 0x2588
		} else {
			output[i] = ' '
		}
	}
	return "[" + string(output) + "]"
}

func renderDownloadProgress(payload engine.DownloadProgressEventPayload, width int, ascii bool) string {
	total := formatBytes(payload.Total)
	received := formatBytes(payload.Received)
	sizeWidth := len(total)*2 + 1
	msgLength := colors.MeasureColorizedString(payload.Msg)

	if msgLength+sizeWidth+10 <= width {
		// room for the message, the size, and a progress bar
		progressWidth := width - (msgLength + sizeWidth + 2)
		if ascii {
			return fmt.Sprintf("%s %s %*s/%s", payload.Msg,
				renderBarASCII(payload.Received, payload.Total, progressWidth),
				len(total), received, total)
		}
		return fmt.Sprintf("%s %s %*s/%s", payload.Msg,
			renderBar(payload.Received, payload.Total, progressWidth),
			len(total), received, total)
	} else if msgLength+10 <= width {
		// room for the message, and a progress bar
		progressWidth := width - (msgLength + 1)
		if ascii {
			return fmt.Sprintf("%s %s", payload.Msg, renderBarASCII(payload.Received, payload.Total, progressWidth))
		}
		return fmt.Sprintf("%s %s", payload.Msg, renderBar(payload.Received, payload.Total, progressWidth))
	} else if msgLength <= width {
		// just the message
		return payload.Msg
	}
	// truncate the message
	return colors.TrimColorizedString(payload.Msg, width)
}
