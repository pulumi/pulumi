// Copyright 2016-2020, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dotnet

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

type nameInfo int

func (nameInfo) Format(name string) string {
	return makeValidIdentifier(name)
}

// lowerExpression amends the expression with intrinsics for C# generation.
func (g *generator) lowerExpression(expr model.Expression, typ model.Type) model.Expression {
	expr = hcl2.RewritePropertyReferences(expr)
	expr, diags := hcl2.RewriteApplies(expr, nameInfo(0), !g.asyncInit)
	contract.Assert(len(diags) == 0)
	expr = hcl2.RewriteConversions(expr, typ)
	if g.asyncInit {
		expr = g.awaitInvokes(expr)
	} else {
		expr = g.outputInvokes(expr)
	}
	return expr
}

// outputInvokes wraps each call to `invoke` with a call to the `output` intrinsic. This rewrite should only be used if
// resources are instantiated within a stack constructor, where `await` operator is not available. We want to avoid the
// nastiness of working with raw `Task` and wrap it into Pulumi's Output immediately to be able to `Apply` on it.
// Note that this depends on the fact that invokes are the only way to introduce promises
// in to a Pulumi program; if this changes in the future, this transform will need to be applied in a more general way
// (e.g. by the apply rewriter).
func (g *generator) outputInvokes(x model.Expression) model.Expression {
	rewriter := func(x model.Expression) (model.Expression, hcl.Diagnostics) {
		// Ignore the node if it is not a call to invoke.
		call, ok := x.(*model.FunctionCallExpression)
		if !ok || call.Name != hcl2.Invoke {
			return x, nil
		}

		_, isOutput := call.Type().(*model.OutputType)
		if isOutput {
			return x, nil
		}

		_, isPromise := call.Type().(*model.PromiseType)
		contract.Assert(isPromise)

		return newOutputCall(call), nil
	}
	x, diags := model.VisitExpression(x, model.IdentityVisitor, rewriter)
	contract.Assert(len(diags) == 0)
	return x
}

// awaitInvokes wraps each call to `invoke` with a call to the `await` intrinsic. This rewrite should only be used
// if we are generating an async Initialize, in which case the apply rewriter should also be configured not to treat
// promises as eventuals. Note that this depends on the fact that invokes are the only way to introduce promises
// in to a Pulumi program; if this changes in the future, this transform will need to be applied in a more general way
// (e.g. by the apply rewriter).
func (g *generator) awaitInvokes(x model.Expression) model.Expression {
	contract.Assert(g.asyncInit)

	rewriter := func(x model.Expression) (model.Expression, hcl.Diagnostics) {
		// Ignore the node if it is not a call to invoke.
		call, ok := x.(*model.FunctionCallExpression)
		if !ok || call.Name != hcl2.Invoke {
			return x, nil
		}

		_, isPromise := call.Type().(*model.PromiseType)
		contract.Assert(isPromise)

		return newAwaitCall(call), nil
	}
	x, diags := model.VisitExpression(x, model.IdentityVisitor, rewriter)
	contract.Assert(len(diags) == 0)
	return x
}

func (g *generator) GetPrecedence(expr model.Expression) int {
	// TODO(msh): Current values copied from Node, update based on
	// https://docs.microsoft.com/en-us/dotnet/csharp/language-reference/operators/
	switch expr := expr.(type) {
	case *model.ConditionalExpression:
		return 4
	case *model.BinaryOpExpression:
		switch expr.Operation {
		case hclsyntax.OpLogicalOr:
			return 5
		case hclsyntax.OpLogicalAnd:
			return 6
		case hclsyntax.OpEqual, hclsyntax.OpNotEqual:
			return 11
		case hclsyntax.OpGreaterThan, hclsyntax.OpGreaterThanOrEqual, hclsyntax.OpLessThan,
			hclsyntax.OpLessThanOrEqual:
			return 12
		case hclsyntax.OpAdd, hclsyntax.OpSubtract:
			return 14
		case hclsyntax.OpMultiply, hclsyntax.OpDivide, hclsyntax.OpModulo:
			return 15
		default:
			contract.Failf("unexpected binary expression %v", expr)
		}
	case *model.UnaryOpExpression:
		return 17
	case *model.FunctionCallExpression:
		switch expr.Name {
		case intrinsicAwait:
			return 17
		default:
			return 20
		}
	case *model.ForExpression, *model.IndexExpression, *model.RelativeTraversalExpression, *model.SplatExpression,
		*model.TemplateJoinExpression:
		return 20
	case *model.AnonymousFunctionExpression, *model.LiteralValueExpression, *model.ObjectConsExpression,
		*model.ScopeTraversalExpression, *model.TemplateExpression, *model.TupleConsExpression:
		return 22
	default:
		contract.Failf("unexpected expression %v of type %T", expr, expr)
	}
	return 0
}

