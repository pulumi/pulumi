package pcl

import pcl "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/pcl"

// Invoke is the name of the PCL `invoke` intrinsic, which can be used to invoke provider functions.
const Invoke = pcl.Invoke

// Detects invoke calls that use an output version of a function.
func IsOutputVersionInvokeCall(call *model.FunctionCallExpression) bool {
	return pcl.IsOutputVersionInvokeCall(call)
}

// Pattern matches to recognize `__convert(objCons(..))` pattern that
// is used to annotate object constructors with appropriate nominal
// types. If the expression matches, returns true followed by the
// constructor expression and the appropriate type.
func RecognizeTypedObjectCons(theExpr model.Expression) (bool, *model.ObjectConsExpression, model.Type) {
	return pcl.RecognizeTypedObjectCons(theExpr)
}

// Pattern matches to recognize an encoded call to an output-versioned
// invoke, such as `invoke(token, __convert(objCons(..)))`. If
// matching, returns the `args` expression and its schema-bound type.
func RecognizeOutputVersionedInvoke(expr *model.FunctionCallExpression) (bool, *model.ObjectConsExpression, model.Type) {
	return pcl.RecognizeOutputVersionedInvoke(expr)
}

