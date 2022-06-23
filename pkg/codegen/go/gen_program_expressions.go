package gen

import (
	"bytes"
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
		g.Fgenf(w, "%s%s %s", leadingSep, makeValidIdentifier(param.Name), g.argumentTypeName(nil, param.Type, isInput))
		leadingSep = ", "
	}

	retType := expr.Signature.ReturnType
	if inApply {
		retType = model.ResolveOutputs(retType)
	}

	retTypeName := g.argumentTypeName(nil, retType, false)
	g.Fgenf(w, ") (%s, error) {\n", retTypeName)

	for _, decl := range bodyPreamble {
		g.Fgenf(w, "%s\n", decl)
	}

	body, temps := g.lowerExpression(expr.Body, retType)
	g.genTempsMultiReturn(w, temps, retTypeName)

	g.Fgenf(w, "return %v, nil", body)
	g.Fgenf(w, "\n}")
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
	// Ternary expressions are not supported in go so we need to allocate temp variables in the parent scope.
	// This is handled by lower expression and rewriteTernaries
	contract.Failf("unlowered conditional expression @ %v", expr.SyntaxNode().Range())
}

// GenForExpression generates code for a ForExpression.
func (g *generator) GenForExpression(w io.Writer, expr *model.ForExpression) {
	g.genNYI(w, "For expression")
}

func (g *generator) genSafeEnum(w io.Writer, to *model.EnumType) func(member *schema.Enum) {
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
		namespace := tokenToModule(to.Token)
		g.Fgenf(w, "%s.%s", namespace, memberTag)
	}
}

func (g *generator) GenFunctionCallExpression(w io.Writer, expr *model.FunctionCallExpression) {
	//nolint:goconst
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
			var underlyingType string
			switch {
			case to.Type.Equals(model.StringType):
				underlyingType = "string"
			default:
				panic(fmt.Sprintf(
					"Unsafe enum conversions from type %s not implemented yet: %s => %s",
					from.Type(), from, to))
			}
			enumTag := fmt.Sprintf("%s.%s",
				tokenToModule(to.Token), tokenToName(to.Token))
			if isOutput {
				g.Fgenf(w,
					"%.v.ApplyT(func(x *%[3]s) %[2]s { return %[2]s(*x) }).(%[2]sOutput)",
					from, enumTag, underlyingType)
				return
			}
			pcl.GenEnum(to, from, g.genSafeEnum(w, to), func(from model.Expression) {
				g.Fgenf(w, "%s(%v)", enumTag, from)
			})
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
			g.Fgenf(w, "%.v", expr.Args[0])
		}
	case pcl.IntrinsicApply:
		g.genApply(w, expr)
	case "element":
		g.genNYI(w, "element")
	case "entries":
		g.genNYI(w, "call %v", expr.Name)
		// switch model.ResolveOutputs(expr.Args[0].Type()).(type) {
		// case *model.ListType, *model.TupleType:
		// 	if call, ok := expr.Args[0].(*model.FunctionCallExpression); ok && call.Name == "range" {
		// 		g.genRange(w, call, true)
		// 		return
		// 	}
		// 	g.Fgenf(w, "%.20v.Select((v, k)", expr.Args[0])
		// case *model.MapType, *model.ObjectType:
		// 	g.genNYI(w, "MapOrObjectEntries")
		// }
		// g.Fgenf(w, " => new { Key = k, Value = v })")
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
	case pcl.Invoke:
		pkg, module, fn, diags := g.functionName(expr.Args[0])
		contract.Assert(len(diags) == 0)
		if module == "" {
			module = pkg
		}
		isOut, outArgs, outArgsType := pcl.RecognizeOutputVersionedInvoke(expr)
		if isOut {
			outTypeName, err := outputVersionFunctionArgTypeName(outArgsType)
			if err != nil {
				panic(fmt.Errorf("Error when generating an output-versioned Invoke: %w", err))
			}
			g.Fgenf(w, "%s.%sOutput(ctx, ", module, fn)
			g.genObjectConsExpressionWithTypeName(w, outArgs, outArgsType, outTypeName)
		} else {
			g.Fgenf(w, "%s.%s(ctx, ", module, fn)
			g.Fgenf(w, "%.v", expr.Args[1])
		}

		optionsBag := ""
		var buf bytes.Buffer
		if len(expr.Args) == 3 {
			g.Fgenf(&buf, ", %.v", expr.Args[2])
		} else {
			g.Fgenf(&buf, ", nil")
		}
		optionsBag = buf.String()
		g.Fgenf(w, "%v)", optionsBag)
	case "join":
		g.Fgenf(w, "strings.Join(%v, %v)", expr.Args[1], expr.Args[0])
	case "length":
		g.genNYI(w, "call %v", expr.Name)
		// g.Fgenf(w, "%.20v.Length", expr.Args[0])
	case "lookup":
		g.genNYI(w, "Lookup")
	case keywordRange:
		g.genNYI(w, "call %v", expr.Name)
		// g.genRange(w, expr, false)
	case "readFile":
		// Assuming the existence of the following helper method located earlier in the preamble
		g.Fgenf(w, "readFileOrPanic(%v)", expr.Args[0])
	case "readDir":
		contract.Failf("unlowered readDir function expression @ %v", expr.SyntaxNode().Range())
	case "secret":
		outputTypeName := "pulumi.Any"
		if model.ResolveOutputs(expr.Type()) != model.DynamicType {
			outputTypeName = g.argumentTypeName(nil, expr.Type(), false)
		}
		g.Fgenf(w, "pulumi.ToSecret(%v).(%sOutput)", expr.Args[0], outputTypeName)
	case "split":
		g.genNYI(w, "call %v", expr.Name)
		// g.Fgenf(w, "%.20v.Split(%v)", expr.Args[1], expr.Args[0])
	case "toBase64":
		g.Fgenf(w, "base64.StdEncoding.EncodeToString([]byte(%v))", expr.Args[0])
	case "toJSON":
		contract.Failf("unlowered toJSON function expression @ %v", expr.SyntaxNode().Range())
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
	case "cwd":
		g.Fgen(w, "func(cwd string, err error) string { if err != nil { panic(err) }; return cwd }(os.Getwd())")
	default:
		g.genNYI(w, "call %v", expr.Name)
	}
}

