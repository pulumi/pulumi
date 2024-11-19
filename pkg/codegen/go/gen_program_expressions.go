// Copyright 2020-2024, Pulumi Corporation.
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

package gen

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

const keywordRange = "range"

func (g *generator) GetPrecedence(expr model.Expression) int {
	// TODO: Current values copied from Node, update based on
	// https://golang.org/ref/spec
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

// GenAnonymousFunctionExpression generates code for an AnonymousFunctionExpression.
func (g *generator) GenAnonymousFunctionExpression(w io.Writer, expr *model.AnonymousFunctionExpression) {
	g.genAnonymousFunctionExpression(w, expr, nil, false)
}

func (g *generator) genAnonymousFunctionExpression(
	w io.Writer,
	expr *model.AnonymousFunctionExpression,
	bodyPreamble []string,
	inApply bool,
) {
	g.Fgenf(w, "func(")
	leadingSep := ""
	for _, param := range expr.Signature.Parameters {
		isInput := isInputty(param.Type)
		g.Fgenf(w, "%s%s %s", leadingSep, makeValidIdentifier(param.Name), g.argumentTypeName(param.Type, isInput))
		leadingSep = ", "
	}

	retType := expr.Signature.ReturnType
	if inApply {
		retType = model.ResolveOutputs(retType)
	}

	retTypeName := g.argumentTypeName(retType, false)
	g.Fgenf(w, ") (%s, error) {\n", retTypeName)

	for _, decl := range bodyPreamble {
		g.Fgenf(w, "%s\n", decl)
	}

	body, temps := g.lowerExpression(expr.Body, retType)
	g.genTempsMultiReturn(w, temps, retTypeName)

	// g.Fgenf(w, "return %v, nil", body)

	// fromBase64 special case
	if b, ok := body.(*model.FunctionCallExpression); ok && b.Name == fromBase64Fn {
		g.Fgenf(w, "value, _ := %v\n", b)
		g.Fgenf(w, "return pulumi.String(value), nil")
	} else if strings.HasPrefix(retTypeName, "pulumi") {
		g.Fgenf(w, "return %s(%v), nil", retTypeName, body)
	} else {
		g.Fgenf(w, "return %v, nil", body)
	}
	g.Fgenf(w, "\n}")
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
	// Ternary expressions are not supported in go so we need to allocate temp variables in the parent scope.
	// This is handled by lower expression and rewriteTernaries
	contract.Failf("unlowered conditional expression @ %v", expr.SyntaxNode().Range())
}

// GenForExpression generates code for a ForExpression.
func (g *generator) GenForExpression(w io.Writer, expr *model.ForExpression) {
	g.genNYI(w, "For expression")
}

func (g *generator) genSafeEnum(w io.Writer, to *model.EnumType, dest model.Type) func(member *schema.Enum) {
	return func(member *schema.Enum) {
		// We know the enum value at the call site, so we can directly stamp in a
		// valid enum instance. We don't need to convert.
		enumName := tokenToName(to.Token)
		memberTag := member.Name
		if memberTag == "" {
			memberTag = member.Value.(string)
		}
		memberTag, err := makeSafeEnumName(memberTag, enumName)
		contract.AssertNoErrorf(err, "Enum is invalid")
		pkg, mod, _, _ := pcl.DecomposeToken(to.Token, to.SyntaxNode().Range())
		mod = g.getModOrAlias(pkg, mod, mod)

		if union, isUnion := dest.(*model.UnionType); isUnion && len(union.Annotations) > 0 {
			if input, ok := union.Annotations[0].(schema.Type); ok {
				if _, ok := codegen.ResolvedType(input).(*schema.UnionType); ok {
					g.Fgenf(w, "pulumi.String(%s.%s)", mod, memberTag)
					return
				}
			}
		}
		g.Fgenf(w, "%s.%s", mod, memberTag)
	}
}

func (g *generator) GenFunctionCallExpression(w io.Writer, expr *model.FunctionCallExpression) {
	switch expr.Name {
	case pcl.IntrinsicConvert:
		from := expr.Args[0]
		to := pcl.LowerConversion(from, expr.Signature.ReturnType)
		output, isOutput := to.(*model.OutputType)
		originalTo := to
		if isOutput {
			to = output.ElementType
		}
		_, isFromOutput := from.Type().(*model.OutputType)

		switch to := to.(type) {
		case *model.EnumType:
			var underlyingType string
			switch {
			case to.Type.Equals(model.StringType):
				underlyingType = "string"
			case to.Type.Equals(model.IntType):
				underlyingType = "int"
			default:
				underlyingType = "float64"
			}
			pkg, mod, typ, _ := pcl.DecomposeToken(to.Token, to.SyntaxNode().Range())
			mod = g.getModOrAlias(pkg, mod, mod)
			enumTag := fmt.Sprintf("%s.%s", mod, typ)
			if isOutput {
				g.Fgenf(w,
					"%.v.ApplyT(func(x *%[3]s) %[2]s { return %[2]s(*x) }).(%[2]sOutput)",
					from, enumTag, underlyingType)
				return
			}
			diag := pcl.GenEnum(to, from, g.genSafeEnum(w, to, expr.Signature.ReturnType), func(from model.Expression) {
				g.Fgenf(w, "%s(%v)", enumTag, from)
			})
			if diag != nil {
				g.diagnostics = append(g.diagnostics, diag)
			}
			return
		}
		switch arg := from.(type) {
		case *model.TupleConsExpression:
			g.genTupleConsExpression(w, arg, expr.Type())
		case *model.ObjectConsExpression:
			isInput := false
			g.genObjectConsExpression(w, arg, expr.Type(), isInput)
		case *model.LiteralValueExpression:
			g.genLiteralValueExpression(w, arg, expr.Type())
		case *model.TemplateExpression:
			g.genTemplateExpression(w, arg, expr.Type())
		case *model.ScopeTraversalExpression:
			g.genScopeTraversalExpression(w, arg, expr.Type())
		default:
			// Add a cast to the type we expect if needed
			if originalTo.AssignableFrom(from.Type()) && (isOutput == isFromOutput) {
				g.Fgenf(w, "%.v", from)
			} else {
				typeName := g.argumentTypeName(to, isOutput)
				// IDOutput has a special case where it can be converted to a string
				var isID bool
				switch expr := from.(type) {
				case *model.ScopeTraversalExpression:
					last := expr.Traversal[len(expr.Traversal)-1]
					if attr, ok := last.(hcl.TraverseAttr); ok && attr.Name == "id" {
						isID = true
					}
				case *model.RelativeTraversalExpression:
					last := expr.Traversal[len(expr.Traversal)-1]
					if attr, ok := last.(hcl.TraverseAttr); ok && attr.Name == "id" {
						isID = true
					}
				}

				if typeName == "" {
					g.Fgenf(w, "%.v", from)
				} else if typeName == "pulumi.String" && isID {
					g.Fgenf(w, "%.v", from)
				} else {
					g.Fgenf(w, "%s(%.v)", typeName, from)
				}
			}
		}
	case pcl.IntrinsicApply:
		g.genApply(w, expr)
	case "fileArchive":
		g.Fgenf(w, "pulumi.NewFileArchive(%.v)", expr.Args[0])
	case "remoteArchive":
		g.Fgenf(w, "pulumi.NewRemoteArchive(%.v)", expr.Args[0])
	case "assetArchive":
		g.Fgenf(w, "pulumi.NewAssetArchive(%.v)", expr.Args[0])
	case "fileAsset":
		g.Fgenf(w, "pulumi.NewFileAsset(%.v)", expr.Args[0])
	case "stringAsset":
		g.Fgenf(w, "pulumi.NewStringAsset(%.v)", expr.Args[0])
	case "remoteAsset":
		g.Fgenf(w, "pulumi.NewRemoteAsset(%.v)", expr.Args[0])
	case "filebase64":
		// Assuming the existence of the following helper method
		g.Fgenf(w, "filebase64OrPanic(%v)", expr.Args[0])
	case "filebase64sha256":
		// Assuming the existence of the following helper method
		g.Fgenf(w, "filebase64sha256OrPanic(%v)", expr.Args[0])
	case "notImplemented":
		g.Fgenf(w, "notImplemented(%v)", expr.Args[0])
	case "singleOrNone":
		g.Fgenf(w, "singleOrNone(%v)", expr.Args[0])
	case pcl.Invoke:
		if expr.Signature.MultiArgumentInputs {
			panic(fmt.Errorf("go program-gen does not implement MultiArgumentInputs for function '%v'",
				expr.Args[0]))
		}

		pkg, module, fn, diags := g.functionName(expr.Args[0])
		contract.Assertf(len(diags) == 0, "We don't allow problems getting the function name")
		if module == "" || module == "index" {
			module = pkg
		}
		isOut, outArgs, outArgsType := pcl.RecognizeOutputVersionedInvoke(expr)
		if isOut {
			outTypeName, err := outputVersionFunctionArgTypeName(outArgsType, g.externalCache)
			if err != nil {
				// We create a diag instead of panicking since panics are caught in go
				// format expressions.
				g.diagnostics = append(g.diagnostics, &hcl.Diagnostic{
					Severity:    hcl.DiagError,
					Summary:     "Error when generating an output-versioned Invoke",
					Detail:      fmt.Sprintf("underlying error: %v", err),
					Subject:     &hcl.Range{},
					Context:     &hcl.Range{},
					Expression:  nil,
					EvalContext: &hcl.EvalContext{},
				})
				g.Fgenf(w, "%q", "failed") // Write a value to avoid syntax errors
				return
			}
			g.Fgenf(w, "%s.%sOutput(ctx, ", module, fn)
			g.genObjectConsExpressionWithTypeName(w, outArgs, outArgsType, outTypeName)
		} else {
			g.Fgenf(w, "%s.%s(ctx, ", module, fn)
			g.Fgenf(w, "%.v", expr.Args[1])
		}

		var optionsBag string
		var buf bytes.Buffer
		if len(expr.Args) == 3 {
			if invokeOptions, ok := expr.Args[2].(*model.ObjectConsExpression); ok {
				g.Fgen(&buf, ", ")
				for i, item := range invokeOptions.Items {
					last := i == len(invokeOptions.Items)-1
					switch pcl.LiteralValueString(item.Key) {
					case "provider":
						g.Fgenf(&buf, "pulumi.Provider(%v)", item.Value)
					case "parent":
						g.Fgenf(&buf, "pulumi.Parent(%v)", item.Value)
					case "version":
						g.Fgenf(&buf, "pulumi.Version(%v)", item.Value)
					case "pluginDownloadUrl":
						g.Fgenf(&buf, "pulumi.PluginDownloadURL(%v)", item.Value)
					case "dependsOn":
						destType := model.NewListType(resourceType)
						value, temps := g.lowerExpression(item.Value, destType)
						contract.Assertf(len(temps) == 0, "can not have temporary variables when converting dependsOn option: %v", temps)
						if isInputty(value.Type()) {
							g.Fgenf(&buf, "pulumi.DependsOnInputs(%v)", value)
						} else {
							g.Fgenf(&buf, "pulumi.DependsOn(%v)", value)
						}
					}

					if !last {
						g.Fgen(&buf, ", ")
					}
				}
			}
		} else {
			g.Fgenf(&buf, ", nil")
		}
		optionsBag = buf.String()
		g.Fgenf(w, "%v)", optionsBag)
	case "join":
		g.Fgenf(w, "strings.Join(%v, %v)", expr.Args[1], expr.Args[0])
	case "length":
		g.Fgenf(w, "len(%.20v)", expr.Args[0])
	case "readFile":
		// Assuming the existence of the following helper method located earlier in the preamble
		g.Fgenf(w, "readFileOrPanic(%v)", expr.Args[0])
	case "secret":
		outputTypeName := "pulumi.Any"
		if model.ResolveOutputs(expr.Type()) != model.DynamicType {
			outputTypeName = g.argumentTypeName(expr.Type(), false)
		}
		g.Fgenf(w, "pulumi.ToSecret(%v).(%sOutput)", expr.Args[0], outputTypeName)
	case "unsecret":
		outputTypeName := "pulumi.Any"
		if model.ResolveOutputs(expr.Type()) != model.DynamicType {
			outputTypeName = g.argumentTypeName(expr.Type(), false)
		}
		g.Fgenf(w, "pulumi.Unsecret(%v).(%sOutput)", expr.Args[0], outputTypeName)
	case "toBase64":
		g.Fgenf(w, "base64.StdEncoding.EncodeToString([]byte(%v))", expr.Args[0])
	case fromBase64Fn:
		g.Fgenf(w, "base64.StdEncoding.DecodeString(%v)", expr.Args[0])
	case "mimeType":
		g.Fgenf(w, "mime.TypeByExtension(path.Ext(%.v))", expr.Args[0])
	case "sha1":
		g.Fgenf(w, "sha1Hash(%v)", expr.Args[0])
	case "goOptionalFloat64":
		g.Fgenf(w, "pulumi.Float64Ref(%.v)", expr.Args[0])
	case "goOptionalBool":
		g.Fgenf(w, "pulumi.BoolRef(%.v)", expr.Args[0])
	case "goOptionalInt":
		g.Fgenf(w, "pulumi.IntRef(%.v)", expr.Args[0])
	case "goOptionalString":
		g.Fgenf(w, "pulumi.StringRef(%.v)", expr.Args[0])
	case "stack":
		g.Fgen(w, "ctx.Stack()")
	case "project":
		g.Fgen(w, "ctx.Project()")
	case "organization":
		g.Fgen(w, "ctx.Organization()")
	case "cwd":
		g.Fgen(w, "func(cwd string, err error) string { if err != nil { panic(err) }; return cwd }(os.Getwd())")
	case "getOutput":
		g.Fgenf(w, "%v.GetOutput(pulumi.String(%v))", expr.Args[0], expr.Args[1])
	default:
		// toJSON and readDir are reduced away, shouldn't see them here
		reducedFunctions := codegen.NewStringSet("toJSON", "readDir")
		contract.Assertf(!reducedFunctions.Has(expr.Name), "unlowered function %s", expr.Name)
		// TODO: implement "element", "entries", "lookup", "split" and "range"
		g.genNYI(w, "call %v", expr.Name)
	}
}

// Currently args type for output-versioned invokes are named
// `FOutputArgs`, but this is not yet understood by `tokenToType`. Use
// this function to compensate.
func outputVersionFunctionArgTypeName(t model.Type, cache *Cache) (string, error) {
	schemaType, ok := pcl.GetSchemaForType(t)
	if !ok {
		return "", errors.New("No schema.Type type found for the given model.Type")
	}

	objType, ok := schemaType.(*schema.ObjectType)
	if !ok {
		return "", fmt.Errorf("Expected a schema.ObjectType, got %s", schemaType.String())
	}

	pkg := &pkgContext{
		pkg:              (&schema.Package{Name: "main"}).Reference(),
		externalPackages: cache,
	}

	var ty string
	if pkg.isExternalReference(objType) {
		extPkg, _ := pkg.contextForExternalReference(objType)
		ty = extPkg.tokenToType(objType.Token)
	} else {
		ty = pkg.tokenToType(objType.Token)
	}

	return strings.TrimSuffix(ty, "Args") + "OutputArgs", nil
}

func (g *generator) GenIndexExpression(w io.Writer, expr *model.IndexExpression) {
	g.Fgenf(w, "%.20v[%.v]", expr.Collection, expr.Key)
}

func (g *generator) GenLiteralValueExpression(w io.Writer, expr *model.LiteralValueExpression) {
	g.genLiteralValueExpression(w, expr, expr.Type())
}

func (g *generator) genLiteralValueExpression(w io.Writer, expr *model.LiteralValueExpression, destType model.Type) {
	exprType := expr.Type()
	if cns, ok := exprType.(*model.ConstType); ok {
		exprType = cns.Type
	}

	if exprType == model.NoneType {
		g.Fgen(w, "nil")
		return
	}

	argTypeName := g.argumentTypeName(destType, false)
	isPulumiType := strings.HasPrefix(argTypeName, "pulumi.")

	switch exprType {
	case model.BoolType:
		if isPulumiType {
			g.Fgenf(w, "%s(%v)", argTypeName, expr.Value.True())
		} else {
			g.Fgenf(w, "%v", expr.Value.True())
		}
	case model.NumberType, model.IntType:
		bf := expr.Value.AsBigFloat()
		if i, acc := bf.Int64(); acc == big.Exact {
			if isPulumiType {
				g.Fgenf(w, "%s(%d)", argTypeName, i)
			} else {
				g.Fgenf(w, "%d", i)
			}
		} else {
			f, _ := bf.Float64()
			if isPulumiType {
				g.Fgenf(w, "%s(%g)", argTypeName, f)
			} else {
				g.Fgenf(w, "%g", f)
			}
		}
	case model.StringType:
		strVal := expr.Value.AsString()
		if isPulumiType {
			g.Fgenf(w, "%s(", argTypeName)
			g.genStringLiteral(w, strVal, true /* allow raw */)
			g.Fgenf(w, ")")
		} else {
			g.genStringLiteral(w, strVal, true /* allow raw */)
		}
	default:
		contract.Failf("unexpected opaque type in GenLiteralValueExpression: %v (%v)", destType,
			expr.SyntaxNode().Range())
	}
}

func (g *generator) GenObjectConsExpression(w io.Writer, expr *model.ObjectConsExpression) {
	switch argType := expr.Type().(type) {
	case *model.ObjectType:
		if len(argType.Annotations) > 0 {
			if configMetadata, ok := argType.Annotations[0].(*ObjectTypeFromConfigMetadata); ok {
				g.genObjectConsExpressionWithTypeName(w, expr, expr.Type(), configMetadata.TypeName)
				return
			}
		}
	}

	isInput := false
	g.genObjectConsExpression(w, expr, expr.Type(), isInput)
}

func (g *generator) genObjectConsExpression(
	w io.Writer,
	expr *model.ObjectConsExpression,
	destType model.Type,
	isInput bool,
) {
	isInput = isInput || isInputty(destType)

	typeName := g.argumentTypeName(destType, isInput)
	if schemaType, ok := pcl.GetSchemaForType(destType); ok {
		if obj, ok := codegen.UnwrapType(schemaType).(*schema.ObjectType); ok {
			if g.useLookupInvokeForm(obj.Token) {
				typeName = strings.Replace(typeName, ".Get", ".Lookup", 1)
			}
		}
	}

	if schemaType, ok := g.toSchemaType(destType); ok {
		if codegen.ResolvedType(schemaType) == schema.AnyType {
			g.Fgenf(w, "pulumi.Any(")
			g.genObjectConsExpressionWithTypeName(w, expr, destType, "map[string]interface{}")
			g.Fgenf(w, ")")
			return
		}
	}

	g.genObjectConsExpressionWithTypeName(w, expr, destType, typeName)
}

func (g *generator) toSchemaType(destType model.Type) (schema.Type, bool) {
	schemaType, ok := pcl.GetSchemaForType(destType)
	if !ok {
		return nil, false
	}
	return codegen.UnwrapType(schemaType), true
}

func (g *generator) genObjectConsExpressionWithTypeName(
	w io.Writer,
	expr *model.ObjectConsExpression,
	destType model.Type,
	typeName string,
) {
	isMap := strings.HasPrefix(typeName, "map[")

	// TODO: retrieve schema and propagate optionals to emit bool ptr, etc.

	if g.inGenTupleConExprListArgs {
		if g.isPtrArg {
			g.Fgenf(w, "&%s", typeName)
		}
	} else if isMap || !strings.HasSuffix(typeName, "Args") || strings.HasSuffix(typeName, "OutputArgs") {
		g.Fgenf(w, "%s", typeName)
	} else {
		g.Fgenf(w, "&%s", typeName)
	}
	g.Fgenf(w, "{\n")

	for _, item := range expr.Items {
		if lit, ok := g.literalKey(item.Key); ok {
			if isMap || strings.HasSuffix(typeName, "Map") {
				g.Fgenf(w, "\"%s\"", lit)
			} else {
				g.Fgenf(w, "%s", Title(lit))
			}
		} else {
			g.Fgenf(w, "%.v", item.Key)
		}

		g.Fgenf(w, ": %.v,\n", item.Value)
	}

	g.Fgenf(w, "}")
}

func (g *generator) GenRelativeTraversalExpression(w io.Writer, expr *model.RelativeTraversalExpression) {
	g.Fgenf(w, "%.20v", expr.Source)
	isRootResource := false
	if ie, ok := expr.Source.(*model.IndexExpression); ok {
		if se, ok := ie.Collection.(*model.ScopeTraversalExpression); ok {
			if _, ok := se.Parts[0].(*pcl.Resource); ok {
				isRootResource = true
			}
		}
	}
	g.genRelativeTraversal(w, expr.Traversal, expr.Parts, isRootResource)
}

func (g *generator) GenScopeTraversalExpression(w io.Writer, expr *model.ScopeTraversalExpression) {
	g.genScopeTraversalExpression(w, expr, expr.Type())
}

func (g *generator) genScopeTraversalExpression(
	w io.Writer, expr *model.ScopeTraversalExpression, destType model.Type,
) {
	rootName := expr.RootName

	if _, ok := expr.Parts[0].(*model.SplatVariable); ok {
		rootName = "val0"
	}

	genIDCall := false

	isInput := false
	if schemaType, ok := pcl.GetSchemaForType(destType); ok {
		_, isInput = schemaType.(*schema.InputType)
	}

	var sourceIsPlain bool
	switch root := expr.Parts[0].(type) {
	case *pcl.Resource:
		isInput = false
		if _, ok := pcl.GetSchemaForType(root.InputType); ok {
			// convert .id into .ID()
			last := expr.Traversal[len(expr.Traversal)-1]
			if attr, ok := last.(hcl.TraverseAttr); ok && attr.Name == "id" {
				genIDCall = true
				expr.Traversal = expr.Traversal[:len(expr.Traversal)-1]
			}
		}
	case *pcl.LocalVariable:
		if root, ok := root.Definition.Value.(*model.FunctionCallExpression); ok && !pcl.IsOutputVersionInvokeCall(root) {
			sourceIsPlain = true
		}
	case *pcl.ConfigVariable:
		if g.isComponent {
			// config variables of components are always of type Input<T>
			// these shouldn't be wrapped in a pulumi.String(...), pulumi.Int(...) etc. functions
			g.Fgenf(w, "args.%s", Title(rootName))
			isRootResource := false
			g.genRelativeTraversal(w, expr.Traversal.SimpleSplit().Rel, expr.Parts[1:], isRootResource)
			return
		}
	}

	// TODO if it's an array type, we need a lowering step to turn []string -> pulumi.StringArray
	if isInput {
		argTypeName := g.argumentTypeName(expr.Type(), isInput)
		if strings.HasSuffix(argTypeName, "Array") {
			destTypeName := g.argumentTypeName(destType, isInput)
			// `argTypeName` == `destTypeName` and `argTypeName` ends with `Array`, we
			// know that `destType` is an outputty type. If the source is plain (and thus
			// not outputty), then the types can never line up and we will need a
			// conversion helper method.
			if argTypeName != destTypeName || sourceIsPlain {
				// use a helper to transform prompt arrays into inputty arrays
				var helper *promptToInputArrayHelper
				if h, ok := g.arrayHelpers[argTypeName]; ok {
					helper = h
				} else {
					// helpers are emitted at the end in the postamble step
					helper = &promptToInputArrayHelper{
						destType: argTypeName,
					}
					g.arrayHelpers[argTypeName] = helper
				}
				// Wrap the emitted expression in a call to the generated helper function.
				g.Fgenf(w, "%s(", helper.getFnName())
				defer g.Fgenf(w, ")")
			}
		} else {
			// Wrap the emitted expression in a type conversion.
			g.Fgenf(w, "%s(", g.argumentTypeName(expr.Type(), isInput))
			defer g.Fgenf(w, ")")
		}
	}

	// TODO: this isn't exhaustively correct as "range" could be a legit var name
	// instead we should probably use a fn call expression here for entries/range
	// similar to other languages
	if rootName == keywordRange {
		part := expr.Traversal[1].(hcl.TraverseAttr).Name
		switch part {
		case "value":
			g.Fgenf(w, "val0")
		case "key":
			g.Fgenf(w, "key0")
		default:
			contract.Failf("unexpected traversal on range expression: %s", part)
		}
	} else {
		g.Fgen(w, makeValidIdentifier(rootName))
		isRootResource := false
		g.genRelativeTraversal(w, expr.Traversal.SimpleSplit().Rel, expr.Parts[1:], isRootResource)
	}

	if genIDCall {
		g.Fgenf(w, ".ID()")
	}
}

// GenSplatExpression generates code for a SplatExpression.
func (g *generator) GenSplatExpression(w io.Writer, expr *model.SplatExpression) {
	contract.Failf("unlowered splat expression @ %v", expr.SyntaxNode().Range())
}

// GenTemplateExpression generates code for a TemplateExpression.
func (g *generator) GenTemplateExpression(w io.Writer, expr *model.TemplateExpression) {
	g.genTemplateExpression(w, expr, expr.Type())
}

func (g *generator) genTemplateExpression(w io.Writer, expr *model.TemplateExpression, destType model.Type) {
	if len(expr.Parts) == 1 {
		if lit, ok := expr.Parts[0].(*model.LiteralValueExpression); ok && model.StringType.AssignableFrom(lit.Type()) {
			g.genLiteralValueExpression(w, lit, destType)
		}

		// If we have a template expression that doesn't start with a string, it indicates
		// an invalid *pcl.Program. Instead of crashing, we continue.
		return
	}
	argTypeName := g.argumentTypeName(destType, false)
	isPulumiType := strings.HasPrefix(argTypeName, "pulumi.")
	isPulumiStr := argTypeName == "pulumi.String"
	if isPulumiType && !isPulumiStr {
		g.Fgenf(w, "%s(", argTypeName)
		defer g.Fgenf(w, ")")
	}

	var fmtStr strings.Builder
	args := new(bytes.Buffer)
	canBeRaw := true
	for _, v := range expr.Parts {
		if lit, ok := v.(*model.LiteralValueExpression); ok && lit.Value.Type().Equals(cty.String) {
			str := lit.Value.AsString()
			// We don't want to accidentally embed a formatting directive in our
			// formatting string.
			if !strings.ContainsRune(str, '%') {
				if canBeRaw && strings.ContainsRune(str, '`') {
					canBeRaw = false
				}
				// Build the formatting string
				fmtStr.WriteString(str)
				continue
			}
		}
		// v cannot be directly inserted into the formatting string, so put it in the
		// argument list.
		fmtStr.WriteString("%v")
		g.Fgenf(args, ", %.v", v)
	}
	if isPulumiStr {
		g.Fgenf(w, "pulumi.Sprintf(")
	} else {
		g.Fgenf(w, "fmt.Sprintf(")
	}
	g.genStringLiteral(w, fmtStr.String(), canBeRaw)
	_, err := args.WriteTo(w)
	contract.AssertNoErrorf(err, "Failed to write arguments")
	g.Fgenf(w, ")")
}

// GenTemplateJoinExpression generates code for a TemplateJoinExpression.
func (g *generator) GenTemplateJoinExpression(w io.Writer, expr *model.TemplateJoinExpression) { /*TODO*/
}

func (g *generator) GenTupleConsExpression(w io.Writer, expr *model.TupleConsExpression) {
	g.genTupleConsExpression(w, expr, expr.Type())
}

// GenTupleConsExpression generates code for a TupleConsExpression.
func (g *generator) genTupleConsExpression(w io.Writer, expr *model.TupleConsExpression, destType model.Type) {
	isInput := isInputty(destType)
	argType := g.argumentTypeName(destType, isInput)
	// don't need to generate type for list args if not a pointer, i.e. []ec2.SubnetSpecArgs{ {Type: ...} }
	// unless it contains an interface, i.e. []map[string]interface{ map[string]interface{"key": "val"} }
	if strings.HasPrefix(argType, "[]") && !strings.Contains(argType, "interface{}") {
		defer func(b bool) { g.inGenTupleConExprListArgs = b }(g.inGenTupleConExprListArgs)
		g.inGenTupleConExprListArgs = true
		if strings.HasPrefix(argType, "[]*") {
			defer func(b bool) { g.isPtrArg = b }(g.isPtrArg)
			g.isPtrArg = true
		}
	}
	g.Fgenf(w, "%s{\n", argType)
	for _, v := range expr.Expressions {
		g.Fgenf(w, "%v,\n", v)
	}
	g.Fgenf(w, "}")
}

func (g *generator) GenUnaryOpExpression(w io.Writer, expr *model.UnaryOpExpression) {
	var opstr string
	precedence := g.GetPrecedence(expr)
	switch expr.Operation {
	case hclsyntax.OpLogicalNot:
		opstr = "!"
	case hclsyntax.OpNegate:
		opstr = "-"
	}
	g.Fgenf(w, "%[2]v%.[1]*[3]v", precedence, opstr, expr.Operand)
}

// argumentTypeName computes the go type for the given model type.
func (g *generator) argumentTypeName(destType model.Type, isInput bool) (result string) {
	if cns, ok := destType.(*model.ConstType); ok {
		destType = cns.Type
	}

	// This can happen with null literals.
	if destType == model.NoneType {
		return ""
	}

	if schemaType, ok := pcl.GetSchemaForType(destType); ok {
		return (&pkgContext{
			pkg:              (&schema.Package{Name: "main"}).Reference(),
			externalPackages: g.externalCache,
		}).argsType(schemaType)
	}

	switch destType := destType.(type) {
	case *model.OpaqueType:
		switch *destType {
		case *model.IntType:
			if isInput {
				return "pulumi.Int"
			}
			return "int"
		case *model.NumberType:
			if isInput {
				return "pulumi.Float64"
			}
			return "float64"
		case *model.StringType:
			if isInput {
				return "pulumi.String"
			}
			return "string"
		case *model.BoolType:
			if isInput {
				return "pulumi.Bool"
			}
			return "bool"
		case *model.DynamicType:
			if isInput {
				return "pulumi.Any"
			}
			return "interface{}"
		default:
			return string(*destType)
		}
	case *model.ObjectType:

		if isInput {
			// check for element type uniformity and return appropriate type if so
			allSameType := true
			var elmType string
			for _, v := range destType.Properties {
				valType := g.argumentTypeName(v, true)
				if elmType != "" && elmType != valType {
					allSameType = false
					break
				}
				elmType = valType
			}
			if allSameType && elmType != "" {
				return elmType + "Map"
			}
			return "pulumi.Map"
		}
		return "map[string]interface{}"
	case *model.MapType:
		valType := g.argumentTypeName(destType.ElementType, isInput)
		if isInput {
			trimmedType := strings.TrimPrefix(valType, "pulumi.")
			return fmt.Sprintf("pulumi.%sMap", Title(trimmedType))
		}
		return "map[string]" + valType
	case *model.ListType:
		argTypeName := g.argumentTypeName(destType.ElementType, isInput)
		if strings.HasPrefix(argTypeName, "pulumi.") && argTypeName != "pulumi.Resource" {
			if argTypeName == "pulumi.Any" {
				return "pulumi.Array"
			}
			return argTypeName + "Array"
		}
		return "[]" + argTypeName
	case *model.TupleType:
		// attempt to collapse tuple types. intentionally does not use model.UnifyTypes
		// correct go code requires all types to match, or use of interface{}
		var elmType model.Type
		for i, t := range destType.ElementTypes {
			if i == 0 {
				elmType = t
				if cns, ok := elmType.(*model.ConstType); ok {
					elmType = cns.Type
				}
				continue
			}

			if !elmType.AssignableFrom(t) {
				elmType = nil
				break
			}
		}

		if elmType != nil {
			argTypeName := g.argumentTypeName(elmType, isInput)
			if strings.HasPrefix(argTypeName, "pulumi.") && argTypeName != "pulumi.Resource" {
				if argTypeName == "pulumi.Any" {
					return "pulumi.Array"
				}
				return argTypeName + "Array"
			}
			return "[]" + argTypeName
		}

		if isInput {
			return "pulumi.Array"
		}
		return "[]interface{}"
	case *model.OutputType:
		isInput = true
		return g.argumentTypeName(destType.ElementType, isInput)
	case *model.UnionType:
		for _, ut := range destType.ElementTypes {
			isOptional := false
			// check if the union contains none, which indicates this is an optional value
			for _, ut := range destType.ElementTypes {
				if ut.Equals(model.NoneType) {
					isOptional = true
				}
			}
			switch ut := ut.(type) {
			case *model.OpaqueType:
				if isOptional {
					return g.argumentTypeNamePtr(ut, isInput)
				}
				return g.argumentTypeName(ut, isInput)
			case *model.ConstType:
				return g.argumentTypeName(ut.Type, isInput)
			case *model.TupleType:
				return g.argumentTypeName(ut, isInput)
			case *model.MapType:
				return g.argumentTypeName(ut, isInput)
			}
		}
		return "interface{}"
	case *model.PromiseType:
		return g.argumentTypeName(destType.ElementType, isInput)
	default:
		contract.Failf("unexpected destType type %T", destType)
	}
	return ""
}

func (g *generator) argumentTypeNamePtr(destType model.Type, isInput bool) (result string) {
	res := g.argumentTypeName(destType, isInput)
	if !strings.HasPrefix(res, "pulumi.") {
		return "*" + res
	}
	return res
}

func (g *generator) genRelativeTraversal(w io.Writer,
	traversal hcl.Traversal, parts []model.Traversable, isRootResource bool,
) {
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

		// TODO handle optionals in go
		// if model.IsOptionalType(model.GetTraversableType(parts[i])) {
		// 	g.Fgen(w, "?")
		// }

		switch key.Type() {
		case cty.String:
			shouldConvert := isRootResource
			if _, ok := parts[i].(*model.OutputType); ok {
				shouldConvert = true
			}
			if key.AsString() == "id" && shouldConvert {
				g.Fgenf(w, ".ID()")
			} else {
				g.Fgenf(w, ".%s", Title(key.AsString()))
			}
		case cty.Number:
			idx, _ := key.AsBigFloat().Int64()
			g.Fgenf(w, "[%d]", idx)
		default:
			contract.Failf("unexpected traversal key of type %T (%v)", key, key.AsString())
		}
	}
}

