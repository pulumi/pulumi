// Copyright 2016-2024, Pulumi Corporation.
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

package asset

import (
	"regexp"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

var (
	functionRegexp    = regexp.MustCompile(`function __.*`)
	withRegexp        = regexp.MustCompile(`    with\({ .* }\) {`)
	environmentRegexp = regexp.MustCompile(`  }\).apply\(.*\).apply\(this, arguments\);`)
	preambleRegexp    = regexp.MustCompile(
		`function __.*\(\) {\n  return \(function\(\) {\n    with \(__closure\) {\n\nreturn `)
	postambleRegexp = regexp.MustCompile(
		`;\n\n    }\n  }\).apply\(__environment\).apply\(this, arguments\);\n}`)
)

// IsUserProgramCode checks to see if this is the special asset containing the users's code
func IsUserProgramCode(a *resource.Asset) bool {
	if !a.IsText() {
		return false
	}

	text := a.Text

	return functionRegexp.MatchString(text) &&
		withRegexp.MatchString(text) &&
		environmentRegexp.MatchString(text)
}

// MassageIfUserProgramCodeAsset takes the text for a function and cleans it up a bit to make the
// user visible diffs less noisy.  Specifically:
//  1. it tries to condense things by changling multiple blank lines into a single blank line.
//  2. it normalizs the sha hashes we emit so that changes to them don't appear in the diff.
//  3. it elides the with-capture headers, as changes there are not generally meaningful.
//
// TODO(https://github.com/pulumi/pulumi/issues/592) this is baking in a lot of knowledge about
// pulumi serialized functions.  We should try to move to an alternative mode that isn't so brittle.
// Options include:
//  1. Have a documented delimeter format that plan.go will look for.  Have the function serializer
//     emit those delimeters around code that should be ignored.
//  2. Have our resource generation code supply not just the resource, but the "user presentable"
//     resource that cuts out a lot of cruft.  We could then just diff that content here.
func MassageIfUserProgramCodeAsset(asset *resource.Asset, debug bool) *resource.Asset {
	if debug {
		return asset
	}

	// Only do this for strings that match our serialized function pattern.
	if !IsUserProgramCode(asset) {
		return asset
	}

	text := asset.Text
	replaceNewlines := func() {
		for {
			newText := strings.ReplaceAll(text, "\n\n\n", "\n\n")
			if len(newText) == len(text) {
				break
			}

			text = newText
		}
	}

	replaceNewlines()

	firstFunc := functionRegexp.FindStringIndex(text)
	text = text[firstFunc[0]:]

	text = withRegexp.ReplaceAllString(text, "    with (__closure) {")
	text = environmentRegexp.ReplaceAllString(text, "  }).apply(__environment).apply(this, arguments);")

	text = preambleRegexp.ReplaceAllString(text, "")
	text = postambleRegexp.ReplaceAllString(text, "")

	replaceNewlines()

	return &resource.Asset{Text: text}
}
