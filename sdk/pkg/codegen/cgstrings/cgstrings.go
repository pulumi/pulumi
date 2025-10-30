// Copyright 2022-2024, Pulumi Corporation.
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

// package cgstrings has various string processing functions that are useful during code generation.
package cgstrings

import (
	"strings"
	"unicode"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
)

// Unhyphenate removes all hyphens from s, then uppercasing the letter following each hyphen.
// For example, "abc-def-ghi" becomes "abcDefGhi".
func Unhyphenate(str string) string {
	return ModifyStringAroundDelimeter(str, "-", UppercaseFirst)
}

// Camel converts s to camelCase.
func Camel(s string) string {
	if s == "" {
		return ""
	}
	s = Unhyphenate(s)
	runes := []rune(s)
	res := slice.Prealloc[rune](len(runes))
	for i, r := range runes {
		if unicode.IsLower(r) {
			res = append(res, runes[i:]...)
			break
		}
		res = append(res, unicode.ToLower(r))
	}
	return string(res)
}

// UppercaseFirst uppercases the first letter of s.
// E.g. "abc" -> "Abc"
func UppercaseFirst(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func ModifyStringAroundDelimeter(str, delim string, modifyNext func(next string) string) string {
	if delim == "" {
		return str
	}
	i := strings.Index(str, delim)
	if i < 0 {
		return str
	}
	nextIdx := i + len(delim)
	if nextIdx >= len(str) {
		// Nothing left after the delimeter, it's at the end of the string.
		return str[:len(str)-len(delim)]
	}
	prev := str[:nextIdx-1]
	next := str[nextIdx:]
	if next != "" {
		next = modifyNext(next)
	}
	return prev + ModifyStringAroundDelimeter(next, delim, modifyNext)
}
