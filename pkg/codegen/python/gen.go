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

// Pulling out some of the repeated strings tokens into constants would harm readability, so we just ignore the
// goconst linter's warning.
//
// nolint: lll, goconst
package python

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/blang/semver"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type typeDetails struct {
	outputType         bool
	inputType          bool
	resourceOutputType bool
	plainType          bool
}

type imports codegen.StringSet

func (imports imports) addType(mod *modContext, t *schema.ObjectType, input bool) {
	imports.addTypeIf(mod, t, input, nil /*predicate*/)
}

func (imports imports) addTypeIf(mod *modContext, t *schema.ObjectType, input bool, predicate func(imp string) bool) {
	if imp := mod.importObjectType(t, input); imp != "" && (predicate == nil || predicate(imp)) {
		codegen.StringSet(imports).Add(imp)
	}
}

func (imports imports) addEnum(mod *modContext, tok string) {
	if imp := mod.importEnumFromToken(tok); imp != "" {
		codegen.StringSet(imports).Add(imp)
	}
}

func (imports imports) addResource(mod *modContext, r *schema.ResourceType) {
	if imp := mod.importResourceType(r); imp != "" {
		codegen.StringSet(imports).Add(imp)
	}
}

func (imports imports) strings() []string {
	result := make([]string, 0, len(imports))
	for imp := range imports {
		result = append(result, imp)
	}
	sort.Strings(result)
	return result
}

func title(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	return string(append([]rune{unicode.ToUpper(runes[0])}, runes[1:]...))
}

type modContext struct {
	pkg                  *schema.Package
	mod                  string
	pyPkgName            string
	types                []*schema.ObjectType
	enums                []*schema.EnumType
	resources            []*schema.Resource
	functions            []*schema.Function
	typeDetails          map[*schema.ObjectType]*typeDetails
	children             []*modContext
	parent               *modContext
	snakeCaseToCamelCase map[string]string
	camelCaseToSnakeCase map[string]string
	tool                 string
	extraSourceFiles     []string
	isConfig             bool

	// Name overrides set in PackageInfo
	modNameOverrides map[string]string // Optional overrides for Pulumi module names
	compatibility    string            // Toggle compatibility mode for a specified target.
}

func (mod *modContext) isTopLevel() bool {
	return mod.parent == nil
}

func (mod *modContext) walkSelfWithDescendants() []*modContext {
	var found []*modContext
	found = append(found, mod)
	for _, childMod := range mod.children {
		found = append(found, childMod.walkSelfWithDescendants()...)
	}
	return found
}

func (mod *modContext) addChild(child *modContext) {
	mod.children = append(mod.children, child)
	child.parent = mod
}

func (mod *modContext) details(t *schema.ObjectType) *typeDetails {
	details, ok := mod.typeDetails[t]
	if !ok {
		details = &typeDetails{}
		if mod.typeDetails == nil {
			mod.typeDetails = map[*schema.ObjectType]*typeDetails{}
		}
		mod.typeDetails[t] = details
	}
	return details
}

func (mod *modContext) modNameAndName(pkg *schema.Package, t schema.Type, input bool) (modName string, name string) {
	var info PackageInfo
	contract.AssertNoError(pkg.ImportLanguages(map[string]schema.Language{"python": Importer}))
	if v, ok := pkg.Language["python"].(PackageInfo); ok {
		info = v
	}

	var token string
	switch t := t.(type) {
	case *schema.EnumType:
		token, name = t.Token, tokenToName(t.Token)
	case *schema.ObjectType:
		namingCtx := &modContext{
			pkg:              pkg,
			modNameOverrides: info.ModuleNameOverrides,
			compatibility:    info.Compatibility,
		}
		token, name = t.Token, namingCtx.unqualifiedObjectTypeName(t, input)
	case *schema.ResourceType:
		token, name = t.Token, tokenToName(t.Token)
	}

	modName = tokenToModule(token, pkg, info.ModuleNameOverrides)
	if modName == mod.mod {
		modName = ""
	}
	if modName != "" {
		modName = strings.ReplaceAll(modName, "/", ".") + "."
	}
	return
}

func (mod *modContext) unqualifiedObjectTypeName(t *schema.ObjectType, input bool) string {
	name := tokenToName(t.Token)

	if mod.compatibility != tfbridge20 && mod.compatibility != kubernetes20 {
		if t.IsInputShape() {
			return name + "Args"
		}
		return name
	}

	switch {
	case input:
		return name + "Args"
	case mod.details(t).plainType:
		return name + "Result"
	}
	return name
}

func (mod *modContext) objectType(t *schema.ObjectType, input bool) string {
	var prefix string
	if !input {
		prefix = "outputs."
	}

	// If it's an external type, reference it via fully qualified name.
	if t.Package != mod.pkg {
		modName, name := mod.modNameAndName(t.Package, t, input)
		return fmt.Sprintf("'%s.%s%s%s'", pyPack(t.Package.Name), modName, prefix, name)
	}

	modName, name := mod.tokenToModule(t.Token), mod.unqualifiedObjectTypeName(t, input)
	if modName == "" && modName != mod.mod {
		rootModName := "_root_outputs."
		if input {
			rootModName = "_root_inputs."
		}
		return fmt.Sprintf("'%s%s'", rootModName, name)
	}

	if modName == mod.mod {
		modName = ""
	}
	if modName != "" {
		modName = "_" + strings.ReplaceAll(modName, "/", ".") + "."
	}

	return fmt.Sprintf("'%s%s%s'", modName, prefix, name)
}

func (mod *modContext) tokenToEnum(tok string) string {
	modName, name := mod.tokenToModule(tok), tokenToName(tok)

	if modName == "" && modName != mod.mod {
		return fmt.Sprintf("'_root_enums.%s'", name)
	}

	if modName == mod.mod {
		modName = ""
	}
	if modName != "" {
		modName = "_" + strings.ReplaceAll(modName, "/", ".") + "."
	}

	return fmt.Sprintf("'%s%s'", modName, name)
}

func (mod *modContext) resourceType(r *schema.ResourceType) string {
	if r.Resource == nil || r.Resource.Package == mod.pkg {
		return mod.tokenToResource(r.Token)
	}

	// Is it a provider resource?
	if strings.HasPrefix(r.Token, "pulumi:providers:") {
		pkgName := strings.TrimPrefix(r.Token, "pulumi:providers:")
		return fmt.Sprintf("pulumi_%s.Provider", pkgName)
	}

	pkg := r.Resource.Package
	modName, name := mod.modNameAndName(pkg, r, false)
	return fmt.Sprintf("%s.%s%s", pyPack(pkg.Name), modName, name)
}

func (mod *modContext) tokenToResource(tok string) string {
	// token := pkg : module : member
	// module := path/to/module

	components := strings.Split(tok, ":")
	contract.Assertf(len(components) == 3, "malformed token %v", tok)

	// Is it a provider resource?
	if components[0] == "pulumi" && components[1] == "providers" {
		return fmt.Sprintf("pulumi_%s.Provider", components[2])
	}

	modName, name := mod.tokenToModule(tok), tokenToName(tok)

	if modName == mod.mod {
		modName = ""
	}
	if modName != "" {
		modName = "_" + strings.ReplaceAll(modName, "/", ".") + "."
	}

	return fmt.Sprintf("%s%s", modName, name)
}

func tokenToName(tok string) string {
	// token := pkg : module : member
	// module := path/to/module

	components := strings.Split(tok, ":")
	contract.Assertf(len(components) == 3, "malformed token %v", tok)

	return title(components[2])
}

func tokenToModule(tok string, pkg *schema.Package, moduleNameOverrides map[string]string) string {
	// See if there's a manually-overridden module name.
	canonicalModName := pkg.TokenToModule(tok)
	if override, ok := moduleNameOverrides[canonicalModName]; ok {
		return override
	}
	// A module can include fileparts, which we want to preserve.
	var modName string
	for i, part := range strings.Split(strings.ToLower(canonicalModName), "/") {
		if i > 0 {
			modName += "/"
		}
		modName += PyName(part)
	}
	return modName
}

func (mod *modContext) tokenToModule(tok string) string {
	return tokenToModule(tok, mod.pkg, mod.modNameOverrides)
}

func printComment(w io.Writer, comment string, indent string) {
	lines := strings.Split(comment, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return
	}

	// Known special characters that need escaping.
	// TODO: Consider replacing the docstring with a raw string which may make sense if this
	// list grows much further.
	replacer := strings.NewReplacer(`"""`, `\"\"\"`, `\x`, `\\x`, `\N`, `\\N`)
	fmt.Fprintf(w, "%s\"\"\"\n", indent)
	for _, l := range lines {
		if l == "" {
			fmt.Fprintf(w, "\n")
		} else {
			escaped := replacer.Replace(l)
			fmt.Fprintf(w, "%s%s\n", indent, escaped)
		}
	}
	fmt.Fprintf(w, "%s\"\"\"\n", indent)
}

func (mod *modContext) genHeader(w io.Writer, needsSDK bool, imports imports) {
	// Set the encoding to UTF-8, in case the comments contain non-ASCII characters.
	fmt.Fprintf(w, "# coding=utf-8\n")

	// Emit a standard warning header ("do not edit", etc).
	fmt.Fprintf(w, "# *** WARNING: this file was generated by %v. ***\n", mod.tool)
	fmt.Fprintf(w, "# *** Do not edit by hand unless you're certain you know what you are doing! ***\n\n")

	// If needed, emit the standard Pulumi SDK import statement.
	if needsSDK {
		rel, err := filepath.Rel(mod.mod, "")
		contract.Assert(err == nil)
		relRoot := path.Dir(rel)
		relImport := relPathToRelImport(relRoot)

		fmt.Fprintf(w, "import warnings\n")
		fmt.Fprintf(w, "import pulumi\n")
		fmt.Fprintf(w, "import pulumi.runtime\n")
		fmt.Fprintf(w, "from typing import Any, Mapping, Optional, Sequence, Union, overload\n")
		fmt.Fprintf(w, "from %s import _utilities\n", relImport)
		for _, imp := range imports.strings() {
			fmt.Fprintf(w, "%s\n", imp)
		}
		fmt.Fprintf(w, "\n")
	}
}

func relPathToRelImport(relPath string) string {
	// Convert relative path to relative import e.g. "../.." -> "..."
	// https://realpython.com/absolute-vs-relative-python-imports/#relative-imports
	relImport := "."
	if relPath == "." {
		return relImport
	}
	for _, component := range strings.Split(relPath, "/") {
		if component == ".." {
			relImport += "."
		} else {
			relImport += component
		}
	}
	return relImport
}

type fs map[string][]byte

func (fs fs) add(path string, contents []byte) {
	_, has := fs[path]
	contract.Assertf(!has, "duplicate file: %s", path)
	fs[path] = contents
}

func (mod *modContext) gen(fs fs) error {
	dir := path.Join(mod.pyPkgName, mod.mod)

	var exports []string
	for p := range fs {
		d := path.Dir(p)
		if d == "." {
			d = ""
		}
		if d == dir {
			exports = append(exports, strings.TrimSuffix(path.Base(p), ".py"))
		}
	}

	addFile := func(name, contents string) {
		p := path.Join(dir, name)
		if !strings.HasSuffix(name, ".pyi") {
			exports = append(exports, name[:len(name)-len(".py")])
		}
		fs.add(p, []byte(contents))
	}

	// Utilities, config, readme
	switch mod.mod {
	case "":
		buffer := &bytes.Buffer{}
		mod.genHeader(buffer, false /*needsSDK*/, nil)
		fmt.Fprintf(buffer, "%s", utilitiesFile)
		fs.add(filepath.Join(dir, "_utilities.py"), buffer.Bytes())
		fs.add(filepath.Join(dir, "py.typed"), []byte{})

		// Ensure that the top-level (provider) module directory contains a README.md file.
		readme := mod.pkg.Language["python"].(PackageInfo).Readme
		if readme == "" {
			readme = mod.pkg.Description
			if readme != "" && readme[len(readme)-1] != '\n' {
				readme += "\n"
			}
			if mod.pkg.Attribution != "" {
				if len(readme) != 0 {
					readme += "\n"
				}
				readme += mod.pkg.Attribution
			}
			if readme != "" && readme[len(readme)-1] != '\n' {
				readme += "\n"
			}
		}
		fs.add(filepath.Join(dir, "README.md"), []byte(readme))

	case "config":
		if len(mod.pkg.Config) > 0 {
			vars, err := mod.genConfig(mod.pkg.Config)
			if err != nil {
				return err
			}
			addFile("vars.py", vars)
			typeStubs, err := mod.genConfigStubs(mod.pkg.Config)
			if err != nil {
				return err
			}
			addFile("__init__.pyi", typeStubs)
		}
	}

	// Resources
	for _, r := range mod.resources {
		res, err := mod.genResource(r)
		if err != nil {
			return err
		}
		name := PyName(tokenToName(r.Token))
		if mod.compatibility == kubernetes20 {
			// To maintain backward compatibility for kubernetes, the file names
			// need to be CamelCase instead of the standard snake_case.
			name = tokenToName(r.Token)
		}
		if r.IsProvider {
			name = "provider"
		}
		addFile(name+".py", res)
	}

	// Functions
	for _, f := range mod.functions {
		fun, err := mod.genFunction(f)
		if err != nil {
			return err
		}
		addFile(PyName(tokenToName(f.Token))+".py", fun)
	}

	// Nested types
	if len(mod.types) > 0 {
		if err := mod.genTypes(dir, fs); err != nil {
			return err
		}
	}

	// Enums
	if len(mod.enums) > 0 {
		buffer := &bytes.Buffer{}
		if err := mod.genEnums(buffer, mod.enums); err != nil {
			return err
		}

		addFile("_enums.py", buffer.String())
	}

	// Index
	if !mod.isEmpty() {
		fs.add(path.Join(dir, "__init__.py"), []byte(mod.genInit(exports)))
	}

	return nil
}