// Currently args type for output-versioned invokes are named
// `FOutputArgs`, but this is not yet understood by `tokenToType`. Use
// this function to compensate.
func outputVersionFunctionArgTypeName(t model.Type) (string, error) {
	schemaType, ok := pcl.GetSchemaForType(t)
	if !ok {
		return "", fmt.Errorf("No schema.Type type found for the given model.Type")
	}

	objType, ok := schemaType.(*schema.ObjectType)
	if !ok {
		return "", fmt.Errorf("Expected a schema.ObjectType, got %s", schemaType.String())
	}

	pkg := &pkgContext{pkg: &schema.Package{Name: "main"}}

	var ty string
	if pkg.isExternalReference(objType) {
		extPkg, _ := pkg.contextForExternalReference(objType)
		ty = extPkg.tokenToType(objType.Token)
	} else {
		ty = pkg.tokenToType(objType.Token)
	}

	return fmt.Sprintf("%sOutputArgs", strings.TrimSuffix(ty, "Args")), nil
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

	argTypeName := g.argumentTypeName(expr, destType, false)
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
			g.genStringLiteral(w, strVal)
			g.Fgenf(w, ")")
		} else {
			g.genStringLiteral(w, strVal)
		}
	default:
		contract.Failf("unexpected opaque type in GenLiteralValueExpression: %v (%v)", destType,
			expr.SyntaxNode().Range())
	}
}

func (g *generator) GenObjectConsExpression(w io.Writer, expr *model.ObjectConsExpression) {
	isInput := false
	g.genObjectConsExpression(w, expr, expr.Type(), isInput)
}

