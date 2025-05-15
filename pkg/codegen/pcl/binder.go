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

package pcl

import (
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
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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

func PluginHost(host plugin.Host) BindOption {
	return Loader(schema.NewPluginLoader(host))
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

// BindProgram performs semantic analysis on the given set of HCL2 files that represent a single program. The given
// host, if any, is used for loading any resource plugins necessary to extract schema information.
func BindProgram(files []*syntax.File, opts ...BindOption) (*Program, hcl.Diagnostics, error) {
	var options bindOptions
	for _, o := range opts {
		o(&options)
	}

	if options.loader == nil {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, nil, err
		}
		ctx, err := plugin.NewContext(context.TODO(), nil, nil, nil, nil, cwd, nil, false, nil)
		if err != nil {
			return nil, nil, err
		}
		options.loader = schema.NewPluginLoader(ctx.Host)

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

	var diagnostics hcl.Diagnostics

	// Load package descriptors from the files
	descriptorMap, descriptorDiags := ReadAllPackageDescriptors(files)
	diagnostics = append(diagnostics, descriptorDiags...)
	for packageName, descriptor := range descriptorMap {
		b.packageDescriptors[packageName] = descriptor
	}

	// Sort files in source order, then declare all top-level nodes in each.
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})
	for _, f := range files {
		fileDiags, err := b.declareNodes(f)
		if err != nil {
			return nil, nil, err
		}
		diagnostics = append(diagnostics, fileDiags...)
	}

	// Now bind the nodes.
	for _, n := range b.nodes {
		diagnostics = append(diagnostics, b.bindNode(n)...)
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

	opts := []BindOption{
		Loader(loader),
		DirPath(directory),
		ComponentBinder(ComponentProgramBinderFromFileSystem()),
	}

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

func ReadAllPackageDescriptors(files []*syntax.File) (map[string]*schema.PackageDescriptor, hcl.Diagnostics) {
	descriptorMap := map[string]*schema.PackageDescriptor{}
	var diagnostics hcl.Diagnostics
	for _, file := range files {
		packageDescriptors, diags := ReadPackageDescriptors(file)
		diagnostics = append(diagnostics, diags...)
		for packageName, descriptor := range packageDescriptors {
			if _, ok := descriptorMap[packageName]; ok {
				message := fmt.Sprintf("package %q was already defined", packageName)
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
func (b *binder) declareNodes(file *syntax.File) (hcl.Diagnostics, error) {
	var diagnostics hcl.Diagnostics

	for _, item := range model.SourceOrderBody(file.Body) {
		switch item := item.(type) {
		case *hclsyntax.Block:
			switch item.Type {
			case "resource":
				if len(item.Labels) != 2 {
					diagnostics = append(diagnostics, labelsErrorf(item, "resource variables must have exactly two labels"))
				}

				resource := &Resource{
					syntax: item,
				}
				declareDiags := b.declareNode(item.Labels[0], resource)
				diagnostics = append(diagnostics, declareDiags...)

				if err := b.loadReferencedPackageSchemas(resource); err != nil {
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

			if err := b.loadReferencedPackageSchemas(v); err != nil {
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

				if err := b.loadReferencedPackageSchemas(v); err != nil {
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

				if err := b.loadReferencedPackageSchemas(v); err != nil {
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
			if len(labels) != 1 {
				diagnostics = append(diagnostics,
					labelsErrorf(node, "package blocks must have exactly one label (the package name)"))
				continue
			}

			packageName := labels[0]
			// make sure we don't declare the same package twice
			if _, ok := packageDescriptors[packageName]; ok {
				diagnostics = append(diagnostics,
					errorf(node.Range(), "package %q was already defined", packageName))
				continue
			}

			// read the attributes of the package block to fill in the package descriptor data
			packageDescriptor := &schema.PackageDescriptor{}

			if node.Body != nil {
				for _, attribute := range node.Body.Attributes {
					switch attribute.Name {
					case "baseProviderName":
						baseProviderName, err := evaluateLiteralExpr(attribute.Expr)
						if err != nil {
							diagnostics = append(diagnostics,
								errorf(attribute.Range(), "invalid base provider name for %q: %v", packageName, err))
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
									"invalid baseProviderVersion %q for %q: %v", version, packageName, err))
							continue
						}
						packageDescriptor.Version = &parsedVersion
					case "baseProviderDownloadUrl":
						downloadURLValue, err := evaluateLiteralExpr(attribute.Expr)
						if err != nil {
							diagnostics = append(diagnostics,
								errorf(attribute.Range(), "invalid download URL for %q: %v", packageName, err))
							continue
						}

						if _, err := url.ParseRequestURI(downloadURLValue); err != nil {
							diagnostics = append(diagnostics,
								errorf(attribute.Range(), "invalid download URL for %q: %v", packageName, err))
							continue
						}
						packageDescriptor.DownloadURL = downloadURLValue
					}
				}
				for _, block := range node.Body.Blocks {
					switch block.Type {
					case "parameterization":
						attributes := map[string]hclsyntax.Expression{}
						for _, item := range block.Body.Attributes {
							attributes[item.Name] = item.Expr
						}
						descriptor, diag := readParameterizationDescriptor(packageName, attributes)
						if diag != nil {
							diagnostics = append(diagnostics, diag)
							continue
						}
						packageDescriptor.Parameterization = descriptor
					}
				}
			}

			if packageDescriptor.Name == "" {
				// baseProviderName was not provided
				// default to the package name
				packageDescriptor.Name = packageName
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
