//nolint: goconst
package python

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/big"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

type nameInfo int

func (nameInfo) Format(name string) string {
	return PyName(name)
}

func (g *generator) lowerExpression(expr model.Expression, typ model.Type) (model.Expression, []*quoteTemp) {
	// TODO(pdg): diagnostics

	expr = hcl2.RewritePropertyReferences(expr)
	expr, _ = hcl2.RewriteApplies(expr, nameInfo(0), false)
	expr, _ = g.lowerProxyApplies(expr)
	expr = hcl2.RewriteConversions(expr, typ)
	expr, quotes, _ := g.rewriteQuotes(expr)

	return expr, quotes
}

func (g *generator) GetPrecedence(expr model.Expression) int {
	// Precedence is taken from https://docs.python.org/3/reference/expressions.html#operator-precedence.
	switch expr := expr.(type) {
	case *model.AnonymousFunctionExpression:
		return 1
	case *model.ConditionalExpression:
		return 2
	case *model.BinaryOpExpression:
		switch expr.Operation {
		case hclsyntax.OpLogicalOr:
			return 3
		case hclsyntax.OpLogicalAnd:
			return 4
		case hclsyntax.OpGreaterThan, hclsyntax.OpGreaterThanOrEqual, hclsyntax.OpLessThan, hclsyntax.OpLessThanOrEqual,
			hclsyntax.OpEqual, hclsyntax.OpNotEqual:
			return 6
		case hclsyntax.OpAdd, hclsyntax.OpSubtract:
			return 11
		case hclsyntax.OpMultiply, hclsyntax.OpDivide, hclsyntax.OpModulo:
			return 12
		default:
			contract.Failf("unexpected binary expression %v", expr)
		}
	case *model.UnaryOpExpression:
		return 13
	case *model.FunctionCallExpression, *model.IndexExpression, *model.RelativeTraversalExpression,
		*model.TemplateJoinExpression:
		return 16
	case *model.ForExpression, *model.ObjectConsExpression, *model.SplatExpression, *model.TupleConsExpression:
		return 17
	case *model.LiteralValueExpression, *model.ScopeTraversalExpression, *model.TemplateExpression:
		return 18
	default:
		contract.Failf("unexpected expression %v of type %T", expr, expr)
	}
	return 0
}

func (g *generator) GenAnonymousFunctionExpression(w io.Writer, expr *model.AnonymousFunctionExpression) {
	g.Fgen(w, "lambda")
	for i, p := range expr.Signature.Parameters {
		if i > 0 {
			g.Fgen(w, ",")
		}
		g.Fgenf(w, " %s", p.Name)
	}

	g.Fgenf(w, ": %.v", expr.Body)
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
		opstr = "and"
	case hclsyntax.OpLogicalOr:
		opstr = "or"
	case hclsyntax.OpModulo:
		opstr = "%"
	case hclsyntax.OpMultiply:
		opstr = "*"
	case hclsyntax.OpNotEqual:
		opstr = "!="
	case hclsyntax.OpSubtract:
		opstr = "-"
	default:
		opstr, precedence = ",", 0
	}

	g.Fgenf(w, "%.[1]*[2]v %[3]v %.[1]*[4]o", precedence, expr.LeftOperand, opstr, expr.RightOperand)
}

func (g *generator) GenConditionalExpression(w io.Writer, expr *model.ConditionalExpression) {
	g.Fgenf(w, "%.2v if %.2v else %.2v", expr.TrueResult, expr.Condition, expr.FalseResult)
}

func (g *generator) GenForExpression(w io.Writer, expr *model.ForExpression) {
	close := "]"
	if expr.Key != nil {
		// Dictionary comprehension
		//
		// TODO(pdg): grouping
		g.Fgenf(w, "{%.v: %.v", expr.Key, expr.Value)
		close = "}"
	} else {
		// List comprehension
		g.Fgenf(w, "[%.v", expr.Value)
	}

	if expr.KeyVariable == nil {
		g.Fgenf(w, " for %v in %.v", expr.ValueVariable.Name, expr.Collection)
	} else {
		g.Fgenf(w, " for %v, %v in %.v", expr.KeyVariable.Name, expr.ValueVariable.Name, expr.Collection)
	}

	if expr.Condition != nil {
		g.Fgenf(w, " if %.v", expr.Condition)
	}

	g.Fprint(w, close)
}

