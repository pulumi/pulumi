package pcl

import pcl "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/pcl"

type DeferredOutputVariable = pcl.DeferredOutputVariable

func SourceOrderNodes(nodes []Node) []Node {
	return pcl.SourceOrderNodes(nodes)
}

func DecomposeToken(tok string, sourceRange hcl.Range) (string, string, string, hcl.Diagnostics) {
	return pcl.DecomposeToken(tok, sourceRange)
}

// Linearize performs a topological sort of the nodes in the program so that they can be processed by tools that need
// to see all of a node's dependencies before the node itself (e.g. a code generator for a programming language that
// requires variables to be defined before they can be referenced). The sort is stable, and nodes are kept in source
// order as much as possible.
func Linearize(p *Program) []Node {
	return pcl.Linearize(p)
}

// Remaps the "pulumi:providers:$Package" token style to "$Package:index:Provider", consistent with code generation.
// This mapping is consistent with how provider resources are projected into the schema and removes special casing logic
// to generate registering explicit providers.
// 
// The resultant program should be a shallow copy of the source with only the modified resource nodes copied.
func MapProvidersAsResources(p *Program) {
	pcl.MapProvidersAsResources(p)
}

func FixupPulumiPackageTokens(r *Resource) {
	pcl.FixupPulumiPackageTokens(r)
}

// SortedFunctionParameters returns a list of properties of the input type from the schema
// for an invoke function call which has multi argument inputs. We assume here
// that the expression is an invoke which has it's args (2nd parameter) annotated
// with the original schema type. The original schema type has properties sorted.
// This is important because model.ObjectType has no guarantee of property order.
func SortedFunctionParameters(expr *model.FunctionCallExpression) []*schema.Property {
	return pcl.SortedFunctionParameters(expr)
}

// GenerateMultiArguments takes the input bag (object) of a function invoke and spreads the values of that object
// into multi-argument function call.
// For example, { a: 1, b: 2 } with multiInputArguments: ["a", "b"] would become: 1, 2
// 
// However, when optional parameters are omitted, then <undefinedLiteral> is used where they should be.
// Take for example { a: 1, c: 3 } with multiInputArguments: ["a", "b", "c"], it becomes 1, <undefinedLiteral>, 3
// because b was omitted and c was provided so b had to be the provided <undefinedLiteral>
func GenerateMultiArguments(f *format.Formatter, w io.Writer, undefinedLiteral string, expr *model.ObjectConsExpression, multiArguments []*schema.Property) {
	pcl.GenerateMultiArguments(f, w, undefinedLiteral, expr, multiArguments)
}

// UnwrapOption returns type T if the input is an Option(T)
func UnwrapOption(exprType model.Type) model.Type {
	return pcl.UnwrapOption(exprType)
}

// VariableAccessed returns whether the given variable name is accessed in the given expression.
func VariableAccessed(variableName string, expr model.Expression) bool {
	return pcl.VariableAccessed(variableName, expr)
}

// LiteralValueString evaluates the given expression and returns the string value if it is a literal value expression
// otherwise returns an empty string for anything else.
func LiteralValueString(x model.Expression) string {
	return pcl.LiteralValueString(x)
}

// inferVariableName infers a variable name from the given traversal expression.
// for example if you have component.firstName.lastName it will become componentFirstNameLastName
func InferVariableName(traversal *model.ScopeTraversalExpression) string {
	return pcl.InferVariableName(traversal)
}

func ExtractDeferredOutputVariables(program *Program, component *Component, expr model.Expression) (model.Expression, []*DeferredOutputVariable) {
	return pcl.ExtractDeferredOutputVariables(program, component, expr)
}

