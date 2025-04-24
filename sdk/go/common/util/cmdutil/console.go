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
	"io"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/rivo/uniseg"
	"golang.org/x/term"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
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
	//nolint:gosec // os.Stdin.Fd() == 0 && os.Stdout.Fd() == 1: uintptr -> int conversion is always safe
	return term.IsTerminal(int(os.Stdin.Fd())) &&
		term.IsTerminal(int(os.Stdout.Fd()))
}

// ReadConsole reads the console with the given prompt text.
func ReadConsole(prompt string) (string, error) {
	//nolint:gosec // os.Stdin.Fd() == 0: uintptr -> int conversion is always safe
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return readConsolePlain(os.Stdout, os.Stdin, prompt)
	}

	return readConsoleFancy(os.Stdout, os.Stdin, prompt, false /* secret */)
}

// ReadConsoleWithDefault reads the console with the given prompt text with support for a default value.
func ReadConsoleWithDefault(prompt string, defaultValue string) (string, error) {
	promptMessage := fmt.Sprintf("%s [%s]", prompt, defaultValue)
	value, err := ReadConsole(promptMessage)
	if err != nil {
		return "", err
	}

	if value == "" {
		value = defaultValue
	}

	return value, nil
}

// readConsolePlain prints the given prompt (if any),
// and reads the user's response from stdin.
//
// It does so without altering the terminal's state in any way,
// and will work even if stdin is not a terminal.
func readConsolePlain(stdout io.Writer, stdin io.Reader, prompt string) (string, error) {
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

// FprintTable prints a grid of rows and columns.  Width of columns is automatically determined by
// the max length of the items in each column.  A default gap of two spaces is printed between each
// column.
func FprintTable(w io.Writer, table Table) error {
	_, err := fmt.Fprint(w, table)
	return err
}

// PrintTable prints the table to stdout.
// See [FprintTable] for details.
func PrintTable(table Table) {
	_ = FprintTable(os.Stdout, table)
	// Ignore error for stdout.
}

// PrintTableWithGap prints a grid of rows and columns.  Width of columns is automatically determined
// by the max length of the items in each column.  A gap can be specified between the columns.
func PrintTableWithGap(table Table, columnGap string) {
	fmt.Print(table.ToStringWithGap(columnGap))
}

func (table Table) String() string {
	return table.ToStringWithGap("  ")
}

// 7-bit C1 ANSI sequences
var ansiEscape = regexp.MustCompile(`\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])`)

// MeasureText returns the number of glyphs in a string.
// Importantly this also ignores ANSI escape sequences, so can be used to calculate layout of colorized strings.
func MeasureText(text string) int {
	// Strip ansi escape sequences
	clean := ansiEscape.ReplaceAllString(text, "")
	// Need to count graphemes not runes or bytes
	return uniseg.StringWidth(clean)
}

// normalizedRows returns the rows of a table in normalized form.
//
// A row is considered normalized if and only if it has no new lines in any of its fields.
func (table Table) normalizedRows() []TableRow {
	rows := slice.Prealloc[TableRow](len(table.Rows))
	for _, row := range table.Rows {
		info := row.AdditionalInfo
		buckets := make([][]string, len(row.Columns))
		maxLines := 0
		for i, column := range row.Columns {
			buckets[i] = strings.Split(column, "\n")
			maxLines = max(maxLines, len(buckets[i]))
		}
		row := []TableRow{}
		for i := 0; i < maxLines; i++ {
			part := TableRow{}
			for _, b := range buckets {
				if i < len(b) {
					part.Columns = append(part.Columns, b[i])
				} else {
					part.Columns = append(part.Columns, "")
				}
			}
			row = append(row, part)
		}
		row[len(row)-1].AdditionalInfo = info
		rows = append(rows, row...)
	}
	return rows
}

func (table Table) ToStringWithGap(columnGap string) string {
	return table.Render(&TableRenderOptions{ColumnGap: columnGap})
}

type TableRenderOptions struct {
	ColumnGap   string
	HeaderStyle []colors.Color
	ColumnStyle []colors.Color
	Color       colors.Colorization
}

func (table Table) Render(opts *TableRenderOptions) string {
	if opts == nil {
		opts = &TableRenderOptions{}
	}
	if opts.ColumnGap == "" {
		opts.ColumnGap = "  "
	}
	if opts.Color == "" {
		opts.Color = colors.Never
	}

	columnCount := len(table.Headers)

	// Figure out the preferred column width for each column.  It will be set to the max length of
	// any item in that column.
	preferredColumnWidths := make([]int, columnCount)

	allRows := []TableRow{{
		Columns: table.Headers,
	}}

	allRows = append(allRows, table.normalizedRows()...)

	for rowIndex, row := range allRows {
		columns := row.Columns
		if len(columns) != len(preferredColumnWidths) {
			panic(fmt.Sprintf(
				"Error printing table.  Column count of row %v didn't match header column count. %v != %v",
				rowIndex, len(columns), len(preferredColumnWidths)))
		}

		for columnIndex, val := range columns {
			preferredColumnWidths[columnIndex] = max(preferredColumnWidths[columnIndex], MeasureText(val))
		}
	}

	var result strings.Builder
	for rowIndex, row := range allRows {
		result.WriteString(table.Prefix)

		for columnIndex, val := range row.Columns {
			style := opts.HeaderStyle
			if rowIndex != 0 {
				style = opts.ColumnStyle
			}

			if len(style) != 0 {
				result.WriteString(opts.Color.Colorize(style[columnIndex]))
			}

			result.WriteString(val)

			if len(style) != 0 {
				result.WriteString(opts.Color.Colorize(colors.Reset))
			}

			if columnIndex < columnCount-1 {
				// Work out how much whitespace we need to add to this string to bring it up to the
				// preferredColumnWidth for this column.

				maxWidth := preferredColumnWidths[columnIndex]
				padding := maxWidth - MeasureText(val)
				result.WriteString(strings.Repeat(" ", padding))

				// Now, ensure we have the requested gap between columns as well.
				result.WriteString(opts.ColumnGap)
			}
			// do not want whitespace appended to the last column.  It would cause wrapping on lines
			// that were not actually long if some other line was very long.
		}

		result.WriteByte('\n')

		if row.AdditionalInfo != "" {
			result.WriteString(row.AdditionalInfo)
		}
	}
	return result.String()
}
