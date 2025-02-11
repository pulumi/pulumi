// Copyright 2020-2025, Pulumi Corporation.
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

package python

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/big"
	"strings"
	"unicode"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type nameInfo int

func (nameInfo) Format(name string) string {
	return PyName(name)
}

func (g *generator) lowerExpression(expr model.Expression, typ model.Type) (model.Expression, []*quoteTemp) {
	// TODO(pdg): diagnostics

	expr = pcl.RewritePropertyReferences(expr)
	skipToJSONWhenRewritingApplies := true
	expr, diags := pcl.RewriteAppliesWithSkipToJSON(expr, nameInfo(0), false, skipToJSONWhenRewritingApplies)
	expr, lowerProxyDiags := g.lowerProxyApplies(expr)
	expr, convertDiags := pcl.RewriteConversions(expr, typ)
	expr, quotes, quoteDiags := g.rewriteQuotes(expr)

	diags = diags.Extend(lowerProxyDiags)
	diags = diags.Extend(convertDiags)
	diags = diags.Extend(quoteDiags)

	g.diagnostics = g.diagnostics.Extend(diags)

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
		g.Fgenf(w, " %s", PyName(p.Name))
	}

	g.Fgenf(w, ": %.v", expr.Body)
}

func (g *generator) GenBinaryOpExpression(w io.Writer, expr *model.BinaryOpExpression) {
	var opstr string
	precedence := g.GetPrecedence(expr)
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
	closedelim := "]"
	if expr.Key != nil {
		// Dictionary comprehension
		//
		// TODO(pdg): grouping
		g.Fgenf(w, "{%.v: %.v", expr.Key, expr.Value)
		closedelim = "}"
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

	g.Fprint(w, closedelim)
}

func (g *generator) genApply(w io.Writer, expr *model.FunctionCallExpression) {
	// Extract the list of outputs and the continuation expression from the `__apply` arguments.
	applyArgs, then := pcl.ParseApplyCall(expr)

	if len(applyArgs) == 1 {
		// If we only have a single output, just generate a normal `.apply`.
		g.Fgenf(w, "%.16v.apply(%.v)", applyArgs[0], then)
	} else {
		// Otherwise, generate a call to `pulumi.all([]).apply()`.
		g.Fgen(w, "pulumi.Output.all(\n")
		g.Indented(func() {
			for i, arg := range applyArgs {
				argName := then.Signature.Parameters[i].Name
				g.Fgenf(w, "%s%s=%v", g.Indent, argName, arg)
				if i < len(applyArgs)-1 {
					g.Fgen(w, ",")
				}
				g.Fgen(w, "\n")
			}
		})
		g.Fgen(w, ").apply(lambda resolved_outputs: ")
		rewrittenLambdaBody := rewriteApplyLambdaBody(then, "resolved_outputs")
		g.Fgenf(w, "%.v)\n", rewrittenLambdaBody)
	}
}

// functionName computes the Python package, module, and name for the given function token.
func functionName(tokenArg model.Expression) (string, string, string, hcl.Diagnostics) {
	token := tokenArg.(*model.TemplateExpression).Parts[0].(*model.LiteralValueExpression).Value.AsString()
	tokenRange := tokenArg.SyntaxNode().Range()

	// Compute the resource type from the Pulumi type token.
	pkg, module, member, diagnostics := pcl.DecomposeToken(token, tokenRange)
	return makeValidIdentifier(pkg), strings.ReplaceAll(module, "/", "."), title(member), diagnostics
}

var functionImports = map[string][]string{
	"fileArchive":      {"pulumi"},
	"remoteArchive":    {"pulumi"},
	"assetArchive":     {"pulumi"},
	"fileAsset":        {"pulumi"},
	"stringAsset":      {"pulumi"},
	"remoteAsset":      {"pulumi"},
	"filebase64":       {"base64"},
	"filebase64sha256": {"base64", "hashlib"},
	"readDir":          {"os"},
	"toBase64":         {"base64"},
	"fromBase64":       {"base64"},
	"toJSON":           {"json"},
	"sha1":             {"hashlib"},
	"stack":            {"pulumi"},
	"project":          {"pulumi"},
	"organization":     {"pulumi"},
	"cwd":              {"os"},
	"mimeType":         {"mimetypes"},
}

func (g *generator) getFunctionImports(x *model.FunctionCallExpression) []string {
	if x.Name != pcl.Invoke {
		return functionImports[x.Name]
	}

	pkg, _, _, diags := functionName(x.Args[0])
	contract.Assertf(len(diags) == 0, "unexpected diagnostics: %v", diags)
	return []string{"pulumi_" + pkg}
}

func (g *generator) GenFunctionCallExpression(w io.Writer, expr *model.FunctionCallExpression) {
	switch expr.Name {
	case pcl.IntrinsicConvert:
		from := expr.Args[0]
		to := pcl.LowerConversion(from, expr.Signature.ReturnType)
		output, isOutput := to.(*model.OutputType)
		if isOutput {
			to = output.ElementType
		}
		switch to := to.(type) {
		case *model.EnumType:
			components := strings.Split(to.Token, ":")
			contract.Assertf(len(components) == 3, "malformed token %v", to.Token)
			enum, ok := pcl.GetSchemaForType(to)
			if !ok {
				// No schema was provided
				g.Fgenf(w, "%.v", expr.Args[0])
				return
			}
			var moduleNameOverrides map[string]string
			if pkg, err := enum.(*schema.EnumType).PackageReference.Definition(); err == nil {
				if pkgInfo, ok := pkg.Language["python"].(PackageInfo); ok {
					moduleNameOverrides = pkgInfo.ModuleNameOverrides
				}
			}
			pkg := strings.ReplaceAll(components[0], "-", "_")
			enumName := tokenToName(to.Token)
			if m := tokenToModule(to.Token, nil, moduleNameOverrides); m != "" {
				modParts := strings.Split(m, "/")
				if len(modParts) == 2 && strings.EqualFold(modParts[1], enumName) {
					m = modParts[0]
				}
				pkg += "." + strings.ReplaceAll(m, "/", ".")
			}

			if isOutput {
				g.Fgenf(w, "%.v.apply(lambda x: %s.%s(x))", from, pkg, enumName)
			} else {
				diag := pcl.GenEnum(to, from, func(member *schema.Enum) {
					tag := member.Name
					if tag == "" {
						tag = fmt.Sprintf("%v", member.Value)
					}
					tag, err := makeSafeEnumName(tag, enumName)
					contract.AssertNoErrorf(err, "error sanitizing enum name")
					g.Fgenf(w, "%s.%s.%s", pkg, enumName, tag)
				}, func(from model.Expression) {
					g.Fgenf(w, "%s.%s(%.v)", pkg, enumName, from)
				})
				if diag != nil {
					g.diagnostics = append(g.diagnostics, diag)
				}
			}
		default:
			switch arg := from.(type) {
			case *model.ObjectConsExpression:
				g.genObjectConsExpression(w, arg, expr.Type())
			default:
				g.Fgenf(w, "%.v", expr.Args[0])
			}
		}
	case pcl.IntrinsicApply:
		g.genApply(w, expr)
	case "element":
		g.Fgenf(w, "%.16v[%.v]", expr.Args[0], expr.Args[1])
	case "entries":
		g.Fgenf(w, `[{"key": k, "value": v} for k, v in %.v]`, expr.Args[0])
	case "fileArchive":
		g.Fgenf(w, "pulumi.FileArchive(%.v)", expr.Args[0])
	case "remoteArchive":
		g.Fgenf(w, "pulumi.RemoteArchive(%.v)", expr.Args[0])
	case "assetArchive":
		g.Fgenf(w, "pulumi.AssetArchive(%.v)", expr.Args[0])
	case "fileAsset":
		g.Fgenf(w, "pulumi.FileAsset(%.v)", expr.Args[0])
	case "stringAsset":
		g.Fgenf(w, "pulumi.StringAsset(%.v)", expr.Args[0])
	case "remoteAsset":
		g.Fgenf(w, "pulumi.RemoteAsset(%.v)", expr.Args[0])
	case "filebase64":
		g.Fgenf(w, "(lambda path: base64.b64encode(open(path).read().encode()).decode())(%.v)", expr.Args[0])
	case "filebase64sha256":
		// Assuming the existence of the following helper method
		g.Fgenf(w, "computeFilebase64sha256(%v)", expr.Args[0])
	case "notImplemented":
		g.Fgenf(w, "not_implemented(%v)", expr.Args[0])
	case "singleOrNone":
		g.Fgenf(w, "single_or_none(%v)", expr.Args[0])
	case "mimeType":
		g.Fgenf(w, "mimetypes.guess_type(%v)[0]", expr.Args[0])
	case pcl.Call:
		self := expr.Args[0]
		method := expr.Args[1].(*model.TemplateExpression).Parts[0].(*model.LiteralValueExpression).Value.AsString()

		if expr.Signature.MultiArgumentInputs {
			err := fmt.Errorf("python program-gen does not implement MultiArgumentInputs for method '%s'", method)
			panic(err)
		}

		validMethod := makeValidIdentifier(method)
		g.Fgenf(w, "%v.%s(", self, validMethod)

		var args *model.ObjectConsExpression
		if converted, objectArgs, _ := pcl.RecognizeTypedObjectCons(expr.Args[2]); converted {
			args = objectArgs
		} else {
			args = expr.Args[2].(*model.ObjectConsExpression)
		}
		if len(args.Items) > 0 {
			indenter := func(f func()) { f() }
			if len(args.Items) > 1 {
				indenter = g.Indented
			}
			indenter(func() {
				for i, item := range args.Items {
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

		g.Fprint(w, ")")
	case pcl.Invoke:
		if expr.Signature.MultiArgumentInputs {
			err := fmt.Errorf("python program-gen does not implement MultiArgumentInputs for function '%v'",
				expr.Args[0])
			panic(err)
		}

		pkg, module, fn, diags := functionName(expr.Args[0])
		contract.Assertf(len(diags) == 0, "unexpected diagnostics: %v", diags)
		if module != "" {
			module = "." + module
		}
		name := fmt.Sprintf("%s%s.%s", pkg, module, PyName(fn))

		isOut := pcl.IsOutputVersionInvokeCall(expr)
		if isOut {
			name = name + "_output"
		}

		if len(expr.Args) == 1 {
			g.Fprintf(w, "%s()", name)
			return
		}

		optionsBag := ""
		if len(expr.Args) == 3 {
			var buf bytes.Buffer
			if invokeOptions, ok := expr.Args[2].(*model.ObjectConsExpression); ok {
				if isOut {
					g.Fgen(&buf, ", opts=pulumi.InvokeOutputOptions(")
				} else {
					g.Fgen(&buf, ", opts=pulumi.InvokeOptions(")
				}
				for i, item := range invokeOptions.Items {
					last := i == len(invokeOptions.Items)-1
					key := PyName(pcl.LiteralValueString(item.Key))
					g.Fgenf(&buf, "%s=%v", key, item.Value)
					if !last {
						g.Fgen(&buf, ", ")
					}
				}
				g.Fgen(&buf, ")")
			}

			optionsBag = buf.String()
		}

		g.Fgenf(w, "%s(", name)

		genFuncArgs := func(objectExpr *model.ObjectConsExpression) {
			indenter := func(f func()) { f() }
			if len(objectExpr.Items) > 1 {
				indenter = g.Indented
			}
			indenter(func() {
				for i, item := range objectExpr.Items {
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

		switch arg := expr.Args[1].(type) {
		case *model.FunctionCallExpression:
			if argsObject, ok := arg.Args[0].(*model.ObjectConsExpression); ok {
				genFuncArgs(argsObject)
			}

		case *model.ObjectConsExpression:
			genFuncArgs(arg)
		}

		g.Fgenf(w, "%v)", optionsBag)
	case "join":
		g.Fgenf(w, "%.16v.join(%v)", expr.Args[0], expr.Args[1])
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
		g.Fgenf(w, "pulumi.Output.secret(%v)", expr.Args[0])
	case "unsecret":
		g.Fgenf(w, "pulumi.Output.unsecret(%v)", expr.Args[0])
	case "split":
		g.Fgenf(w, "%.16v.split(%.v)", expr.Args[1], expr.Args[0])
	case "toBase64":
		g.Fgenf(w, "base64.b64encode(%.16v.encode()).decode()", expr.Args[0])
	case "fromBase64":
		g.Fgenf(w, "base64.b64decode(%.16v.encode()).decode()", expr.Args[0])
	case "toJSON":
		if model.ContainsOutputs(expr.Args[0].Type()) {
			g.Fgenf(w, "pulumi.Output.json_dumps(%.v)", expr.Args[0])
		} else {
			g.Fgenf(w, "json.dumps(%.v)", expr.Args[0])
		}
	case "sha1":
		g.Fgenf(w, "hashlib.sha1(%v.encode()).hexdigest()", expr.Args[0])
	case "project":
		g.Fgen(w, "pulumi.get_project()")
	case "stack":
		g.Fgen(w, "pulumi.get_stack()")
	case "organization":
		g.Fgen(w, "pulumi.get_organization()")
	case "cwd":
		g.Fgen(w, "os.getcwd()")
	case "getOutput":
		g.Fgenf(w, "%v.get_output(%v)", expr.Args[0], expr.Args[1])
	case "try":
		g.genTry(w, expr.Args)
	default:
		var rng hcl.Range
		if expr.Syntax != nil {
			rng = expr.Syntax.Range()
		}
		g.genNYI(w, "FunctionCallExpression: %v (%v)", expr.Name, rng)
	}
}

// genTry generates code for a `try` expression. Each argument is transformed into a closure to prevent its evaluation
// (which may fail) from happening until the `try_` utility function chooses. This results in an expression of the form:
//
//	try_(
//	    lambda: <arg1>,
//	    lambda: <arg2>,
//	    ...
//	)
func (g *generator) genTry(w io.Writer, args []model.Expression) {
	contract.Assertf(len(args) > 0, "expected at least one argument to try")

	g.Fprintf(w, "try_(")
	for i, arg := range args {
		g.Indented(func() {
			g.Fgenf(w, "\n%slambda: %v", g.Indent, arg)
		})
		if i < len(args)-1 {
			g.Fgen(w, ",")
		} else {
			g.Fgen(w, "\n")
		}
	}
	g.Fprintf(w, "%s)", g.Indent)
}

func (g *generator) GenIndexExpression(w io.Writer, expr *model.IndexExpression) {
	g.Fgenf(w, "%.16v[%.v]", expr.Collection, expr.Key)
}

type runeWriter interface {
	WriteRune(c rune) (int, error)
}

func escapeRune(c rune) string {
	if c < 0xFF {
		return fmt.Sprintf("\\x%02x", c)
	} else if c < 0xFFFF {
		return fmt.Sprintf("\\u%04x", c)
	}
	return fmt.Sprintf("\\U%08x", c)
}

//nolint:errcheck
func (g *generator) genEscapedString(w runeWriter, v string, escapeNewlines, escapeBraces bool) {
	for _, c := range v {
		switch c {
		case '\n':
			if escapeNewlines {
				w.WriteRune('\\')
				w.WriteRune('n')
			} else {
				w.WriteRune(c)
			}
			continue
		case '"', '\\':
			if escapeNewlines {
				w.WriteRune('\\')
				w.WriteRune(c)
				continue
			}
		case '{', '}':
			if escapeBraces {
				w.WriteRune(c)
				w.WriteRune(c)
				continue
			}
		}

		if unicode.IsPrint(c) {
			w.WriteRune(c)
			continue
		}

		// This is a non-printable character. We'll emit a Python escape sequence for it.
		codepoint := escapeRune(c)
		for _, r := range codepoint {
			w.WriteRune(r)
		}
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

func objectKey(item model.ObjectConsItem) string {
	switch key := item.Key.(type) {
	case *model.LiteralValueExpression:
		return key.Value.AsString()
	case *model.TemplateExpression:
		// assume a template expression has one constant part that is a LiteralValueExpression
		if len(key.Parts) == 1 {
			if literal, ok := key.Parts[0].(*model.LiteralValueExpression); ok {
				return literal.Value.AsString()
			}
		}
	}

	return ""
}

func (g *generator) genObjectConsExpression(w io.Writer, expr *model.ObjectConsExpression, destType model.Type) {
	targetType := pcl.LowerConversion(expr, destType)
	typeName := g.argumentTypeName(expr, targetType) // Example: aws.s3.BucketLoggingArgs
	td := g.typedDictEnabled(expr, targetType)
	if typeName != "" && !td {
		// If a typeName exists, and it's not for a typedDict, treat this as an Input Class
		// e.g. aws.s3.BucketLoggingArgs(key="value", foo="bar", ...)
		if len(expr.Items) == 0 {
			g.Fgenf(w, "%s()", typeName)
		} else {
			g.Fgenf(w, "%s(\n", typeName)
			g.Indented(func() {
				for _, item := range expr.Items {
					g.Fgenf(w, "%s", g.Indent)
					propertyKey := objectKey(item)
					g.Fprint(w, PyName(propertyKey))
					g.Fgenf(w, "=%.v,\n", item.Value)
				}
			})
			g.Fgenf(w, "%s)", g.Indent)
		}
	} else {
		// Otherwise treat this as a typed or untyped dictionary e.g. {"key": "value", "foo": "bar", ...}
		if len(expr.Items) == 0 {
			g.Fgen(w, "{}")
		} else {
			g.Fgen(w, "{")
			g.Indented(func() {
				for _, item := range expr.Items {
					if td {
						// If we're inside a typedDict, use the PyName of the key.
						propertyKey := objectKey(item)
						key := PyName(propertyKey)
						g.Fgenf(w, "\n%s%q: %.v,", g.Indent, key, item.Value)
					} else {
						g.Fgenf(w, "\n%s%.v: %.v,", g.Indent, item.Key, item.Value)
					}
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
			key = cty.StringVal(PyName(traverser.Name))
		case hcl.TraverseIndex:
			key = traverser.Key
		default:
			contract.Failf("unexpected traverser of type %T (%v)", traverser, traverser.SourceRange())
		}

		switch key.Type() {
		case cty.String:
			keyVal := PyName(key.AsString())
			contract.Assertf(isLegalIdentifier(keyVal), "illegal identifier: %q", keyVal)
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
	if g.isComponent {
		configVars := map[string]*pcl.ConfigVariable{}
		for _, configVar := range g.program.ConfigVariables() {
			configVars[configVar.Name()] = configVar
		}

		if _, isConfig := configVars[expr.RootName]; isConfig {
			if _, configReference := expr.Parts[0].(*pcl.ConfigVariable); configReference {
				rootName = fmt.Sprintf("args[\"%s\"]", expr.RootName)
			}
		}
	}

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
