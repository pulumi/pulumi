package codegen

import (
	"strings"
	"unicode"
)

// ConvertHyphens removes all hyphens from s, replacing them with r, uppercasing the letter following each hyphen.
func ConvertHyphens(s string, r string) string {
	sep := "-"
	if !strings.Contains(s, sep) {
		return s
	}
	parts := Separate(s, sep)
	sb := strings.Builder{}
	for i, part := range parts {
		if part == sep {
			sb.WriteString(r)
			continue
		}
		// Capitalize only if this isn't the first chunk.
		// That is, we want "ping-thud" to become "pingThud", not "PingThud".
		if i > 0 && parts[i-1] == sep {
			rep := string(unicode.ToUpper(rune(part[0]))) + part[1:]
			sb.WriteString(rep)
			continue
		}
		sb.WriteString(part)
	}
	return sb.String()
}

// Camel converts s to camelCase.
func Camel(s string) string {
	if s == "" {
		return ""
	}
	s = ConvertHyphens(s, "")
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

// PythonCase converts s to PascalCase, ignoring underscores, e.g. __myWords -> __MyWords.
func PythonCase(s string) string {
	var underscores string
	noUnderscores := strings.TrimLeftFunc(s, func(r rune) bool {
		if r != '_' {
			return false
		}
		underscores += "_"
		return true
	})
	c := Camel(noUnderscores)
	return underscores + strings.ToUpper(c[:1]) + c[1:]
}

// Separate is similar to strings.Split but it keeps separator tokens in the results.
// For example, Separate("a,b,c", ",") returns []string{"a", ",", "b", ",", "c"}.
func Separate(s, separator string) []string {
	if s == "" {
		return nil
	}
	if separator == "" {
		// If no separator is specified then the behavior is identical to strings.Split
		return strings.Split(s, "")
	}

	if strings.HasPrefix(s, separator) {
		rest := Separate(s[len(separator):], separator)
		return append([]string{separator}, rest...)
	}
	rest := Separate(s[1:], separator)
	if len(rest) == 0 || rest[0] == separator {
		rest = append([]string{""}, rest...)
	}
	rest[0] = string(s[0]) + rest[0]
	return rest
}