type nameInfo int

func (nameInfo) Format(name string) string {
	// TODO
	return name
}

// lowerExpression amends the expression with intrinsics for Go generation.
func (g *generator) lowerExpression(expr model.Expression, typ model.Type) (
	model.Expression, []interface{},
) {
	expr = pcl.RewritePropertyReferences(expr)
	expr, diags := pcl.RewriteApplies(expr, nameInfo(0), false /*TODO*/)
	expr, sTemps, splatDiags := g.rewriteSplat(expr, g.splatSpiller)

	expr, convertDiags := pcl.RewriteConversions(expr, typ)
	expr, tTemps, ternDiags := g.rewriteTernaries(expr, g.ternaryTempSpiller)
	expr, jTemps, jsonDiags := g.rewriteToJSON(expr)
	expr, rTemps, readDirDiags := g.rewriteReadDir(expr, g.readDirTempSpiller)
	expr, oTemps, optDiags := g.rewriteOptionals(expr, g.optionalSpiller)

	bufferSize := len(tTemps) + len(jTemps) + len(rTemps) + len(sTemps) + len(oTemps)
	temps := slice.Prealloc[interface{}](bufferSize)
	for _, t := range tTemps {
		temps = append(temps, t)
	}
	for _, t := range jTemps {
		temps = append(temps, t)
	}
	for _, t := range rTemps {
		temps = append(temps, t)
	}
	for _, t := range sTemps {
		temps = append(temps, t)
	}
	for _, t := range oTemps {
		temps = append(temps, t)
	}
	diags = append(diags, convertDiags...)
	diags = append(diags, ternDiags...)
	diags = append(diags, jsonDiags...)
	diags = append(diags, readDirDiags...)
	diags = append(diags, splatDiags...)
	diags = append(diags, optDiags...)
	g.diagnostics = g.diagnostics.Extend(diags)
	return expr, temps
}

