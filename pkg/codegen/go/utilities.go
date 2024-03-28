// Copyright 2016-2020, Pulumi Corporation.
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

package gen

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/cgstrings"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// isReservedWord returns true if s is a Go reserved word as per
// https://golang.org/ref/spec#Keywords
func isReservedWord(s string) bool {
	switch s {
	case "break", "default", "func", " interface", "select",
		"case", "defer", "go", "map", "struct",
		"chan", "else", "goto", "package", "switch",
		"const", "fallthrough", "if", "range", "type",
		"continue", "for", "import", "return", "var":
		return true

	default:
		return false
	}
}

// isReservedResourceField returns true if s would conflict with a method on a generated
// resource.
func isReservedResourceField(resourceName, s string) bool {
	switch s {
	case "ID", "URN", "GetProvider", "ElementType":
		return true
	default:
		if resourceName != "" {
			toOutput := "To" + resourceName + "Output"
			return s == toOutput || s == toOutput+"WithContext"
		}
		return false
	}
}

// isLegalIdentifierStart returns true if it is legal for c to be the first character of a Go identifier as per
// https://golang.org/ref/spec#Identifiers
func isLegalIdentifierStart(c rune) bool {
	return c == '_' || unicode.In(c, unicode.Letter)
}

// isLegalIdentifierPart returns true if it is legal for c to be part of a Go identifier (besides the first character)
// https://golang.org/ref/spec#Identifiers
func isLegalIdentifierPart(c rune) bool {
	return c == '_' ||
		unicode.In(c, unicode.Letter, unicode.Digit)
}

// makeValidIdentifier replaces characters that are not allowed in Go identifiers with underscores. A reserved word is
// prefixed with _. No attempt is made to ensure that the result is unique.
func makeValidIdentifier(name string) string {
	var builder strings.Builder
	for i, c := range name {
		// ptr dereference
		if i == 0 && c == '&' {
			builder.WriteRune(c)
			continue
		}
		if !isLegalIdentifierPart(c) {
			builder.WriteRune('_')
		} else {
			if i == 0 && !isLegalIdentifierStart(c) {
				builder.WriteRune('_')
			}
			builder.WriteRune(c)
		}
	}
	name = builder.String()
	if isReservedWord(name) {
		return "_" + name
	}
	return name
}

func makeSafeEnumName(name, typeName string) (string, error) {
	safeName := codegen.ExpandShortEnumName(name)

	// If the name is one illegal character, return an error.
	if len(safeName) == 1 && !isLegalIdentifierStart(rune(safeName[0])) {
		return "", fmt.Errorf("enum name %s is not a valid identifier", safeName)
	}

	// Capitalize and make a valid identifier.
	safeName = enumTitle(safeName)
	safeName = makeValidIdentifier(safeName)

	// If there are multiple underscores in a row, replace with one.
	regex := regexp.MustCompile(`_+`)
	safeName = regex.ReplaceAllString(safeName, "_")

	// Add the type to the name to disambiguate constants used for enum values
	if strings.Contains(safeName, "_") && !strings.HasPrefix(safeName, "_") {
		safeName = "_" + safeName
	}

	safeName = typeName + safeName

	return safeName, nil
}

// Title converts the input string to a title case
// where only the initial letter is upper-cased.
// It also removes $-prefix if any.
func enumTitle(s string) string {
	if s == "" {
		return ""
	}
	if s[0] == '$' {
		return Title(s[1:])
	}
	s = cgstrings.UppercaseFirst(s)
	return cgstrings.ModifyStringAroundDelimeter(s, "-", func(next string) string {
		return "_" + cgstrings.UppercaseFirst(next)
	})
}

// Calculate the name of a field in a resource
func fieldName(pkg *pkgContext, r *schema.Resource, p *schema.Property) string {
	s := Title(p.Name)
	var name string
	if r != nil {
		name = disambiguatedResourceName(r, pkg)
	}
	if !isReservedResourceField(name, s) {
		return s
	}

	res := s + "_"
	contract.Assertf(!isReservedResourceField(name, res), "Name %q is reserved on resource %q", name, res)
	return res
}
