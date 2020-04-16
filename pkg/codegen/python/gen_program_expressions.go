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
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

type nameInfo int

func (nameInfo) IsReservedWord(word string) bool {
	return isReservedWord(word)
}

func (g *generator) lowerExpression(expr model.Expression) model.Expression {
	// TODO(pdg): diagnostics

	expr, _ = hcl2.RewriteApplies(expr, nameInfo(0), false)
	expr, _ = g.lowerProxyApplies(expr)
	return expr
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

// functionName computes the NodeJS package, module, and name for the given function token.
func functionName(tokenArg model.Expression) (string, string, string, hcl.Diagnostics) {
	token := tokenArg.(*model.TemplateExpression).Parts[0].(*model.LiteralValueExpression).Value.AsString()
	tokenRange := tokenArg.SyntaxNode().Range()

	// Compute the resource type from the Pulumi type token.
	pkg, module, member, diagnostics := hcl2.DecomposeToken(token, tokenRange)
	return cleanName(pkg), strings.Replace(module, "/", ".", -1), title(member), diagnostics
}

func (g *generator) GenFunctionCallExpression(w io.Writer, expr *model.FunctionCallExpression) {
	switch expr.Name {
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
	case "invoke":
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

		g.Fgenf(w, "%s(%.v%v)", name, expr.Args[1], optionsBag)
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
			w.WriteRune('\\')
		case '{', '}':
			if escapeBraces {
				w.WriteRune(c)
			}
		}
		w.WriteRune(c)
	}
}

func (g *generator) genStringLiteral(w io.Writer, v string) {
	builder := &strings.Builder{}
	newlines := strings.Count(v, "\n")
	if newlines == 0 || newlines == 1 && (v[0] == '\n' || v[len(v)-1] == '\n') {
		// This string either does not contain newlines or contains a single leading or trailing newline, so we'll
		// Generate a short string literal. Quotes, backslashes, and newlines will be escaped in conformance with
		// https://docs.python.org/3.7/reference/lexical_analysis.html#literals.
		builder.WriteRune('"')
		g.genEscapedString(builder, v, true, false)
		builder.WriteRune('"')
	} else {
		// This string does contain newlines, so we'll generate a long string literal. "${", backquotes, and
		// backslashes will be escaped in conformance with
		// https://docs.python.org/3.7/reference/lexical_analysis.html#literals.
		builder.WriteString(`"""`)
		g.genEscapedString(builder, v, false, false)
		builder.WriteString(`"""`)
	}

	g.Fgenf(w, "%s", builder.String())
}

func (g *generator) GenLiteralValueExpression(w io.Writer, expr *model.LiteralValueExpression) {
	switch expr.Type() {
	case model.BoolType:
		if expr.Value.True() {
			g.Fgen(w, "True")
		} else {
			g.Fgen(w, "False")
		}
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
				g.Fgenf(w, "\n%s%.v: %.v,", g.Indent, item.Key, item.Value)
			}
		})
		g.Fgenf(w, "\n%s}", g.Indent)
	}
}

func (g *generator) genRelativeTraversal(w io.Writer, traversal hcl.Traversal, types []model.Traversable) {
	for _, part := range traversal {
		var key cty.Value
		switch part := part.(type) {
		case hcl.TraverseAttr:
			key = cty.StringVal(part.Name)
		case hcl.TraverseIndex:
			key = part.Key
		default:
			contract.Failf("unexpected traversal part of type %T (%v)", part, part.SourceRange())
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
	if len(expr.Parts) == 1 {
		if lit, ok := expr.Parts[0].(*model.LiteralValueExpression); ok && lit.Type() == model.StringType {
			g.GenLiteralValueExpression(w, lit)
			return
		}
	}

	isMultiLine, quotes := false, `"`
	for i, part := range expr.Parts {
		if lit, ok := part.(*model.LiteralValueExpression); ok && lit.Type() == model.StringType {
			v := lit.Value.AsString()
			switch strings.Count(v, "\n") {
			case 0:
				continue
			case 1:
				if i == 0 && v[0] == '\n' || i == len(expr.Parts)-1 && v[len(v)-1] == '\n' {
					continue
				}
			}
			isMultiLine, quotes = true, `"""`
			break
		}
	}

	b := bufio.NewWriter(w)
	defer b.Flush()

	g.Fprintf(b, `f%s`, quotes)
	for _, expr := range expr.Parts {
		if lit, ok := expr.(*model.LiteralValueExpression); ok && lit.Type() == model.StringType {
			g.genEscapedString(b, lit.Value.AsString(), !isMultiLine, true)
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
