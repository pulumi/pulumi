package nodejs

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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

type nameInfo int

func (nameInfo) Format(name string) string {
	return makeValidIdentifier(name)
}

func (g *generator) lowerExpression(expr model.Expression) model.Expression {
	// TODO(pdg): diagnostics
	if g.asyncMain {
		expr = g.awaitInvokes(expr)
	}
	expr = hcl2.RewritePropertyReferences(expr)
	expr, _ = hcl2.RewriteApplies(expr, nameInfo(0), !g.asyncMain)
	expr, _ = g.lowerProxyApplies(expr)
	return expr
}

func (g *generator) GetPrecedence(expr model.Expression) int {
	// Precedence is derived from
	// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Operator_Precedence.
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
		case intrinsicInterpolate:
			return 22
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
	default:
		g.Fgen(w, "([")
		for i, p := range expr.Signature.Parameters {
			if i > 0 {
				g.Fgen(w, ", ")
			}
			g.Fgenf(w, "%s", p.Name)
		}
		g.Fgen(w, "])")
	}

	g.Fgenf(w, " => %.v", expr.Body)
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
	switch expr.Collection.Type().(type) {
	case *model.ListType, *model.TupleType:
		if expr.KeyVariable == nil {
			g.Fgenf(w, "%.20v", expr.Collection)
		} else {
			g.Fgenf(w, "%.20v.map((v, k) => [k, v])", expr.Collection)
		}
	case *model.MapType, *model.ObjectType:
		if expr.KeyVariable == nil {
			g.Fgenf(w, "Object.values(%.v)", expr.Collection)
		} else {
			g.Fgenf(w, "Object.entries(%.v)", expr.Collection)
		}
	}

	fnParams, reduceParams := expr.ValueVariable.Name, expr.ValueVariable.Name
	if expr.KeyVariable != nil {
		reduceParams = fmt.Sprintf("[%.v, %.v]", expr.KeyVariable.Name, expr.ValueVariable.Name)
		fnParams = fmt.Sprintf("(%v)", reduceParams)
	}

	if expr.Condition != nil {
		g.Fgenf(w, ".filter(%s => %.v)", fnParams, expr.Condition)
	}

	if expr.Key != nil {
		// TODO(pdg): grouping
		g.Fgenf(w, ".reduce((__obj, %s) => { ...__obj, [%.v]: %.v })", reduceParams, expr.Key, expr.Value)
	} else {
		g.Fgenf(w, ".map(%s => %.v)", fnParams, expr.Value)
	}
}

func (g *generator) genApply(w io.Writer, expr *model.FunctionCallExpression) {
	// Extract the list of outputs and the continuation expression from the `__apply` arguments.
	applyArgs, then := hcl2.ParseApplyCall(expr)

	// If all of the arguments are promises, use promise methods. If any argument is an output, convert all other args
	// to outputs and use output methods.
	anyOutputs := false
	for _, arg := range applyArgs {
		if isOutputType(arg.Type()) {
			anyOutputs = true
		}
	}

	apply, all := "then", "Promise.all"
	if anyOutputs {
		apply, all = "apply", "pulumi.all"
	}

	if len(applyArgs) == 1 {
		// If we only have a single output, just generate a normal `.apply` or `.then`.
		g.Fgenf(w, "%.20v.%v(%.v)", applyArgs[0], apply, then)
	} else {
		// Otherwise, generate a call to `pulumi.all([]).apply()`.
		g.Fgenf(w, "%v([", all)
		for i, o := range applyArgs {
			if i > 0 {
				g.Fgen(w, ", ")
			}
			g.Fgenf(w, "%v", o)
		}
		g.Fgenf(w, "]).%v(%.v)", apply, then)
	}
}

// functionName computes the NodeJS package, module, and name for the given function token.
func functionName(tokenArg model.Expression) (string, string, string, hcl.Diagnostics) {
	token := tokenArg.(*model.TemplateExpression).Parts[0].(*model.LiteralValueExpression).Value.AsString()
	tokenRange := tokenArg.SyntaxNode().Range()

	// Compute the resource type from the Pulumi type token.
	pkg, module, member, diagnostics := hcl2.DecomposeToken(token, tokenRange)
	return pkg, strings.Replace(module, "/", ".", -1), member, diagnostics
}