func (mod *modContext) hasTypes(input bool) bool {
	for _, t := range mod.types {
		if input && mod.details(t).inputType {
			return true
		}
		if !input && mod.details(t).outputType {
			return true
		}
	}
	return false
}

func (mod *modContext) isEmpty() bool {
	if len(mod.extraSourceFiles) > 0 || len(mod.functions) > 0 || len(mod.resources) > 0 || len(mod.types) > 0 ||
		mod.isConfig {
		return false
	}
	for _, child := range mod.children {
		if !child.isEmpty() {
			return false
		}
	}
	return true
}

func (mod *modContext) submodulesExist() bool {
	return len(mod.children) > 0
}

func (mod *modContext) unqualifiedImportName() string {
	name := mod.mod

	// Extract version suffix from child modules. Nested versions will have their own __init__.py file.
	// Example: apps/v1beta1 -> v1beta1
	parts := strings.Split(name, "/")
	if len(parts) > 1 {
		name = parts[len(parts)-1]
	}

	return PyName(name)
}

func (mod *modContext) fullyQualifiedImportName() string {
	name := mod.unqualifiedImportName()
	if mod.parent == nil && name == "" {
		return mod.pyPkgName
	}
	if mod.parent == nil {
		return fmt.Sprintf("%s.%s", pyPack(mod.pkg.Name), name)
	}
	return fmt.Sprintf("%s.%s", mod.parent.fullyQualifiedImportName(), name)
}

// genInit emits an __init__.py module, optionally re-exporting other members or submodules.
func (mod *modContext) genInit(exports []string) string {
	w := &bytes.Buffer{}
	mod.genHeader(w, false /*needsSDK*/, nil)
	if mod.isConfig {
		fmt.Fprintf(w, "import sys\n")
		fmt.Fprintf(w, "from .vars import _ExportableConfig\n")
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "sys.modules[__name__].__class__ = _ExportableConfig\n")
		return w.String()
	}
	fmt.Fprintf(w, "%s\n", mod.genUtilitiesImport())
	fmt.Fprintf(w, "import typing\n")

	// Import anything to export flatly that is a direct export rather than sub-module.
	if len(exports) > 0 {
		sort.Slice(exports, func(i, j int) bool {
			return PyName(exports[i]) < PyName(exports[j])
		})

		fmt.Fprintf(w, "# Export this package's modules as members:\n")
		for _, exp := range exports {
			name := PyName(exp)
			if mod.compatibility == kubernetes20 {
				// To maintain backward compatibility for kubernetes, the file names
				// need to be CamelCase instead of the standard snake_case.
				name = exp
			}
			fmt.Fprintf(w, "from .%s import *\n", name)
		}
	}
	if mod.hasTypes(true /*input*/) {
		fmt.Fprintf(w, "from ._inputs import *\n")
	}
	if mod.hasTypes(false /*input*/) {
		fmt.Fprintf(w, "from . import outputs\n")
	}

	// If there are subpackages, import them with importlib.
	if mod.submodulesExist() {

		children := make([]*modContext, len(mod.children))
		copy(children, mod.children)

		sort.Slice(children, func(i, j int) bool {
			return PyName(children[i].mod) < PyName(children[j].mod)
		})

		fmt.Fprintf(w, "\n# Make subpackages available:\n")
		fmt.Fprintf(w, "if typing.TYPE_CHECKING:\n")

		for _, submod := range children {
			if !submod.isEmpty() {
				fmt.Fprintf(w, "    import %s as %s\n",
					submod.fullyQualifiedImportName(),
					submod.unqualifiedImportName())
			}
		}

		fmt.Fprintf(w, "else:\n")

		for _, submod := range children {
			if !submod.isEmpty() {
				fmt.Fprintf(w, "    %s = _utilities.lazy_import('%s')\n",
					submod.unqualifiedImportName(),
					submod.fullyQualifiedImportName())
			}
		}

		fmt.Fprintf(w, "\n")
	}

	// If there are resources in this module, register the module with the runtime.
	if len(mod.resources) != 0 {
		err := genResourceMappings(mod, w)
		contract.Assert(err == nil)
	}

	return w.String()
}

func (mod *modContext) getRelImportFromRoot() string {
	rel, err := filepath.Rel(mod.mod, "")
	contract.Assert(err == nil)
	relRoot := path.Dir(rel)
	return relPathToRelImport(relRoot)
}

func (mod *modContext) genUtilitiesImport() string {
	rel, err := filepath.Rel(mod.mod, "")
	contract.Assert(err == nil)
	relRoot := path.Dir(rel)
	relImport := relPathToRelImport(relRoot)
	return fmt.Sprintf("from %s import _utilities", relImport)
}

func (mod *modContext) importObjectType(t *schema.ObjectType, input bool) string {
	if t.Package != mod.pkg {
		return fmt.Sprintf("import %s", pyPack(t.Package.Name))
	}

	tok := t.Token
	parts := strings.Split(tok, ":")
	contract.Assert(len(parts) == 3)
	refPkgName := parts[0]

	modName := mod.tokenToModule(tok)
	if modName == mod.mod {
		if input {
			return "from ._inputs import *"
		}
		return "from . import outputs"
	}

	importPath := mod.getRelImportFromRoot()
	if mod.pkg.Name != parts[0] {
		importPath = fmt.Sprintf("pulumi_%s", refPkgName)
	}

	if modName == "" {
		imp, as := "outputs", "_root_outputs"
		if input {
			imp, as = "_inputs", "_root_inputs"
		}
		return fmt.Sprintf("from %s import %s as %s", importPath, imp, as)
	}

	components := strings.Split(modName, "/")
	return fmt.Sprintf("from %s import %[2]s as _%[2]s", importPath, components[0])
}

func (mod *modContext) importEnumFromToken(tok string) string {
	modName := mod.tokenToModule(tok)
	if modName == mod.mod {
		return "from ._enums import *"
	}

	importPath := mod.getRelImportFromRoot()

	if modName == "" {
		return fmt.Sprintf("from %s import _enums as _root_enums", importPath)
	}

	components := strings.Split(modName, "/")
	return fmt.Sprintf("from %s import %s", importPath, components[0])
}

func (mod *modContext) importResourceType(r *schema.ResourceType) string {
	if r.Resource != nil && r.Resource.Package != mod.pkg {
		return fmt.Sprintf("import %s", pyPack(r.Resource.Package.Name))
	}

	tok := r.Token
	parts := strings.Split(tok, ":")
	contract.Assert(len(parts) == 3)

	// If it's a provider resource, import the top-level package.
	if parts[0] == "pulumi" && parts[1] == "providers" {
		return fmt.Sprintf("import pulumi_%s", parts[2])
	}

	refPkgName := parts[0]

	modName := mod.tokenToResource(tok)

	importPath := mod.getRelImportFromRoot()
	if mod.pkg.Name != parts[0] {
		importPath = fmt.Sprintf("pulumi_%s", refPkgName)
	}

	name := PyName(tokenToName(r.Token))
	if mod.compatibility == kubernetes20 {
		// To maintain backward compatibility for kubernetes, the file names
		// need to be CamelCase instead of the standard snake_case.
		name = tokenToName(r.Token)
	}
	if r.Resource != nil && r.Resource.IsProvider {
		name = "provider"
	}

	components := strings.Split(modName, "/")
	return fmt.Sprintf("from %s%s import %s", importPath, name, components[0])
}

// genConfig emits all config variables in the given module, returning the resulting file.
func (mod *modContext) genConfig(variables []*schema.Property) (string, error) {
	w := &bytes.Buffer{}

	imports := imports{}
	mod.collectImports(variables, imports, false /*input*/)

	mod.genHeader(w, true /*needsSDK*/, imports)
	fmt.Fprintf(w, "import types\n")
	fmt.Fprintf(w, "\n")

	// Create a config bag for the variables to pull from.
	fmt.Fprintf(w, "__config__ = pulumi.Config('%s')\n", mod.pkg.Name)
	fmt.Fprintf(w, "\n\n")

	// To avoid a breaking change to the existing config getters, we define a class that extends
	// the `ModuleType` type and implements property getters for each config key. We then overwrite
	// the `__class__` attribute of the current module as described in the proposal for PEP-549. This allows
	// us to maintain the existing interface for users but implement dynamic getters behind the scenes.
	fmt.Fprintf(w, "class _ExportableConfig(types.ModuleType):\n")
	indent := "    "

	// Emit an entry for all config variables.
	for _, p := range variables {
		configFetch, err := genConfigFetch(p)
		if err != nil {
			return "", err
		}

		typeString := genConfigVarType(p)
		fmt.Fprintf(w, "%s@property\n", indent)
		fmt.Fprintf(w, "%sdef %s(self) -> %s:\n", indent, PyName(p.Name), typeString)
		dblIndent := strings.Repeat(indent, 2)

		printComment(w, p.Comment, dblIndent)
		fmt.Fprintf(w, "%sreturn %s\n", dblIndent, configFetch)
		fmt.Fprintf(w, "\n")
	}

	return w.String(), nil
}

func genConfigFetch(configVar *schema.Property) (string, error) {
	getFunc := "get"
	unwrappedType := codegen.UnwrapType(configVar.Type)
	switch unwrappedType {
	case schema.BoolType:
		getFunc = "get_bool"
	case schema.IntType:
		getFunc = "get_int"
	case schema.NumberType:
		getFunc = "get_float"
	}

	configFetch := fmt.Sprintf("__config__.%s('%s')", getFunc, configVar.Name)
	if configVar.DefaultValue != nil {
		v, err := getDefaultValue(configVar.DefaultValue, unwrappedType)
		if err != nil {
			return "", err
		}
		configFetch += " or " + v
	}
	return configFetch, nil
}

func genConfigVarType(configVar *schema.Property) string {
	// For historical reasons and to maintain backwards compatibility, the config variables for python
	// are typed as `Optional[str`] or `str` for complex objects since the getters only use config.get().
	// To return the rich objects would be a breaking change, tracked in https://github.com/pulumi/pulumi/issues/7493
	typeString := "str"
	switch codegen.UnwrapType(configVar.Type) {
	case schema.BoolType:
		typeString = "bool"
	case schema.IntType:
		typeString = "int"
	case schema.NumberType:
		typeString = "float"
	}

	if configVar.DefaultValue == nil || configVar.DefaultValue.Value == nil {
		typeString = "Optional[" + typeString + "]"
	}
	return typeString
}

// genConfigStubs emits all type information for the config variables in the given module, returning the resulting file.
// We do this because we lose IDE autocomplete by implementing the dynamic config getters described in genConfig.
// Emitting these stubs allows us to maintain type hints and autocomplete for users.
func (mod *modContext) genConfigStubs(variables []*schema.Property) (string, error) {
	w := &bytes.Buffer{}

	imports := imports{}
	mod.collectImports(variables, imports, false /*input*/)

	mod.genHeader(w, true /*needsSDK*/, imports)

	// Emit an entry for all config variables.
	for _, p := range variables {
		typeString := genConfigVarType(p)
		fmt.Fprintf(w, "%s: %s\n", p.Name, typeString)
		printComment(w, p.Comment, "")
		fmt.Fprintf(w, "\n")
	}

	return w.String(), nil
}

