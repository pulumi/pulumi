// Copyright 2016 Marapongo, Inc. All rights reserved.

package colors

import (
	"github.com/reconquest/loreley"

	"github.com/marapongo/mu/pkg/util/contract"
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

func Colorize(s string) string {
	c, err := loreley.CompileAndExecuteToString(s, nil, nil)
	contract.Assertf(err == nil, "Expected no errors during string colorization; str=%v, err=%v", s, err)
	return c
}

// Basic
var (
	Black         = Command("fg 0")
	Red           = Command("fg 1")
	Green         = Command("fg 2")
	Yellow        = Command("fg 3")
	Blue          = Command("fg 4")
	Magenta       = Command("fg 5")
	Cyan          = Command("fg 6")
	White         = Command("fg 7")
	BrightBlack   = Command("fg 8")
	BrightRed     = Command("fg 9")
	BrightGreen   = Command("fg 10")
	BrightYellow  = Command("fg 11")
	BrightBlue    = Command("fg 12")
	BrightMagenta = Command("fg 13")
	BrightCyan    = Command("fg 14")
	BrightWhite   = Command("fg 15")
	Reset         = Command("reset")
)

// Special predefined colors for logical conditions.
var (
	SpecInfo      = Magenta      // for information.
	SpecError     = Red          // for errors.
	SpecWarning   = Yellow       // for warnings.
	SpecLocation  = Cyan         // for source locations.
	SpecFatal     = BrightRed    // for fatal errors
	SpecNote      = White        // for notes
	SpecImportant = BrightYellow // for particularly noteworthy messages.

	SpecAdded   = Green  // for adds (in the diff sense).
	SpecChanged = Yellow // for changes (in the diff sense).
	SpecDeleted = Red    // for deletes (in the diff sense).
)
