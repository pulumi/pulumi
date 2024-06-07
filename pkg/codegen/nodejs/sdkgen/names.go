package sdkgen

import (
	"unicode"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
)

// titleCase capitalizes the first rune in s.
//
// Examples:
// "hello"   => "Hello"
// "hiAlice" => "HiAlice"
// "hi.Bob"  => "Hi.Bob"
//
// Note: This is expected to work on strings which are not valid identifiers.
func titleCase(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	return string(append([]rune{unicode.ToUpper(runes[0])}, runes[1:]...))
}

// camelCase converts s to camel-case.
//
// Examples:
// "helloWorld"    => "helloWorld"
// "HelloWorld"    => "helloWorld"
// "JSONObject"    => "jsonobject"
// "My-FRIEND.Bob" => "my-FRIEND.Bob"
func camelCase(s string) string {
	if s == "" {
		return ""
	}
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