func (g *generator) GenAnonymousFunctionExpression(w io.Writer, expr *model.AnonymousFunctionExpression) {
	switch len(expr.Signature.Parameters) {
	case 0:
		g.Fgen(w, "()")
	case 1:
		g.Fgenf(w, "%s", expr.Signature.Parameters[0].Name)
		g.Fgenf(w, " => %v", expr.Body)
	default:
		g.Fgen(w, "values =>\n")
		g.Fgenf(w, "%s{\n", g.Indent)
		g.Indented(func() {
			for i, p := range expr.Signature.Parameters {
				g.Fgenf(w, "%svar %s = values.Item%d;\n", g.Indent, p.Name, i+1)
			}
			g.Fgenf(w, "%sreturn %v;\n", g.Indent, expr.Body)
		})
		g.Fgenf(w, "%s}", g.Indent)
	}
}

func (g *generator) GenBinaryOpExpression(w io.Writer, expr *model.BinaryOpExpression) {
	opstr, precedence := "", g.GetPrecedence(expr)
	switch expr.Operation {
	case hclsyntax.OpAdd:
		opstr = "+"
	case hclsyntax.OpDivide:
		opstr = "/"
	case hclsyntax.OpEqual:
		opstr = "=="
	case hclsyntax.OpGreaterThan:
		opstr = ">"
	case hclsyntax.OpGreaterThanOrEqual:
		opstr = ">="
	case hclsyntax.OpLessThan:
		opstr = "<"
	case hclsyntax.OpLessThanOrEqual:
		opstr = "<="
	case hclsyntax.OpLogicalAnd:
		opstr = "&&"
	case hclsyntax.OpLogicalOr:
		opstr = "||"
	case hclsyntax.OpModulo:
		opstr = "%"
	case hclsyntax.OpMultiply:
		opstr = "*"
	case hclsyntax.OpNotEqual:
		opstr = "!="
	case hclsyntax.OpSubtract:
		opstr = "-"
	default:
		opstr, precedence = ",", 1
	}

	g.Fgenf(w, "%.[1]*[2]v %[3]v %.[1]*[4]o", precedence, expr.LeftOperand, opstr, expr.RightOperand)
}

func (g *generator) GenConditionalExpression(w io.Writer, expr *model.ConditionalExpression) {
	g.Fgenf(w, "%.4v ? %.4v : %.4v", expr.Condition, expr.TrueResult, expr.FalseResult)
}

func (g *generator) GenForExpression(w io.Writer, expr *model.ForExpression) {
	g.genNYI(w, "ForExpression")
}

func (g *generator) genApply(w io.Writer, expr *model.FunctionCallExpression) {
	// Extract the list of outputs and the continuation expression from the `__apply` arguments.
	applyArgs, then := hcl2.ParseApplyCall(expr)

	if len(applyArgs) == 1 {
		// If we only have a single output, just generate a normal `.Apply`
		g.Fgenf(w, "%.v.Apply(%.v)", applyArgs[0], then)
	} else {
		// Otherwise, generate a call to `Output.Tuple().Apply()`.
		g.Fgen(w, "Output.Tuple(")
		for i, o := range applyArgs {
			if i > 0 {
				g.Fgen(w, ", ")
			}
			g.Fgenf(w, "%.v", o)
		}

		g.Fgenf(w, ").Apply(%.v)", then)
	}
}

func (g *generator) genRange(w io.Writer, call *model.FunctionCallExpression, entries bool) {
	g.genNYI(w, "Range %.v %.v", call, entries)
}

var functionNamespaces = map[string][]string{
	"readDir":  {"System.IO", "System.Linq"},
	"readFile": {"System.IO"},
	"toJSON":   {"System.Text.Json", "System.Collections.Generic"},
}

