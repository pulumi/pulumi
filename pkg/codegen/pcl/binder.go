// Copyright 2016, Pulumi Corporation.
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

package pcl

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"

	"github.com/blang/semver"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/maputil"
	"github.com/zclconf/go-cty/cty"
)

const (
	pulumiPackage          = "pulumi"
	LogicalNamePropertyKey = "__logicalName"
)

type ComponentProgramBinderArgs struct {
	AllowMissingVariables        bool
	AllowMissingProperties       bool
	SkipResourceTypecheck        bool
	SkipInvokeTypecheck          bool
	SkipRangeTypecheck           bool
	PreferOutputVersionedInvokes bool
	BinderDirPath                string
	BinderLoader                 schema.Loader
	ComponentSource              string
	ComponentNodeRange           hcl.Range
	PackageCache                 *PackageCache
}

type ComponentProgramBinder = func(ComponentProgramBinderArgs) (*Program, hcl.Diagnostics, error)

type bindOptions struct {
	allowMissingVariables        bool
	allowMissingProperties       bool
	skipResourceTypecheck        bool
	skipInvokeTypecheck          bool
	skipRangeTypecheck           bool
	preferOutputVersionedInvokes bool
	loader                       schema.Loader
	packageCache                 *PackageCache
	// the directory path of the PCL program being bound
	// we use this to locate the source of the component blocks
	// which refer to a component resource in a relative directory
	dirPath                string
	componentProgramBinder ComponentProgramBinder
	// extraScopeVariables, if non-empty, are additional variables to define in the binder's root scope before
	// binding the input file. Used by snippet bindings to inject references to resources owned by another source.
	extraScopeVariables map[string]*model.Variable
	// extraPackageDescriptors, if non-empty, are package descriptors supplied by the caller rather than read from
	// `package { ... }` blocks in the source. Used by snippet bindings, which carry the descriptor structurally on
	// the Snippet record rather than as PCL syntax. Merged into the descriptor map read from files; same-key entries
	// from this option take precedence.
	extraPackageDescriptors map[string]*schema.PackageDescriptor
}

func (opts bindOptions) modelOptions() []model.BindOption {
	var options []model.BindOption
	if opts.allowMissingVariables {
		options = append(options, model.AllowMissingVariables)
	}
	if opts.skipRangeTypecheck {
		options = append(options, model.SkipRangeTypechecking)
	}
	return options
}

type binder struct {
	options bindOptions

	packageDescriptors map[string]*schema.PackageDescriptor
	referencedPackages map[string]schema.PackageReference
	schemaTypes        map[schema.Type]model.Type

	tokens syntax.TokenMap
	nodes  []Node
	root   *model.Scope
}

type BindOption func(*bindOptions)

func AllowMissingVariables(options *bindOptions) {
	options.allowMissingVariables = true
}

func AllowMissingProperties(options *bindOptions) {
	options.allowMissingProperties = true
}

func SkipResourceTypechecking(options *bindOptions) {
	options.skipResourceTypecheck = true
}

func SkipRangeTypechecking(options *bindOptions) {
	options.skipRangeTypecheck = true
}

func PreferOutputVersionedInvokes(options *bindOptions) {
	options.preferOutputVersionedInvokes = true
}

func SkipInvokeTypechecking(options *bindOptions) {
	options.skipInvokeTypecheck = true
}

func PluginHost(pctx *plugin.Context) BindOption {
	return Loader(schema.NewPluginLoader(pctx))
}

func Loader(loader schema.Loader) BindOption {
	return func(options *bindOptions) {
		options.loader = loader
	}
}

func Cache(cache *PackageCache) BindOption {
	return func(options *bindOptions) {
		options.packageCache = cache
	}
}

func DirPath(path string) BindOption {
	return func(options *bindOptions) {
		options.dirPath = path
	}
}

func ComponentBinder(binder ComponentProgramBinder) BindOption {
	return func(options *bindOptions) {
		options.componentProgramBinder = binder
	}
}

// ExtraScopeVariables returns a BindOption that defines additional variables in the binder's root scope before
// the input file is bound. Used by snippet bindings to inject references to resources owned by another source so
// expressions like `someResource.someProp` can typecheck without the binder seeing a `resource` block for them.
func ExtraScopeVariables(extras map[string]*model.Variable) BindOption {
	return func(options *bindOptions) {
		options.extraScopeVariables = extras
	}
}