func (mod *modContext) genTypes(dir string, fs fs) error {
	genTypes := func(file string, input bool) error {
		w := &bytes.Buffer{}

		imports := imports{}
		for _, t := range mod.types {
			if input && mod.details(t).inputType {
				visitObjectTypes(t.Properties, func(t schema.Type) {
					switch t := t.(type) {
					case *schema.ObjectType:
						imports.addTypeIf(mod, t, true /*input*/, func(imp string) bool {
							// No need to import `._inputs` inside _inputs.py.
							return imp != "from ._inputs import *"
						})
					case *schema.EnumType:
						imports.addEnum(mod, t.Token)
					case *schema.ResourceType:
						imports.addResource(mod, t)
					}
				})
			}
			if !input && mod.details(t).outputType {
				mod.collectImports(t.Properties, imports, false /*input*/)
			}
		}
		for _, e := range mod.enums {
			imports.addEnum(mod, e.Token)
		}

		mod.genHeader(w, true /*needsSDK*/, imports)

		// Export only the symbols we want exported.
		fmt.Fprintf(w, "__all__ = [\n")
		for _, t := range mod.types {
			if input && mod.details(t).inputType || !input && mod.details(t).outputType {
				fmt.Fprintf(w, "    '%s',\n", mod.unqualifiedObjectTypeName(t, input))
			}
		}
		fmt.Fprintf(w, "]\n\n")

		var hasTypes bool
		for _, t := range mod.types {
			if input && mod.details(t).inputType {
				if err := mod.genObjectType(w, t, true); err != nil {
					return err
				}
				hasTypes = true
			}
			if !input && mod.details(t).outputType {
				if err := mod.genObjectType(w, t, false); err != nil {
					return err
				}
				hasTypes = true
			}
		}
		if hasTypes {
			fs.add(path.Join(dir, file), w.Bytes())
		}
		return nil
	}
	if err := genTypes("_inputs.py", true); err != nil {
		return err
	}
	if err := genTypes("outputs.py", false); err != nil {
		return err
	}
	return nil
}

func awaitableTypeNames(tok string) (baseName, awaitableName string) {
	baseName = pyClassName(tokenToName(tok))
	awaitableName = "Awaitable" + baseName
	return
}

func (mod *modContext) genAwaitableType(w io.Writer, obj *schema.ObjectType) string {
	baseName, awaitableName := awaitableTypeNames(obj.Token)

	// Produce a class definition with optional """ comment.
	fmt.Fprint(w, "@pulumi.output_type\n")
	fmt.Fprintf(w, "class %s:\n", baseName)
	printComment(w, obj.Comment, "    ")

	// Now generate an initializer with properties for all inputs.
	fmt.Fprintf(w, "    def __init__(__self__")
	for _, prop := range obj.Properties {
		fmt.Fprintf(w, ", %s=None", PyName(prop.Name))
	}
	fmt.Fprintf(w, "):\n")
	for _, prop := range obj.Properties {
		// Check that required arguments are present.  Also check that types are as expected.
		pname := PyName(prop.Name)
		ptype := mod.pyType(prop.Type)
		fmt.Fprintf(w, "        if %s and not isinstance(%s, %s):\n", pname, pname, ptype)
		fmt.Fprintf(w, "            raise TypeError(\"Expected argument '%s' to be a %s\")\n", pname, ptype)

		if prop.DeprecationMessage != "" {
			escaped := strings.ReplaceAll(prop.DeprecationMessage, `"`, `\"`)
			fmt.Fprintf(w, "        if %s is not None:\n", pname)
			fmt.Fprintf(w, "            warnings.warn(\"\"\"%s\"\"\", DeprecationWarning)\n", escaped)
			fmt.Fprintf(w, "            pulumi.log.warn(\"\"\"%s is deprecated: %s\"\"\")\n\n", pname, escaped)
		}

		// Now perform the assignment.
		fmt.Fprintf(w, "        pulumi.set(__self__, \"%[1]s\", %[1]s)\n", pname)
	}
	fmt.Fprintf(w, "\n")

	// Write out Python property getters for each property.
	mod.genProperties(w, obj.Properties, false /*setters*/, "", func(prop *schema.Property) string {
		return mod.typeString(prop.Type, false /*input*/, false /*acceptMapping*/)
	})

	// Produce an awaitable subclass.
	fmt.Fprint(w, "\n")
	fmt.Fprintf(w, "class %s(%s):\n", awaitableName, baseName)

	// Emit __await__ and __iter__ in order to make this type awaitable.
	//
	// Note that we need __await__ to be an iterator, but we only want it to return one value. As such, we use
	// `if False: yield` to construct this.
	//
	// We also need the result of __await__ to be a plain, non-awaitable value. We achieve this by returning a new
	// instance of the base class.
	fmt.Fprintf(w, "    # pylint: disable=using-constant-test\n")
	fmt.Fprintf(w, "    def __await__(self):\n")
	fmt.Fprintf(w, "        if False:\n")
	fmt.Fprintf(w, "            yield self\n")
	fmt.Fprintf(w, "        return %s(\n", baseName)
	for i, prop := range obj.Properties {
		if i > 0 {
			fmt.Fprintf(w, ",\n")
		}
		pname := PyName(prop.Name)
		fmt.Fprintf(w, "            %s=self.%s", pname, pname)
	}
	fmt.Fprintf(w, ")\n")

	return awaitableName
}

func resourceName(res *schema.Resource) string {
	name := pyClassName(tokenToName(res.Token))
	if res.IsProvider {
		name = "Provider"
	}
	return name
}

func (mod *modContext) genResource(res *schema.Resource) (string, error) {
	w := &bytes.Buffer{}

	imports := imports{}
	mod.collectImportsForResource(res.Properties, imports, false /*input*/, res)
	mod.collectImportsForResource(res.InputProperties, imports, true /*input*/, res)
	if res.StateInputs != nil {
		mod.collectImportsForResource(res.StateInputs.Properties, imports, true /*input*/, res)
	}
	for _, method := range res.Methods {
		if method.Function.Inputs != nil {
			mod.collectImportsForResource(method.Function.Inputs.Properties, imports, true /*input*/, res)
		}
		if method.Function.Outputs != nil {
			mod.collectImportsForResource(method.Function.Outputs.Properties, imports, false /*input*/, res)
		}
	}

	mod.genHeader(w, true /*needsSDK*/, imports)

	name := resourceName(res)

	resourceArgsName := fmt.Sprintf("%sArgs", name)
	// Some providers (e.g. Kubernetes) have types with the same name as resources (e.g. StorageClass in Kubernetes).
	// We've already shipped the input type (e.g. StorageClassArgs) in the same module as the resource, so we can't use
	// the same name for the resource's args class. When an input type exists that would conflict with the name of the
	// resource args class, we'll use a different name: `<Resource>InitArgs` instead of `<Resource>Args`.
	const alternateSuffix = "InitArgs"
	for _, t := range mod.types {
		if mod.details(t).inputType {
			if mod.unqualifiedObjectTypeName(t, true) == resourceArgsName {
				resourceArgsName = name + alternateSuffix
				break
			}
		}
	}
	// If we're using the alternate name, ensure the alternate name doesn't conflict with an input type.
	if strings.HasSuffix(resourceArgsName, alternateSuffix) {
		for _, t := range mod.types {
			if mod.details(t).inputType {
				if mod.unqualifiedObjectTypeName(t, true) == resourceArgsName {
					return "", errors.Errorf(
						"resource args class named %s in %s conflicts with input type", resourceArgsName, mod.mod)
				}
			}
		}
	}

	// Export only the symbols we want exported.
	fmt.Fprintf(w, "__all__ = ['%s', '%s']\n\n", resourceArgsName, name)

	// Produce an args class.
	argsComment := fmt.Sprintf("The set of arguments for constructing a %s resource.", name)
	err := mod.genType(w, resourceArgsName, argsComment, res.InputProperties, true, false)
	if err != nil {
		return "", err
	}

	// Produce an unexported state class. It's currently only used internally inside the `get` method to opt-in to
	// the type/name metadata based translation behavior.
	// We can consider making use of it publicly in the future: removing the underscore prefix, exporting it from
	// `__all__`, and adding a static `get` overload that accepts it as an argument.
	hasStateInputs := !res.IsProvider && !res.IsComponent && res.StateInputs != nil &&
		len(res.StateInputs.Properties) > 0
	if hasStateInputs {
		stateComment := fmt.Sprintf("Input properties used for looking up and filtering %s resources.", name)
		err = mod.genType(w, fmt.Sprintf("_%sState", name), stateComment, res.StateInputs.Properties, true, false)
		if err != nil {
			return "", err
		}
	}

	var baseType string
	switch {
	case res.IsProvider:
		baseType = "pulumi.ProviderResource"
	case res.IsComponent:
		baseType = "pulumi.ComponentResource"
	default:
		baseType = "pulumi.CustomResource"
	}

	if !res.IsProvider && res.DeprecationMessage != "" && mod.compatibility != kubernetes20 {
		escaped := strings.ReplaceAll(res.DeprecationMessage, `"`, `\"`)
		fmt.Fprintf(w, "warnings.warn(\"\"\"%s\"\"\", DeprecationWarning)\n\n\n", escaped)
	}

	// Produce a class definition with optional """ comment.
	fmt.Fprintf(w, "class %s(%s):\n", name, baseType)
	if res.DeprecationMessage != "" && mod.compatibility != kubernetes20 {
		escaped := strings.ReplaceAll(res.DeprecationMessage, `"`, `\"`)
		fmt.Fprintf(w, "    warnings.warn(\"\"\"%s\"\"\", DeprecationWarning)\n\n", escaped)
	}

	// Determine if all inputs are optional.
	allOptionalInputs := true
	for _, prop := range res.InputProperties {
		allOptionalInputs = allOptionalInputs && !prop.IsRequired()
	}

	// Emit __init__ overloads and implementation...

	// Helper for generating an init method with inputs as function arguments.
	emitInitMethodSignature := func(methodName string) {
		fmt.Fprintf(w, "    def %s(__self__,\n", methodName)
		fmt.Fprintf(w, "                 resource_name: str,\n")
		fmt.Fprintf(w, "                 opts: Optional[pulumi.ResourceOptions] = None")

		// If there's an argument type, emit it.
		for _, prop := range res.InputProperties {
			ty := mod.typeString(codegen.OptionalType(prop), true, true /*acceptMapping*/)
			fmt.Fprintf(w, ",\n                 %s: %s = None", InitParamName(prop.Name), ty)
		}

		fmt.Fprintf(w, ",\n                 __props__=None):\n")
	}

	// Emit an __init__ overload that accepts the resource's inputs as function arguments.
	fmt.Fprintf(w, "    @overload\n")
	emitInitMethodSignature("__init__")
	mod.genInitDocstring(w, res, resourceArgsName, false /*argsOverload*/)
	fmt.Fprintf(w, "        ...\n")

	// Emit an __init__ overload that accepts the resource's inputs from the args class.
	fmt.Fprintf(w, "    @overload\n")
	fmt.Fprintf(w, "    def __init__(__self__,\n")
	fmt.Fprintf(w, "                 resource_name: str,\n")
	if allOptionalInputs {
		fmt.Fprintf(w, "                 args: Optional[%s] = None,\n", resourceArgsName)
	} else {
		fmt.Fprintf(w, "                 args: %s,\n", resourceArgsName)
	}
	fmt.Fprintf(w, "                 opts: Optional[pulumi.ResourceOptions] = None):\n")
	mod.genInitDocstring(w, res, resourceArgsName, true /*argsOverload*/)
	fmt.Fprintf(w, "        ...\n")

	// Emit the actual implementation of __init__, which does the appropriate thing based on which
	// overload was called.
	fmt.Fprintf(w, "    def __init__(__self__, resource_name: str, *args, **kwargs):\n")
	fmt.Fprintf(w, "        resource_args, opts = _utilities.get_resource_args_opts(%s, pulumi.ResourceOptions, *args, **kwargs)\n", resourceArgsName)
	fmt.Fprintf(w, "        if resource_args is not None:\n")
	fmt.Fprintf(w, "            __self__._internal_init(resource_name, opts, **resource_args.__dict__)\n")
	fmt.Fprintf(w, "        else:\n")
	fmt.Fprintf(w, "            __self__._internal_init(resource_name, *args, **kwargs)\n")
	fmt.Fprintf(w, "\n")

	// Emit the _internal_init helper method which provides the bulk of the __init__ implementation.
	emitInitMethodSignature("_internal_init")
	if res.DeprecationMessage != "" && mod.compatibility != kubernetes20 {
		fmt.Fprintf(w, "        pulumi.log.warn(\"\"\"%s is deprecated: %s\"\"\")\n", name, res.DeprecationMessage)
	}
	fmt.Fprintf(w, "        if opts is None:\n")
	fmt.Fprintf(w, "            opts = pulumi.ResourceOptions()\n")
	fmt.Fprintf(w, "        if not isinstance(opts, pulumi.ResourceOptions):\n")
	fmt.Fprintf(w, "            raise TypeError('Expected resource options to be a ResourceOptions instance')\n")
	fmt.Fprintf(w, "        if opts.version is None:\n")
	fmt.Fprintf(w, "            opts.version = _utilities.get_version()\n")
	if res.IsComponent {
		fmt.Fprintf(w, "        if opts.id is not None:\n")
		fmt.Fprintf(w, "            raise ValueError('ComponentResource classes do not support opts.id')\n")
		fmt.Fprintf(w, "        else:\n")
	} else {
		fmt.Fprintf(w, "        if opts.id is None:\n")
	}
	fmt.Fprintf(w, "            if __props__ is not None:\n")
	fmt.Fprintf(w, "                raise TypeError(")
	fmt.Fprintf(w, "'__props__ is only valid when passed in combination with a valid opts.id to get an existing resource')\n")

	// We use an instance of the `<Resource>Args` class for `__props__` to opt-in to the type/name metadata based
	// translation behavior. The instance is created using `__new__` to avoid any validation in the `__init__` method,
	// values are set directly on its `__dict__`, including any additional output properties.
	fmt.Fprintf(w, "            __props__ = %[1]s.__new__(%[1]s)\n\n", resourceArgsName)
	fmt.Fprintf(w, "")

	ins := codegen.NewStringSet()
	for _, prop := range res.InputProperties {
		pname := InitParamName(prop.Name)
		var arg interface{}
		var err error

		// Fill in computed defaults for arguments.
		if prop.DefaultValue != nil {
			dv, err := getDefaultValue(prop.DefaultValue, codegen.UnwrapType(prop.Type))
			if err != nil {
				return "", err
			}
			fmt.Fprintf(w, "            if %s is None:\n", pname)
			fmt.Fprintf(w, "                %s = %s\n", pname, dv)
		}

		// Check that required arguments are present.
		if prop.IsRequired() {
			fmt.Fprintf(w, "            if %s is None and not opts.urn:\n", pname)
			fmt.Fprintf(w, "                raise TypeError(\"Missing required property '%s'\")\n", pname)
		}

		// Check that the property isn't deprecated
		if prop.DeprecationMessage != "" {
			escaped := strings.ReplaceAll(prop.DeprecationMessage, `"`, `\"`)
			fmt.Fprintf(w, "            if %s is not None and not opts.urn:\n", pname)
			fmt.Fprintf(w, "                warnings.warn(\"\"\"%s\"\"\", DeprecationWarning)\n", escaped)
			fmt.Fprintf(w, "                pulumi.log.warn(\"\"\"%s is deprecated: %s\"\"\")\n", pname, escaped)
		}

		// And add it to the dictionary.
		arg = pname

		if prop.ConstValue != nil {
			arg, err = getConstValue(prop.ConstValue)
			if err != nil {
				return "", err
			}
		}

		// If this resource is a provider then, regardless of the schema of the underlying provider
		// type, we must project all properties as strings. For all properties that are not strings,
		// we'll marshal them to JSON and use the JSON string as a string input.
		if res.IsProvider && !isStringType(prop.Type) {
			arg = fmt.Sprintf("pulumi.Output.from_input(%s).apply(pulumi.runtime.to_json) if %s is not None else None", arg, arg)
		}
		name := PyName(prop.Name)
		if prop.Secret {
			fmt.Fprintf(w, "            __props__.__dict__[%[1]q] = None if %[2]s is None else pulumi.Output.secret(%[2]s)\n", name, arg)
		} else {
			fmt.Fprintf(w, "            __props__.__dict__[%q] = %s\n", name, arg)
		}

		ins.Add(prop.Name)
	}

	var secretProps []string
	for _, prop := range res.Properties {
		// Default any pure output properties to None.  This ensures they are available as properties, even if
		// they don't ever get assigned a real value, and get documentation if available.
		if !ins.Has(prop.Name) {
			fmt.Fprintf(w, "            __props__.__dict__[%q] = None\n", PyName(prop.Name))
		}

		if prop.Secret {
			secretProps = append(secretProps, prop.Name)
		}
	}

	if len(res.Aliases) > 0 {
		fmt.Fprintf(w, `        alias_opts = pulumi.ResourceOptions(aliases=[`)

		for i, alias := range res.Aliases {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			mod.writeAlias(w, alias)
		}

		fmt.Fprintf(w, "])\n")
		fmt.Fprintf(w, "        opts = pulumi.ResourceOptions.merge(opts, alias_opts)\n")
	}

	if len(secretProps) > 0 {
		fmt.Fprintf(w, `        secret_opts = pulumi.ResourceOptions(additional_secret_outputs=["%s"])`, strings.Join(secretProps, `", "`))
		fmt.Fprintf(w, "\n        opts = pulumi.ResourceOptions.merge(opts, secret_opts)\n")
	}

	// Finally, chain to the base constructor, which will actually register the resource.
	tok := res.Token
	if res.IsProvider {
		tok = mod.pkg.Name
	}
	fmt.Fprintf(w, "        super(%s, __self__).__init__(\n", name)
	fmt.Fprintf(w, "            '%s',\n", tok)
	fmt.Fprintf(w, "            resource_name,\n")
	fmt.Fprintf(w, "            __props__,\n")
	if res.IsComponent {
		fmt.Fprintf(w, "            opts,\n")
		fmt.Fprintf(w, "            remote=True)\n")
	} else {
		fmt.Fprintf(w, "            opts)\n")
	}
	fmt.Fprintf(w, "\n")

	if !res.IsProvider && !res.IsComponent {
		fmt.Fprintf(w, "    @staticmethod\n")
		fmt.Fprintf(w, "    def get(resource_name: str,\n")
		fmt.Fprintf(w, "            id: pulumi.Input[str],\n")
		fmt.Fprintf(w, "            opts: Optional[pulumi.ResourceOptions] = None")

		if hasStateInputs {
			for _, prop := range res.StateInputs.Properties {
				pname := InitParamName(prop.Name)
				ty := mod.typeString(codegen.OptionalType(prop), true, true /*acceptMapping*/)
				fmt.Fprintf(w, ",\n            %s: %s = None", pname, ty)
			}
		}
		fmt.Fprintf(w, ") -> '%s':\n", name)
		mod.genGetDocstring(w, res)
		fmt.Fprintf(w,
			"        opts = pulumi.ResourceOptions.merge(opts, pulumi.ResourceOptions(id=id))\n")
		fmt.Fprintf(w, "\n")
		if hasStateInputs {
			fmt.Fprintf(w, "        __props__ = _%[1]sState.__new__(_%[1]sState)\n\n", name)
		} else {
			// If we don't have any state inputs, we'll just instantiate the `<Resource>Args` class,
			// to opt-in to the improved translation behavior.
			fmt.Fprintf(w, "        __props__ = %[1]s.__new__(%[1]s)\n\n", resourceArgsName)
		}

		stateInputs := codegen.NewStringSet()
		if res.StateInputs != nil {
			for _, prop := range res.StateInputs.Properties {
				stateInputs.Add(prop.Name)
				fmt.Fprintf(w, "        __props__.__dict__[%q] = %s\n", PyName(prop.Name), InitParamName(prop.Name))
			}
		}
		for _, prop := range res.Properties {
			if !stateInputs.Has(prop.Name) {
				fmt.Fprintf(w, "        __props__.__dict__[%q] = None\n", PyName(prop.Name))
			}
		}

		fmt.Fprintf(w, "        return %s(resource_name, opts=opts, __props__=__props__)\n\n", name)
	}

	// Write out Python property getters for each of the resource's properties.
	mod.genProperties(w, res.Properties, false /*setters*/, "", func(prop *schema.Property) string {
		ty := mod.typeString(prop.Type, false /*input*/, false /*acceptMapping*/)
		return fmt.Sprintf("pulumi.Output[%s]", ty)
	})

	// Write out methods.
	mod.genMethods(w, res)

	return w.String(), nil
}

