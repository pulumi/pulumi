// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package colors

import (
	"regexp"

	"github.com/pulumi/pulumi/pkg/util/contract"
)

var tagRegexp = regexp.MustCompile(`<\{%(.*?)%\}>`)

// Colorization is an instruction to perform a certain kind of colorization.
type Colorization string

const (
	// Always colorizes text.
	Always Colorization = "always"
	// Never colorizes text.
	Never Colorization = "never"
	// Raw returns text with the raw control sequences, rather than colorizing them.
	Raw Colorization = "raw"
)

// Colorize conditionally colorizes the given string based on the kind of colorization selected.
func (c Colorization) Colorize(v string) string {
	switch c {
	case Raw:
		// Don't touch the string.  Output control sequences as is.
		return v
	case Always:
		// Convert the constrol sequences into appropriate console escapes for the platform we're on.
		return ColorizeText(v)
	case Never:
		// Remove all the colors that any other layers added.
		return tagRegexp.ReplaceAllString(v, "")
	default:
		contract.Failf("Unexpected colorization value: %v", c)
		return ""
	}
}
