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

package dotnet

import (
	"strings"
	"unicode"
)

// isReservedWord returns true if s is a C# reserved word as per
// https://docs.microsoft.com/en-us/dotnet/csharp/language-reference/language-specification/lexical-structure#keywords
func isReservedWord(s string) bool {
	switch s {
	case "abstract", "as", "base", "bool", "break", "byte", "case", "catch", "char", "checked", "class", "const",
	     "continue", "decimal", "default", "delegate", "do", "double", "else", "enum", "event", "explicit", "extern",
	     "false", "finally", "fixed", "float", "for", "foreach", "goto", "if", "implicit", "in", "int", "interface",
	     "internal", "is", "lock", "long", "namespace", "new", "null", "object", "operator", "out", "override",
	     "params", "private", "protected", "public", "readonly" ,"ref", "return", "sbyte", "sealed", "short",
	     "sizeof", "stackalloc", "static", "string", "struct", "switch", "this", "throw", "true", "try", "typeof",
	     "uint", "ulong", "unchecked", "unsafe", "ushort", "using", "virtual", "void", "volatile", "while":
		return true

	default:
		return false
	}
}

// isLegalIdentifierStart returns true if it is legal for c to be the first character of a C# identifier as per
// https://docs.microsoft.com/en-us/dotnet/csharp/language-reference/language-specification/lexical-structure#identifiers
func isLegalIdentifierStart(c rune) bool {
	return c == '_' ||
		unicode.In(c, unicode.Lu, unicode.Ll, unicode.Lt, unicode.Lm, unicode.Lo, unicode.Nl)
}

// isLegalIdentifierPart returns true if it is legal for c to be part of a C# identifier (besides the first character)
// as per https://docs.microsoft.com/en-us/dotnet/csharp/language-reference/language-specification/lexical-structure#identifiers.
func isLegalIdentifierPart(c rune) bool {
	return isLegalIdentifierStart(c) || unicode.In(c, unicode.Mn, unicode.Mc, unicode.Nd, unicode.Pc, unicode.Cf)
}

// validIdentifier replaces characters that are not allowed in C# identifiers with underscores. A reserved word is
// prefixed with @. No attempt is made to ensure that the result is unique.
func validIdentifier(name string) string {
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
	name = builder.String()
	if isReservedWord(name) {
		return "@" + name
	}
	return name
}

// propertyName returns a name as a valid identifier in title case.
func propertyName(name string) string {
	return Title(validIdentifier(name))
}