func (mod *modContext) genProperties(w io.Writer, properties []*schema.Property, setters bool, indent string,
	propType func(prop *schema.Property) string) {
	// Write out Python properties for each property. If there is a property named "property", it will
	// be emitted last to avoid conflicting with the built-in `@property` decorator function. We do
	// this instead of importing `builtins` and fully qualifying the decorator as `@builtins.property`
	// because that wouldn't address the problem if there was a property named "builtins".
	emitProp := func(pname string, prop *schema.Property) {
		ty := propType(prop)
		fmt.Fprintf(w, "%s    @property\n", indent)
		if pname == prop.Name {
			fmt.Fprintf(w, "%s    @pulumi.getter\n", indent)
		} else {
			fmt.Fprintf(w, "%s    @pulumi.getter(name=%q)\n", indent, prop.Name)
		}
		fmt.Fprintf(w, "%s    def %s(self) -> %s:\n", indent, pname, ty)
		if prop.Comment != "" {
			printComment(w, prop.Comment, indent+"        ")
		}
		fmt.Fprintf(w, "%s        return pulumi.get(self, %q)\n\n", indent, pname)

		if setters {
			fmt.Fprintf(w, "%s    @%s.setter\n", indent, pname)
			fmt.Fprintf(w, "%s    def %s(self, value: %s):\n", indent, pname, ty)
			fmt.Fprintf(w, "%s        pulumi.set(self, %q, value)\n\n", indent, pname)
		}
	}
	var propNamedProperty *schema.Property
	for _, prop := range properties {
		pname := PyName(prop.Name)
		// If there is a property named "property", skip it, and emit it last.
		if pname == "property" {
			propNamedProperty = prop
			continue
		}
		emitProp(pname, prop)
	}
	if propNamedProperty != nil {
		emitProp("property", propNamedProperty)
	}
}

func (mod *modContext) genMethods(w io.Writer, res *schema.Resource) {
	genReturnType := func(method *schema.Method) string {
		obj := method.Function.Outputs
		name := pyClassName(title(method.Name)) + "Result"

		// Produce a class definition with optional """ comment.
		fmt.Fprintf(w, "    @pulumi.output_type\n")
		fmt.Fprintf(w, "    class %s:\n", name)
		printComment(w, obj.Comment, "        ")

		// Now generate an initializer with properties for all inputs.
		fmt.Fprintf(w, "        def __init__(__self__")
		for _, prop := range obj.Properties {
			fmt.Fprintf(w, ", %s=None", PyName(prop.Name))
		}
		fmt.Fprintf(w, "):\n")
		for _, prop := range obj.Properties {
			// Check that required arguments are present.  Also check that types are as expected.
			pname := PyName(prop.Name)
			ptype := mod.pyType(prop.Type)
			fmt.Fprintf(w, "            if %s and not isinstance(%s, %s):\n", pname, pname, ptype)
			fmt.Fprintf(w, "                raise TypeError(\"Expected argument '%s' to be a %s\")\n", pname, ptype)

			if prop.DeprecationMessage != "" {
				escaped := strings.ReplaceAll(prop.DeprecationMessage, `"`, `\"`)
				fmt.Fprintf(w, "            if %s is not None:\n", pname)
				fmt.Fprintf(w, "                warnings.warn(\"\"\"%s\"\"\", DeprecationWarning)\n", escaped)
				fmt.Fprintf(w, "                pulumi.log.warn(\"\"\"%s is deprecated: %s\"\"\")\n\n", pname, escaped)
			}

			// Now perform the assignment.
			fmt.Fprintf(w, "            pulumi.set(__self__, \"%[1]s\", %[1]s)\n", pname)
		}
		fmt.Fprintf(w, "\n")

		// Write out Python property getters for each property.
		mod.genProperties(w, obj.Properties, false /*setters*/, "    ", func(prop *schema.Property) string {
			return mod.typeString(prop.Type, false /*input*/, false /*acceptMapping*/)
		})

		return name
	}

	genMethod := func(method *schema.Method) {
		methodName := PyName(method.Name)
		fun := method.Function

		// If there is a return type, emit it.
		var retTypeName, retTypeNameQualified, retTypeNameQualifiedOutput string
		if fun.Outputs != nil {
			retTypeName = genReturnType(method)
			retTypeNameQualified = fmt.Sprintf("%s.%s", resourceName(res), retTypeName)
			retTypeNameQualifiedOutput = fmt.Sprintf("pulumi.Output['%s']", retTypeNameQualified)
		}

		var args []*schema.Property
		if fun.Inputs != nil {
			// Filter out the __self__ argument from the inputs.
			args = make([]*schema.Property, 0, len(fun.Inputs.InputShape.Properties)-1)
			for _, arg := range fun.Inputs.InputShape.Properties {
				if arg.Name == "__self__" {
					continue
				}
				args = append(args, arg)
			}
			// Sort required args first.
			sort.Slice(args, func(i, j int) bool {
				pi, pj := args[i], args[j]
				switch {
				case pi.IsRequired() != pj.IsRequired():
					return pi.IsRequired() && !pj.IsRequired()
				default:
					return pi.Name < pj.Name
				}
			})
		}

		// Write out the function signature.
		def := fmt.Sprintf("    def %s(", methodName)
		var indent string
		if len(args) > 0 {
			indent = strings.Repeat(" ", len(def))
		}
		fmt.Fprintf(w, "%s__self__", def)
		// Bare `*` argument to force callers to use named arguments.
		if len(args) > 0 {
			fmt.Fprintf(w, ", *")
		}
		for _, arg := range args {
			pname := PyName(arg.Name)
			ty := mod.typeString(arg.Type, true, false /*acceptMapping*/)
			var defaultValue string
			if !arg.IsRequired() {
				defaultValue = " = None"
			}
			fmt.Fprintf(w, ",\n%s%s: %s%s", indent, pname, ty, defaultValue)
		}
		if retTypeNameQualifiedOutput != "" {
			fmt.Fprintf(w, ") -> %s:\n", retTypeNameQualifiedOutput)
		} else {
			fmt.Fprintf(w, ") -> None:\n")
		}

		// If this func has documentation, write it at the top of the docstring, otherwise use a generic comment.
		docs := &bytes.Buffer{}
		if fun.Comment != "" {
			fmt.Fprintln(docs, codegen.FilterExamples(fun.Comment, "python"))
		}
		if len(args) > 0 {
			fmt.Fprintln(docs, "")
			for _, arg := range args {
				mod.genPropDocstring(docs, PyName(arg.Name), arg, false /*acceptMapping*/)
			}
		}
		printComment(w, docs.String(), "        ")

		if fun.DeprecationMessage != "" {
			fmt.Fprintf(w, "        pulumi.log.warn(\"\"\"%s is deprecated: %s\"\"\")\n", methodName,
				fun.DeprecationMessage)
		}

		// Copy the function arguments into a dictionary.
		fmt.Fprintf(w, "        __args__ = dict()\n")
		fmt.Fprintf(w, "        __args__['__self__'] = __self__\n")
		for _, arg := range args {
			pname := PyName(arg.Name)
			fmt.Fprintf(w, "        __args__['%s'] = %s\n", arg.Name, pname)
		}

		// Now simply call the function with the arguments.
		var typ, ret string
		if retTypeNameQualified != "" {
			// Pass along the private output_type we generated, so any nested output classes are instantiated by
			// the call.
			typ = fmt.Sprintf(", typ=%s", retTypeNameQualified)
			ret = "return "
		}
		fmt.Fprintf(w, "        %spulumi.runtime.call('%s', __args__, res=__self__%s)\n", ret, fun.Token, typ)
		fmt.Fprintf(w, "\n")
	}

	for _, method := range res.Methods {
		genMethod(method)
	}
}