func (g *generator) genFunctionUsings(x *model.FunctionCallExpression) []string {
	if x.Name != hcl2.Invoke {
		return functionNamespaces[x.Name]
	}

	pkg, _ := g.functionName(x.Args[0])
	return []string{fmt.Sprintf("%s = Pulumi.%[1]s", pkg)}
}

func (g *generator) GenFunctionCallExpression(w io.Writer, expr *model.FunctionCallExpression) {
	switch expr.Name {
	case hcl2.IntrinsicConvert:
		switch arg := expr.Args[0].(type) {
		case *model.ObjectConsExpression:
			g.genObjectConsExpression(w, arg, expr.Type())
		default:
			g.Fgenf(w, "%.v", expr.Args[0]) // <- probably wrong w.r.t. precedence
		}
	case hcl2.IntrinsicApply:
		g.genApply(w, expr)
	case intrinsicAwait:
		g.Fgenf(w, "await %.17v", expr.Args[0])
	case intrinsicOutput:
		g.Fgenf(w, "Output.Create(%.v)", expr.Args[0])
	case "element":
		g.Fgenf(w, "%.20v[%.v]", expr.Args[0], expr.Args[1])
	case "entries":
		switch model.ResolveOutputs(expr.Args[0].Type()).(type) {
		case *model.ListType, *model.TupleType:
			if call, ok := expr.Args[0].(*model.FunctionCallExpression); ok && call.Name == "range" {
				g.genRange(w, call, true)
				return
			}
			g.Fgenf(w, "%.20v.Select((v, k)", expr.Args[0])
		case *model.MapType, *model.ObjectType:
			g.genNYI(w, "MapOrObjectEntries")
		}
		g.Fgenf(w, " => new { Key = k, Value = v })")
	case "fileArchive":
		g.Fgenf(w, "new FileArchive(%.v)", expr.Args[0])
	case "fileAsset":
		g.Fgenf(w, "new FileAsset(%.v)", expr.Args[0])
	case hcl2.Invoke:
		_, name := g.functionName(expr.Args[0])

		optionsBag := ""
		if len(expr.Args) == 3 {
			var buf bytes.Buffer
			g.Fgenf(&buf, ", %.v", expr.Args[2])
			optionsBag = buf.String()
		}

		g.Fgenf(w, "%s.InvokeAsync(%.v%v)", name, expr.Args[1], optionsBag)
	case "length":
		g.Fgenf(w, "%.20v.Length", expr.Args[0])
	case "lookup":
		g.Fgenf(w, "%v[%v]", expr.Args[0], expr.Args[1])
		if len(expr.Args) == 3 {
			g.Fgenf(w, " ?? %v", expr.Args[2])
		}
	case "range":
		g.genRange(w, expr, false)
	case "readFile":
		g.Fgenf(w, "File.ReadAllText(%v)", expr.Args[0])
	case "readDir":
		g.Fgenf(w, "Directory.GetFiles(%.v).Select(Path.GetFileName)", expr.Args[0])
	case "secret":
		g.Fgenf(w, "Output.CreateSecret(%v)", expr.Args[0])
	case "split":
		g.Fgenf(w, "%.20v.Split(%v)", expr.Args[1], expr.Args[0])
	case "toJSON":
		g.Fgen(w, "JsonSerializer.Serialize(")
		g.genDictionary(w, expr.Args[0])
		g.Fgen(w, ")")
	default:
		g.genNYI(w, "call %v", expr.Name)
	}
}

func (g *generator) genDictionary(w io.Writer, expr model.Expression) {
	switch expr := expr.(type) {
	case *model.ObjectConsExpression:
		g.Fgen(w, "new Dictionary<string, object?>\n")
		g.Fgenf(w, "%s{\n", g.Indent)
		g.Indented(func() {
			for _, item := range expr.Items {
				g.Fgenf(w, "%s{ %.v, ", g.Indent, item.Key)
				g.genDictionary(w, item.Value)
				g.Fgen(w, " },\n")
			}
		})
		g.Fgenf(w, "%s}", g.Indent)
	case *model.TupleConsExpression:
		g.Fgen(w, "new[]\n")
		g.Indented(func() {
			g.Fgenf(w, "%[1]s{\n", g.Indent)
			g.Indented(func() {
				for _, v := range expr.Expressions {
					g.Fgenf(w, "%s", g.Indent)
					g.genDictionary(w, v)
					g.Fgen(w, ",\n")
				}
			})
			g.Fgenf(w, "%s}", g.Indent)
		})
		g.Fgenf(w, "\n%s", g.Indent)
	default:
		g.Fgenf(w, "%.v", expr)
	}
}

