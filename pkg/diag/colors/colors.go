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

package colors

import (
	"fmt"
	"strings"

	"github.com/reconquest/loreley"

	"github.com/pulumi/pulumi/pkg/util/contract"
)

const colorLeft = "<{%"
const colorRight = "%}>"

func init() {
	// Change the Loreley delimiters from { and }, to something more complex, to avoid accidental collisions.
	loreley.DelimLeft = colorLeft
	loreley.DelimRight = colorRight
}

func Command(s string) string {
	return colorLeft + s + colorRight
}

// TrimPartialCommand returns the input string with any partial colorization command trimmed off of the right end of
// the string.
func TrimPartialCommand(s string) string {
	// First check for a partial left delimiter at the end of the string.
	partialDelimLeft := colorLeft
	if len(partialDelimLeft) > len(s) {
		partialDelimLeft = partialDelimLeft[:len(s)]
	}
	for len(partialDelimLeft) > 0 {
		trailer := s[len(s)-len(partialDelimLeft):]
		if trailer == partialDelimLeft {
			return s[:len(s)-len(partialDelimLeft)]
		}
		partialDelimLeft = partialDelimLeft[:len(partialDelimLeft)-1]
	}

	// Next check for a complete left delimiter. If there no complete left delimiter, just return the string as-is.
	lastDelimLeft := strings.LastIndex(s, colorLeft)
	if lastDelimLeft == -1 {
		return s
	}

	// If there is a complete left delimiter, look for a matching complete right delimiter. If there is a match, return
	// the string as-is.
	if strings.Contains(s[lastDelimLeft:], colorRight) {
		return s
	}

	// Otherwise, return the string up to but not including the incomplete left delimiter.
	return s[:lastDelimLeft]
}

func Colorize(s fmt.Stringer) string {
	txt := s.String()
	return colorizeText(txt)
}

func colorizeText(s string) string {
	c, err := loreley.CompileAndExecuteToString(s, nil, nil)
	contract.Assertf(err == nil, "Expected no errors during string colorization; str=%v, err=%v", s, err)
	return c
}

// Highlight takes an input string, a sequence of commands, and replaces all occurrences of that string with
// a "highlighted" version surrounded by those commands and a final reset afterwards.
func Highlight(s, text, commands string) string {
	return strings.Replace(s, text, commands+text+Reset, -1)
}

var (
	Reset     = Command("reset")
	Bold      = Command("bold")
	Underline = Command("underline")
)

// Basic colors.
var (
	Red           = Command("fg 1")
	Green         = Command("fg 2")
	Yellow        = Command("fg 3")
	Blue          = Command("fg 4")
	Magenta       = Command("fg 5")
	Cyan          = Command("fg 6")
	BrightRed     = Command("fg 9")
	BrightGreen   = Command("fg 10")
	BrightBlue    = Command("fg 12")
	BrightMagenta = Command("fg 13")
	BrightCyan    = Command("fg 14")

	// We explicitly do not expose blacks/whites.  They're problematic given that we don't know what
	// terminal settings the user has.  Best to avoid them and not run into contrast problems.

	// Black         = Command("fg 0")
	// White         = Command("fg 7")
	// BrightBlack   = Command("fg 8")
	// BrightYellow  = Command("fg 11")
	// BrightWhite   = Command("fg 15")
)

// Special predefined colors for logical conditions.
var (
	SpecImportant = Yellow // for particularly noteworthy messages.

	// for notes that can be skimmed or aren't very important.  Just use the standard terminal text
	// color.
	SpecUnimportant = Reset

	SpecDebug   = SpecUnimportant // for debugging.
	SpecInfo    = Magenta         // for information.
	SpecError   = Red             // for errors.
	SpecWarning = Yellow          // for warnings.

	SpecHeadline  = BrightMagenta + Bold // for headings in the CLI.
	SpecPrompt    = Cyan + Bold          // for prompting the user
	SpecAttention = BrightRed            // for messages that are meant to grab attention.

	// for simple notes.  Just use the standard terminal text color.
	SpecNote = Reset

	SpecCreate            = Green         // for adds (in the diff sense).
	SpecUpdate            = Yellow        // for changes (in the diff sense).
	SpecReplace           = BrightMagenta // for replacements (in the diff sense).
	SpecDelete            = Red           // for deletes (in the diff sense).
	SpecCreateReplacement = BrightGreen   // for replacement creates (in the diff sense).
	SpecDeleteReplaced    = BrightRed     // for replacement deletes (in the diff sense).

	// for reads (relatively unimportant).  Just use the standard terminal text color.
	SpecRead = Reset
)
