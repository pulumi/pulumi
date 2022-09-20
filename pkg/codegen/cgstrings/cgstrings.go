// package cgstrings has various string processing functions that are useful during code generation.
package cgstrings

import (
	"strings"
	"unicode"
)

// Unhyphenate removes all hyphens from s, then uppercasing the letter following each hyphen.
// For example, "abc-def-ghi" becomes "abcDefGhi".
func Unhyphenate(str string) string {
	return modifyStringAroundDelimeter(str, "-", UppercaseFirst)
}

// Camel converts s to camelCase.
func Camel(s string) string {
	if s == "" {
		return ""
	}
	s = Unhyphenate(s)
	runes := []rune(s)
	res := make([]rune, 0, len(runes))
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
	return strings.ToUpper(s[:1]) + s[1:]
}

func modifyStringAroundDelimeter(str, delim string, modifyNext func(next string) string) string {
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
	return prev + modifyStringAroundDelimeter(next, delim, modifyNext)
}