func (g *generator) genRange(w io.Writer, call *model.FunctionCallExpression, entries bool) {
	var from, to model.Expression
	switch len(call.Args) {
	case 1:
		from, to = &model.LiteralValueExpression{Value: cty.NumberIntVal(0)}, call.Args[0]
	case 2:
		from, to = call.Args[0], call.Args[1]
	default:
		contract.Failf("expected range() to have exactly 1 or 2 args; got %v", len(call.Args))
	}

	genPrefix := func() { g.Fprint(w, "((from, to) => (new Array(to - from))") }
	mapValue := "from + i"
	genSuffix := func() { g.Fgenf(w, ")(%.v, %.v)", from, to) }

	if litFrom, ok := from.(*model.LiteralValueExpression); ok {
		fromV, err := convert.Convert(litFrom.Value, cty.Number)
		contract.Assert(err == nil)

		from, _ := fromV.AsBigFloat().Int64()
		if litTo, ok := to.(*model.LiteralValueExpression); ok {
			toV, err := convert.Convert(litTo.Value, cty.Number)
			contract.Assert(err == nil)

			to, _ := toV.AsBigFloat().Int64()
			if from == 0 {
				mapValue = "i"
			} else {
				mapValue = fmt.Sprintf("%d + i", from)
			}
			genPrefix = func() { g.Fprintf(w, "(new Array(%d))", to-from) }
			genSuffix = func() {}
		} else if from == 0 {
			genPrefix = func() { g.Fgenf(w, "(new Array(%.v))", to) }
			mapValue = "i"
			genSuffix = func() {}
		}
	}

	if entries {
		mapValue = fmt.Sprintf("{key: %[1]s, value: %[1]s}", mapValue)
	}

	genPrefix()
	g.Fprintf(w, ".map((_, i) => %v)", mapValue)
	genSuffix()
}

var functionImports = map[string]string{
	intrinsicInterpolate: "@pulumi/pulumi",
	"fileArchive":        "@pulumi/pulumi",
	"fileAsset":          "@pulumi/pulumi",
	"readFile":           "fs",
	"readDir":            "fs",
}

func (g *generator) getFunctionImports(x *model.FunctionCallExpression) string {
	if x.Name != hcl2.Invoke {
		return functionImports[x.Name]
	}

	pkg, _, _, diags := functionName(x.Args[0])
	contract.Assert(len(diags) == 0)
	return "@pulumi/" + pkg
}