func (mod *modContext) writeAlias(w io.Writer, alias *schema.Alias) {
	fmt.Fprint(w, "pulumi.Alias(")
	parts := []string{}
	if alias.Name != nil {
		parts = append(parts, fmt.Sprintf("name=\"%v\"", *alias.Name))
	}
	if alias.Project != nil {
		parts = append(parts, fmt.Sprintf("project=\"%v\"", *alias.Project))
	}
	if alias.Type != nil {
		parts = append(parts, fmt.Sprintf("type_=\"%v\"", *alias.Type))
	}

	for i, part := range parts {
		if i > 0 {
			fmt.Fprint(w, ", ")
		}
		fmt.Fprint(w, part)
	}
	fmt.Fprint(w, ")")
}

func (mod *modContext) genFunction(fun *schema.Function) (string, error) {
	w := &bytes.Buffer{}

	imports := imports{}
	if fun.Inputs != nil {
		mod.collectImports(fun.Inputs.Properties, imports, true)
	}
	if fun.Outputs != nil {
		mod.collectImports(fun.Outputs.Properties, imports, false)
	}

	mod.genHeader(w, true /*needsSDK*/, imports)

	var baseName, awaitableName string
	if fun.Outputs != nil {
		baseName, awaitableName = awaitableTypeNames(fun.Outputs.Token)
	}
	name := PyName(tokenToName(fun.Token))

	// Export only the symbols we want exported.
	fmt.Fprintf(w, "__all__ = [\n")
	if fun.Outputs != nil {
		fmt.Fprintf(w, "    '%s',\n", baseName)
		fmt.Fprintf(w, "    '%s',\n", awaitableName)
	}
	fmt.Fprintf(w, "    '%s',\n", name)
	fmt.Fprintf(w, "]\n\n")

	if fun.DeprecationMessage != "" {
		escaped := strings.ReplaceAll(fun.DeprecationMessage, `"`, `\"`)
		fmt.Fprintf(w, "warnings.warn(\"\"\"%s\"\"\", DeprecationWarning)\n\n", escaped)
	}

	// If there is a return type, emit it.
	retTypeName := ""
	var rets []*schema.Property
	if fun.Outputs != nil {
		retTypeName, rets = mod.genAwaitableType(w, fun.Outputs), fun.Outputs.Properties
		fmt.Fprintf(w, "\n\n")
	}

	var args []*schema.Property
	if fun.Inputs != nil {
		args = fun.Inputs.Properties
	}

	// Write out the function signature.
	def := fmt.Sprintf("def %s(", name)
	var indent string
	if len(args) > 0 {
		indent = strings.Repeat(" ", len(def))
	}
	fmt.Fprintf(w, def)
	for i, arg := range args {
		var ind string
		if i != 0 {
			ind = indent
		}
		pname := PyName(arg.Name)
		ty := mod.typeString(codegen.OptionalType(arg), true, true /*acceptMapping*/)
		fmt.Fprintf(w, "%s%s: %s = None,\n", ind, pname, ty)
	}
	fmt.Fprintf(w, "%sopts: Optional[pulumi.InvokeOptions] = None", indent)
	if retTypeName != "" {
		fmt.Fprintf(w, ") -> %s:\n", retTypeName)
	} else {
		fmt.Fprintf(w, "):\n")
	}

	// If this func has documentation, write it at the top of the docstring, otherwise use a generic comment.
	docs := &bytes.Buffer{}
	if fun.Comment != "" {
		fmt.Fprintln(docs, codegen.FilterExamples(fun.Comment, "python"))
	} else {
		fmt.Fprintln(docs, "Use this data source to access information about an existing resource.")
	}
	if len(args) > 0 {
		fmt.Fprintln(docs, "")
		for _, arg := range args {
			mod.genPropDocstring(docs, PyName(arg.Name), arg, true /*acceptMapping*/)
		}
	}
	printComment(w, docs.String(), "    ")

	if fun.DeprecationMessage != "" {
		fmt.Fprintf(w, "    pulumi.log.warn(\"\"\"%s is deprecated: %s\"\"\")\n", name, fun.DeprecationMessage)
	}

	// Copy the function arguments into a dictionary.
	fmt.Fprintf(w, "    __args__ = dict()\n")
	for _, arg := range args {
		// TODO: args validation.
		fmt.Fprintf(w, "    __args__['%s'] = %s\n", arg.Name, PyName(arg.Name))
	}

	// If the caller explicitly specified a version, use it, otherwise inject this package's version.
	fmt.Fprintf(w, "    if opts is None:\n")
	fmt.Fprintf(w, "        opts = pulumi.InvokeOptions()\n")
	fmt.Fprintf(w, "    if opts.version is None:\n")
	fmt.Fprintf(w, "        opts.version = _utilities.get_version()\n")

	// Now simply invoke the runtime function with the arguments.
	var typ string
	if fun.Outputs != nil {
		// Pass along the private output_type we generated, so any nested outputs classes are instantiated by
		// the call to invoke.
		typ = fmt.Sprintf(", typ=%s", baseName)
	}
	fmt.Fprintf(w, "    __ret__ = pulumi.runtime.invoke('%s', __args__, opts=opts%s).value\n", fun.Token, typ)
	fmt.Fprintf(w, "\n")

	// And copy the results to an object, if there are indeed any expected returns.
	if fun.Outputs != nil {
		fmt.Fprintf(w, "    return %s(\n", retTypeName)
		for i, ret := range rets {
			// Use the get_dict_value utility instead of calling __ret__.get directly in case the __ret__
			// object has a get property that masks the underlying dict subclass's get method.
			fmt.Fprintf(w, "        %[1]s=__ret__.%[1]s", PyName(ret.Name))
			if i == len(rets)-1 {
				fmt.Fprintf(w, ")\n")
			} else {
				fmt.Fprintf(w, ",\n")
			}
		}
	}

	return w.String(), nil
}

func (mod *modContext) genEnums(w io.Writer, enums []*schema.EnumType) error {
	// Header
	mod.genHeader(w, false /*needsSDK*/, nil)

	// Enum import
	fmt.Fprintf(w, "from enum import Enum\n\n")

	// Export only the symbols we want exported.
	fmt.Fprintf(w, "__all__ = [\n")
	for _, enum := range enums {
		fmt.Fprintf(w, "    '%s',\n", tokenToName(enum.Token))

	}
	fmt.Fprintf(w, "]\n\n\n")

	for i, enum := range enums {
		if err := mod.genEnum(w, enum); err != nil {
			return err
		}
		if i != len(enums)-1 {
			fmt.Fprintf(w, "\n\n")
		}
	}
	return nil
}

func (mod *modContext) genEnum(w io.Writer, enum *schema.EnumType) error {
	indent := "    "
	enumName := tokenToName(enum.Token)
	underlyingType := mod.typeString(enum.ElementType, false, false)

	switch enum.ElementType {
	case schema.StringType, schema.IntType, schema.NumberType:
		fmt.Fprintf(w, "class %s(%s, Enum):\n", enumName, underlyingType)
		printComment(w, enum.Comment, indent)
		for _, e := range enum.Elements {
			// If the enum doesn't have a name, set the value as the name.
			if e.Name == "" {
				e.Name = fmt.Sprintf("%v", e.Value)
			}

			name, err := makeSafeEnumName(e.Name, enumName)
			if err != nil {
				return err
			}
			e.Name = name

			fmt.Fprintf(w, "%s%s = ", indent, e.Name)
			if val, ok := e.Value.(string); ok {
				fmt.Fprintf(w, "%q\n", val)
			} else {
				fmt.Fprintf(w, "%v\n", e.Value)
			}
			if e.Comment != "" {
				fmt.Fprintf(w, "%s\"\"\"%s\"\"\"\n", indent, e.Comment)
			}
		}
	default:
		return errors.Errorf("enums of type %s are not yet implemented for this language", enum.ElementType.String())
	}

	return nil
}

func visitObjectTypes(properties []*schema.Property, visitor func(objectOrResource schema.Type)) {
	codegen.VisitTypeClosure(properties, func(t schema.Type) {
		switch st := t.(type) {
		case *schema.EnumType, *schema.ObjectType, *schema.ResourceType:
			visitor(st)
		}
	})
}

func (mod *modContext) collectImports(properties []*schema.Property, imports imports, input bool) {
	mod.collectImportsForResource(properties, imports, input, nil)
}

func (mod *modContext) collectImportsForResource(properties []*schema.Property, imports imports, input bool,
	res *schema.Resource) {
	codegen.VisitTypeClosure(properties, func(t schema.Type) {
		switch t := t.(type) {
		case *schema.ObjectType:
			imports.addType(mod, t, input)
		case *schema.EnumType:
			imports.addEnum(mod, t.Token)
		case *schema.ResourceType:
			// Don't import itself.
			if t.Resource != res {
				imports.addResource(mod, t)
			}
		}
	})
}

var requirementRegex = regexp.MustCompile(`^>=([^,]+),<[^,]+$`)
var pep440AlphaRegex = regexp.MustCompile(`^(\d+\.\d+\.\d)+a(\d+)$`)
var pep440BetaRegex = regexp.MustCompile(`^(\d+\.\d+\.\d+)b(\d+)$`)
var pep440RCRegex = regexp.MustCompile(`^(\d+\.\d+\.\d+)rc(\d+)$`)
var pep440DevRegex = regexp.MustCompile(`^(\d+\.\d+\.\d+)\.dev(\d+)$`)

var oldestAllowedPulumi = semver.Version{
	Major: 0,
	Minor: 17,
	Patch: 28,
}

func sanitizePackageDescription(description string) string {
	lines := strings.SplitN(description, "\n", 2)
	if len(lines) > 0 {
		return lines[0]
	}
	return ""
}

func genPulumiPluginFile(pkg *schema.Package) ([]byte, error) {
	plugin := &plugin.PulumiPluginJSON{
		Resource: true,
		Name:     pkg.Name,
		Version:  "${PLUGIN_VERSION}",
		Server:   pkg.PluginDownloadURL,
	}
	return plugin.JSON()
}

