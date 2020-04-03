package nodejs

import (
	"bytes"
	"io"
	"math/big"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/codegen/hcl2/model"
	"github.com/pulumi/pulumi/sdk/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

type nameInfo int

func (nameInfo) IsReservedWord(word string) bool {
	return isReservedWord(word)
}

func (g *generator) genExpression(expr model.Expression) string {
	// TODO(pdg): diagnostics

	expr, _ = hcl2.RewriteApplies(expr, nameInfo(0))
	expr, _ = g.lowerProxyApplies(expr)

	var buf bytes.Buffer
	g.Fgen(&buf, expr)
	return buf.String()
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

	g.Fgenf(w, " => %v", expr.Body)
}

func (g *generator) GenBinaryOpExpression(w io.Writer, expr *model.BinaryOpExpression) {
	opstr := ","
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
	}

	g.Fgenf(w, "%v %v %v", expr.LeftOperand, opstr, expr.RightOperand)
}

func (g *generator) GenConditionalExpression(w io.Writer, expr *model.ConditionalExpression) {
	g.Fgenf(w, "%v ? %v : %v", expr.Condition, expr.TrueResult, expr.FalseResult)
}

func (g *generator) GenForExpression(w io.Writer, expr *model.ForExpression) {
	switch expr.Collection.Type().(type) {
	case *model.ListType, *model.TupleType:
		g.Fgenf(w, "Array.from(%v.entries())", expr.Collection)
	case *model.MapType, *model.ObjectType:
		g.Fgenf(w, "Object.entries(%v)", expr.Collection)
	}

	keyVariableName := "_"
	if expr.KeyVariable != nil {
		keyVariableName = expr.KeyVariable.Name
	}

	if expr.Condition != nil {
		g.Fgenf(w, ".filter(([%v, %v]) => %v)", keyVariableName, expr.ValueVariable.Name, expr.Condition)
	}

	if expr.Key != nil {
		// TODO(pdg): grouping
		g.Fgenf(w, ".reduce((__obj, [%v, %v]) => { ...__obj, [%v]: %v })", keyVariableName, expr.ValueVariable.Name,
			expr.Key, expr.Value)
	} else {
		g.Fgenf(w, ".map(([%v, %v]) => %v)", keyVariableName, expr.ValueVariable.Name, expr.Value)
	}
}

func (g *generator) genApply(w io.Writer, expr *model.FunctionCallExpression) {
	//	g.inApplyCall = true
	//	defer func() { g.inApplyCall = false }()

	// Extract the list of outputs and the continuation expression from the `__apply` arguments.
	applyArgs, then := hcl2.ParseApplyCall(expr)
	//g.applyArgs, g.applyArgNames = applyArgs, g.assignApplyArgNames(applyArgs, then)
	//defer func() { g.applyArgs = nil }()

	if len(applyArgs) == 1 {
		// If we only have a single output, just generate a normal `.apply`.
		//g.genApplyOutput(w, g.applyArgs[0])
		g.Fgenf(w, "%v.apply(%v)", applyArgs[0], then)
	} else {
		// Otherwise, generate a call to `pulumi.all([]).apply()`.
		g.Fgen(w, "pulumi.all([")
		for i, o := range applyArgs {
			if i > 0 {
				g.Fgen(w, ", ")
			}
			//g.genApplyOutput(w, o)
			g.Fgenf(w, "%v", o)
		}
		g.Fgenf(w, "]).apply(%v)", then)
	}
}

func (g *generator) GenFunctionCallExpression(w io.Writer, expr *model.FunctionCallExpression) {
	switch expr.Name {
	case hcl2.IntrinsicApply:
		g.genApply(w, expr)
	case intrinsicInterpolate:
		g.Fgen(w, "pulumi.interpolate`")
		for _, part := range expr.Args {
			if lit, ok := part.(*model.LiteralValueExpression); ok && lit.Type() == model.StringType {
				g.Fgen(w, lit.Value.AsString())
			} else {
				g.Fgenf(w, "${%v}", part)
			}
		}
		g.Fgen(w, "`")
	case "fileArchive":
		g.Fgenf(w, "new pulumi.asset.FileArchive(%v)", expr.Args[0])
	case "fileAsset":
		g.Fgenf(w, "new pulumi.asset.FileAsset(%v)", expr.Args[0])
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
	g.Fgenf(w, "%v[%v]", expr.Collection, expr.Key)
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
	switch expr.Type() {
	case model.BoolType:
		g.Fgenf(w, "%v", expr.Value.True())
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
	if len(expr.Items) == 0 {
		g.Fgen(w, "{}")
	} else {
		g.Fgen(w, "{")
		g.Indented(func() {
			for _, item := range expr.Items {
				g.Fgenf(w, "\n%s", g.Indent)
				if lit, isLit := item.Key.(*model.LiteralValueExpression); isLit && lit.Type() == model.StringType {
					key := lit.Value.AsString()
					if isLegalIdentifier(key) {
						g.Fprint(w, key)
					} else {
						g.Fgen(w, lit)
					}
				} else {
					g.Fgen(w, item.Key)
				}

				g.Fgenf(w, ": %v,", item.Value)
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
			g.Fgen(w, "!")
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
	g.Fgen(w, expr.Source)
	g.genRelativeTraversal(w, expr.Traversal, expr.Parts)
}

func (g *generator) GenScopeTraversalExpression(w io.Writer, expr *model.ScopeTraversalExpression) {
	rootName := expr.RootName
	if v, ok := expr.Parts[0].(*model.Variable); ok && g.anonymousVariables.Has(v) {
		rootName = "__item"
	}

	g.Fgen(w, rootName)
	g.genRelativeTraversal(w, expr.Traversal.SimpleSplit().Rel, expr.Parts)
}

func (g *generator) GenSplatExpression(w io.Writer, expr *model.SplatExpression) {
	g.anonymousVariables.Add(expr.Item)
	g.Fgenf(w, "%v.map(__item => %v)", expr.Source, expr.Each)
}

func (g *generator) GenTemplateExpression(w io.Writer, expr *model.TemplateExpression) {
	if len(expr.Parts) == 1 {
		if lit, ok := expr.Parts[0].(*model.LiteralValueExpression); ok && lit.Type() == model.StringType {
			g.GenLiteralValueExpression(w, lit)
			return
		}
	}

	g.Fgen(w, "`")
	for _, expr := range expr.Parts {
		if lit, ok := expr.(*model.LiteralValueExpression); ok && lit.Type() == model.StringType {
			g.Fgen(w, lit.Value.AsString())
		} else {
			g.Fgenf(w, "${%v}", expr)
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
		g.Fgenf(w, "[%v]", expr.Expressions[0])
	default:
		g.Fgen(w, "[")
		g.Indented(func() {
			for _, v := range expr.Expressions {
				g.Fgenf(w, "\n%s%v,", g.Indent, v)
			}
		})
		g.Fgen(w, "\n", g.Indent, "]")
	}
}

func (g *generator) GenUnaryOpExpression(w io.Writer, expr *model.UnaryOpExpression) {
	opstr := ""
	switch expr.Operation {
	case hclsyntax.OpLogicalNot:
		opstr = "!"
	case hclsyntax.OpNegate:
		opstr = "-"
	}
	g.Fgenf(w, "%v%v", opstr, expr.Operand)
}