// PackageDescriptors returns a BindOption that supplies pre-built package descriptors to BindProgram, as if they
// were declared by `package { ... }` blocks in the source. Used by snippet bindings, which carry the descriptor
// structurally on the Snippet record rather than as PCL syntax.
func PackageDescriptors(descriptors map[string]*schema.PackageDescriptor) BindOption {
	return func(options *bindOptions) {
		options.extraPackageDescriptors = descriptors
	}
}

// NonStrictBindOptions returns a set of bind options that make the binder lenient about type checking.
// Changing errors into warnings when possible
func NonStrictBindOptions() []BindOption {
	return []BindOption{
		AllowMissingVariables,
		AllowMissingProperties,
		SkipResourceTypechecking,
		SkipInvokeTypechecking,
		SkipRangeTypechecking,
	}
}

// bindInputFile is the binder setup shared by BindFunction and BindResource: it constructs a binder, registers the
// standard PCL builtins, walks the file's top-level attributes, and returns the bound arguments along with each
// input's name range (for diagnostic Subjects).
func bindInputFile(file *syntax.File, opts ...BindOption) (
	*binder, []*model.Attribute, map[string]hcl.Range, hcl.Diagnostics,
) {
	var options bindOptions
	for _, o := range opts {
		o(&options)
	}

	b := &binder{
		options:            options,
		tokens:             syntax.NewTokenMapForFiles([]*syntax.File{file}),
		packageDescriptors: map[string]*schema.PackageDescriptor{},
		referencedPackages: map[string]schema.PackageReference{},
		schemaTypes:        map[schema.Type]model.Type{},
		root:               model.NewRootScope(syntax.None),
	}

	b.root.Define("null", &model.Constant{
		Name:          "null",
		ConstantValue: cty.NullVal(cty.DynamicPseudoType),
	})
	for name, fn := range pulumiBuiltins(options) {
		b.root.DefineFunction(name, fn)
	}
	b.root.DefineFunction(Invoke, model.NewFunction(model.GenericFunctionSignature(b.bindInvokeSignature)))
	b.root.DefineFunction(Call, model.NewFunction(model.GenericFunctionSignature(b.bindCallSignature)))

	var diagnostics hcl.Diagnostics
	args := make([]*model.Attribute, 0, len(file.Body.Attributes))
	inputRanges := map[string]hcl.Range{}
	for name, value := range file.Body.Attributes {
		expr, diags := model.BindExpression(value.Expr, b.root, b.tokens, options.modelOptions()...)
		diagnostics = append(diagnostics, diags...)
		inputRanges[name] = value.NameRange
		args = append(args, &model.Attribute{
			Syntax: value,
			Tokens: syntax.NewAttributeTokens(name),
			Name:   name,
			Value:  expr,
		})
	}

	for _, block := range file.Body.Blocks {
		diagnostics = append(diagnostics, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("unexpected block %q", block.Type),
			Subject:  &block.TypeRange,
		})
	}

	return b, args, inputRanges, diagnostics
}

// BindFunction binds a PCL file as an invoke function input and returns the bound arguments along with the model
// type the inputs were typechecked against. The model type is used downstream (e.g. by RewriteConversions during
// evaluation) so that conversions reference the same type instances the binder built.
func BindFunction(
	file *syntax.File, fn *schema.Function,
	opts ...BindOption,
) ([]*model.Attribute, model.Type, hcl.Diagnostics) {
	b, args, inputRanges, diagnostics := bindInputFile(file, opts...)

	argProperties := make(map[string]model.Type, len(args))
	for _, item := range args {
		argProperties[item.Name] = item.Value.Type()
	}
	argsType := model.NewObjectType(argProperties)

	sig, err := b.signatureForArgs(fn, argsType)
	if err != nil {
		diagnostics = append(diagnostics, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("invalid function arguments: %v", err),
		})
		return nil, nil, diagnostics
	}

	contract.Assertf(
		len(sig.Parameters) == 3,
		"expected signature to have exactly three parameters, got %d", len(sig.Parameters))
	inputType := sig.Parameters[1].Type
	diagnostics = append(diagnostics,
		typecheckObjectArgs(inputType, file.Body.Range().Ptr(), args, inputRanges)...)

	if diagnostics.HasErrors() {
		return nil, nil, diagnostics
	}

	return args, inputType, diagnostics
}