// genPackageMetadata generates all the non-code metadata required by a Pulumi package.
func genPackageMetadata(
	tool string, pkg *schema.Package, pyPkgName string, emitPulumiPluginFile bool, requires map[string]string) (string, error) {

	w := &bytes.Buffer{}
	(&modContext{tool: tool}).genHeader(w, false /*needsSDK*/, nil)

	// Now create a standard Python package from the metadata.
	fmt.Fprintf(w, "import errno\n")
	fmt.Fprintf(w, "from setuptools import setup, find_packages\n")
	fmt.Fprintf(w, "from setuptools.command.install import install\n")
	fmt.Fprintf(w, "from subprocess import check_call\n")
	fmt.Fprintf(w, "\n\n")

	// Create a constant for the version number to replace during build
	fmt.Fprintf(w, "VERSION = \"0.0.0\"\n")
	fmt.Fprintf(w, "PLUGIN_VERSION = \"0.0.0\"\n\n")

	// Create a command that will install the Pulumi plugin for this resource provider.
	fmt.Fprintf(w, "class InstallPluginCommand(install):\n")
	fmt.Fprintf(w, "    def run(self):\n")
	fmt.Fprintf(w, "        install.run(self)\n")
	fmt.Fprintf(w, "        try:\n")
	if pkg.PluginDownloadURL == "" {
		fmt.Fprintf(w, "            check_call(['pulumi', 'plugin', 'install', 'resource', '%s', PLUGIN_VERSION])\n", pkg.Name)
	} else {
		fmt.Fprintf(w, "            check_call(['pulumi', 'plugin', 'install', 'resource', '%s', PLUGIN_VERSION, '--server', '%s'])\n", pkg.Name, pkg.PluginDownloadURL)
	}
	fmt.Fprintf(w, "        except OSError as error:\n")
	fmt.Fprintf(w, "            if error.errno == errno.ENOENT:\n")
	fmt.Fprintf(w, "                print(f\"\"\"\n")
	fmt.Fprintf(w, "                There was an error installing the %s resource provider plugin.\n", pkg.Name)
	fmt.Fprintf(w, "                It looks like `pulumi` is not installed on your system.\n")
	fmt.Fprintf(w, "                Please visit https://pulumi.com/ to install the Pulumi CLI.\n")
	fmt.Fprintf(w, "                You may try manually installing the plugin by running\n")
	fmt.Fprintf(w, "                `pulumi plugin install resource %s {PLUGIN_VERSION}`\n", pkg.Name)
	fmt.Fprintf(w, "                \"\"\")\n")
	fmt.Fprintf(w, "            else:\n")
	fmt.Fprintf(w, "                raise\n")
	fmt.Fprintf(w, "\n\n")

	// Generate a readme method which will load README.rst, we use this to fill out the
	// long_description field in the setup call.
	fmt.Fprintf(w, "def readme():\n")
	fmt.Fprintf(w, "    try:\n")
	fmt.Fprintf(w, "        with open('README.md', encoding='utf-8') as f:\n")
	fmt.Fprintf(w, "            return f.read()\n")
	fmt.Fprintf(w, "    except FileNotFoundError:\n")
	fmt.Fprintf(w, "        return \"%s Pulumi Package - Development Version\"\n", pkg.Name)
	fmt.Fprintf(w, "\n\n")

	// Finally, the actual setup part.
	fmt.Fprintf(w, "setup(name='%s',\n", pyPkgName)
	fmt.Fprintf(w, "      version=VERSION,\n")
	if pkg.Description != "" {
		fmt.Fprintf(w, "      description=%q,\n", sanitizePackageDescription(pkg.Description))
	}
	fmt.Fprintf(w, "      long_description=readme(),\n")
	fmt.Fprintf(w, "      long_description_content_type='text/markdown',\n")
	fmt.Fprintf(w, "      cmdclass={\n")
	fmt.Fprintf(w, "          'install': InstallPluginCommand,\n")
	fmt.Fprintf(w, "      },\n")
	if pkg.Keywords != nil {
		fmt.Fprintf(w, "      keywords='")
		for i, kw := range pkg.Keywords {
			if i > 0 {
				fmt.Fprint(w, " ")
			}
			fmt.Fprint(w, kw)
		}
		fmt.Fprintf(w, "',\n")
	}
	if pkg.Homepage != "" {
		fmt.Fprintf(w, "      url='%s',\n", pkg.Homepage)
	}
	if pkg.Repository != "" {
		fmt.Fprintf(w, "      project_urls={\n")
		fmt.Fprintf(w, "          'Repository': '%s'\n", pkg.Repository)
		fmt.Fprintf(w, "      },\n")
	}
	if pkg.License != "" {
		fmt.Fprintf(w, "      license='%s',\n", pkg.License)
	}
	fmt.Fprintf(w, "      packages=find_packages(),\n")

	// Publish type metadata: PEP 561
	fmt.Fprintf(w, "      package_data={\n")
	fmt.Fprintf(w, "          '%s': [\n", pyPkgName)
	fmt.Fprintf(w, "              'py.typed',\n")
	if emitPulumiPluginFile {
		fmt.Fprintf(w, "              'pulumiplugin.json',\n")
	}

	fmt.Fprintf(w, "          ]\n")
	fmt.Fprintf(w, "      },\n")

	// Ensure that the Pulumi SDK has an entry if not specified. If the SDK _is_ specified, ensure
	// that it specifies an acceptable version range.
	if pulumiReq, ok := requires["pulumi"]; ok {
		// We expect a specific pattern of ">=version,<version" here.
		matches := requirementRegex.FindStringSubmatch(pulumiReq)
		if len(matches) != 2 {
			return "", errors.Errorf("invalid requirement specifier \"%s\"; expected \">=version1,<version2\"", pulumiReq)
		}

		lowerBound, err := pep440VersionToSemver(matches[1])
		if err != nil {
			return "", errors.Errorf("invalid version for lower bound: %v", err)
		}
		if lowerBound.LT(oldestAllowedPulumi) {
			return "", errors.Errorf("lower version bound must be at least %v", oldestAllowedPulumi)
		}
	} else {
		if requires == nil {
			requires = map[string]string{}
		}
		requires["pulumi"] = ""
	}

	// Sort the entries so they are deterministic.
	reqNames := []string{
		"semver>=2.8.1",
		"parver>=0.2.1",
	}
	for req := range requires {
		reqNames = append(reqNames, req)
	}
	sort.Strings(reqNames)

	fmt.Fprintf(w, "      install_requires=[\n")
	for i, req := range reqNames {
		var comma string
		if i < len(reqNames)-1 {
			comma = ","
		}
		fmt.Fprintf(w, "          '%s%s'%s\n", req, requires[req], comma)
	}
	fmt.Fprintf(w, "      ],\n")

	fmt.Fprintf(w, "      zip_safe=False)\n")
	return w.String(), nil
}

func pep440VersionToSemver(v string) (semver.Version, error) {
	switch {
	case pep440AlphaRegex.MatchString(v):
		parts := pep440AlphaRegex.FindStringSubmatch(v)
		v = parts[1] + "-alpha." + parts[2]
	case pep440BetaRegex.MatchString(v):
		parts := pep440BetaRegex.FindStringSubmatch(v)
		v = parts[1] + "-beta." + parts[2]
	case pep440RCRegex.MatchString(v):
		parts := pep440RCRegex.FindStringSubmatch(v)
		v = parts[1] + "-rc." + parts[2]
	case pep440DevRegex.MatchString(v):
		parts := pep440DevRegex.FindStringSubmatch(v)
		v = parts[1] + "-dev." + parts[2]
	}

	return semver.ParseTolerant(v)
}

// recordProperty records the given property's name and member names. For each property name contained in the given
// property, the name is converted to snake case and recorded in the snake case to camel case table.
//
// Once all resources have been emitted, the table is written out to a format usable for implementations of
// translate_input_property and translate_output_property.
func buildCaseMappingTables(pkg *schema.Package, snakeCaseToCamelCase, camelCaseToSnakeCase map[string]string, seenTypes codegen.Set) {
	// Add provider input properties to translation tables.
	for _, p := range pkg.Provider.InputProperties {
		recordProperty(p, snakeCaseToCamelCase, camelCaseToSnakeCase, seenTypes)
	}

	for _, r := range pkg.Resources {
		// Calculate casing tables. We do this up front because our docstring generator (which is run during
		// genResource) requires them.
		for _, prop := range r.Properties {
			recordProperty(prop, snakeCaseToCamelCase, camelCaseToSnakeCase, seenTypes)
		}
		for _, prop := range r.InputProperties {
			recordProperty(prop, snakeCaseToCamelCase, camelCaseToSnakeCase, seenTypes)
		}
	}
	for _, typ := range pkg.Types {
		typ, ok := typ.(*schema.ObjectType)
		if ok {
			for _, prop := range typ.Properties {
				recordProperty(prop, snakeCaseToCamelCase, camelCaseToSnakeCase, seenTypes)
			}
		}
	}
}

func recordProperty(prop *schema.Property, snakeCaseToCamelCase, camelCaseToSnakeCase map[string]string, seenTypes codegen.Set) {
	mapCase := true
	if python, ok := prop.Language["python"]; ok {
		v, ok := python.(PropertyInfo)
		mapCase = ok && v.MapCase
	}
	if mapCase {
		snakeCaseName := PyNameLegacy(prop.Name)
		if snakeCaseToCamelCase != nil {
			if _, ok := snakeCaseToCamelCase[snakeCaseName]; !ok {
				snakeCaseToCamelCase[snakeCaseName] = prop.Name
			}
		}
		if camelCaseToSnakeCase != nil {
			if _, ok := camelCaseToSnakeCase[prop.Name]; !ok {
				camelCaseToSnakeCase[prop.Name] = snakeCaseName
			}
		}
	}

	if obj, ok := prop.Type.(*schema.ObjectType); ok {
		if !seenTypes.Has(prop.Type) {
			// Avoid infinite calls in case of recursive types.
			seenTypes.Add(prop.Type)

			for _, p := range obj.Properties {
				recordProperty(p, snakeCaseToCamelCase, camelCaseToSnakeCase, seenTypes)
			}
		}
	}
}

// genInitDocstring emits the docstring for the __init__ method of the given resource type.
//
// Sphinx (the documentation generator that we use to generate Python docs) does not draw a
// distinction between documentation comments on the class itself and documentation comments on the
// __init__ method of a class. The docs repo instructs Sphinx to concatenate the two together, which
// means that we don't need to emit docstrings on the class at all as long as the __init__ docstring
// is good enough.
//
// The docstring we generate here describes both the class itself and the arguments to the class's
// constructor. The format of the docstring is in "Sphinx form":
//   1. Parameters are introduced using the syntax ":param <type> <name>: <comment>". Sphinx parses this and uses it
//      to populate the list of parameters for this function.
//   2. The doc string of parameters is expected to be indented to the same indentation as the type of the parameter.
//      Sphinx will complain and make mistakes if this is not the case.
//   3. The doc string can't have random newlines in it, or Sphinx will complain.
//
// This function does the best it can to navigate these constraints and produce a docstring that
// Sphinx can make sense of.
func (mod *modContext) genInitDocstring(w io.Writer, res *schema.Resource, resourceArgsName string, argOverload bool) {
	// b contains the full text of the docstring, without the leading and trailing triple quotes.
	b := &bytes.Buffer{}

	// If this resource has documentation, write it at the top of the docstring, otherwise use a generic comment.
	if res.Comment != "" {
		fmt.Fprintln(b, codegen.FilterExamples(res.Comment, "python"))
	} else {
		fmt.Fprintf(b, "Create a %s resource with the given unique name, props, and options.\n", tokenToName(res.Token))
	}

	// All resources have a resource_name parameter and opts parameter.
	fmt.Fprintln(b, ":param str resource_name: The name of the resource.")
	if argOverload {
		fmt.Fprintf(b, ":param %s args: The arguments to use to populate this resource's properties.\n",
			resourceArgsName)
	}
	fmt.Fprintln(b, ":param pulumi.ResourceOptions opts: Options for the resource.")
	if !argOverload {
		for _, prop := range res.InputProperties {
			mod.genPropDocstring(b, InitParamName(prop.Name), prop, true /*acceptMapping*/)
		}
	}

	// printComment handles the prefix and triple quotes.
	printComment(w, b.String(), "        ")
}

func (mod *modContext) genGetDocstring(w io.Writer, res *schema.Resource) {
	// "buf" contains the full text of the docstring, without the leading and trailing triple quotes.
	b := &bytes.Buffer{}

	fmt.Fprintf(b, "Get an existing %s resource's state with the given name, id, and optional extra\n"+
		"properties used to qualify the lookup.\n", tokenToName(res.Token))
	fmt.Fprintln(b, "")

	fmt.Fprintln(b, ":param str resource_name: The unique name of the resulting resource.")
	fmt.Fprintln(b, ":param pulumi.Input[str] id: The unique provider ID of the resource to lookup.")
	fmt.Fprintln(b, ":param pulumi.ResourceOptions opts: Options for the resource.")
	if res.StateInputs != nil {
		for _, prop := range res.StateInputs.Properties {
			mod.genPropDocstring(b, InitParamName(prop.Name), prop, true /*acceptMapping*/)
		}
	}

	// printComment handles the prefix and triple quotes.
	printComment(w, b.String(), "        ")
}

func (mod *modContext) genTypeDocstring(w io.Writer, comment string, properties []*schema.Property) {
	// b contains the full text of the docstring, without the leading and trailing triple quotes.
	b := &bytes.Buffer{}

	// If this type has documentation, write it at the top of the docstring.
	if comment != "" {
		fmt.Fprintln(b, comment)
	}

	for _, prop := range properties {
		mod.genPropDocstring(b, PyName(prop.Name), prop, false /*acceptMapping*/)
	}

	// printComment handles the prefix and triple quotes.
	printComment(w, b.String(), "        ")
}