func (g *generator) genNYI(w io.Writer, reason string, vs ...interface{}) {
	message := "not yet implemented: " + fmt.Sprintf(reason, vs...)
	g.diagnostics = append(g.diagnostics, &hcl.Diagnostic{
		Severity: hcl.DiagWarning,
		Summary:  message,
		Detail:   message,
	})
	g.Fgenf(w, "\"TODO: %s\"", fmt.Sprintf(reason, vs...))
}

func (g *generator) genApply(w io.Writer, expr *model.FunctionCallExpression) {
	// Extract the list of outputs and the continuation expression from the `__apply` arguments.
	applyArgs, then := pcl.ParseApplyCall(expr)
	isInput := false
	retType := g.argumentTypeName(then.Signature.ReturnType, isInput)
	// TODO account for outputs in other namespaces like aws
	// TODO[pulumi/pulumi#8453] incomplete pattern code below.
	var typeAssertion string
	if retType == "[]string" {
		typeAssertion = ".(pulumi.StringArrayOutput)"
	} else {
		if strings.HasPrefix(retType, "*") {
			retType = Title(strings.TrimPrefix(retType, "*")) + "Ptr"
			switch then.Body.(type) {
			case *model.ScopeTraversalExpression:
				traversal := then.Body.(*model.ScopeTraversalExpression)
				traversal.RootName = "&" + traversal.RootName
				then.Body = traversal
			}
		}
		typeAssertion = fmt.Sprintf(".(%sOutput)", retType)
		if !strings.Contains(retType, ".") {
			typeAssertion = fmt.Sprintf(".(pulumi.%sOutput)", Title(retType))
		}
	}

	if len(applyArgs) == 1 {
		// If we only have a single output, just generate a normal `.Apply`
		g.Fgenf(w, "%.v.ApplyT(%.v)%s", applyArgs[0], then, typeAssertion)
	} else {
		g.Fgenf(w, "pulumi.All(%.v", applyArgs[0])
		applyArgs = applyArgs[1:]
		for _, a := range applyArgs {
			g.Fgenf(w, ",%.v", a)
		}
		allApplyThen, typeConvDecls := g.rewriteThenForAllApply(then)
		g.Fgenf(w, ").ApplyT(")
		g.genAnonymousFunctionExpression(w, allApplyThen, typeConvDecls, true)
		g.Fgenf(w, ")%s", typeAssertion)
	}
}