// BindResource binds a PCL file as a resource input and returns the bound arguments along with the model type the
// inputs were typechecked against. The model type is used downstream (e.g. by RewriteConversions during evaluation)
// so that conversions reference the same type instances the binder built.
func BindResource(
	file *syntax.File, res *schema.Resource,
	opts ...BindOption,
) ([]*model.Attribute, model.Type, hcl.Diagnostics) {
	b, args, inputRanges, diagnostics := bindInputFile(file, opts...)

	// resolveInputUnions expects a name → expression map; rebuild it from args rather than tracking the same thing
	// twice during attribute binding.
	inputs := make(map[string]model.Expression, len(args))
	for _, item := range args {
		inputs[item.Name] = item.Value
	}
	inputProperties := b.resolveInputUnions(inputs, res.InputProperties)
	inputType := b.schemaTypeToType(&schema.ObjectType{Properties: inputProperties})

	diagnostics = append(diagnostics,
		typecheckObjectArgs(inputType, file.Body.Range().Ptr(), args, inputRanges)...)

	if diagnostics.HasErrors() {
		return nil, nil, diagnostics
	}

	return args, inputType, diagnostics
}

// BindResourceProgram binds a PCL file body as a single resource program. Unlike BindResource,
// this binds the full resource shape, including options and range, so the resulting program can be
// evaluated through the normal resource registration path.
func BindResourceProgram(
	file *syntax.File, name, token string,
	opts ...BindOption,
) (*Program, hcl.Diagnostics, error) {
	bodyRange := file.Body.Range()
	labelRange := hcl.Range{
		Filename: bodyRange.Filename,
		Start:    bodyRange.Start,
		End:      bodyRange.Start,
	}
	block := &hclsyntax.Block{
		Type:        "resource",
		Labels:      []string{name, token},
		Body:        file.Body,
		TypeRange:   labelRange,
		LabelRanges: []hcl.Range{labelRange, labelRange},
		OpenBraceRange: hcl.Range{
			Filename: bodyRange.Filename,
			Start:    bodyRange.Start,
			End:      bodyRange.Start,
		},
		CloseBraceRange: hcl.Range{
			Filename: bodyRange.Filename,
			Start:    bodyRange.End,
			End:      bodyRange.End,
		},
	}
	resourceFile := &syntax.File{
		Name: file.Name,
		Body: &hclsyntax.Body{
			Blocks:   []*hclsyntax.Block{block},
			SrcRange: bodyRange,
			EndRange: bodyRange,
		},
		Bytes:  file.Bytes,
		Tokens: file.Tokens,
	}
	return BindProgram([]*syntax.File{resourceFile}, opts...)
}

// BindResourceList binds a PCL file as a resource list input and returns the bound arguments. This is used for `do` to
// type check and evaluate resource list inputs.
func BindResourceList(
	file *syntax.File, res *schema.Resource,
	opts ...BindOption,
) ([]*model.Attribute, model.Type, hcl.Diagnostics) {
	b, args, inputRanges, diagnostics := bindInputFile(file, opts...)

	if res.ListInputs == nil {
		diagnostics = append(diagnostics, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("resource %s does not support list", res.Token),
		})
		return nil, nil, diagnostics
	}

	inputs := make(map[string]model.Expression, len(args))
	for _, item := range args {
		inputs[item.Name] = item.Value
	}
	inputProperties := b.resolveInputUnions(inputs, res.ListInputs.Properties)
	inputType := b.schemaTypeToType(&schema.ObjectType{Properties: inputProperties})

	diagnostics = append(diagnostics,
		typecheckObjectArgs(inputType, file.Body.Range().Ptr(), args, inputRanges)...)

	if diagnostics.HasErrors() {
		return nil, nil, diagnostics
	}

	return args, inputType, diagnostics
}

func typecheckObjectArgs(
	inputType model.Type,
	rng *hcl.Range,
	args []*model.Attribute,
	inputRanges map[string]hcl.Range,
) hcl.Diagnostics {
	// Function signatures wrap input-less argument objects in Optional (Union[Object{}, None]); strip that so we can
	// still report unsupported attributes against the underlying ObjectType.
	objectType, ok := unwrapOptionalType(inputType).(*model.ObjectType)
	if !ok {
		return nil
	}

	var diagnostics hcl.Diagnostics
	attrNames := map[string]struct{}{}
	for _, item := range args {
		attrNames[item.Name] = struct{}{}

		expected, ok := objectType.Properties[item.Name]
		if !ok {
			diagnostics = append(diagnostics, unsupportedAttribute(item.Name, inputRanges[item.Name]))
			continue
		}
		if !expected.ConversionFrom(item.Value.Type()).Exists() {
			valueRange := item.Value.SyntaxNode().Range()
			diagnostics = append(diagnostics, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Subject:  &valueRange,
				Summary:  fmt.Sprintf("Cannot assign value to input %q", item.Name),
				Detail: fmt.Sprintf("Cannot assign value %s to input %q of type %s",
					item.Value.Type().Pretty().String(), item.Name, expected.Pretty().String()),
			})
		}
	}

	for _, name := range maputil.SortedKeys(objectType.Properties) {
		expected := objectType.Properties[name]
		_, hasAttribute := attrNames[name]
		if model.IsOptionalType(expected) || hasAttribute {
			continue
		}
		if model.IsConstType(expected) {
			continue
		}
		diagnostics = append(diagnostics, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Subject:  rng,
			Summary:  fmt.Sprintf("Missing required input %q", name),
		})
	}

	return diagnostics
}