func (mod *modContext) genPropDocstring(w io.Writer, name string, prop *schema.Property, acceptMapping bool) {
	if prop.Comment == "" {
		return
	}

	ty := mod.typeString(codegen.RequiredType(prop), true, acceptMapping)

	// If this property has some documentation associated with it, we need to split it so that it is indented
	// in a way that Sphinx can understand.
	lines := strings.Split(prop.Comment, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i, docLine := range lines {
		// If it's the first line, print the :param header.
		if i == 0 {
			fmt.Fprintf(w, ":param %s %s: %s\n", ty, name, docLine)
		} else {
			// Otherwise, print out enough padding to align with the first char of the type.
			fmt.Fprintf(w, "       %s\n", docLine)
		}
	}
}

func (mod *modContext) typeString(t schema.Type, input, acceptMapping bool) string {
	switch t := t.(type) {
	case *schema.OptionalType:
		return fmt.Sprintf("Optional[%s]", mod.typeString(t.ElementType, input, acceptMapping))
	case *schema.InputType:
		typ := mod.typeString(codegen.SimplifyInputUnion(t.ElementType), input, acceptMapping)
		if typ == "Any" {
			return typ
		}
		return fmt.Sprintf("pulumi.Input[%s]", typ)
	case *schema.EnumType:
		return mod.tokenToEnum(t.Token)
	case *schema.ArrayType:
		return fmt.Sprintf("Sequence[%s]", mod.typeString(t.ElementType, input, acceptMapping))
	case *schema.MapType:
		return fmt.Sprintf("Mapping[str, %s]", mod.typeString(t.ElementType, input, acceptMapping))
	case *schema.ObjectType:
		typ := mod.objectType(t, input)
		if !acceptMapping {
			return typ
		}
		return fmt.Sprintf("pulumi.InputType[%s]", typ)
	case *schema.ResourceType:
		return fmt.Sprintf("'%s'", mod.resourceType(t))
	case *schema.TokenType:
		// Use the underlying type for now.
		if t.UnderlyingType != nil {
			return mod.typeString(t.UnderlyingType, input, acceptMapping)
		}
		return "Any"
	case *schema.UnionType:
		if !input {
			for _, e := range t.ElementTypes {
				// If this is an output and a "relaxed" enum, emit the type as the underlying primitive type rather than the union.
				// Eg. Output[str] rather than Output[Any]
				if typ, ok := e.(*schema.EnumType); ok {
					return mod.typeString(typ.ElementType, input, acceptMapping)
				}
			}
			if t.DefaultType != nil {
				return mod.typeString(t.DefaultType, input, acceptMapping)
			}
			return "Any"
		}

		elementTypeSet := codegen.NewStringSet()
		elements := make([]string, 0, len(t.ElementTypes))
		for _, e := range t.ElementTypes {
			et := mod.typeString(e, input, acceptMapping)
			if !elementTypeSet.Has(et) {
				elementTypeSet.Add(et)
				elements = append(elements, et)
			}
		}

		if len(elements) == 1 {
			return elements[0]
		}
		return fmt.Sprintf("Union[%s]", strings.Join(elements, ", "))
	default:
		switch t {
		case schema.BoolType:
			return "bool"
		case schema.IntType:
			return "int"
		case schema.NumberType:
			return "float"
		case schema.StringType:
			return "str"
		case schema.ArchiveType:
			return "pulumi.Archive"
		case schema.AssetType:
			return "Union[pulumi.Asset, pulumi.Archive]"
		case schema.JSONType:
			fallthrough
		case schema.AnyType:
			return "Any"
		}
	}

	panic(fmt.Errorf("unexpected type %T", t))
}

// pyType returns the expected runtime type for the given variable.  Of course, being a dynamic language, this
// check is not exhaustive, but it should be good enough to catch 80% of the cases early on.
func (mod *modContext) pyType(typ schema.Type) string {
	switch typ := typ.(type) {
	case *schema.OptionalType:
		return mod.pyType(typ.ElementType)
	case *schema.EnumType:
		return mod.pyType(typ.ElementType)
	case *schema.ArrayType:
		return "list"
	case *schema.MapType, *schema.ObjectType, *schema.UnionType:
		return "dict"
	case *schema.ResourceType:
		return mod.resourceType(typ)
	case *schema.TokenType:
		if typ.UnderlyingType != nil {
			return mod.pyType(typ.UnderlyingType)
		}
		return "dict"
	default:
		switch typ {
		case schema.BoolType:
			return "bool"
		case schema.IntType:
			return "int"
		case schema.NumberType:
			return "float"
		case schema.StringType:
			return "str"
		case schema.ArchiveType:
			return "pulumi.Archive"
		case schema.AssetType:
			return "Union[pulumi.Asset, pulumi.Archive]"
		default:
			return "dict"
		}
	}
}

func isStringType(t schema.Type) bool {
	t = codegen.UnwrapType(t)

	for tt, ok := t.(*schema.TokenType); ok; tt, ok = t.(*schema.TokenType) {
		t = tt.UnderlyingType
	}

	return t == schema.StringType
}

// pyPack returns the suggested package name for the given string.
func pyPack(s string) string {
	return "pulumi_" + strings.ReplaceAll(s, "-", "_")
}

// pyClassName turns a raw name into one that is suitable as a Python class name.
func pyClassName(name string) string {
	return EnsureKeywordSafe(name)
}

// InitParamName returns a PyName-encoded name but also deduplicates the name against built-in parameters of resource __init__.
func InitParamName(name string) string {
	result := PyName(name)
	switch result {
	case "resource_name", "opts":
		return result + "_"
	default:
		return result
	}
}

func (mod *modContext) genObjectType(w io.Writer, obj *schema.ObjectType, input bool) error {
	name := mod.unqualifiedObjectTypeName(obj, input)
	resourceOutputType := !input && mod.details(obj).resourceOutputType
	return mod.genType(w, name, obj.Comment, obj.Properties, input, resourceOutputType)
}

func (mod *modContext) genType(w io.Writer, name, comment string, properties []*schema.Property, input, resourceOutput bool) error {
	// Sort required props first.
	props := make([]*schema.Property, len(properties))
	copy(props, properties)
	sort.Slice(props, func(i, j int) bool {
		pi, pj := props[i], props[j]
		switch {
		case pi.IsRequired() != pj.IsRequired():
			return pi.IsRequired() && !pj.IsRequired()
		default:
			return pi.Name < pj.Name
		}
	})

	decorator := "@pulumi.output_type"
	if input {
		decorator = "@pulumi.input_type"
	}

	var suffix string
	if !input {
		suffix = "(dict)"
	}

	fmt.Fprintf(w, "%s\n", decorator)
	fmt.Fprintf(w, "class %s%s:\n", name, suffix)
	if !input && comment != "" {
		printComment(w, comment, "    ")
	}

	// To help users migrate to using the properly snake_cased property getters, emit warnings when camelCase keys are
	// accessed. We emit this at the top of the class in case we have a `get` property that will be redefined later.
	if resourceOutput {
		var needsCaseWarning bool
		for _, prop := range props {
			pname := PyName(prop.Name)
			if pname != prop.Name {
				needsCaseWarning = true
				break
			}
		}
		if needsCaseWarning {
			fmt.Fprintf(w, "    @staticmethod\n")
			fmt.Fprintf(w, "    def __key_warning(key: str):\n")
			fmt.Fprintf(w, "        suggest = None\n")
			prefix := "if"
			for _, prop := range props {
				pname := PyName(prop.Name)
				if pname == prop.Name {
					continue
				}
				fmt.Fprintf(w, "        %s key == %q:\n", prefix, prop.Name)
				fmt.Fprintf(w, "            suggest = %q\n", pname)
				prefix = "elif"
			}
			fmt.Fprintf(w, "\n")
			fmt.Fprintf(w, "        if suggest:\n")
			fmt.Fprintf(w, "            pulumi.log.warn(f\"Key '{key}' not found in %s. Access the value via the '{suggest}' property getter instead.\")\n", name)
			fmt.Fprintf(w, "\n")
			fmt.Fprintf(w, "    def __getitem__(self, key: str) -> Any:\n")
			fmt.Fprintf(w, "        %s.__key_warning(key)\n", name)
			fmt.Fprintf(w, "        return super().__getitem__(key)\n")
			fmt.Fprintf(w, "\n")
			fmt.Fprintf(w, "    def get(self, key: str, default = None) -> Any:\n")
			fmt.Fprintf(w, "        %s.__key_warning(key)\n", name)
			fmt.Fprintf(w, "        return super().get(key, default)\n")
			fmt.Fprintf(w, "\n")
		}
	}

	// Generate an __init__ method.
	fmt.Fprintf(w, "    def __init__(__self__")
	// Bare `*` argument to force callers to use named arguments.
	if len(props) > 0 {
		fmt.Fprintf(w, ", *")
	}
	for _, prop := range props {
		pname := PyName(prop.Name)
		ty := mod.typeString(prop.Type, input, false /*acceptMapping*/)
		var defaultValue string
		if !prop.IsRequired() {
			defaultValue = " = None"
		}
		fmt.Fprintf(w, ",\n                 %s: %s%s", pname, ty, defaultValue)
	}
	fmt.Fprintf(w, "):\n")
	mod.genTypeDocstring(w, comment, props)
	if len(props) == 0 {
		fmt.Fprintf(w, "        pass\n")
	}
	for _, prop := range props {
		pname := PyName(prop.Name)
		var arg interface{}
		var err error

		// Fill in computed defaults for arguments.
		if prop.DefaultValue != nil {
			dv, err := getDefaultValue(prop.DefaultValue, codegen.UnwrapType(prop.Type))
			if err != nil {
				return err
			}
			fmt.Fprintf(w, "        if %s is None:\n", pname)
			fmt.Fprintf(w, "            %s = %s\n", pname, dv)
		}

		// Check that the property isn't deprecated.
		if input && prop.DeprecationMessage != "" {
			escaped := strings.ReplaceAll(prop.DeprecationMessage, `"`, `\"`)
			fmt.Fprintf(w, "        if %s is not None:\n", pname)
			fmt.Fprintf(w, "            warnings.warn(\"\"\"%s\"\"\", DeprecationWarning)\n", escaped)
			fmt.Fprintf(w, "            pulumi.log.warn(\"\"\"%s is deprecated: %s\"\"\")\n", pname, escaped)
		}

		// And add it to the dictionary.
		arg = pname

		if prop.ConstValue != nil {
			arg, err = getConstValue(prop.ConstValue)
			if err != nil {
				return err
			}
		}

		var indent string
		if !prop.IsRequired() {
			fmt.Fprintf(w, "        if %s is not None:\n", pname)
			indent = "    "
		}

		fmt.Fprintf(w, "%s        pulumi.set(__self__, \"%s\", %s)\n", indent, pname, arg)
	}
	fmt.Fprintf(w, "\n")

	// Generate properties. Input types have getters and setters, output types only have getters.
	mod.genProperties(w, props, input /*setters*/, "", func(prop *schema.Property) string {
		return mod.typeString(prop.Type, input, false /*acceptMapping*/)
	})

	fmt.Fprintf(w, "\n")
	return nil
}

func getPrimitiveValue(value interface{}) (string, error) {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			return "True", nil
		}
		return "False", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64), nil
	case reflect.String:
		return fmt.Sprintf("'%s'", v.String()), nil
	default:
		return "", errors.Errorf("unsupported default value of type %T", value)
	}
}

func getConstValue(cv interface{}) (string, error) {
	if cv == nil {
		return "", nil
	}
	return getPrimitiveValue(cv)
}

func getDefaultValue(dv *schema.DefaultValue, t schema.Type) (string, error) {
	defaultValue := ""
	if dv.Value != nil {
		v, err := getPrimitiveValue(dv.Value)
		if err != nil {
			return "", err
		}
		defaultValue = v
	}

	if len(dv.Environment) > 0 {
		envFunc := "_utilities.get_env"
		switch t {
		case schema.BoolType:
			envFunc = "_utilities.get_env_bool"
		case schema.IntType:
			envFunc = "_utilities.get_env_int"
		case schema.NumberType:
			envFunc = "_utilities.get_env_float"
		}

		envVars := fmt.Sprintf("'%s'", dv.Environment[0])
		for _, e := range dv.Environment[1:] {
			envVars += fmt.Sprintf(", '%s'", e)
		}
		if defaultValue == "" {
			defaultValue = fmt.Sprintf("%s(%s)", envFunc, envVars)
		} else {
			defaultValue = fmt.Sprintf("(%s(%s) or %s)", envFunc, envVars, defaultValue)
		}
	}

	return defaultValue, nil
}

