package cgstrings

import cgstrings "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/cgstrings"

// Unhyphenate removes all hyphens from s, then uppercasing the letter following each hyphen.
// For example, "abc-def-ghi" becomes "abcDefGhi".
func Unhyphenate(str string) string {
	return cgstrings.Unhyphenate(str)
}

// Camel converts s to camelCase.
func Camel(s string) string {
	return cgstrings.Camel(s)
}

// UppercaseFirst uppercases the first letter of s.
// E.g. "abc" -> "Abc"
func UppercaseFirst(s string) string {
	return cgstrings.UppercaseFirst(s)
}

func ModifyStringAroundDelimeter(str, delim string, modifyNext func(string) string) string {
	return cgstrings.ModifyStringAroundDelimeter(str, delim, modifyNext)
}