// BindProgram performs semantic analysis on the given set of HCL2 files that represent a single program. The given
// host, if any, is used for loading any resource plugins necessary to extract schema information.
func BindProgram(files []*syntax.File, opts ...BindOption) (*Program, hcl.Diagnostics, error) {
	ctx := context.TODO()
	var options bindOptions
	for _, o := range opts {
		o(&options)
	}

	if options.loader == nil {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, nil, err
		}
		pluginHost, err := pkghost.New(
			context.WithoutCancel(ctx), nil, nil, nil, pkgWorkspace.EnsureLanguageInstalled,
			schema.NewLoaderServerFromContext, convert.NewMapperServerFromContext)
		if err != nil {
			return nil, nil, err
		}
		// The host is owned here, not by the context; the deferred closes run host-last.
		defer contract.IgnoreClose(pluginHost)
		ctx, err := plugin.NewContext(ctx, nil, nil, pluginHost, nil, cwd, nil, false, nil)
		if err != nil {
			return nil, nil, err
		}
		options.loader = schema.NewPluginLoader(ctx)

		defer contract.IgnoreClose(ctx)
	}

	if options.packageCache == nil {
		options.packageCache = NewPackageCache()
	}

	b := &binder{
		options:            options,
		tokens:             syntax.NewTokenMapForFiles(files),
		packageDescriptors: map[string]*schema.PackageDescriptor{},
		referencedPackages: map[string]schema.PackageReference{},
		schemaTypes:        map[schema.Type]model.Type{},
		root:               model.NewRootScope(syntax.None),
	}

	// Define null.
	b.root.Define("null", &model.Constant{
		Name:          "null",
		ConstantValue: cty.NullVal(cty.DynamicPseudoType),
	})
	// Define builtin functions.
	for name, fn := range pulumiBuiltins(options) {
		b.root.DefineFunction(name, fn)
	}
	// Define the invoke function.
	b.root.DefineFunction(Invoke, model.NewFunction(model.GenericFunctionSignature(b.bindInvokeSignature)))
	// Define the call function.
	b.root.DefineFunction(Call, model.NewFunction(model.GenericFunctionSignature(b.bindCallSignature)))
	// Define any external scope variables supplied by the caller (e.g. resources owned by another source that
	// a snippet program references). The caller chooses each variable's VariableType.
	for name, v := range options.extraScopeVariables {
		b.root.Define(name, v)
	}

	var diagnostics hcl.Diagnostics

	// Load package descriptors from the files
	descriptorMap, descriptorDiags := ReadAllPackageDescriptors(files)
	diagnostics = append(diagnostics, descriptorDiags...)
	for packageName, descriptor := range descriptorMap {
		b.packageDescriptors[packageName] = descriptor
	}
	// Caller-supplied descriptors (snippet bindings carry these structurally rather than as PCL syntax)
	// take precedence over any file-declared block for the same package.
	for packageName, descriptor := range options.extraPackageDescriptors {
		b.packageDescriptors[packageName] = descriptor
	}

	// Sort files in source order, then declare all top-level nodes in each.
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})
	for _, f := range files {
		fileDiags, err := b.declareNodes(ctx, f)
		if err != nil {
			return nil, nil, err
		}
		diagnostics = append(diagnostics, fileDiags...)
	}

	// Now bind the nodes.
	for _, n := range b.nodes {
		diagnostics = append(diagnostics, b.bindNode(ctx, n)...)
	}

	if diagnostics.HasErrors() {
		return nil, diagnostics, diagnostics
	}

	return &Program{
		Nodes:  b.nodes,
		files:  files,
		binder: b,
	}, diagnostics, nil
}

