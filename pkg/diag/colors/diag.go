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
	"regexp"

	"github.com/pulumi/pulumi/pkg/util/contract"
)

var tagRegexp = regexp.MustCompile(`<\{%(.*?)%\}>`)

// Colorization is an instruction to perform a certain kind of colorization.
type Colorization string

const (
	// Determine if we should colorize depending on the environment we're in.
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
		return colorizeText(v)
	case Never:
		// Remove all the colors that any other layers added.
		return tagRegexp.ReplaceAllString(v, "")
	default:
		contract.Failf("Unexpected colorization value: %v", c)
		return ""
	}
}

// SplitIntoTextAndTags breaks up a colorized string into alternating sections of a raw-text-chunk
// and then a color-tag.  The returned array will always be non empty, with the first and last
// elements in the array being (possibly empty) raw-text-chunks.  For example, if you started with
// "<%fg 8>hello<%fg 7>", this would return:  ["", "<%fg 8>", "hello", "<%fg 7>", ""]
func SplitIntoTextAndTags(v string) []string {
	tagIndices := tagRegexp.FindAllStringIndex(v, -1)

	currentIndex := 0
	textAndTags := []string{}
	for _, tagPair := range tagIndices {
		tagStart := tagPair[0]
		tagEnd := tagPair[1]
		textAndTags = append(textAndTags, v[currentIndex:tagStart])
		textAndTags = append(textAndTags, v[tagStart:tagEnd])
		currentIndex = tagEnd
	}

	textAndTags = append(textAndTags, v[currentIndex:])

	return textAndTags
}

// TrimColorizedString takes a string with embedded color tags and returns a new string (still with
// embedded color tags) such that the length of the *non-tag* portion of the string is no greater
// than maxLength.  This is useful for scenarios where the string has to be printed in a a context
// where there is a max allowed width.  In these scenarios, we can't just measure the length of the
// string as the embedded color tags would count against it, even though they end up with no length
// when actually interpretted by the console.
func TrimColorizedString(v string, maxRuneLength int) string {
	textAndTags := SplitIntoTextAndTags(v)

	currentRuneLength := 0
	trimmed := ""

	for i := 0; i < len(textAndTags); i++ {
		textOrTag := textAndTags[i]

		if i%2 == 0 {
			contract.Assertf(!tagRegexp.MatchString(textOrTag), "Got a tag when we did not expect it")

			chunk := textOrTag
			chunkRunes := []rune(chunk)
			chunkRunesLen := len(chunkRunes)

			if currentRuneLength+chunkRunesLen > maxRuneLength {
				// adding this text chunk will cause us to go past the max length we allow.
				// just take whatever subportion we can and stop what we're doing.
				trimmed += string(chunkRunes[0 : maxRuneLength-currentRuneLength])
				break
			} else {
				// can safely add this text chunk
				trimmed += chunk
				currentRuneLength += chunkRunesLen
			}
		} else {
			contract.Assertf(tagRegexp.MatchString(textOrTag), "Should have gotten a tag")

			// can safely add the tag to the trimmed string.  tags don't contribute any actual length.
			trimmed += textOrTag
		}
	}

	// add a trailing reset, so that any unclosed tags will be closed.
	trimmed += Reset

	return trimmed
}
