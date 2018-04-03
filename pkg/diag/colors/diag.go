// Copyright 2017-2018, Pulumi Corporation.
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