func (g *generator) GenFunctionCallExpression(w io.Writer, expr *model.FunctionCallExpression) {
	switch expr.Name {
	case hcl2.IntrinsicApply:
		g.genApply(w, expr)
	case intrinsicAwait:
		g.Fgenf(w, "await %.17v", expr.Args[0])
	case intrinsicInterpolate:
		g.Fgen(w, "pulumi.interpolate`")
		for _, part := range expr.Args {
			if lit, ok := part.(*model.LiteralValueExpression); ok && model.StringType.AssignableFrom(lit.Type()) {
				g.Fgen(w, lit.Value.AsString())
			} else {
				g.Fgenf(w, "${%.v}", part)
			}
		}
		g.Fgen(w, "`")
	case "element":
		g.Fgenf(w, "%.20v[%.v]", expr.Args[0], expr.Args[1])
	case "entries":
		switch model.ResolveOutputs(expr.Args[0].Type()).(type) {
		case *model.ListType, *model.TupleType:
			if call, ok := expr.Args[0].(*model.FunctionCallExpression); ok && call.Name == "range" {
				g.genRange(w, call, true)
				return
			}
			g.Fgenf(w, "%.20v.map((k, v)", expr.Args[0])
		case *model.MapType, *model.ObjectType:
			g.Fgenf(w, "Object.entries(%.v).map(([k, v])", expr.Args[0])
		}
		g.Fgenf(w, " => {key: k, value: v})")
	case "fileArchive":
		g.Fgenf(w, "new pulumi.asset.FileArchive(%.v)", expr.Args[0])
	case "fileAsset":
		g.Fgenf(w, "new pulumi.asset.FileAsset(%.v)", expr.Args[0])
	case hcl2.Invoke:
		pkg, module, fn, diags := functionName(expr.Args[0])
		contract.Assert(len(diags) == 0)
		if module != "" {
			module = "." + module
		}
		name := fmt.Sprintf("%s%s.%s", makeValidIdentifier(pkg), module, fn)

		optionsBag := ""
		if len(expr.Args) == 3 {
			var buf bytes.Buffer
			g.Fgenf(&buf, ", %.v", expr.Args[2])
			optionsBag = buf.String()
		}

		g.Fgenf(w, "%s(%.v%v)", name, expr.Args[1], optionsBag)
	case "length":
		g.Fgenf(w, "%.20v.length", expr.Args[0])
	case "lookup":
		g.Fgenf(w, "%v[%v]", expr.Args[0], expr.Args[1])
		if len(expr.Args) == 3 {
			g.Fgenf(w, " || %v", expr.Args[2])
		}
	case "range":
		g.genRange(w, expr, false)
	case "readFile":
		g.Fgenf(w, "fs.readFileSync(%v)", expr.Args[0])
	case "readDir":
		g.Fgenf(w, "fs.readDirSync(%v)", expr.Args[0])
	case "secret":
		g.Fgenf(w, "pulumi.secret(%v)", expr.Args[0])
	case "split":
		g.Fgenf(w, "%.20v.split(%v)", expr.Args[1], expr.Args[0])
	case "toJSON":
		g.Fgenf(w, "JSON.stringify(%v)", expr.Args[0])
	default:
		var rng hcl.Range
		if expr.Syntax != nil {
			rng = expr.Syntax.Range()
		}
		g.genNYI(w, "FunctionCallExpression: %v (%v)", expr.Name, rng)
	}
}

func (g *generator) GenIndexExpression(w io.Writer, expr *model.IndexExpression) {
	g.Fgenf(w, "%.20v[%.v]", expr.Collection, expr.Key)
}

