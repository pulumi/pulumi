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
	Black         = Command("fg 0")
	Red           = Command("fg 1")
	Green         = Command("fg 2")
	Yellow        = Command("fg 3")
	Blue          = Command("fg 4")
	Magenta       = Command("fg 5")
	Cyan          = Command("fg 6")
	White         = Command("fg 7")
	BrightBlack   = Reset
	BrightRed     = Command("fg 9")
	BrightGreen   = Command("fg 10")
	BrightYellow  = Command("fg 11")
	BrightBlue    = Command("fg 12")
	BrightMagenta = Command("fg 13")
	BrightCyan    = Command("fg 14")
	BrightWhite   = Bold
)

// Special predefined colors for logical conditions.
var (
	SpecImportant   = BrightYellow // for particularly noteworthy messages.
	SpecUnimportant = BrightBlack  // for notes that can be skimmed or aren't very important.

	SpecDebug   = SpecUnimportant // for debugging.
	SpecInfo    = Magenta         // for information.
	SpecError   = Red             // for errors.
	SpecWarning = Yellow          // for warnings.

	SpecLocation  = Cyan      // for source locations.
	SpecAttention = BrightRed // for messages that are meant to grab attention.
	SpecNote      = White     // for simple notes.

	SpecCreate            = Green        // for adds (in the diff sense).
	SpecUpdate            = BrightYellow // for changes (in the diff sense).
	SpecRead              = BrightWhite  // for reads (relatively unimportant).
	SpecReplace           = Yellow       // for replacements (in the diff sense).
	SpecDelete            = Red          // for deletes (in the diff sense).
	SpecCreateReplacement = BrightGreen  // for replacement creates (in the diff sense).
	SpecDeleteReplaced    = BrightRed    // for replacement deletes (in the diff sense).
)
