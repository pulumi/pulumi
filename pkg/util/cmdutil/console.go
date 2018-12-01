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

package cmdutil

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/pulumi/pulumi/pkg/util/ciutil"
)

// Emoji controls whether emojis will by default be printed in the output.
// While some Linux systems can display Emoji's in the terminal by default, we restrict this to just macOS, like Yarn.
var Emoji = (runtime.GOOS == "darwin")

// EmojiOr returns the emoji string e if emojis are enabled, or the string or if emojis are disabled.
func EmojiOr(e, or string) string {
	if Emoji {
		return e
	}
	return or
}

// DisableInteractive may be set to true in order to disable prompts. This is useful when running in a non-attended
// scenario, such as in continuous integration, or when using the Pulumi CLI/SDK in a programmatic way.
var DisableInteractive bool

// Interactive returns true if we should be running in interactive mode. That is, we have an interactive terminal
// session, interactivity hasn't been explicitly disabled, and we're not running in a known CI system.
func Interactive() bool {
	return !DisableInteractive && InteractiveTerminal() && !ciutil.IsCI()
}

// InteractiveTerminal returns true if the current terminal session is interactive.
func InteractiveTerminal() bool {
	// If there's a 'TERM' variable and the terminal is 'dumb', then disable interactive mode.
	if v := strings.ToLower(os.Getenv("TERM")); v == "dumb" {
		return false
	}

	// if we're piping in stdin, we're clearly not interactive, as there's no way for a user to
	// provide input.  If we're piping stdout, we also can't be interactive as there's no way for
	// users to see prompts to interact with them.
	return terminal.IsTerminal(int(os.Stdin.Fd())) &&
		terminal.IsTerminal(int(os.Stdout.Fd()))
}

// ReadConsole reads the console with the given prompt text.
func ReadConsole(prompt string) (string, error) {
	if prompt != "" {
		fmt.Print(prompt + ": ")
	}

	reader := bufio.NewReader(os.Stdin)
	raw, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return RemoveTralingNewline(raw), nil
}

// IsTruthy returns true if the given string represents a CLI input interpreted as "true".
func IsTruthy(s string) bool {
	return s == "1" || strings.EqualFold(s, "true")
}

// RemoveTralingNewline removes a trailing newline from a string. On windows, we'll remove either \r\n or \n, on other
// platforms, we just remove \n.
func RemoveTralingNewline(s string) string {
	s = strings.TrimSuffix(s, "\n")

	if runtime.GOOS == "windows" {
		s = strings.TrimSuffix(s, "\r")
	}

	return s
}

// PrintTable prints a grid of rows and columns.  Width of columns is automatically
// determined by the max length of the items in each column.
func PrintTable(table [][]string) {
	if len(table) == 0 {
		return
	}

	maxColumnWidths := make([]int, len(table[0]))
	for i := range maxColumnWidths {
		maxColumnWidths[i] = -1
	}

	PrintTableEx(table, maxColumnWidths, "  " /*columnGap*/)
}

const ellipses = "..."

// PrintTableEx prints a grid of rows and columns.  Width of columns is provided.  Column values
// longer than this width will be automatically truncated.  Use -1 to indicate no max for a
// particular column.  A gap can be specified between the columns.
func PrintTableEx(table [][]string, maxColumnWidths []int, columnGap string) {
	if len(table) == 0 {
		return
	}

	// Figure out the preferred column width for each column.  It will be set to the max length of
	// any item in that column.  However, it wil lbe clamped to the value in maxColumnWidths if it
	// is not -1.
	preferredColumnWidths := make([]int, len(maxColumnWidths))

	for rowIndex, row := range table {
		if len(row) != len(maxColumnWidths) {
			panic(fmt.Sprintf(
				"Error printing table.  Column count of row %v didn't match maxColumnWidths. %v != %v",
				rowIndex, len(row), len(maxColumnWidths)))
		}

		for columnIndex, val := range row {
			preferredColumnWidths[columnIndex] = max(preferredColumnWidths[columnIndex], len(val))
		}
	}

	for columnIndex := range preferredColumnWidths {
		maxWidth := maxColumnWidths[columnIndex]
		if maxWidth != -1 {
			preferredColumnWidths[columnIndex] = min(preferredColumnWidths[columnIndex], maxWidth)
		}
	}

	format := ""
	for _, maxWidth := range preferredColumnWidths {
		format += "%-" + strconv.Itoa(maxWidth+len(columnGap)) + "s"
	}
	format += "\n"

	columnCount := len(table[0])
	columns := make([]interface{}, columnCount)
	for _, row := range table {
		for columnIndex, value := range row {
			valueLen := len(value)
			width := preferredColumnWidths[columnIndex]
			if valueLen > width {
				// First, try to trim down the value so we can include the ...
				value = value[0:max(width-len(ellipses), 0)] + ellipses

				// However, with a sufficiently small width, that might still be too long. So always
				// trim to width as requested.
				value = value[0:width]
			}

			// Now, ensure we have the requested gap between columns as well.
			if columnIndex < columnCount-1 {
				value += columnGap
			}

			columns[columnIndex] = value
		}

		fmt.Printf(format, columns...)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
