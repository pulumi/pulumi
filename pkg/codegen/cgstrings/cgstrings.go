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

// Unpunctuate removes *most* punctuation and whitespace from the given string.
// The result is camelCase.
//
// Underscores are preserved for backwards compatibility with schemas that
// already contain underscores.
func Unpunctuate(str string) string {
	var builder strings.Builder
	var parts []string

	needsCapitalization := false
	for _, r := range str {
		if !(unicode.IsPunct(r) || unicode.IsSpace(r)) || r == '_' {
			if needsCapitalization {
				r = unicode.ToUpper(r)
				needsCapitalization = false
			}
			builder.WriteRune(r)
			continue
		}
		if builder.Len() > 0 {
			parts = append(parts, builder.String())
			builder.Reset()
			needsCapitalization = true
		}
	}
	if builder.Len() > 0 {
		parts = append(parts, builder.String())
	}

	return strings.Join(parts, "")
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