func (g *generator) GenIndexExpression(w io.Writer, expr *model.IndexExpression) {
	g.Fgenf(w, "%.20v[%.v]", expr.Collection, expr.Key)
}

func (g *generator) escapeString(v string, verbatim, expressions bool) string {
	builder := strings.Builder{}
	for _, c := range v {
		if verbatim {
			if c == '"' {
				builder.WriteRune('"')
			}
		} else {
			if c == '"' || c == '\\' {
				builder.WriteRune('\\')
			}
		}
		if expressions && (c == '{' || c == '}') {
			builder.WriteRune(c)
		}
		builder.WriteRune(c)
	}
	return builder.String()
}

func (g *generator) genStringLiteral(w io.Writer, v string) {
	newlines := strings.Contains(v, "\n")
	if !newlines {
		// This string does not contain newlines so we'll generate a regular string literal. Quotes and backslashes
		// will be escaped in conformance with
		// https://docs.microsoft.com/en-us/dotnet/csharp/language-reference/language-specification/lexical-structure
		g.Fgen(w, "\"")
		g.Fgen(w, g.escapeString(v, false, false))
		g.Fgen(w, "\"")
	} else {
		// This string does contain newlines, so we'll generate a verbatim string literal. Quotes will be escaped
		// in conformance with
		// https://docs.microsoft.com/en-us/dotnet/csharp/language-reference/language-specification/lexical-structure
		g.Fgen(w, "@\"")
		g.Fgen(w, g.escapeString(v, true, false))
		g.Fgen(w, "\"")
	}
}

func (g *generator) GenLiteralValueExpression(w io.Writer, expr *model.LiteralValueExpression) {
	typ := expr.Type()
	if cns, ok := typ.(*model.ConstType); ok {
		typ = cns.Type
	}

	switch typ {
	case model.BoolType:
		g.Fgenf(w, "%v", expr.Value.True())
	case model.NoneType:
		g.Fgen(w, "null")
	case model.NumberType:
		bf := expr.Value.AsBigFloat()
		if i, acc := bf.Int64(); acc == big.Exact {
			g.Fgenf(w, "%d", i)
		} else {
			f, _ := bf.Float64()
			g.Fgenf(w, "%g", f)
		}
	case model.StringType:
		g.genStringLiteral(w, expr.Value.AsString())
	default:
		contract.Failf("unexpected literal type in GenLiteralValueExpression: %v (%v)", expr.Type(),
			expr.SyntaxNode().Range())
	}
}

func (g *generator) GenObjectConsExpression(w io.Writer, expr *model.ObjectConsExpression) {
	g.genObjectConsExpression(w, expr, expr.Type())
}

func (g *generator) genObjectConsExpression(w io.Writer, expr *model.ObjectConsExpression, destType model.Type) {
	if len(expr.Items) == 0 {
		return
	}

	typeName := g.argumentTypeName(expr, destType)
	if typeName != "" {
		g.Fgenf(w, "new %s", typeName)
		g.Fgenf(w, "\n%s{\n", g.Indent)
		g.Indented(func() {
			for _, item := range expr.Items {
				g.Fgenf(w, "%s", g.Indent)
				lit := item.Key.(*model.LiteralValueExpression)
				g.Fprint(w, propertyName(lit.Value.AsString()))
				g.Fgenf(w, " = %.v,\n", item.Value)
			}
		})
		g.Fgenf(w, "%s}", g.Indent)
	} else {
		g.Fgenf(w, "\n%s{\n", g.Indent)
		g.Indented(func() {
			for _, item := range expr.Items {
				g.Fgenf(w, "%s{ %.v, %.v },\n", g.Indent, item.Key, item.Value)
			}
		})
		g.Fgenf(w, "%s}", g.Indent)
	}
}