// Used by language plugins to bind a PCL program in the given directory.
func BindDirectory(
	directory string,
	loader schema.ReferenceLoader,
	extraOptions ...BindOption,
) (*Program, hcl.Diagnostics, error) {
	parser := syntax.NewParser()
	parseDiagnostics, err := ParseDirectory(parser, directory)
	if err != nil {
		return nil, parseDiagnostics, fmt.Errorf("parse directory: %w", err)
	}
	if parseDiagnostics.HasErrors() {
		return nil, parseDiagnostics, nil
	}

	opts := make([]BindOption, 0, 3+len(extraOptions))
	opts = append(opts,
		Loader(loader),
		DirPath(directory),
		ComponentBinder(ComponentProgramBinderFromFileSystem()),
	)

	opts = append(opts, extraOptions...)

	program, bindDiagnostics, err := BindProgram(parser.Files, opts...)

	// err will be the same as bindDiagnostics if there are errors, but we don't want to return that here.
	// err _could_ also be a context setup error in which case bindDiagnotics will be nil and that we do want to return.
	if bindDiagnostics != nil {
		err = nil
	}

	allDiagnostics := append(parseDiagnostics, bindDiagnostics...)
	return program, allDiagnostics, err
}

func ParseFiles(parser *syntax.Parser, directory string, files []fs.DirEntry) (hcl.Diagnostics, error) {
	var parseDiagnostics hcl.Diagnostics
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileName := file.Name()
		path := filepath.Join(directory, fileName)

		if filepath.Ext(path) == ".pp" {
			file, err := os.Open(path)
			if err != nil {
				return nil, err
			}

			err = parser.ParseFile(file, filepath.Base(path))
			contract.IgnoreClose(file)
			if err != nil {
				return nil, err
			}
			parseDiagnostics = append(parseDiagnostics, parser.Diagnostics...)
		}
	}

	return parseDiagnostics, nil
}

// / ParseDirectory parses all of the PCL files in the given directory into the state of the parser.
func ParseDirectory(parser *syntax.Parser, directory string) (hcl.Diagnostics, error) {
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	parseDiagnostics, err := ParseFiles(parser, directory, files)
	if err != nil {
		return nil, err
	}

	return parseDiagnostics, nil
}

func packageDescriptorsEqual(a, b *schema.PackageDescriptor) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Name != b.Name || a.DownloadURL != b.DownloadURL {
		return false
	}
	if (a.Version == nil) != (b.Version == nil) {
		return false
	}
	if a.Version != nil && !a.Version.Equals(*b.Version) {
		return false
	}
	if (a.Parameterization == nil) != (b.Parameterization == nil) {
		return false
	}
	if a.Parameterization != nil {
		ap, bp := a.Parameterization, b.Parameterization
		if ap.Name != bp.Name || !ap.Version.Equals(bp.Version) || !bytes.Equal(ap.Value, bp.Value) {
			return false
		}
	}
	return true
}

func ReadAllPackageDescriptors(files []*syntax.File) (map[string]*schema.PackageDescriptor, hcl.Diagnostics) {
	descriptorMap := map[string]*schema.PackageDescriptor{}
	var diagnostics hcl.Diagnostics
	for _, file := range files {
		packageDescriptors, diags := ReadPackageDescriptors(file)
		diagnostics = append(diagnostics, diags...)
		for packageName, descriptor := range packageDescriptors {
			existing, ok := descriptorMap[packageName]
			if ok {
				if packageDescriptorsEqual(existing, descriptor) {
					// Identical duplicate — silently skip. This happens when the same package block
					// appears in multiple files (e.g. main.pp and a per-package .pp file).
					continue
				}
				message := fmt.Sprintf("package %q was already defined with different parameters", packageName)
				subjectRange := file.Body.Range()
				diagnostics = append(diagnostics, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  message,
					Detail:   message,
					Subject:  &subjectRange,
				})
				continue
			}
			descriptorMap[packageName] = descriptor
		}
	}

	return descriptorMap, diagnostics
}

func makeObjectPropertiesOptional(objectType *model.ObjectType) *model.ObjectType {
	for property, propertyType := range objectType.Properties {
		if !model.IsOptionalType(propertyType) {
			objectType.Properties[property] = model.NewOptionalType(propertyType)
		}
	}

	return objectType
}