// rewriteThenForAllApply rewrites an apply func after a .All replacing params with []interface{}
// other languages like javascript take advantage of destructuring to simplify All.Apply
// by generating something like [a1, a2, a3]
// Go doesn't support this syntax so we create a set of var decls with type assertions
// to prepend to the body: a1 := _args[0].(string) ... etc.
func (g *generator) rewriteThenForAllApply(
	then *model.AnonymousFunctionExpression,
) (*model.AnonymousFunctionExpression, []string) {
	typeConvDecls := slice.Prealloc[string](len(then.Parameters))
	for i, v := range then.Parameters {
		typ := g.argumentTypeName(v.VariableType, false)
		decl := fmt.Sprintf("%s := _args[%d].(%s)", v.Name, i, typ)
		typeConvDecls = append(typeConvDecls, decl)
	}

	// dummy type that will produce []interface{} for argumentTypeName
	interfaceArrayType := &model.TupleType{
		ElementTypes: []model.Type{
			model.BoolType, model.StringType, model.IntType,
		},
	}

	then.Parameters = []*model.Variable{{
		Name:         "_args",
		VariableType: interfaceArrayType,
	}}
	then.Signature.Parameters = []model.Parameter{{
		Name: "_args",
		Type: interfaceArrayType,
	}}

	return then, typeConvDecls
}