func (g *generator) genRelativeTraversal(w io.Writer,
	traversal hcl.Traversal, parts []model.Traversable, objType *schema.ObjectType) {

	for i, part := range traversal {
		var key cty.Value
		switch part := part.(type) {
		case hcl.TraverseAttr:
			key = cty.StringVal(part.Name)
			if objType != nil {
				if p, ok := objType.Property(part.Name); ok {
					if info, ok := p.Language["csharp"].(CSharpPropertyInfo); ok && info.Name != "" {
						key = cty.StringVal(info.Name)
					}
				}
			}
		case hcl.TraverseIndex:
			key = part.Key
		default:
			contract.Failf("unexpected traversal part of type %T (%v)", part, part.SourceRange())
		}

		if model.IsOptionalType(model.GetTraversableType(parts[i])) {
			g.Fgen(w, "?")
		}

		switch key.Type() {
		case cty.String:
			g.Fgenf(w, ".%s", propertyName(key.AsString()))
		case cty.Number:
			idx, _ := key.AsBigFloat().Int64()
			g.Fgenf(w, "[%d]", idx)
		default:
			contract.Failf("unexpected traversal key of type %T (%v)", key, key.AsString())
		}
	}
}

func (g *generator) GenRelativeTraversalExpression(w io.Writer, expr *model.RelativeTraversalExpression) {
	g.Fgenf(w, "%.20v", expr.Source)
	g.genRelativeTraversal(w, expr.Traversal, expr.Parts, nil)
}

func (g *generator) GenScopeTraversalExpression(w io.Writer, expr *model.ScopeTraversalExpression) {
	rootName := makeValidIdentifier(expr.RootName)
	if _, ok := expr.Parts[0].(*model.SplatVariable); ok {
		rootName = "__item"
	}

	g.Fgen(w, rootName)

	var objType *schema.ObjectType
	if resource, ok := expr.Parts[0].(*hcl2.Resource); ok {
		if schemaType, ok := hcl2.GetSchemaForType(resource.InputType); ok {
			objType, _ = schemaType.(*schema.ObjectType)
		}
	}
	g.genRelativeTraversal(w, expr.Traversal.SimpleSplit().Rel, expr.Parts, objType)
}

func (g *generator) GenSplatExpression(w io.Writer, expr *model.SplatExpression) {
	g.Fgenf(w, "%.20v.Select(__item => %.v).ToList()", expr.Source, expr.Each)
}

func (g *generator) GenTemplateExpression(w io.Writer, expr *model.TemplateExpression) {
	multiLine := false
	expressions := false
	for _, expr := range expr.Parts {
		if lit, ok := expr.(*model.LiteralValueExpression); ok && model.StringType.AssignableFrom(lit.Type()) {
			if strings.Contains(lit.Value.AsString(), "\n") {
				multiLine = true
			}
		} else {
			expressions = true
		}
	}

	if multiLine {
		g.Fgen(w, "@")
	}
	if expressions {
		g.Fgen(w, "$")
	}
	g.Fgen(w, "\"")
	for _, expr := range expr.Parts {
		if lit, ok := expr.(*model.LiteralValueExpression); ok && model.StringType.AssignableFrom(lit.Type()) {
			g.Fgen(w, g.escapeString(lit.Value.AsString(), multiLine, expressions))
		} else {
			g.Fgenf(w, "{%.v}", expr)
		}
	}
	g.Fgen(w, "\"")
}

func (g *generator) GenTemplateJoinExpression(w io.Writer, expr *model.TemplateJoinExpression) {
	g.genNYI(w, "TemplateJoinExpression")
}

func (g *generator) GenTupleConsExpression(w io.Writer, expr *model.TupleConsExpression) {
	switch len(expr.Expressions) {
	case 0:
		g.Fgen(w, "{}")
	default:
		g.Fgenf(w, "\n%s{", g.Indent)
		g.Indented(func() {
			for _, v := range expr.Expressions {
				g.Fgenf(w, "\n%s%.v,", g.Indent, v)
			}
		})
		g.Fgenf(w, "\n%s}", g.Indent)
	}
}

func (g *generator) GenUnaryOpExpression(w io.Writer, expr *model.UnaryOpExpression) {
	opstr, precedence := "", g.GetPrecedence(expr)
	switch expr.Operation {
	case hclsyntax.OpLogicalNot:
		opstr = "!"
	case hclsyntax.OpNegate:
		opstr = "-"
	}
	g.Fgenf(w, "%[2]v%.[1]*[3]v", precedence, opstr, expr.Operand)
}