// declareNodes declares all of the top-level nodes in the given file. This includes config, resources, outputs, and
// locals.
// Temporarily, we load all resources first, as convert sets the highest package version seen
// under all resources' options. Once this is supported for invokes, the order of declaration will not
// impact which package is actually loaded.
func (b *binder) declareNodes(ctx context.Context, file *syntax.File) (hcl.Diagnostics, error) {
	var diagnostics hcl.Diagnostics

	for _, item := range model.SourceOrderBody(file.Body) {
		switch item := item.(type) {
		case *hclsyntax.Block:
			switch item.Type {
			case "resource", "read":
				if len(item.Labels) != 2 {
					diagnostics = append(diagnostics, labelsErrorf(item, "%s variables must have exactly two labels", item.Type))
				}

				var node Node
				switch item.Type {
				case "resource":
					node = &Resource{syntax: item}
				case "read":
					node = &ReadResource{syntax: item}
				}

				declareDiags := b.declareNode(item.Labels[0], node)
				diagnostics = append(diagnostics, declareDiags...)

				if err := b.loadReferencedPackageSchemas(ctx, node); err != nil {
					return nil, err
				}
			}
		}
	}

	for _, item := range model.SourceOrderBody(file.Body) {
		switch item := item.(type) {
		case *hclsyntax.Attribute:
			v := &LocalVariable{syntax: item}
			attrDiags := b.declareNode(item.Name, v)
			diagnostics = append(diagnostics, attrDiags...)

			if err := b.loadReferencedPackageSchemas(ctx, v); err != nil {
				return nil, err
			}
		case *hclsyntax.Block:
			switch item.Type {
			case "config":
				name, typ := "<unnamed>", model.Type(model.DynamicType)
				switch len(item.Labels) {
				case 1:
					name = item.Labels[0]
				case 2:
					name = item.Labels[0]

					typeExpr, diags := model.BindExpressionText(item.Labels[1], model.TypeScope, item.LabelRanges[1].Start)
					diagnostics = append(diagnostics, diags...)
					if typeExpr == nil {
						return diagnostics, fmt.Errorf("cannot bind expression: %v", diagnostics)
					}
					typ = typeExpr.Type()
					switch configType := typ.(type) {
					case *model.ObjectType:
						typ = makeObjectPropertiesOptional(configType)
					case *model.ListType:
						switch elementType := configType.ElementType.(type) {
						case *model.ObjectType:
							modifiedElementType := makeObjectPropertiesOptional(elementType)
							typ = model.NewListType(modifiedElementType)
						}
					case *model.MapType:
						switch elementType := configType.ElementType.(type) {
						case *model.ObjectType:
							modifiedElementType := makeObjectPropertiesOptional(elementType)
							typ = model.NewMapType(modifiedElementType)
						}
					}
				default:
					diagnostics = append(diagnostics, labelsErrorf(item, "config variables must have exactly one or two labels"))
				}

				// TODO(pdg): check body for valid contents

				v := &ConfigVariable{
					typ:    typ,
					syntax: item,
				}
				diags := b.declareNode(name, v)
				diagnostics = append(diagnostics, diags...)

				if err := b.loadReferencedPackageSchemas(ctx, v); err != nil {
					return nil, err
				}
			case "output":
				name, typ := "<unnamed>", model.Type(model.DynamicType)
				switch len(item.Labels) {
				case 1:
					name = item.Labels[0]
				case 2:
					name = item.Labels[0]

					typeExpr, diags := model.BindExpressionText(item.Labels[1], model.TypeScope, item.LabelRanges[1].Start)
					diagnostics = append(diagnostics, diags...)
					typ = typeExpr.Type()
				default:
					diagnostics = append(diagnostics, labelsErrorf(item, "config variables must have exactly one or two labels"))
				}

				v := &OutputVariable{
					typ:    typ,
					syntax: item,
				}
				diags := b.declareNode(name, v)
				diagnostics = append(diagnostics, diags...)

				if err := b.loadReferencedPackageSchemas(ctx, v); err != nil {
					return nil, err
				}
			case "component":
				if len(item.Labels) != 2 {
					diagnostics = append(diagnostics, labelsErrorf(item, "components must have exactly two labels"))
					continue
				}
				name := item.Labels[0]
				source := item.Labels[1]

				v := &Component{
					name:         name,
					syntax:       item,
					source:       source,
					VariableType: model.DynamicType,
				}
				diags := b.declareNode(name, v)
				diagnostics = append(diagnostics, diags...)
			case "hook":
				labels := item.Labels
				if len(labels) != 1 {
					diagnostics = append(diagnostics, labelsErrorf(item, "hook blocks must have exactly one label"))
					continue
				}
				name := labels[0]
				v := &Hook{
					syntax:      item,
					logicalName: name,
				}
				diags := b.declareNode(name, v)
				diagnostics = append(diagnostics, diags...)
			case "pulumi":
				labels := item.Labels
				if len(labels) != 0 {
					diagnostics = append(diagnostics, labelsErrorf(item, "pulumi block must not have any labels"))
					continue
				}
				v := &PulumiBlock{
					syntax: item,
				}
				diags := b.declareNode(PulumiBlockName, v)
				diagnostics = append(diagnostics, diags...)
			}
		}
	}

	return diagnostics, nil
}