func (g *generator) genStringLiteral(w io.Writer, v string) {
	builder := strings.Builder{}
	newlines := strings.Count(v, "\n")
	if newlines == 0 || newlines == 1 && (v[0] == '\n' || v[len(v)-1] == '\n') {
		// This string either does not contain newlines or contains a single leading or trailing newline, so we'll
		// Generate a normal string literal. Quotes, backslashes, and newlines will be escaped in conformance with
		// ECMA-262 11.8.4 ("String Literals").
		builder.WriteRune('"')
		for _, c := range v {
			if c == '\n' {
				builder.WriteString(`\n`)
			} else {
				if c == '"' || c == '\\' {
					builder.WriteRune('\\')
				}
				builder.WriteRune(c)
			}
		}
		builder.WriteRune('"')
	} else {
		// This string does contain newlines, so we'll Generate a template string literal. "${", backquotes, and
		// backslashes will be escaped in conformance with ECMA-262 11.8.6 ("Template Literal Lexical Components").
		runes := []rune(v)
		builder.WriteRune('`')
		for i, c := range runes {
			switch c {
			case '$':
				if i < len(runes)-1 && runes[i+1] == '{' {
					builder.WriteRune('\\')
				}
			case '`', '\\':
				builder.WriteRune('\\')
			}
			builder.WriteRune(c)
		}
		builder.WriteRune('`')
	}

	g.Fgenf(w, "%s", builder.String())
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
		g.Fgen(w, "undefined")
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

func (g *generator) literalKey(x model.Expression) (string, bool) {
	strKey := ""
	switch x := x.(type) {
	case *model.LiteralValueExpression:
		if model.StringType.AssignableFrom(x.Type()) {
			strKey = x.Value.AsString()
			break
		}
		var buf bytes.Buffer
		g.GenLiteralValueExpression(&buf, x)
		return buf.String(), true
	case *model.TemplateExpression:
		if len(x.Parts) == 1 {
			if lit, ok := x.Parts[0].(*model.LiteralValueExpression); ok && model.StringType.AssignableFrom(lit.Type()) {
				strKey = lit.Value.AsString()
				break
			}
		}
		var buf bytes.Buffer
		g.GenTemplateExpression(&buf, x)
		return buf.String(), true
	default:
		return "", false
	}

	if isLegalIdentifier(strKey) {
		return strKey, true
	}
	return fmt.Sprintf("%q", strKey), true

}

func (g *generator) GenObjectConsExpression(w io.Writer, expr *model.ObjectConsExpression) {
	if len(expr.Items) == 0 {
		g.Fgen(w, "{}")
	} else {
		g.Fgen(w, "{")
		g.Indented(func() {
			for _, item := range expr.Items {
				g.Fgenf(w, "\n%s", g.Indent)
				if lit, ok := g.literalKey(item.Key); ok {
					g.Fgenf(w, "%s", lit)
				} else {
					g.Fgenf(w, "[%.v]", item.Key)
				}
				g.Fgenf(w, ": %.v,", item.Value)
			}
		})
		g.Fgenf(w, "\n%s}", g.Indent)
	}
}

func (g *generator) genRelativeTraversal(w io.Writer, traversal hcl.Traversal, parts []model.Traversable) {
	for i, part := range traversal {
		var key cty.Value
		switch part := part.(type) {
		case hcl.TraverseAttr:
			key = cty.StringVal(part.Name)
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
			keyVal := key.AsString()
			if isLegalIdentifier(keyVal) {
				g.Fgenf(w, ".%s", keyVal)
			} else {
				g.Fgenf(w, "[%q]", keyVal)
			}
		case cty.Number:
			idx, _ := key.AsBigFloat().Int64()
			g.Fgenf(w, "[%d]", idx)
		default:
			g.Fgenf(w, "[%q]", key.AsString())
		}
	}
}

func (g *generator) GenRelativeTraversalExpression(w io.Writer, expr *model.RelativeTraversalExpression) {
	g.Fgenf(w, "%.20v", expr.Source)
	g.genRelativeTraversal(w, expr.Traversal, expr.Parts)
}

func (g *generator) GenScopeTraversalExpression(w io.Writer, expr *model.ScopeTraversalExpression) {
	rootName := makeValidIdentifier(expr.RootName)
	if _, ok := expr.Parts[0].(*model.SplatVariable); ok {
		rootName = "__item"
	}

	g.Fgen(w, rootName)
	g.genRelativeTraversal(w, expr.Traversal.SimpleSplit().Rel, expr.Parts)
}

func (g *generator) GenSplatExpression(w io.Writer, expr *model.SplatExpression) {
	g.Fgenf(w, "%.20v.map(__item => %.v)", expr.Source, expr.Each)
}

func (g *generator) GenTemplateExpression(w io.Writer, expr *model.TemplateExpression) {
	if len(expr.Parts) == 1 {
		if lit, ok := expr.Parts[0].(*model.LiteralValueExpression); ok && model.StringType.AssignableFrom(lit.Type()) {
			g.GenLiteralValueExpression(w, lit)
			return
		}
	}

	g.Fgen(w, "`")
	for _, expr := range expr.Parts {
		if lit, ok := expr.(*model.LiteralValueExpression); ok && model.StringType.AssignableFrom(lit.Type()) {
			g.Fgen(w, lit.Value.AsString())
		} else {
			g.Fgenf(w, "${%.v}", expr)
		}
	}
	g.Fgen(w, "`")
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
		opstr = "!"
	case hclsyntax.OpNegate:
		opstr = "-"
	}
	g.Fgenf(w, "%[2]v%.[1]*[3]v", precedence, opstr, expr.Operand)
}