func (g *generator) genApply(w io.Writer, expr *model.FunctionCallExpression) {
	// Extract the list of outputs and the continuation expression from the `__apply` arguments.
	applyArgs, then := hcl2.ParseApplyCall(expr)

	if len(applyArgs) == 1 {
		// If we only have a single output, just generate a normal `.apply`.
		g.Fgenf(w, "%.16v.apply(%.v)", applyArgs[0], then)
	} else {
		// Otherwise, generate a call to `pulumi.all([]).apply()`.
		g.Fgen(w, "pulumi.Output.all(")
		for i, o := range applyArgs {
			if i > 0 {
				g.Fgen(w, ", ")
			}
			g.Fgenf(w, "%.v", o)
		}
		g.Fgenf(w, ").apply(%.v)", then)
	}
}

// functionName computes the Python package, module, and name for the given function token.
func functionName(tokenArg model.Expression) (string, string, string, hcl.Diagnostics) {
	token := tokenArg.(*model.TemplateExpression).Parts[0].(*model.LiteralValueExpression).Value.AsString()
	tokenRange := tokenArg.SyntaxNode().Range()

	// Compute the resource type from the Pulumi type token.
	pkg, module, member, diagnostics := hcl2.DecomposeToken(token, tokenRange)
	return makeValidIdentifier(pkg), strings.Replace(module, "/", ".", -1), title(member), diagnostics
}

var functionImports = map[string]string{
	"fileArchive": "pulumi",
	"fileAsset":   "pulumi",
	"readDir":     "os",
	"toJSON":      "json",
}

func (g *generator) getFunctionImports(x *model.FunctionCallExpression) string {
	if x.Name != hcl2.Invoke {
		return functionImports[x.Name]
	}

	pkg, _, _, diags := functionName(x.Args[0])
	contract.Assert(len(diags) == 0)
	return "pulumi_" + pkg
}

func (g *generator) GenFunctionCallExpression(w io.Writer, expr *model.FunctionCallExpression) {
	switch expr.Name {
	case hcl2.IntrinsicConvert:
		switch arg := expr.Args[0].(type) {
		case *model.ObjectConsExpression:
			g.genObjectConsExpression(w, arg, expr.Type())
		default:
			g.Fgenf(w, "%.v", expr.Args[0])
		}
	case hcl2.IntrinsicApply:
		g.genApply(w, expr)
	case "element":
		g.Fgenf(w, "%.16v[%.v]", expr.Args[0], expr.Args[1])
	case "entries":
		g.Fgenf(w, `[{"key": k, "value": v} for k, v in %.v]`, expr.Args[0])
	case "fileArchive":
		g.Fgenf(w, "pulumi.FileArchive(%.v)", expr.Args[0])
	case "fileAsset":
		g.Fgenf(w, "pulumi.FileAsset(%.v)", expr.Args[0])
	case hcl2.Invoke:
		pkg, module, fn, diags := functionName(expr.Args[0])
		contract.Assert(len(diags) == 0)
		if module != "" {
			module = "." + module
		}
		name := fmt.Sprintf("%s%s.%s", pkg, module, PyName(fn))

		optionsBag := ""
		if len(expr.Args) == 3 {
			var buf bytes.Buffer
			g.Fgenf(&buf, ", %.v", expr.Args[2])
			optionsBag = buf.String()
		}

		g.Fgenf(w, "%s(", name)

		casingTable := g.casingTables[pkg]
		if obj, ok := expr.Args[1].(*model.FunctionCallExpression); ok {
			if obj, ok := obj.Args[0].(*model.ObjectConsExpression); ok {
				g.lowerObjectKeys(expr.Args[1], casingTable)

				indenter := func(f func()) { f() }
				if len(obj.Items) > 1 {
					indenter = g.Indented
				}
				indenter(func() {
					for i, item := range obj.Items {
						// Ignore non-literal keys
						key, ok := item.Key.(*model.LiteralValueExpression)
						if !ok || !key.Value.Type().Equals(cty.String) {
							continue
						}

						keyVal := PyName(key.Value.AsString())
						if i == 0 {
							g.Fgenf(w, "%s=%.v", keyVal, item.Value)
						} else {
							g.Fgenf(w, ",\n%s%s=%.v", g.Indent, keyVal, item.Value)
						}
					}
				})
			}
		}

		g.Fgenf(w, "%v)", optionsBag)
	case "length":
		g.Fgenf(w, "len(%.v)", expr.Args[0])
	case "lookup":
		if len(expr.Args) == 3 {
			g.Fgenf(w, "(lambda v, def: v if v is not None else def)(%.16v[%.v], %.v)",
				expr.Args[0], expr.Args[1], expr.Args[2])
		} else {
			g.Fgenf(w, "%.16v[%.v]", expr.Args[0], expr.Args[1])
		}
	case "range":
		g.Fprint(w, "range(")
		for i, arg := range expr.Args {
			if i > 0 {
				g.Fprint(w, ", ")
			}
			g.Fgenf(w, "%.v", arg)
		}
		g.Fprint(w, ")")
	case "readFile":
		g.Fgenf(w, "(lambda path: open(path).read())(%.v)", expr.Args[0])
	case "readDir":
		g.Fgenf(w, "os.listdir(%.v)", expr.Args[0])
	case "secret":
		g.Fgenf(w, "pulumi.secret(%v)", expr.Args[0])
	case "split":
		g.Fgenf(w, "%.16v.split(%.v)", expr.Args[1], expr.Args[0])
	case "toJSON":
		g.Fgenf(w, "json.dumps(%.v)", expr.Args[0])
	default:
		var rng hcl.Range
		if expr.Syntax != nil {
			rng = expr.Syntax.Range()
		}
		g.genNYI(w, "FunctionCallExpression: %v (%v)", expr.Name, rng)
	}
}