// evaluateLiteralExpr evaluates a syntax expression to a string if it is a literal string expression.
func evaluateLiteralExpr(expr hclsyntax.Expression) (string, error) {
	switch expr := expr.(type) {
	case *hclsyntax.LiteralValueExpr:
		if expr.Val.Type() != cty.String {
			return "", fmt.Errorf("expected string literal, got %v", expr.Val.Type())
		}
		return expr.Val.AsString(), nil
	case *hclsyntax.TemplateExpr:
		if len(expr.Parts) != 1 {
			return "", fmt.Errorf("expected string literal, got %v", expr)
		}

		part, ok := expr.Parts[0].(*hclsyntax.LiteralValueExpr)
		if !ok || part.Val.Type() != cty.String {
			return "", fmt.Errorf("expected string literal, got %v", part.Val.Type())
		}
		return part.Val.AsString(), nil
	default:
		return "", fmt.Errorf("expected string literal, got %T", expr)
	}
}

// Evaluate a constant string attribute that's internal to Pulumi, e.g.: the Logical Name on a resource or output.
func getStringAttrValue(attr *model.Attribute) (string, *hcl.Diagnostic) {
	if literal, err := evaluateLiteralExpr(attr.Syntax.Expr); err == nil {
		return literal, nil
	}

	return "", stringAttributeError(attr)
}

// Returns the value of constant boolean attribute
func getBooleanAttributeValue(attr *model.Attribute) (bool, *hcl.Diagnostic) {
	switch lit := attr.Syntax.Expr.(type) {
	case *hclsyntax.LiteralValueExpr:
		if lit.Val.Type() != cty.Bool {
			return false, boolAttributeError(attr)
		}
		return lit.Val.True(), nil
	default:
		return false, boolAttributeError(attr)
	}
}

// readParameterizationDescriptor parses a map of attributes that match contents of a parameterization block in the
// form of:
//
//	parameterization {
//	    name = <name>
//	    version = <version>
//	    value = <base64 encoded string>
//	}
func readParameterizationDescriptor(
	packageName string,
	attributes map[string]hclsyntax.Expression,
) (*schema.ParameterizationDescriptor, *hcl.Diagnostic) {
	descriptor := &schema.ParameterizationDescriptor{}

	// read name, version, and value attributes
	for key, value := range attributes {
		switch key {
		case "name":
			name, err := evaluateLiteralExpr(value)
			if err != nil {
				return nil, errorf(value.Range(), "invalid name attribute of parameterization of %q: %v", packageName, err)
			}
			if name == "" {
				return nil, errorf(value.Range(), "name attribute of parameterization of %q cannot be empty", packageName)
			}
			descriptor.Name = name
		case "version":
			version, _ := evaluateLiteralExpr(value)
			parsedVersion, err := semver.Make(version)
			if err != nil {
				return nil, errorf(value.Range(),
					"invalid version %q of parameterization for package %q: %v", version, packageName, err)
			}
			descriptor.Version = parsedVersion

		case "value":
			// value must be a base64 encoded string
			base64EncodedValue, err := evaluateLiteralExpr(value)
			if err != nil {
				return nil, errorf(value.Range(), "invalid base64 encoded value for parameterization of %q: %v", packageName, err)
			}

			decoded, err := base64.StdEncoding.DecodeString(base64EncodedValue)
			if err != nil {
				return nil, errorf(value.Range(), "invalid base64 encoded value for parameterization of %q: %v", packageName, err)
			}

			descriptor.Value = decoded
		}
	}

	return descriptor, nil
}

