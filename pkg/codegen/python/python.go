package python

import python "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/python"

// Keywords is a map of reserved keywords used by Python 2 and 3.  We use this to avoid generating unspeakable
// names in the resulting code.  This map was sourced by merging the following reference material:
// 
//   - Python 2: https://docs.python.org/2.5/ref/keywords.html
//   - Python 3: https://docs.python.org/3/reference/lexical_analysis.html#keywords
var Keywords = python.Keywords

// PyName turns a variable or function name, normally using camelCase, to an underscore_case name.
func PyName(name string) string {
	return python.PyName(name)
}

// EnsureKeywordSafe adds a trailing underscore if the generated name clashes with a Python 2 or 3 keyword, per
// PEP 8: https://www.python.org/dev/peps/pep-0008/?#function-and-method-arguments
func EnsureKeywordSafe(name string) string {
	return python.EnsureKeywordSafe(name)
}