func (g *generator) genObjectConsExpression(
	w io.Writer,
	expr *model.ObjectConsExpression,
	destType model.Type,
	isInput bool) {

	isInput = isInput || isInputty(destType)

	typeName := g.argumentTypeName(expr, destType, isInput)
	if schemaType, ok := pcl.GetSchemaForType(destType); ok {
		if obj, ok := codegen.UnwrapType(schemaType).(*schema.ObjectType); ok {
			if g.useLookupInvokeForm(obj.Token) {
				typeName = strings.Replace(typeName, ".Get", ".Lookup", 1)
			}
		}
	}

	g.genObjectConsExpressionWithTypeName(w, expr, destType, typeName)
}

func (g *generator) genObjectConsExpressionWithTypeName(
	w io.Writer,
	expr *model.ObjectConsExpression,
	destType model.Type,
	typeName string) {

	if len(expr.Items) == 0 {
		g.Fgenf(w, "nil")
		return
	}

	var temps []interface{}
	// TODO: @pgavlin --- ineffectual assignment, was there some work in flight here?
	// if strings.HasSuffix(typeName, "Args") {
	// 	isInput = true
	// }
	// // invokes are not inputty
	// if strings.Contains(typeName, ".Lookup") || strings.Contains(typeName, ".Get") {
	// 	isInput = false
	// }
	isMap := strings.HasPrefix(typeName, "map[")

	// TODO: retrieve schema and propagate optionals to emit bool ptr, etc.

	// first lower all inner expressions and emit temps
	for i, item := range expr.Items {
		// don't treat keys as inputs
		//nolint: revive
		k, kTemps := g.lowerExpression(item.Key, item.Key.Type())
		temps = append(temps, kTemps...)
		item.Key = k
		x, xTemps := g.lowerExpression(item.Value, item.Value.Type())
		temps = append(temps, xTemps...)
		item.Value = x
		expr.Items[i] = item
	}
	g.genTemps(w, temps)

	if isMap || !strings.HasSuffix(typeName, "Args") || strings.HasSuffix(typeName, "OutputArgs") {
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

func (g *generator) genRelativeTraversalExpression(
	w io.Writer, expr *model.RelativeTraversalExpression, isInput bool) {

	if _, ok := expr.Parts[0].(*model.PromiseType); ok {
		isInput = false
	}
	if _, ok := expr.Parts[0].(*pcl.Resource); ok {
		isInput = false
	}
	if isInput {
		g.Fgenf(w, "%s(", g.argumentTypeName(expr, expr.Type(), isInput))
	}
	g.GenRelativeTraversalExpression(w, expr)
	if isInput {
		g.Fgenf(w, ")")
	}
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
	w io.Writer, expr *model.ScopeTraversalExpression, destType model.Type) {
	rootName := expr.RootName

	if _, ok := expr.Parts[0].(*model.SplatVariable); ok {
		rootName = "val0"
	}

	genIDCall := false

	isInput := false
	if schemaType, ok := pcl.GetSchemaForType(destType); ok {
		_, isInput = schemaType.(*schema.InputType)
	}

	if resource, ok := expr.Parts[0].(*pcl.Resource); ok {
		isInput = false
		if _, ok := pcl.GetSchemaForType(resource.InputType); ok {
			// convert .id into .ID()
			last := expr.Traversal[len(expr.Traversal)-1]
			if attr, ok := last.(hcl.TraverseAttr); ok && attr.Name == "id" {
				genIDCall = true
				expr.Traversal = expr.Traversal[:len(expr.Traversal)-1]
			}
		}
	}

	// TODO if it's an array type, we need a lowering step to turn []string -> pulumi.StringArray
	if isInput {
		argType := g.argumentTypeName(expr, expr.Type(), isInput)
		if strings.HasSuffix(argType, "Array") {
			destTypeName := g.argumentTypeName(expr, destType, isInput)
			if argType != destTypeName {
				// use a helper to transform prompt arrays into inputty arrays
				var helper *promptToInputArrayHelper
				if h, ok := g.arrayHelpers[argType]; ok {
					helper = h
				} else {
					// helpers are emitted at the end in the postamble step
					helper = &promptToInputArrayHelper{
						destType: argType,
					}
					g.arrayHelpers[argType] = helper
				}
				g.Fgenf(w, "%s(", helper.getFnName())
				defer g.Fgenf(w, ")")
			}
		} else {
			g.Fgenf(w, "%s(", g.argumentTypeName(expr, expr.Type(), isInput))
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
	argTypeName := g.argumentTypeName(expr, destType, false)
	isPulumiType := strings.HasPrefix(argTypeName, "pulumi.")
	if isPulumiType {
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
	g.Fgenf(w, "fmt.Sprintf(")
	str := fmtStr.String()
	if canBeRaw && len(str) > 50 && strings.Count(str, "\n") > 5 {
		fmt.Fprintf(w, "`%s`", str)
	} else {
		g.genStringLiteral(w, fmtStr.String())
	}
	_, err := args.WriteTo(w)
	contract.AssertNoError(err)
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

	var temps []interface{}
	for i, item := range expr.Expressions {
		item, itemTemps := g.lowerExpression(item, item.Type())
		temps = append(temps, itemTemps...)
		expr.Expressions[i] = item
	}
	g.genTemps(w, temps)
	argType := g.argumentTypeName(expr, destType, isInput)
	g.Fgenf(w, "%s{\n", argType)
	switch len(expr.Expressions) {
	case 0:
		// empty array
		break
	default:
		for _, v := range expr.Expressions {
			g.Fgenf(w, "%v,\n", v)
		}
	}
	g.Fgenf(w, "}")
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

var typeNameID = 0

// argumentTypeName computes the go type for the given expression and model type.
func (g *generator) argumentTypeName(expr model.Expression, destType model.Type, isInput bool) (result string) {
	//	defer func(id int, t model.Type) {
	//		schemaType, _ := pcl.GetSchemaForType(destType)
	//		log.Printf("%v: argumentTypeName(%v, %v, %v) = %v", id, t, isInput, schemaType, result)
	//	}(typeNameID, destType)
	typeNameID++

	if cns, ok := destType.(*model.ConstType); ok {
		destType = cns.Type
	}

	// This can happen with null literals.
	if destType == model.NoneType {
		return ""
	}

	if schemaType, ok := pcl.GetSchemaForType(destType); ok {
		pkg := &pkgContext{pkg: &schema.Package{Name: "main"}}
		return pkg.argsType(schemaType)
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
				valType := g.argumentTypeName(nil, v, true)
				if elmType != "" && elmType != valType {
					allSameType = false
					break
				}
				elmType = valType
			}
			if allSameType && elmType != "" {
				return fmt.Sprintf("%sMap", elmType)
			}
			return "pulumi.Map"
		}
		return "map[string]interface{}"
	case *model.MapType:
		valType := g.argumentTypeName(nil, destType.ElementType, isInput)
		if isInput {
			return fmt.Sprintf("pulumi.%sMap", Title(valType))
		}
		return fmt.Sprintf("map[string]%s", valType)
	case *model.ListType:
		argTypeName := g.argumentTypeName(nil, destType.ElementType, isInput)
		if strings.HasPrefix(argTypeName, "pulumi.") && argTypeName != "pulumi.Resource" {
			return fmt.Sprintf("%sArray", argTypeName)
		}
		return fmt.Sprintf("[]%s", argTypeName)
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
			argTypeName := g.argumentTypeName(nil, elmType, isInput)
			if strings.HasPrefix(argTypeName, "pulumi.") && argTypeName != "pulumi.Resource" {
				return fmt.Sprintf("%sArray", argTypeName)
			}
			return fmt.Sprintf("[]%s", argTypeName)
		}

		if isInput {
			return "pulumi.Array"
		}
		return "[]interface{}"
	case *model.OutputType:
		isInput = true
		return g.argumentTypeName(expr, destType.ElementType, isInput)
	case *model.UnionType:
		for _, ut := range destType.ElementTypes {
			switch ut := ut.(type) {
			case *model.OpaqueType:
				return g.argumentTypeName(expr, ut, isInput)
			case *model.ConstType:
				return g.argumentTypeName(expr, ut.Type, isInput)
			case *model.TupleType:
				return g.argumentTypeName(expr, ut, isInput)
			}
		}
		return "interface{}"
	case *model.PromiseType:
		return g.argumentTypeName(expr, destType.ElementType, isInput)
	default:
		contract.Failf("unexpected destType type %T", destType)
	}
	return ""
}

func (g *generator) genRelativeTraversal(w io.Writer,
	traversal hcl.Traversal, parts []model.Traversable, isRootResource bool) {

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
	model.Expression, []interface{}) {
	expr = pcl.RewritePropertyReferences(expr)
	expr, diags := pcl.RewriteApplies(expr, nameInfo(0), false /*TODO*/)
	expr = pcl.RewriteConversions(expr, typ)
	expr, tTemps, ternDiags := g.rewriteTernaries(expr, g.ternaryTempSpiller)
	expr, jTemps, jsonDiags := g.rewriteToJSON(expr)
	expr, rTemps, readDirDiags := g.rewriteReadDir(expr, g.readDirTempSpiller)
	expr, sTemps, splatDiags := g.rewriteSplat(expr, g.splatSpiller)
	expr, oTemps, optDiags := g.rewriteOptionals(expr, g.optionalSpiller)

	var temps []interface{}
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
	diags = append(diags, ternDiags...)
	diags = append(diags, jsonDiags...)
	diags = append(diags, readDirDiags...)
	diags = append(diags, splatDiags...)
	diags = append(diags, optDiags...)
	contract.Assert(len(diags) == 0)
	return expr, temps
}

func (g *generator) genNYI(w io.Writer, reason string, vs ...interface{}) {
	message := fmt.Sprintf("not yet implemented: %s", fmt.Sprintf(reason, vs...))
	g.diagnostics = append(g.diagnostics, &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  message,
		Detail:   message,
	})
	g.Fgenf(w, "\"TODO: %s\"", fmt.Sprintf(reason, vs...))
}

func (g *generator) genApply(w io.Writer, expr *model.FunctionCallExpression) {
	// Extract the list of outputs and the continuation expression from the `__apply` arguments.
	applyArgs, then := pcl.ParseApplyCall(expr)
	isInput := false
	retType := g.argumentTypeName(nil, then.Signature.ReturnType, isInput)
	// TODO account for outputs in other namespaces like aws
	// TODO[pulumi/pulumi#8453] incomplete pattern code below.
	var typeAssertion string
	if retType == "[]string" {
		typeAssertion = ".(pulumi.StringArrayOutput)"
	} else {
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
	var typeConvDecls []string
	for i, v := range then.Parameters {
		typ := g.argumentTypeName(nil, v.VariableType, false)
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

func (g *generator) genStringLiteral(w io.Writer, v string) {
	g.Fgen(w, "\"")
	g.Fgen(w, g.escapeString(v))
	g.Fgen(w, "\"")
}

func (g *generator) escapeString(v string) string {
	builder := strings.Builder{}
	for _, c := range v {
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

// nolint: lll
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
	modOrAlias := g.getModOrAlias(pkg, module)
	mod := strings.Replace(modOrAlias, "/", ".", -1)
	return pkg, mod, Title(member), diagnostics
}

var functionPackages = map[string][]string{
	"join":             {"strings"},
	"mimeType":         {"mime", "path"},
	"readDir":          {"io/ioutil"},
	"readFile":         {"io/ioutil"},
	"filebase64":       {"io/ioutil", "encoding/base64"},
	"toBase64":         {"encoding/base64"},
	"toJSON":           {"encoding/json"},
	"sha1":             {"fmt", "crypto/sha1"},
	"filebase64sha256": {"fmt", "io/ioutil", "crypto/sha256"},
	"cwd":              {"os"},
}

func (g *generator) genFunctionPackages(x *model.FunctionCallExpression) []string {
	return functionPackages[x.Name]
}