// ReadPackageDescriptors reads the package blocks in the given file and returns a map of package names to the package.
// The descriptors blocks are top-level program blocks have the following format:
//
//	package <name> {
//	  baseProviderName = <name>
//	  baseProviderVersion = <version>
//	  baseProviderDownloadUrl = <url>
//	  parameterization {
//	      name = <name>
//	      version = <version>
//	      value = <base64 encoded string>
//	  }
//	}
//
// Notes:
//   - parameterization block is optional.
//   - the value of the parameterization is base64-encoded string because we want to specify binary data in PCL form
//
// These package descriptors allow the binder loader to load package schema information from a package source.
func ReadPackageDescriptors(file *syntax.File) (map[string]*schema.PackageDescriptor, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics
	packageDescriptors := map[string]*schema.PackageDescriptor{}
	for _, node := range model.SourceOrderBody(file.Body) {
		switch node := node.(type) {
		case *hclsyntax.Block:
			if node.Type != "package" {
				// we only care about blocks of the form:
				//    package <name> { ... }
				continue
			}

			labels := node.Labels
			if len(labels) > 1 {
				diagnostics = append(diagnostics,
					labelsErrorf(node, "package blocks must have at most one label"))
				continue
			}

			if len(labels) == 1 {
				labelRange := node.LabelRanges[0]
				diagnostics = append(diagnostics, &hcl.Diagnostic{
					Severity: hcl.DiagWarning,
					Summary:  "package block label is deprecated",
					Detail:   "Package block labels should be replaced by baseProviderName.",
					Subject:  &labelRange,
				})
			}

			// read the attributes of the package block to fill in the package descriptor data
			packageDescriptor := &schema.PackageDescriptor{}

			if node.Body == nil {
				diagnostics = append(diagnostics,
					errorf(node.Range(), "package blocks must set base provider information"))
				continue
			}

			for _, attribute := range node.Body.Attributes {
				switch attribute.Name {
				case "baseProviderName":
					baseProviderName, err := evaluateLiteralExpr(attribute.Expr)
					if err != nil {
						diagnostics = append(diagnostics,
							errorf(attribute.Range(), "invalid base provider name for package: %v", err))
						continue
					}

					packageDescriptor.Name = baseProviderName
				case "baseProviderVersion":
					version, _ := evaluateLiteralExpr(attribute.Expr)
					parsedVersion, err := semver.Make(version)
					if err != nil {
						// parsing the version failed, error out and skip this package
						diagnostics = append(diagnostics,
							errorf(attribute.Range(),
								"invalid baseProviderVersion %q for package: %v", version, err))
						continue
					}
					packageDescriptor.Version = &parsedVersion
				case "baseProviderDownloadUrl":
					downloadURLValue, err := evaluateLiteralExpr(attribute.Expr)
					if err != nil {
						diagnostics = append(diagnostics,
							errorf(attribute.Range(), "invalid download URL for package: %v", err))
						continue
					}

					if _, err := url.ParseRequestURI(downloadURLValue); err != nil {
						diagnostics = append(diagnostics,
							errorf(attribute.Range(), "invalid download URL for package: %v", err))
						continue
					}
					packageDescriptor.DownloadURL = downloadURLValue
				}
			}

			if packageDescriptor.Name == "" && len(labels) == 1 {
				// Backwards compatibility: if the package block has a single label and doesn't set baseProviderName,
				// use the label as the package name
				packageDescriptor.Name = labels[0]
			}

			if packageDescriptor.Name == "" {
				diagnostics = append(diagnostics,
					errorf(node.Range(), "package blocks must set baseProviderName"))
				continue
			}

			for _, block := range node.Body.Blocks {
				switch block.Type {
				case "parameterization":
					attributes := map[string]hclsyntax.Expression{}
					for _, item := range block.Body.Attributes {
						attributes[item.Name] = item.Expr
					}
					descriptor, diag := readParameterizationDescriptor(packageDescriptor.Name, attributes)
					if diag != nil {
						diagnostics = append(diagnostics, diag)
						continue
					}
					packageDescriptor.Parameterization = descriptor
				}
			}

			packageName := packageDescriptor.PackageName()
			if packageName == "" {
				diagnostics = append(diagnostics,
					errorf(node.Range(), "package blocks must resolve a package name"))
				continue
			}

			// make sure we don't declare the same package twice
			if _, ok := packageDescriptors[packageName]; ok {
				diagnostics = append(diagnostics,
					errorf(node.Range(), "package %q was already defined", packageName))
				continue
			}
			packageDescriptors[packageName] = packageDescriptor
		}
	}
	return packageDescriptors, diagnostics
}

// declareNode declares a single top-level node. If a node with the same name has already been declared, it returns an
// appropriate diagnostic.
func (b *binder) declareNode(name string, n Node) hcl.Diagnostics {
	if !b.root.Define(name, n) {
		existing, _ := b.root.BindReference(name)
		return hcl.Diagnostics{errorf(existing.SyntaxNode().Range(), "%q already declared", name)}
	}
	b.nodes = append(b.nodes, n)
	return nil
}

func (b *binder) bindExpression(node hclsyntax.Node) (model.Expression, hcl.Diagnostics) {
	return model.BindExpression(node, b.root, b.tokens, b.options.modelOptions()...)
}

func modelExprToString(expr *model.Expression) string {
	if expr == nil {
		return ""
	}
	return (*expr).(*model.TemplateExpression).
		Parts[0].(*model.LiteralValueExpression).Value.AsString()
}