// Writes a Go string literal.
// The literal will be a raw string literal if allowRaw is true
// and the string is long enough to benefit from it.
func (g *generator) genStringLiteral(w io.Writer, v string, allowRaw bool) {
	// If the string is longer than 50 characters,
	// contains at least 5 newlines,
	// and does not contain a backtick,
	// use a backtick string literal for readability.
	canBeRaw := len(v) > 50 &&
		strings.Count(v, "\n") >= 5 &&
		!strings.Contains(v, "`")
	if allowRaw && canBeRaw {
		fmt.Fprintf(w, "`%s`", v)
		return
	}

	g.Fgen(w, "\"")
	g.Fgen(w, g.escapeString(v))
	g.Fgen(w, "\"")
}

func (g *generator) escapeString(v string) string {
	builder := strings.Builder{}
	for _, c := range v {
		if c == '\x00' {
			// escape NUL bytes
			builder.WriteString(fmt.Sprintf("\\u%04x", c))
			continue
		}
		if c == '"' || c == '\\' {
			builder.WriteRune('\\')
		}
		if c == '\n' {
			builder.WriteRune('\\')
			builder.WriteRune('n')
			continue
		}
		builder.WriteRune(c)
	}
	return builder.String()
}

//nolint:lll
func isInputty(destType model.Type) bool {
	// TODO this needs to be more robust, likely the inverse of:
	// https://github.com/pulumi/pulumi/blob/5330c97684cad78bcc60d8867f1b28704bd8a555/pkg/codegen/hcl2/model/type_eventuals.go#L244
	switch destType := destType.(type) {
	case *model.UnionType:
		for _, t := range destType.ElementTypes {
			if _, ok := t.(*model.OutputType); ok {
				return true
			}
		}
	case *model.OutputType:
		return true
	}
	return false
}

