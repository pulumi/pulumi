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
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/ciutil"
)

// Emoji controls whether emojis will by default be printed in the output.
// While some Linux systems can display Emoji's in the terminal by default, we restrict this to just macOS, like Yarn.
var Emoji = (runtime.GOOS == "darwin")

// EmojiOr returns the emoji string e if emojis are enabled, or the string or if emojis are disabled.
func EmojiOr(e, or string) string {
	if Emoji && Interactive() {
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

	var raw strings.Builder
	for {
		var b [1]byte
		if _, err := os.Stdin.Read(b[:]); err != nil {
			return "", err
		}
		if b[0] == '\n' {
			break
		}
		raw.WriteByte(b[0])
	}
	return RemoveTrailingNewline(raw.String()), nil
}

// IsTruthy returns true if the given string represents a CLI input interpreted as "true".
func IsTruthy(s string) bool {
	return s == "1" || strings.EqualFold(s, "true")
}

// RemoveTrailingNewline removes a trailing newline from a string. On windows, we'll remove either \r\n or \n, on other
// platforms, we just remove \n.
func RemoveTrailingNewline(s string) string {
	s = strings.TrimSuffix(s, "\n")

	if runtime.GOOS == "windows" {
		s = strings.TrimSuffix(s, "\r")
	}

	return s
}

// EndKeypadTransmitMode switches the terminal out of the keypad transmit 'application' mode back to 'normal' mode.
func EndKeypadTransmitMode() {
	if runtime.GOOS != "windows" && Interactive() {
		// Print an escape sequence to switch the keypad mode, same as 'tput rmkx'.
		// Work around https://github.com/pulumi/pulumi/issues/3480.
		// A better fix might be fixing upstream https://github.com/AlecAivazis/survey/issues/228.
		fmt.Print("\033[?1l")
	}
}

type Table struct {
	Headers []string
	Rows    []TableRow // Rows of the table.
	Prefix  string     // Optional prefix to print before each row
}

// TableRow is a row in a table we want to print.  It can be a series of a columns, followed
// by an additional line of information.
type TableRow struct {
	Columns        []string // Columns of the row
	AdditionalInfo string   // an optional line of information to print after the row
}

// PrintTable prints a grid of rows and columns.  Width of columns is automatically determined by
// the max length of the items in each column.  A default gap of two spaces is printed between each
// column.
func PrintTable(table Table) {
	PrintTableWithGap(table, "  ")
}

// PrintTableWithGap prints a grid of rows and columns.  Width of columns is automatically determined
// by the max length of the items in each column.  A gap can be specified between the columns.
func PrintTableWithGap(table Table, columnGap string) {
	columnCount := len(table.Headers)

	// Figure out the preferred column width for each column.  It will be set to the max length of
	// any item in that column.
	preferredColumnWidths := make([]int, columnCount)

	allRows := []TableRow{{
		Columns: table.Headers,
	}}

	allRows = append(allRows, table.Rows...)

	for rowIndex, row := range allRows {
		columns := row.Columns
		if len(columns) != len(preferredColumnWidths) {
			panic(fmt.Sprintf(
				"Error printing table.  Column count of row %v didn't match header column count. %v != %v",
				rowIndex, len(columns), len(preferredColumnWidths)))
		}

		for columnIndex, val := range columns {
			preferredColumnWidths[columnIndex] = max(preferredColumnWidths[columnIndex], len(val))
		}
	}

	format := ""
	for i, maxWidth := range preferredColumnWidths {
		if i < len(preferredColumnWidths)-1 {
			format += "%-" + strconv.Itoa(maxWidth+len(columnGap)) + "s"
		} else {
			// do not want whitespace appended to the last column.  It would cause wrapping on lines
			// that were not actually long if some other line was very long.
			format += "%s"
		}
	}
	format += "\n"

	columns := make([]interface{}, columnCount)
	for _, row := range allRows {
		for columnIndex, value := range row.Columns {
			// Now, ensure we have the requested gap between columns as well.
			if columnIndex < columnCount-1 {
				value += columnGap
			}

			columns[columnIndex] = value
		}

		fmt.Printf(table.Prefix+format, columns...)
		if row.AdditionalInfo != "" {
			fmt.Print(row.AdditionalInfo)
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
