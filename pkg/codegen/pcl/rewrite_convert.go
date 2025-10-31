package pcl

import pcl "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/pcl"

// RewriteConversions wraps automatic conversions indicated by the HCL2 spec and conversions to schema-annotated types
// in calls to the __convert intrinsic.
// 
// Note that the result is a bit out of line with the HCL2 spec, as static conversions may happen earlier than they
// would at runtime. For example, consider the case of a tuple of strings that is being converted to a list of numbers:
// 
// 	[a, b, c]
// 
// Calling RewriteConversions on this expression with a destination type of list(number) would result in this IR:
// 
// 	[__convert(a), __convert(b), __convert(c)]
// 
// If any of these conversions fail, the evaluation of the tuple itself fails. The HCL2 evaluation semantics, however,
// would convert the tuple _after_ it has been evaluated. The IR that matches these semantics is
// 
// 	__convert([a, b, c])
// 
// This transform uses the former representation so that it can appropriately insert calls to `__convert` in the face
// of schema-annotated types. There is a reasonable argument to be made that RewriteConversions should not be
// responsible for propagating schema annotations, and that this pass should be split in two: one pass would insert
// conversions that match HCL2 evaluation semantics, and another would insert calls to some separate intrinsic in order
// to propagate schema information.
func RewriteConversions(x model.Expression, to model.Type) (model.Expression, hcl.Diagnostics) {
	return pcl.RewriteConversions(x, to)
}

// LowerConversion lowers a conversion for a specific value, such that
// converting `from` to a value of the returned type will produce valid code.
// The algorithm prioritizes safe conversions over unsafe conversions. If no
// conversion can be found, nil, false is returned.
// 
// This is useful because it cuts out conversion steps which the caller doesn't
// need to worry about. For example:
// Given inputs
// 
// 	from = string("foo") # a constant string with value "foo"
// 	to = union(enum(string: "foo", "bar"), input(enum(string: "foo", "bar")), none)
// 
// We would receive output type:
// 
// 	enum(string: "foo", "bar")
// 
// since the caller can convert string("foo") to the enum directly, and does not
// need to consider the union.
// 
// For another example consider inputs:
// 
// 	from = var(string) # A variable of type string
// 	to = union(enum(string: "foo", "bar"), string)
// 
// We would return type:
// 
// 	string
// 
// since var(string) can be safely assigned to string, but unsafely assigned to
// enum(string: "foo", "bar").
func LowerConversion(from model.Expression, to model.Type) model.Type {
	return pcl.LowerConversion(from, to)
}