func (g *generator) GenIndexExpression(w io.Writer, expr *model.IndexExpression) {
	g.Fgenf(w, "%.16v[%.v]", expr.Collection, expr.Key)
}

type runeWriter interface {
	WriteRune(c rune) (int, error)
}

// nolint: errcheck
func (g *generator) genEscapedString(w runeWriter, v string, escapeNewlines, escapeBraces bool) {
	for _, c := range v {
		switch c {
		case '\n':
			if escapeNewlines {
				w.WriteRune('\\')
				c = 'n'
			}
		case '"', '\\':
			if escapeNewlines {
				w.WriteRune('\\')
			}
		case '{', '}':
			if escapeBraces {
				w.WriteRune(c)
			}
		}
		w.WriteRune(c)
	}
}

func (g *generator) genStringLiteral(w io.Writer, quotes, v string) {
	builder := &strings.Builder{}

	builder.WriteString(quotes)
	escapeNewlines := quotes == `"` || quotes == `'`
	g.genEscapedString(builder, v, escapeNewlines, false)
	builder.WriteString(quotes)

	g.Fgenf(w, "%s", builder.String())
}

func (g *generator) GenLiteralValueExpression(w io.Writer, expr *model.LiteralValueExpression) {
	typ := expr.Type()
	if cns, ok := typ.(*model.ConstType); ok {
		typ = cns.Type
	}

	switch typ {
	case model.BoolType:
		if expr.Value.True() {
			g.Fgen(w, "True")
		} else {
			g.Fgen(w, "False")
		}
	case model.NoneType:
		g.Fgen(w, "None")
	case model.NumberType:
		bf := expr.Value.AsBigFloat()
		if i, acc := bf.Int64(); acc == big.Exact {
			g.Fgenf(w, "%d", i)
		} else {
			f, _ := bf.Float64()
			g.Fgenf(w, "%g", f)
		}
	case model.StringType:
		quotes := g.quotes[expr]
		g.genStringLiteral(w, quotes, expr.Value.AsString())
	default:
		contract.Failf("unexpected literal type in GenLiteralValueExpression: %v (%v)", expr.Type(),
			expr.SyntaxNode().Range())
	}
}

func (g *generator) GenObjectConsExpression(w io.Writer, expr *model.ObjectConsExpression) {
	g.genObjectConsExpression(w, expr, expr.Type())
}

func (g *generator) genObjectConsExpression(w io.Writer, expr *model.ObjectConsExpression, destType model.Type) {
	typeName := g.argumentTypeName(expr, destType) // Example: aws.s3.BucketLoggingArgs
	if typeName != "" {
		// If a typeName exists, treat this as an Input Class e.g. aws.s3.BucketLoggingArgs(key="value", foo="bar", ...)
		if len(expr.Items) == 0 {
			g.Fgenf(w, "%s()", typeName)
		} else {
			g.Fgenf(w, "%s(\n", typeName)
			g.Indented(func() {
				for _, item := range expr.Items {
					g.Fgenf(w, "%s", g.Indent)
					lit := item.Key.(*model.LiteralValueExpression)
					g.Fprint(w, PyName(lit.Value.AsString()))
					g.Fgenf(w, "=%.v,\n", item.Value)
				}
			})
			g.Fgenf(w, "%s)", g.Indent)
		}
	} else {
		// Otherwise treat this as an untyped dictionary e.g. {"key": "value", "foo": "bar", ...}
		if len(expr.Items) == 0 {
			g.Fgen(w, "{}")
		} else {
			g.Fgen(w, "{")
			g.Indented(func() {
				for _, item := range expr.Items {
					g.Fgenf(w, "\n%s%.v: %.v,", g.Indent, item.Key, item.Value)
				}
			})
			g.Fgenf(w, "\n%s}", g.Indent)
		}
	}
}

