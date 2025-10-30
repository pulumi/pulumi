package pretty

import pretty "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model/pretty"

// A formatter understands how to turn itself into a string while respecting a desired
// column target.
type Formatter = pretty.Formatter

// A Formatter that wraps an inner value with prefixes and postfixes.
// 
// Wrap attempts to respect its column target by changing if the prefix and postfix are on
// the same line as the inner value, or their own lines.
// 
// As an example, consider the following instance of Wrap:
// 
// 	Wrap {
// 	  Prefix: "number(", Postfix: ")"
// 	  Value: FromString("123456")
// 	}
// 
// It could be rendered as
// 
// 	number(123456)
// 
// or
// 
// 	number(
// 	  123456
// 	)
// 
// depending on the column constrains.
type Wrap = pretty.Wrap

// Object is a Formatter that prints string-Formatter pairs, respecting columns where
// possible.
// 
// It does this by deciding if the object should be compressed into a single line, or have
// one field per line.
type Object = pretty.Object

// An ordered set of items displayed with a separator between them.
// 
// Items can be displayed on a single line if it fits within the column constraint.
// Otherwise items will be displayed across multiple lines.
type List = pretty.List

const DefaultColumns = pretty.DefaultColumns

const DefaultIndent = pretty.DefaultIndent

// Create a new Formatter of a raw string.
func FromString(s string) Formatter {
	return pretty.FromString(s)
}

// Create a new Formatter from a fmt.Stringer.
func FromStringer(s fmt.Stringer) Formatter {
	return pretty.FromStringer(s)
}