func generateModuleContextMap(tool string, pkg *schema.Package, info PackageInfo, extraFiles map[string][]byte) (map[string]*modContext, error) {
	// Build case mapping tables
	snakeCaseToCamelCase, camelCaseToSnakeCase := map[string]string{}, map[string]string{}
	seenTypes := codegen.Set{}
	buildCaseMappingTables(pkg, snakeCaseToCamelCase, camelCaseToSnakeCase, seenTypes)

	// determine whether to use the default Python package name
	pyPkgName := info.PackageName
	if pyPkgName == "" {
		pyPkgName = fmt.Sprintf("pulumi_%s", strings.ReplaceAll(pkg.Name, "-", "_"))
	}

	// group resources, types, and functions into modules
	modules := map[string]*modContext{}

	var getMod func(modName string, p *schema.Package) *modContext
	getMod = func(modName string, p *schema.Package) *modContext {
		mod, ok := modules[modName]
		if !ok {
			mod = &modContext{
				pkg:                  p,
				pyPkgName:            pyPkgName,
				mod:                  modName,
				tool:                 tool,
				snakeCaseToCamelCase: snakeCaseToCamelCase,
				camelCaseToSnakeCase: camelCaseToSnakeCase,
				modNameOverrides:     info.ModuleNameOverrides,
				compatibility:        info.Compatibility,
			}

			if modName != "" && p == pkg {
				parentName := path.Dir(modName)
				if parentName == "." {
					parentName = ""
				}
				parent := getMod(parentName, p)
				parent.addChild(mod)
			}

			// Save the module only if it's for the current package.
			// This way, modules for external packages are not saved.
			if p == pkg {
				modules[modName] = mod
			}
		}
		return mod
	}

	getModFromToken := func(tok string, p *schema.Package) *modContext {
		modName := tokenToModule(tok, p, info.ModuleNameOverrides)
		return getMod(modName, p)
	}

	// Create the config module if necessary.
	if len(pkg.Config) > 0 &&
		info.Compatibility != kubernetes20 { // k8s SDK doesn't use config.
		configMod := getMod("config", pkg)
		configMod.isConfig = true
	}

	visitObjectTypes(pkg.Config, func(t schema.Type) {
		if t, ok := t.(*schema.ObjectType); ok {
			getModFromToken(t.Token, t.Package).details(t).outputType = true
		}
	})

	// Find input and output types referenced by resources.
	scanResource := func(r *schema.Resource) {
		mod := getModFromToken(r.Token, pkg)
		mod.resources = append(mod.resources, r)
		visitObjectTypes(r.Properties, func(t schema.Type) {
			switch T := t.(type) {
			case *schema.ObjectType:
				getModFromToken(T.Token, T.Package).details(T).outputType = true
				getModFromToken(T.Token, T.Package).details(T).resourceOutputType = true
			}
		})
		visitObjectTypes(r.InputProperties, func(t schema.Type) {
			switch T := t.(type) {
			case *schema.ObjectType:
				getModFromToken(T.Token, T.Package).details(T).inputType = true
			}
		})
		if r.StateInputs != nil {
			visitObjectTypes(r.StateInputs.Properties, func(t schema.Type) {
				switch T := t.(type) {
				case *schema.ObjectType:
					getModFromToken(T.Token, T.Package).details(T).inputType = true
				case *schema.ResourceType:
					getModFromToken(T.Token, T.Resource.Package)
				}
			})
		}
	}

	scanResource(pkg.Provider)
	for _, r := range pkg.Resources {
		scanResource(r)
	}

	// Find input and output types referenced by functions.
	for _, f := range pkg.Functions {
		mod := getModFromToken(f.Token, f.Package)
		if !f.IsMethod {
			mod.functions = append(mod.functions, f)
		}
		if f.Inputs != nil {
			visitObjectTypes(f.Inputs.Properties, func(t schema.Type) {
				switch T := t.(type) {
				case *schema.ObjectType:
					getModFromToken(T.Token, T.Package).details(T).inputType = true
					getModFromToken(T.Token, T.Package).details(T).plainType = true
				case *schema.ResourceType:
					getModFromToken(T.Token, T.Resource.Package)
				}
			})
		}
		if f.Outputs != nil {
			visitObjectTypes(f.Outputs.Properties, func(t schema.Type) {
				switch T := t.(type) {
				case *schema.ObjectType:
					getModFromToken(T.Token, T.Package).details(T).outputType = true
					getModFromToken(T.Token, T.Package).details(T).plainType = true
				case *schema.ResourceType:
					getModFromToken(T.Token, T.Resource.Package)
				}
			})
		}
	}

	// Find nested types.
	for _, t := range pkg.Types {
		switch typ := t.(type) {
		case *schema.ObjectType:
			mod := getModFromToken(typ.Token, typ.Package)
			d := mod.details(typ)
			if d.inputType || d.outputType {
				mod.types = append(mod.types, typ)
			}
		case *schema.EnumType:
			mod := getModFromToken(typ.Token, pkg)
			mod.enums = append(mod.enums, typ)
		default:
			continue
		}
	}

	// Add python source files to the corresponding modules. Note that we only add the file names; the contents are
	// still laid out manually in GeneratePackage.
	for p := range extraFiles {
		if path.Ext(p) != ".py" {
			continue
		}

		modName := path.Dir(p)
		if modName == "/" || modName == "." {
			modName = ""
		}
		mod := getMod(modName, pkg)
		mod.extraSourceFiles = append(mod.extraSourceFiles, p)
	}

	return modules, nil
}

// LanguageResource is derived from the schema and can be used by downstream codegen.
type LanguageResource struct {
	*schema.Resource

	Name    string // The resource name (e.g. Deployment)
	Package string // The package name (e.g. pulumi_kubernetes.apps.v1)
}

// LanguageResources returns a map of resources that can be used by downstream codegen. The map
// key is the resource schema token.
func LanguageResources(tool string, pkg *schema.Package) (map[string]LanguageResource, error) {
	resources := map[string]LanguageResource{}

	if err := pkg.ImportLanguages(map[string]schema.Language{"python": Importer}); err != nil {
		return nil, err
	}
	info, _ := pkg.Language["python"].(PackageInfo)

	modules, err := generateModuleContextMap(tool, pkg, info, nil)
	if err != nil {
		return nil, err
	}

	for modName, mod := range modules {
		if modName == "" {
			continue
		}
		for _, r := range mod.resources {
			packagePath := strings.Replace(modName, "/", ".", -1)
			lr := LanguageResource{
				Resource: r,
				Package:  packagePath,
				Name:     pyClassName(tokenToName(r.Token)),
			}
			resources[r.Token] = lr
		}
	}

	return resources, nil
}

func GeneratePackage(tool string, pkg *schema.Package, extraFiles map[string][]byte) (map[string][]byte, error) {
	// Decode python-specific info
	if err := pkg.ImportLanguages(map[string]schema.Language{"python": Importer}); err != nil {
		return nil, err
	}
	info, _ := pkg.Language["python"].(PackageInfo)

	modules, err := generateModuleContextMap(tool, pkg, info, extraFiles)
	if err != nil {
		return nil, err
	}

	pkgName := info.PackageName
	if pkgName == "" {
		pkgName = pyPack(pkg.Name)
	}

	files := fs{}
	for p, f := range extraFiles {
		files.add(filepath.Join(pkgName, p), f)
	}

	for _, mod := range modules {
		if err := mod.gen(files); err != nil {
			return nil, err
		}
	}

	// Generate pulumiplugin.json, if requested.
	if info.EmitPulumiPluginFile {
		plugin, err := genPulumiPluginFile(pkg)
		if err != nil {
			return nil, err
		}
		files.add(filepath.Join(pkgName, "pulumiplugin.json"), plugin)
	}

	// Finally emit the package metadata (setup.py).
	setup, err := genPackageMetadata(tool, pkg, pkgName, info.EmitPulumiPluginFile, info.Requires)
	if err != nil {
		return nil, err
	}
	files.add("setup.py", []byte(setup))

	return files, nil
}

const utilitiesFile = `
import json
import os
import sys
import importlib.util
import pkg_resources

import pulumi
import pulumi.runtime

from semver import VersionInfo as SemverVersion
from parver import Version as PEP440Version


def get_env(*args):
    for v in args:
        value = os.getenv(v)
        if value is not None:
            return value
    return None


def get_env_bool(*args):
    str = get_env(*args)
    if str is not None:
        # NOTE: these values are taken from https://golang.org/src/strconv/atob.go?s=351:391#L1, which is what
        # Terraform uses internally when parsing boolean values.
        if str in ["1", "t", "T", "true", "TRUE", "True"]:
            return True
        if str in ["0", "f", "F", "false", "FALSE", "False"]:
            return False
    return None


def get_env_int(*args):
    str = get_env(*args)
    if str is not None:
        try:
            return int(str)
        except:
            return None
    return None


def get_env_float(*args):
    str = get_env(*args)
    if str is not None:
        try:
            return float(str)
        except:
            return None
    return None


def _get_semver_version():
    # __name__ is set to the fully-qualified name of the current module, In our case, it will be
    # <some module>._utilities. <some module> is the module we want to query the version for.
    root_package, *rest = __name__.split('.')

    # pkg_resources uses setuptools to inspect the set of installed packages. We use it here to ask
    # for the currently installed version of the root package (i.e. us) and get its version.

    # Unfortunately, PEP440 and semver differ slightly in incompatible ways. The Pulumi engine expects
    # to receive a valid semver string when receiving requests from the language host, so it's our
    # responsibility as the library to convert our own PEP440 version into a valid semver string.

    pep440_version_string = pkg_resources.require(root_package)[0].version
    pep440_version = PEP440Version.parse(pep440_version_string)
    (major, minor, patch) = pep440_version.release
    prerelease = None
    if pep440_version.pre_tag == 'a':
        prerelease = f"alpha.{pep440_version.pre}"
    elif pep440_version.pre_tag == 'b':
        prerelease = f"beta.{pep440_version.pre}"
    elif pep440_version.pre_tag == 'rc':
        prerelease = f"rc.{pep440_version.pre}"
    elif pep440_version.dev is not None:
        prerelease = f"dev.{pep440_version.dev}"

    # The only significant difference between PEP440 and semver as it pertains to us is that PEP440 has explicit support
    # for dev builds, while semver encodes them as "prerelease" versions. In order to bridge between the two, we convert
    # our dev build version into a prerelease tag. This matches what all of our other packages do when constructing
    # their own semver string.
    return SemverVersion(major=major, minor=minor, patch=patch, prerelease=prerelease)


# Determine the version once and cache the value, which measurably improves program performance.
_version = _get_semver_version()
_version_str = str(_version)


def get_version():
    return _version_str


def get_resource_args_opts(resource_args_type, resource_options_type, *args, **kwargs):
    """
    Return the resource args and options given the *args and **kwargs of a resource's
    __init__ method.
    """

    resource_args, opts = None, None

    # If the first item is the resource args type, save it and remove it from the args list.
    if args and isinstance(args[0], resource_args_type):
        resource_args, args = args[0], args[1:]

    # Now look at the first item in the args list again.
    # If the first item is the resource options class, save it.
    if args and isinstance(args[0], resource_options_type):
        opts = args[0]

    # If resource_args is None, see if "args" is in kwargs, and, if so, if it's typed as the
    # the resource args type.
    if resource_args is None:
        a = kwargs.get("args")
        if isinstance(a, resource_args_type):
            resource_args = a

    # If opts is None, look it up in kwargs.
    if opts is None:
        opts = kwargs.get("opts")

    return resource_args, opts


# Temporary: just use pulumi._utils.lazy_import once everyone upgrades.
def lazy_import(fullname):

    import pulumi._utils as u
    f = getattr(u, 'lazy_import', None)
    if f is None:
        f = _lazy_import_temp

    return f(fullname)


# Copied from pulumi._utils.lazy_import, see comments there.
def _lazy_import_temp(fullname):
    m = sys.modules.get(fullname, None)
    if m is not None:
        return m

    spec = importlib.util.find_spec(fullname)

    m = sys.modules.get(fullname, None)
    if m is not None:
        return m

    loader = importlib.util.LazyLoader(spec.loader)
    spec.loader = loader
    module = importlib.util.module_from_spec(spec)

    m = sys.modules.get(fullname, None)
    if m is not None:
        return m

    sys.modules[fullname] = module
    loader.exec_module(module)
    return module


class Package(pulumi.runtime.ResourcePackage):
    def __init__(self, pkg_info):
        super().__init__()
        self.pkg_info = pkg_info

    def version(self):
        return _version

    def construct_provider(self, name: str, typ: str, urn: str) -> pulumi.ProviderResource:
        if typ != self.pkg_info['token']:
            raise Exception(f"unknown provider type {typ}")
        Provider = getattr(lazy_import(self.pkg_info['fqn']), self.pkg_info['class'])
        return Provider(name, pulumi.ResourceOptions(urn=urn))


class Module(pulumi.runtime.ResourceModule):
    def __init__(self, mod_info):
        super().__init__()
        self.mod_info = mod_info

    def version(self):
        return _version

    def construct(self, name: str, typ: str, urn: str) -> pulumi.Resource:
        class_name = self.mod_info['classes'].get(typ, None)

        if class_name is None:
            raise Exception(f"unknown resource type {typ}")

        TheClass = getattr(lazy_import(self.mod_info['fqn']), class_name)
        return TheClass(name, pulumi.ResourceOptions(urn=urn))


def register(resource_modules, resource_packages):
    resource_modules = json.loads(resource_modules)
    resource_packages = json.loads(resource_packages)

    for pkg_info in resource_packages:
        pulumi.runtime.register_resource_package(pkg_info['pkg'], Package(pkg_info))

    for mod_info in resource_modules:
        pulumi.runtime.register_resource_module(
            mod_info['pkg'],
            mod_info['mod'],
            Module(mod_info))
`
