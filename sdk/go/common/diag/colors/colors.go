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
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const colorLeft = "<{%"
const colorRight = "%}>"

var disableColorization bool

func command(s string) string {
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
	return colorizeText(s.String(), Always, -1)
}

func writeCodes(w io.StringWriter, codes ...string) {
	_, err := w.WriteString("\x1b[")
	contract.IgnoreError(err)
	_, err = w.WriteString(strings.Join(codes, ";"))
	contract.IgnoreError(err)
	_, err = w.WriteString("m")
	contract.IgnoreError(err)
}

func writeDirective(w io.StringWriter, c Colorization, directive string) {
	if disableColorization || c == Never {
		return
	}
	if c == Raw {
		_, err := w.WriteString(directive)
		contract.IgnoreError(err)
		return
	}

	switch directive {
	case Reset: // command("reset")
		writeCodes(w, "0")
	case Bold: // command("bold")
		writeCodes(w, "1")
	case Underline: // command("underline")
		writeCodes(w, "4")
	case Red: // command("fg 1")
		writeCodes(w, "38", "5", "1")
	case Green: // command("fg 2")
		writeCodes(w, "38", "5", "2")
	case Yellow: // command("fg 3")
		writeCodes(w, "38", "5", "3")
	case Blue: // command("fg 4")
		writeCodes(w, "38", "5", "4")
	case Magenta: // command("fg 5")
		writeCodes(w, "38", "5", "5")
	case Cyan: // command("fg 6")
		writeCodes(w, "38", "5", "6")
	case BrightRed: // command("fg 9")
		writeCodes(w, "38", "5", "9")
	case BrightGreen: // command("fg 10")
		writeCodes(w, "38", "5", "10")
	case BrightBlue: // command("fg 12")
		writeCodes(w, "38", "5", "12")
	case BrightMagenta: // command("fg 13")
		writeCodes(w, "38", "5", "13")
	case BrightCyan: // command("fg 14")
		writeCodes(w, "38", "5", "14")
	case RedBackground: // command("bg 1")
		writeCodes(w, "48", "5", "1")
	case GreenBackground: // command("bg 2")
		writeCodes(w, "48", "5", "2")
	case YellowBackground: // command("bg 3")
		writeCodes(w, "48", "5", "3")
	case BlueBackground: // command("bg 4")
		writeCodes(w, "48", "5", "4")
	case Black: // command("fg 0") // Only use with background colors.
		writeCodes(w, "38", "5", "0")
	}
}

func colorizeText(s string, c Colorization, maxLen int) string {
	var buf bytes.Buffer

	textLen, reset := 0, false
	for input := s; len(input) > 0; {
		// Do we have another directive to process?
		nextDirectiveStart := strings.Index(input, colorLeft)
		if nextDirectiveStart == -1 {
			// If there are no more directives and we still have the entire original string, return it as-is: there
			// must not have been any directives.
			if len(input) == len(s) {
				if maxLen >= 0 && len(input) > maxLen {
					return input[:maxLen]
				}
				return input
			}

			// Otherwise, set the start of the next directive to the end of the string and continue.
			nextDirectiveStart = len(input)
		}
		if buf.Cap() < len(input) {
			buf.Grow(len(input))
		}

		// Copy the text up to but not including the delimiter into the buffer.
		text := input[:nextDirectiveStart]
		if maxLen >= 0 && textLen+len(text) > maxLen {
			_, err := buf.WriteString(text[:maxLen-textLen])
			contract.IgnoreError(err)
			if reset {
				writeDirective(&buf, c, Reset)
			}
			break
		}
		_, err := buf.WriteString(text)
		contract.IgnoreError(err)
		textLen += len(text)

		// If we have a start delimiter but no end delimiter, terminate. The partial command will not be present in the
		// output.
		nextDirectiveEnd := strings.Index(input, colorRight)
		if nextDirectiveEnd == -1 {
			break
		}

		directive := command(input[nextDirectiveStart+len(colorLeft) : nextDirectiveEnd])
		writeDirective(&buf, c, directive)
		input = input[nextDirectiveEnd+len(colorRight):]

		reset = directive != Reset
	}

	return buf.String()
}

// Highlight takes an input string, a sequence of commands, and replaces all occurrences of that string with
// a "highlighted" version surrounded by those commands and a final reset afterwards.
func Highlight(s, text, commands string) string {
	return strings.Replace(s, text, commands+text+Reset, -1)
}

var (
	Reset     = command("reset")
	Bold      = command("bold")
	Underline = command("underline")
)

// Basic colors.
var (
	Red           = command("fg 1")
	Green         = command("fg 2")
	Yellow        = command("fg 3")
	Blue          = command("fg 4")
	Magenta       = command("fg 5")
	Cyan          = command("fg 6")
	BrightRed     = command("fg 9")
	BrightGreen   = command("fg 10")
	BrightBlue    = command("fg 12")
	BrightMagenta = command("fg 13")
	BrightCyan    = command("fg 14")

	RedBackground    = command("bg 1")
	GreenBackground  = command("bg 2")
	YellowBackground = command("bg 3")
	BlueBackground   = command("bg 4")

	// We explicitly do not expose blacks/whites.  They're problematic given that we don't know what
	// terminal settings the user has.  Best to avoid them and not run into contrast problems.

	Black = command("fg 0") // Only use with background colors.
	// White         = command("fg 7")
	// BrightBlack   = command("fg 8")
	// BrightYellow  = command("fg 11")
	// BrightWhite   = command("fg 15")
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

	SpecHeadline    = BrightMagenta + Bold // for headings in the CLI.
	SpecSubHeadline = Bold                 // for subheadings in the CLI.
	SpecPrompt      = Cyan + Bold          // for prompting the user.
	SpecAttention   = BrightRed            // for messages that are meant to grab attention.

	// for simple notes.  Just use the standard terminal text color.
	SpecNote = Reset

	SpecCreate            = Green         // for adds (in the diff sense).
	SpecUpdate            = Yellow        // for changes (in the diff sense).
	SpecReplace           = BrightMagenta // for replacements (in the diff sense).
	SpecDelete            = Red           // for deletes (in the diff sense).
	SpecCreateReplacement = BrightGreen   // for replacement creates (in the diff sense).
	SpecDeleteReplaced    = BrightRed     // for replacement deletes (in the diff sense).
	SpecRead              = BrightCyan    // for reads
)
