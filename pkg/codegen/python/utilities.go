// Copyright 2020-2024, Pulumi Corporation.
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

package python

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
)

// isLegalIdentifierStart returns true if it is legal for c to be the first character of a Python identifier as per
// https://docs.python.org/3.7/reference/lexical_analysis.html#identifiers.
func isLegalIdentifierStart(c rune) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' ||
		unicode.In(c, unicode.Lu, unicode.Ll, unicode.Lt, unicode.Lm, unicode.Lo, unicode.Nl)
}

// isLegalIdentifierPart returns true if it is legal for c to be part of a Python identifier (besides the first
// character) as per https://docs.python.org/3.7/reference/lexical_analysis.html#identifiers.
func isLegalIdentifierPart(c rune) bool {
	return isLegalIdentifierStart(c) || c >= '0' && c <= '9' ||
		unicode.In(c, unicode.Lu, unicode.Ll, unicode.Lt, unicode.Lm, unicode.Lo, unicode.Nl, unicode.Mn, unicode.Mc,
			unicode.Nd, unicode.Pc)
}

// isLegalIdentifier returns true if s is a legal Python identifier as per
// https://docs.python.org/3.7/reference/lexical_analysis.html#identifiers.
func isLegalIdentifier(s string) bool {
	reader := strings.NewReader(s)
	c, _, _ := reader.ReadRune()
	if !isLegalIdentifierStart(c) {
		return false
	}
	for {
		c, _, err := reader.ReadRune()
		if err != nil {
			return err == io.EOF
		}
		if !isLegalIdentifierPart(c) {
			return false
		}
	}
}

// makeValidIdentifier replaces characters that are not allowed in Python identifiers with underscores. No attempt is
// made to ensure that the result is unique.
func makeValidIdentifier(name string) string {
	var builder strings.Builder
	for i, c := range name {
		if !isLegalIdentifierPart(c) {
			builder.WriteRune('_')
		} else {
			if i == 0 && !isLegalIdentifierStart(c) {
				builder.WriteRune('_')
			}
			builder.WriteRune(c)
		}
	}
	return builder.String()
}

func makeSafeEnumName(name, typeName string) (string, error) {
	// Replace common single character enum names.
	safeName := codegen.ExpandShortEnumName(name)

	// If the name is one illegal character, return an error.
	if len(safeName) == 1 && !isLegalIdentifierStart(rune(safeName[0])) {
		return "", fmt.Errorf("enum name %s is not a valid identifier", safeName)
	}

	// If it's camelCase, change it to snake_case.
	safeName = PyName(safeName)

	// Change to uppercase and make a valid identifier.
	safeName = makeValidIdentifier(strings.ToTitle(safeName))

	// If the enum name starts with an underscore, add the type name as a prefix.
	if strings.HasPrefix(safeName, "_") {
		pyTypeName := strings.ToTitle(PyName(typeName))
		safeName = pyTypeName + safeName
	}

	// If there are multiple underscores in a row, replace with one.
	regex := regexp.MustCompile(`_+`)
	safeName = regex.ReplaceAllString(safeName, "_")

	return safeName, nil
}

var pypiReleaseTranslations = []struct {
	prefix     string
	replacment string
}{
	{"alpha", "a"},
	{"beta", "b"},
}

// A valid release tag for pypi
var pypiRelease = regexp.MustCompile("^(a|b|rc)[0-9]+$")

// A valid dev tag for pypi
var pypiDev = regexp.MustCompile("^dev[0-9]+$")

// A valid post tag for pypi
var pypiPost = regexp.MustCompile("^post[0-9]+$")

// Transforms 0.0.1-alpha.18 to 0.0.1-alpha18; our users want to be able to use the previous form but semver.Version
// parses it into two Pre segments ["alpha", "18"] which trips up translation. The same treatment is given beta and rc
// versions.
func normPypiVersion(v semver.Version) semver.Version {
	s := v.String()
	s = regexp.MustCompile(`(alpha|beta|rc)[.](\d+)`).ReplaceAllString(s, `$1$2`)
	return semver.MustParse(s)
}

// PypiVersion translates semver 2.0 into pypi's versioning scheme:
// Details can be found here: https://www.python.org/dev/peps/pep-0440/#version-scheme
// [N!]N(.N)*[{a|b|rc}N][.postN][.devN]
func PypiVersion(v semver.Version) string {
	v = normPypiVersion(v)

	localList := slice.Prealloc[string](len(pypiReleaseTranslations))

	getRelease := func(maybeRelease string) string {
		for _, tup := range pypiReleaseTranslations {
			if strings.HasPrefix(maybeRelease, tup.prefix) {
				guess := tup.replacment + maybeRelease[len(tup.prefix):]
				if pypiRelease.MatchString(guess) {
					return guess
				}
			}
		}
		if pypiRelease.MatchString(maybeRelease) {
			return maybeRelease
		}
		return ""
	}
	getDev := func(maybeDev string) string {
		if pypiDev.MatchString(maybeDev) {
			return "." + maybeDev
		}
		return ""
	}

	getPost := func(maybePost string) string {
		if pypiPost.MatchString(maybePost) {
			return "." + maybePost
		}
		return ""
	}

	var preListIndex int

	var release string
	var dev string
	var post string
	// We allow the first pre-release in `v` to indicate the release for the
	// pypi version.
	for _, special := range []struct {
		getFunc  func(string) string
		maybeSet *string
	}{
		{getRelease, &release},
		{getDev, &dev},
		{getPost, &post},
	} {
		if len(v.Pre) > preListIndex && special.getFunc(v.Pre[preListIndex].VersionStr) != "" {
			*special.maybeSet = special.getFunc(v.Pre[preListIndex].VersionStr)
			preListIndex++
		}
	}

	// All other pre-release segments are added to the local identifier. If we
	// didn't find a release, the first pre-release is also added to the local
	// identifier.
	if release != "" {
		preListIndex = 1
	}
	for ; preListIndex < len(v.Pre); preListIndex++ {
		// This can only contain [0-9a-zA-Z-] because semver enforces that set
		// and '-' we need only replace '-' with a valid character: '.'
		localList = append(localList, strings.ReplaceAll(v.Pre[preListIndex].VersionStr, "-", "."))
	}
	// All build flags are added to the local identifier list
	for _, b := range v.Build {
		// This can only contain [0-9a-zA-Z-] because semver enforces that set
		// and '-' we need only replace '-' with a valid character: '.'
		localList = append(localList, strings.ReplaceAll(b, "-", "."))
	}
	local := ""
	if len(localList) > 0 {
		local = "+" + strings.Join(localList, ".")
	}
	return fmt.Sprintf("%d.%d.%d%s%s%s%s", v.Major, v.Minor, v.Patch, release, dev, post, local)
}

// pythonCase converts s to PascalCase, ignoring underscores, e.g. __myWords -> __MyWords.
func pythonCase(s string) string {
	var underscores string
	noUnderscores := strings.TrimLeftFunc(s, func(r rune) bool {
		if r != '_' {
			return false
		}
		underscores += "_"
		return true
	})
	c := cgstrings.Unhyphenate(noUnderscores)
	return underscores + cgstrings.UppercaseFirst(c)
}