func (g *generator) literalKey(x model.Expression) (string, bool) {
	var strKey string
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
		return "", false
	default:
		return "", false
	}

	return strKey, true
}

// functionName computes the go package, module, and name for the given function token.
func (g *generator) functionName(tokenArg model.Expression) (string, string, string, hcl.Diagnostics) {
	token := tokenArg.(*model.TemplateExpression).Parts[0].(*model.LiteralValueExpression).Value.AsString()
	tokenRange := tokenArg.SyntaxNode().Range()

	// Compute the resource type from the Pulumi type token.
	pkg, module, member, diagnostics := pcl.DecomposeToken(token, tokenRange)
	if strings.HasPrefix(member, "get") {
		if g.useLookupInvokeForm(token) {
			member = strings.Replace(member, "get", "lookup", 1)
		}
	}
	modOrAlias := g.getModOrAlias(pkg, module, module)
	mod := strings.ReplaceAll(modOrAlias, "/", ".")
	return goPackage(pkg), mod, Title(member), diagnostics
}

var functionPackages = map[string][]string{
	"join":             {"strings"},
	"mimeType":         {"mime", "path"},
	"readDir":          {"os"},
	"readFile":         {"os"},
	"filebase64":       {"encoding/base64", "os"},
	"toBase64":         {"encoding/base64"},
	"fromBase64":       {"encoding/base64"},
	"toJSON":           {"encoding/json"},
	"sha1":             {"crypto/sha1", "encoding/hex"},
	"filebase64sha256": {"crypto/sha256", "os"},
	"cwd":              {"os"},
	"singleOrNone":     {"fmt"},
}

func (g *generator) genFunctionPackages(x *model.FunctionCallExpression) []string {
	return functionPackages[x.Name]
}