func (g *generator) genRelativeTraversal(w io.Writer, traversal hcl.Traversal, parts []model.Traversable) {
	for _, traverser := range traversal {
		var key cty.Value
		switch traverser := traverser.(type) {
		case hcl.TraverseAttr:
			key = cty.StringVal(traverser.Name)
		case hcl.TraverseIndex:
			key = traverser.Key
		default:
			contract.Failf("unexpected traverser of type %T (%v)", traverser, traverser.SourceRange())
		}

		switch key.Type() {
		case cty.String:
			keyVal := key.AsString()
			contract.Assert(isLegalIdentifier(keyVal))
			g.Fgenf(w, ".%s", keyVal)
		case cty.Number:
			idx, _ := key.AsBigFloat().Int64()
			g.Fgenf(w, "[%d]", idx)
		default:
			keyExpr := &model.LiteralValueExpression{Value: key}
			diags := keyExpr.Typecheck(false)
			contract.Ignore(diags)

			g.Fgenf(w, "[%v]", keyExpr)
		}
	}
}

func (g *generator) GenRelativeTraversalExpression(w io.Writer, expr *model.RelativeTraversalExpression) {
	g.Fgenf(w, "%.16v", expr.Source)
	g.genRelativeTraversal(w, expr.Traversal, expr.Parts)
}

func (g *generator) GenScopeTraversalExpression(w io.Writer, expr *model.ScopeTraversalExpression) {
	rootName := PyName(expr.RootName)
	if _, ok := expr.Parts[0].(*model.SplatVariable); ok {
		rootName = "__item"
	}

	g.Fgen(w, rootName)
	g.genRelativeTraversal(w, expr.Traversal.SimpleSplit().Rel, expr.Parts)
}

func (g *generator) GenSplatExpression(w io.Writer, expr *model.SplatExpression) {
	g.Fgenf(w, "[%.v for __item in %.v]", expr.Each, expr.Source)
}

func (g *generator) GenTemplateExpression(w io.Writer, expr *model.TemplateExpression) {
	quotes := g.quotes[expr]
	escapeNewlines := quotes == `"` || quotes == `'`

	prefix, escapeBraces := "", false
	for _, part := range expr.Parts {
		if lit, ok := part.(*model.LiteralValueExpression); !ok || !model.StringType.AssignableFrom(lit.Type()) {
			prefix, escapeBraces = "f", true
			break
		}
	}

	b := bufio.NewWriter(w)
	defer b.Flush()

	g.Fprintf(b, "%s%s", prefix, quotes)
	for _, expr := range expr.Parts {
		if lit, ok := expr.(*model.LiteralValueExpression); ok && model.StringType.AssignableFrom(lit.Type()) {
			g.genEscapedString(b, lit.Value.AsString(), escapeNewlines, escapeBraces)
		} else {
			g.Fgenf(b, "{%.v}", expr)
		}
	}
	g.Fprint(b, quotes)
}

func (g *generator) GenTemplateJoinExpression(w io.Writer, expr *model.TemplateJoinExpression) {
	g.genNYI(w, "TemplateJoinExpression")
}

func (g *generator) GenTupleConsExpression(w io.Writer, expr *model.TupleConsExpression) {
	switch len(expr.Expressions) {
	case 0:
		g.Fgen(w, "[]")
	case 1:
		g.Fgenf(w, "[%.v]", expr.Expressions[0])
	default:
		g.Fgen(w, "[")
		g.Indented(func() {
			for _, v := range expr.Expressions {
				g.Fgenf(w, "\n%s%.v,", g.Indent, v)
			}
		})
		g.Fgen(w, "\n", g.Indent, "]")
	}
}

func (g *generator) GenUnaryOpExpression(w io.Writer, expr *model.UnaryOpExpression) {
	opstr, precedence := "", g.GetPrecedence(expr)
	switch expr.Operation {
	case hclsyntax.OpLogicalNot:
		opstr = "not "
	case hclsyntax.OpNegate:
		opstr = "-"
	}
	g.Fgenf(w, "%[2]v%.[1]*[3]v", precedence, opstr, expr.Operand)
}
