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
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

type Colorization string

const (
	// Auto determines if we should colorize depending on the surrounding environment we're in.
	Auto Colorization = "auto"
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
		return colorizeText(v, Always, -1)
	case Never:
		return colorizeText(v, Never, -1)
	default:
		contract.Failf("Unexpected colorization value: %v", c)
		return ""
	}
}

// TrimColorizedString takes a string with embedded color tags and returns a new string (still with
// embedded color tags) such that the length of the *non-tag* portion of the string is no greater
// than maxLength.  This is useful for scenarios where the string has to be printed in a a context
// where there is a max allowed width.  In these scenarios, we can't just measure the length of the
// string as the embedded color tags would count against it, even though they end up with no length
// when actually interpretted by the console.
func TrimColorizedString(v string, maxRuneLength int) string {
	return colorizeText(v, Raw, maxRuneLength)
}
