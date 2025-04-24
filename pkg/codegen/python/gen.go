// Copyright 2016-2021, Pulumi Corporation.
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
//nolint:lll, goconst
package python

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
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

	"github.com/BurntSushi/toml"
	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const (
	InputTypesSettingClasses         = "classes"
	InputTypesSettingClassesAndDicts = "classes-and-dicts"
)

func typedDictEnabled(setting string) bool {
	return setting != InputTypesSettingClasses
}

type typeDetails struct {
	outputType         bool
	inputType          bool
	resourceOutputType bool
	plainType          bool
}

type imports codegen.StringSet

// defaultMinPythonVersion is what we use as the minimum version field in generated
// package metadata if the schema does not provide a value. This version corresponds
// to the minimum supported version as listed in the reference documentation:
// https://www.pulumi.com/docs/languages-sdks/python/
const defaultMinPythonVersion = ">=3.9"

func (imports imports) addType(mod *modContext, t *schema.ObjectType, input bool) {
	imports.addTypeIf(mod, t, input, nil /*predicate*/)
}

func (imports imports) addTypeIf(mod *modContext, t *schema.ObjectType, input bool, predicate func(imp string) bool) {
	if imp := mod.importObjectType(t, input); imp != "" && (predicate == nil || predicate(imp)) {
		codegen.StringSet(imports).Add(imp)
	}
}

func (imports imports) addEnum(mod *modContext, enum *schema.EnumType) {
	if imp := mod.importEnumType(enum); imp != "" {
		codegen.StringSet(imports).Add(imp)
	}
}

func (imports imports) addResource(mod *modContext, r *schema.ResourceType) {
	if imp := mod.importResourceType(r); imp != "" {
		codegen.StringSet(imports).Add(imp)
	}
}

func (imports imports) strings() []string {
	result := slice.Prealloc[string](len(imports))
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

type modLocator struct {
	// Returns defining module for a given ObjectType. Returns nil
	// for types that are not being generated in the current
	// GeneratePackage call.
	objectTypeMod func(*schema.ObjectType) *modContext
}

type modContext struct {
	pkg              schema.PackageReference
	modLocator       *modLocator
	mod              string
	pyPkgName        string
	types            []*schema.ObjectType
	enums            []*schema.EnumType
	resources        []*schema.Resource
	functions        []*schema.Function
	typeDetails      map[*schema.ObjectType]*typeDetails
	children         []*modContext
	parent           *modContext
	tool             string
	extraSourceFiles []string
	isConfig         bool

	// Name overrides set in PackageInfo
	modNameOverrides map[string]string // Optional overrides for Pulumi module names
	compatibility    string            // Toggle compatibility mode for a specified target.

	// Determine whether to lift single-value method return values
	liftSingleValueMethodReturns bool

	// Controls what types are used for inputs, see PackageInfo.InputTypes.
	inputTypes string
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
	m := mod

	if mod.modLocator != nil {
		if actualMod := mod.modLocator.objectTypeMod(t); actualMod != nil {
			m = actualMod
		}
	}

	details, ok := m.typeDetails[t]
	if !ok {
		details = &typeDetails{}
		if m.typeDetails == nil {
			m.typeDetails = map[*schema.ObjectType]*typeDetails{}
		}
		m.typeDetails[t] = details
	}
	return details
}

func (mod *modContext) modNameAndName(pkg schema.PackageReference, t schema.Type, input bool) (modName string, name string) {
	var info PackageInfo
	p, err := pkg.Definition()
	contract.AssertNoErrorf(err, "error loading definition for package %q", pkg.Name())
	contract.AssertNoErrorf(p.ImportLanguages(map[string]schema.Language{"python": Importer}),
		"error importing python language plugin for package %q", pkg.Name())
	if v, ok := p.Language["python"].(PackageInfo); ok {
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

func (mod *modContext) objectType(t *schema.ObjectType, input bool, forDict bool) string {
	var prefix string
	if !input {
		prefix = "outputs."
	}

	// If it's an external type, reference it via fully qualified name.

	if !codegen.PkgEquals(t.PackageReference, mod.pkg) {
		modName, name := mod.modNameAndName(t.PackageReference, t, input)
		if forDict {
			pkg, err := t.PackageReference.Definition()
			contract.AssertNoErrorf(err, "error loading definition for package %q", t.PackageReference.Name())
			info, ok := pkg.Language["python"].(PackageInfo)
			// TODO[https://github.com/pulumi/pulumi/issues/16702]
			// We don't yet assume that external packages support TypedDicts by default.
			// Remove empty string check to enable TypedDicts for external packages by default.
			typedDicts := ok && typedDictEnabled(info.InputTypes) && info.InputTypes != ""
			if typedDicts {
				name = name + "Dict"
			}
		}
		return fmt.Sprintf("'%s.%s%s%s'", PyPack(t.PackageReference.Namespace(), t.PackageReference.Name()), modName, prefix, name)
	}

	modName, name := mod.tokenToModule(t.Token), mod.unqualifiedObjectTypeName(t, input)
	if forDict {
		name = name + "Dict"
	}
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

func (mod *modContext) enumType(enum *schema.EnumType) string {
	tok := enum.Token
	pkgName, modName, name := enum.PackageReference.Name(), mod.tokenToModule(tok), tokenToName(tok)

	if pkgName == mod.pkg.Name() && modName == "" && mod.mod != "" {
		return fmt.Sprintf("'_root_enums.%s'", name)
	}

	parts := []string{}

	// Add package name if needed.
	if pkgName != mod.pkg.Name() {
		// Foreign reference. Add package import alias.
		parts = append(parts, PyPack(mod.pkg.Namespace(), pkgName))
	}

	// Add module name if needed.
	if modName != "" {
		if pkgName == mod.pkg.Name() && modName == mod.mod {
			// Enum is in the same module, don't add the module name.
		} else {
			// Add the module name because it's referencing a different module.
			parts = append(parts, strings.ReplaceAll(modName, "/", "."))
		}
	}
	parts = append(parts, name)

	return fmt.Sprintf("'%s'", strings.Join(parts, "."))
}

func (mod *modContext) resourceType(r *schema.ResourceType) string {
	if r.Resource == nil || codegen.PkgEquals(r.Resource.PackageReference, mod.pkg) {
		return mod.tokenToResource(r.Token)
	}

	// Is it a provider resource?
	if strings.HasPrefix(r.Token, "pulumi:providers:") {
		pkgName := strings.TrimPrefix(r.Token, "pulumi:providers:")
		return fmt.Sprintf("pulumi_%s.Provider", pkgName)
	}

	pkg := r.Resource.PackageReference
	modName, name := mod.modNameAndName(pkg, r, false)
	return fmt.Sprintf("%s.%s%s", PyPack(pkg.Namespace(), pkg.Name()), modName, name)
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

func tokenToModule(tok string, pkg schema.PackageReference, moduleNameOverrides map[string]string) string {
	// See if there's a manually-overridden module name.
	if pkg == nil {
		// If pkg is nil, we use the default `TokenToModule` scheme.
		pkg = (&schema.Package{}).Reference()
	}
	canonicalModName := pkg.TokenToModule(tok)
	return moduleToPythonModule(canonicalModName, moduleNameOverrides)
}

func moduleToPythonModule(canonicalModName string, moduleNameOverrides map[string]string) string {
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
	replacer := strings.NewReplacer(`\`, `\\`, `"""`, `\"\"\"`)
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

func genStandardHeader(w io.Writer, tool string) {
	// Set the encoding to UTF-8, in case the comments contain non-ASCII characters.
	fmt.Fprintf(w, "# coding=utf-8\n")

	// Emit a standard warning header ("do not edit", etc).
	fmt.Fprintf(w, "# *** WARNING: this file was generated by %v. ***\n", tool)
	fmt.Fprintf(w, "# *** Do not edit by hand unless you're certain you know what you are doing! ***\n\n")
}

func typingImports() []string {
	return []string{
		"Any",
		"Mapping",
		"Optional",
		"Sequence",
		"Union",
		"overload",
	}
}

func (mod *modContext) generateCommonImports(w io.Writer, imports imports, typingImports []string) {
	rel, err := filepath.Rel(mod.mod, "")
	contract.AssertNoErrorf(err, "could not turn %q into a relative path", mod.mod)
	relRoot := path.Dir(rel)
	relImport := relPathToRelImport(relRoot)

	fmt.Fprintf(w, "import copy\n")
	fmt.Fprintf(w, "import warnings\n")
	if typedDictEnabled(mod.inputTypes) {
		fmt.Fprintf(w, "import sys\n")
	}
	fmt.Fprintf(w, "import pulumi\n")
	fmt.Fprintf(w, "import pulumi.runtime\n")
	fmt.Fprintf(w, "from typing import %s\n", strings.Join(typingImports, ", "))
	if typedDictEnabled(mod.inputTypes) {
		fmt.Fprintf(w, "if sys.version_info >= (3, 11):\n")
		fmt.Fprintf(w, "    from typing import NotRequired, TypedDict, TypeAlias\n")
		fmt.Fprintf(w, "else:\n")
		fmt.Fprintf(w, "    from typing_extensions import NotRequired, TypedDict, TypeAlias\n")
	}
	fmt.Fprintf(w, "from %s import _utilities\n", relImport)
	for _, imp := range imports.strings() {
		fmt.Fprintf(w, "%s\n", imp)
	}
	fmt.Fprintf(w, "\n")
}

func (mod *modContext) genHeader(w io.Writer, needsSDK bool, imports imports) {
	genStandardHeader(w, mod.tool)

	// Always import builtins as we use fully qualified type names `builtins.int` rather than just `int`.
	fmt.Fprintf(w, "import builtins\n")

	// If needed, emit the standard Pulumi SDK import statement.
	if needsSDK {
		typings := typingImports()
		mod.generateCommonImports(w, imports, typings)
	}
}

func (mod *modContext) genFunctionHeader(w io.Writer, function *schema.Function, imports imports) {
	genStandardHeader(w, mod.tool)
	typings := typingImports()
	if function.Outputs == nil || len(function.Outputs.Properties) == 0 {
		typings = append(typings, "Awaitable")
	}

	// Always import builtins as we use fully qualified type names `builtins.int` rather than just `int`.
	fmt.Fprintf(w, "import builtins\n")
	mod.generateCommonImports(w, imports, typings)
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

func (mod *modContext) genUtilitiesFile() ([]byte, error) {
	buffer := &bytes.Buffer{}
	genStandardHeader(buffer, mod.tool)
	fmt.Fprint(buffer, utilitiesFile)
	optionalURL := "None"
	pkg, err := mod.pkg.Definition()
	if err != nil {
		return nil, err
	}
	if url := pkg.PluginDownloadURL; url != "" {
		optionalURL = fmt.Sprintf("%q", url)
	}
	_, err = fmt.Fprintf(buffer, `
def get_plugin_download_url():
	return %s
`, optionalURL)
	if err != nil {
		return nil, err
	}

	// If a new style support pack library is being generated then write the _actual_ version to the utilities file
	// rather than relying on re-parsing the pypi version from setup.py.
	if pkg.SupportPack {
		if pkg.Version == nil {
			return nil, errors.New("package version is required")
		}
		_, err = fmt.Fprintf(buffer, `
def get_version():
    return %q
`, pkg.Version.String())
	} else {
		_, err = fmt.Fprintf(buffer, `
def get_version():
     return _version_str
`)
	}
	if err != nil {
		return nil, err
	}

	if pkg.Parameterization != nil {
		// If a parameterized package is being generated then we _need_ to use package references
		param := base64.StdEncoding.EncodeToString(pkg.Parameterization.Parameter)

		_, err = fmt.Fprintf(buffer, `
_package_lock = asyncio.Lock()
_package_ref = ...
async def get_package():
	global _package_ref
	if _package_ref is ...:
		if pulumi.runtime.settings._sync_monitor_supports_parameterization():
			async with _package_lock:
				if _package_ref is ...:
					monitor = pulumi.runtime.settings.get_monitor()
					parameterization = resource_pb2.Parameterization(
						name=%q,
						version=get_version(),
						value=base64.b64decode(%q),
					)
					registerPackageResponse = monitor.RegisterPackage(
						resource_pb2.RegisterPackageRequest(
							name=%q,
							version=%q,
							download_url=get_plugin_download_url(),
							parameterization=parameterization,
						))
					_package_ref = registerPackageResponse.ref
	# TODO: This check is only needed for parameterized providers, normal providers can return None for get_package when we start
	# using package with them.
	if _package_ref is None or _package_ref is ...:
		raise Exception("The Pulumi CLI does not support parameterization. Please update the Pulumi CLI.")
	return _package_ref
	`,
			pkg.Name, param, pkg.Parameterization.BaseProvider.Name, pkg.Parameterization.BaseProvider.Version)
		if err != nil {
			return nil, err
		}
	}

	return buffer.Bytes(), nil
}

func (mod *modContext) gen(fs codegen.Fs) error {
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
		fs.Add(p, []byte(contents))
	}

	// Utilities, config, readme
	switch mod.mod {
	case "":
		utils, err := mod.genUtilitiesFile()
		if err != nil {
			return err
		}
		fs.Add(filepath.Join(dir, "_utilities.py"), utils)
		fs.Add(filepath.Join(dir, "py.typed"), []byte{})

		// Ensure that the top-level (provider) module directory contains a README.md file.

		pkg, err := mod.pkg.Definition()
		if err != nil {
			return err
		}

		var readme string
		if pythonInfo, ok := pkg.Language["python"]; ok {
			if typedInfo, ok := pythonInfo.(PackageInfo); ok {
				readme = typedInfo.Readme
			}
		}

		if readme == "" {
			readme = mod.pkg.Description()
			if readme != "" && readme[len(readme)-1] != '\n' {
				readme += "\n"
			}
			if pkg.Attribution != "" {
				if len(readme) != 0 {
					readme += "\n"
				}
				readme += pkg.Attribution
			}
			if readme != "" && readme[len(readme)-1] != '\n' {
				readme += "\n"
			}
		}
		fs.Add(filepath.Join(dir, "README.md"), []byte(readme))

	case "config":
		config, err := mod.pkg.Config()
		if err != nil {
			return err
		}
		if len(config) > 0 {
			vars, err := mod.genConfig(config)
			if err != nil {
				return err
			}
			addFile("vars.py", vars)
			typeStubs, err := mod.genConfigStubs(config)
			if err != nil {
				return err
			}
			addFile("__init__.pyi", typeStubs)
		}
	}

	// Resources
	for _, r := range mod.resources {
		if r.IsOverlay {
			// This resource code is generated by the provider, so no further action is required.
			continue
		}

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
		if f.IsOverlay {
			// This function code is generated by the provider, so no further action is required.
			continue
		}

		if f.MultiArgumentInputs {
			return fmt.Errorf("python SDK-gen does not implement MultiArgumentInputs for function '%s'",
				f.Token)
		}

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
		fs.Add(path.Join(dir, "__init__.py"), []byte(mod.genInit(exports)))
	}

	return nil
}

func (mod *modContext) hasTypes(input bool) bool {
	if allTypesAreOverlays(mod.types) {
		return false
	}
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
		len(mod.enums) > 0 || mod.isConfig {
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
	for _, submod := range mod.children {
		if !submod.isEmpty() {
			return true
		}
	}
	return false
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
		return fmt.Sprintf("%s.%s", PyPack(mod.pkg.Namespace(), mod.pkg.Name()), name)
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
				unq := submod.unqualifiedImportName()

				// The `__iam = iam` hack enables
				// PyCharm and VSCode completion to do
				// better.
				//
				// See https://github.com/pulumi/pulumi/issues/7367
				fmt.Fprintf(w, "    import %s as __%s\n    %s = __%s\n",
					submod.fullyQualifiedImportName(),
					unq,
					unq,
					unq)
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
		contract.AssertNoErrorf(err, "error generating resource mappings")
	}

	return w.String()
}

func (mod *modContext) getRelImportFromRoot(target string) string {
	rel, err := filepath.Rel(mod.mod, target)
	contract.AssertNoErrorf(err, "error turning %q into a relative path", mod.mod)
	if path.Base(rel) == "." {
		rel = path.Dir(rel)
	}
	return relPathToRelImport(rel)
}

func (mod *modContext) genUtilitiesImport() string {
	rel, err := filepath.Rel(mod.mod, "")
	contract.AssertNoErrorf(err, "error turning %q into a relative path", mod.mod)
	relRoot := path.Dir(rel)
	relImport := relPathToRelImport(relRoot)
	return fmt.Sprintf("from %s import _utilities", relImport)
}

func (mod *modContext) importObjectType(t *schema.ObjectType, input bool) string {
	if !codegen.PkgEquals(t.PackageReference, mod.pkg) {
		return "import " + PyPack(t.PackageReference.Namespace(), t.PackageReference.Name())
	}

	tok := t.Token
	parts := strings.Split(tok, ":")
	contract.Assertf(len(parts) == 3, "type token %q is not in the form '<pkg>:<mod>:<type>'", tok)

	modName := mod.tokenToModule(tok)
	if modName == mod.mod {
		if input {
			return "from ._inputs import *"
		}
		return "from . import outputs"
	}

	importPath := mod.getRelImportFromRoot("")

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

func (mod *modContext) importEnumType(e *schema.EnumType) string {
	if !codegen.PkgEquals(e.PackageReference, mod.pkg) {
		return "import " + PyPack(e.PackageReference.Namespace(), e.PackageReference.Name())
	}

	modName := mod.tokenToModule(e.Token)
	if modName == mod.mod {
		return "from ._enums import *"
	}

	importPath := mod.getRelImportFromRoot("")

	if modName == "" {
		return fmt.Sprintf("from %s import _enums as _root_enums", importPath)
	}

	components := strings.Split(modName, "/")
	return fmt.Sprintf("from %s import %s", importPath, components[0])
}

func (mod *modContext) importResourceType(r *schema.ResourceType) string {
	if r.Resource != nil && !codegen.PkgEquals(r.Resource.PackageReference, mod.pkg) {
		return "import " + PyPack(r.Resource.PackageReference.Namespace(), r.Resource.PackageReference.Name())
	}

	tok := r.Token
	parts := strings.Split(tok, ":")
	contract.Assertf(len(parts) == 3, "type token %q is not in the form '<pkg>:<mod>:<type>'", tok)

	// If it's a provider resource, import the top-level package.
	if parts[0] == "pulumi" && parts[1] == "providers" {
		return "import pulumi_" + parts[2]
	}

	modName := mod.tokenToModule(tok)
	if mod.mod == modName || modName == "" {
		// We want a relative import if we're in the same module: from .some_member import SomeMember
		importPath := mod.getRelImportFromRoot(modName)

		name := PyName(tokenToName(r.Token))
		if mod.compatibility == kubernetes20 {
			// To maintain backward compatibility for kubernetes, the file names
			// need to be CamelCase instead of the standard snake_case.
			name = tokenToName(r.Token)
		}
		if r.Resource != nil && r.Resource.IsProvider {
			name = "provider"
		}

		if strings.HasSuffix(importPath, ".") {
			importPath += name
		} else {
			importPath = importPath + "." + name
		}

		resourceName := mod.tokenToResource(tok)

		return fmt.Sprintf("from %s import %s", importPath, resourceName)
	}

	components := strings.Split(modName, "/")
	return fmt.Sprintf("from %s import %[2]s as _%[2]s", mod.getRelImportFromRoot(""), components[0])
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
	fmt.Fprintf(w, "__config__ = pulumi.Config('%s')\n", mod.pkg.Name())
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

func allTypesAreOverlays(types []*schema.ObjectType) bool {
	for _, t := range types {
		if !t.IsOverlay {
			return false
		}
	}
	return true
}

func (mod *modContext) genTypes(dir string, fs codegen.Fs) error {
	genTypes := func(file string, input bool) error {
		w := &bytes.Buffer{}

		if allTypesAreOverlays(mod.types) {
			// If all resources in this module are overlays, skip further code generation.
			return nil
		}

		imports := imports{}
		for _, t := range mod.types {
			if t.IsOverlay {
				// This type is generated by the provider, so no further action is required.
				continue
			}

			if input && mod.details(t).inputType {
				visitObjectTypes(t.Properties, func(t schema.Type) {
					switch t := t.(type) {
					case *schema.ObjectType:
						imports.addTypeIf(mod, t, true /*input*/, func(imp string) bool {
							// No need to import `._inputs` inside _inputs.py.
							return imp != "from ._inputs import *"
						})
					case *schema.EnumType:
						imports.addEnum(mod, t)
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
			imports.addEnum(mod, e)
		}

		mod.genHeader(w, true /*needsSDK*/, imports)

		// Export only the symbols we want exported.
		fmt.Fprintf(w, "__all__ = [\n")
		for _, t := range mod.types {
			if t.IsOverlay {
				// This type is generated by the provider, so no further action is required.
				continue
			}

			if input && mod.details(t).inputType || !input && mod.details(t).outputType {
				fmt.Fprintf(w, "    '%s',\n", mod.unqualifiedObjectTypeName(t, input))
			}
			if input && typedDictEnabled(mod.inputTypes) && mod.details(t).inputType {
				fmt.Fprintf(w, "    '%sDict',\n", mod.unqualifiedObjectTypeName(t, input))
			}
		}
		fmt.Fprintf(w, "]\n\n")

		if input && typedDictEnabled(mod.inputTypes) {
			fmt.Fprintf(w, "MYPY = False\n\n")
		}

		var hasTypes bool
		for _, t := range mod.types {
			if t.IsOverlay {
				// This type is generated by the provider, so no further action is required.
				continue
			}

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
			fs.Add(path.Join(dir, file), w.Bytes())
		}
		return nil
	}
	if err := genTypes("_inputs.py", true); err != nil {
		return err
	}
	return genTypes("outputs.py", false)
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
	if len(obj.Properties) == 0 {
		fmt.Fprintf(w, "        pass")
	}
	for _, prop := range obj.Properties {
		// Check that required arguments are present.  Also check that types are as expected.
		pname := PyName(prop.Name)
		ptype := mod.pyType(prop.Type)
		fmt.Fprintf(w, "        if %s and not isinstance(%s, %s):\n", pname, pname, ptype)
		fmt.Fprintf(w, "            raise TypeError(\"Expected argument '%s' to be a %s\")\n", pname, ptype)

		// Now perform the assignment.
		fmt.Fprintf(w, "        pulumi.set(__self__, \"%[1]s\", %[1]s)\n", pname)
	}
	fmt.Fprintf(w, "\n")

	// Write out Python property getters for each property.
	// Note that deprecation messages will be emitted on access to the property, rather than initialization.
	// This avoids spamming end users with irrelevant deprecation messages.
	mod.genProperties(w, obj.Properties, false /*setters*/, "", func(prop *schema.Property) string {
		return mod.typeString(prop.Type, false /*input*/, false /*acceptMapping*/, false /*forDict*/)
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
	fmt.Fprintf(w, "        return %s(", baseName)
	for i, prop := range obj.Properties {
		if i > 0 {
			fmt.Fprintf(w, ",")
		}
		pname := PyName(prop.Name)
		fmt.Fprintf(w, "\n            %s=self.%s", pname, pname)
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
		returnType := returnTypeObject(method.Function)
		if returnType != nil {
			mod.collectImportsForResource(returnType.Properties, imports, false /*input*/, res)
		} else if method.Function.ReturnTypePlain {
			mod.collectImportsForResource([]*schema.Property{{
				Name:  "res",
				Type:  method.Function.ReturnType,
				Plain: true,
			}}, imports, false /*input*/, res)
		}
	}

	mod.genHeader(w, true /*needsSDK*/, imports)

	name := resourceName(res)

	resourceArgsName := name + "Args"
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
					return "", fmt.Errorf("resource args class named %s in %s conflicts with input type", resourceArgsName, mod.mod)
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

	fmt.Fprintln(w)
	fmt.Fprintf(w, "    pulumi_type = \"%s\"\n", res.Token)
	fmt.Fprintln(w)

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
			ty := mod.typeString(codegen.OptionalType(prop), true, true /*acceptMapping*/, false /*forDict*/)
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
	fmt.Fprintf(w, "        opts = pulumi.ResourceOptions.merge(_utilities.get_resource_opts_defaults(), opts)\n")
	fmt.Fprintf(w, "        if not isinstance(opts, pulumi.ResourceOptions):\n")
	fmt.Fprintf(w, "            raise TypeError('Expected resource options to be a ResourceOptions instance')\n")
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
		handledSecret := false
		if res.IsProvider && !isStringType(prop.Type) {
			if prop.Secret {
				arg = fmt.Sprintf("pulumi.Output.secret(%s).apply(pulumi.runtime.to_json) if %s is not None else None", arg, arg)
				handledSecret = true
			} else {
				arg = fmt.Sprintf("pulumi.Output.from_input(%s).apply(pulumi.runtime.to_json) if %s is not None else None", arg, arg)
			}
		}
		name := PyName(prop.Name)
		if prop.Secret && !handledSecret {
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
			fmt.Fprintf(w, "pulumi.Alias(type_=\"%v\")", alias.Type)
			if i != len(res.Aliases)-1 {
				fmt.Fprintf(w, ", ")
			}
		}

		fmt.Fprintf(w, "])\n")
		fmt.Fprintf(w, "        opts = pulumi.ResourceOptions.merge(opts, alias_opts)\n")
	}

	if len(secretProps) > 0 {
		fmt.Fprintf(w, `        secret_opts = pulumi.ResourceOptions(additional_secret_outputs=["%s"])`, strings.Join(secretProps, `", "`))
		fmt.Fprintf(w, "\n        opts = pulumi.ResourceOptions.merge(opts, secret_opts)\n")
	}

	replaceOnChangesProps, errList := res.ReplaceOnChanges()
	for _, err := range errList {
		cmdutil.Diag().Warningf(&diag.Diag{Message: err.Error()})
	}
	if len(replaceOnChangesProps) > 0 {
		replaceOnChangesStrings := schema.PropertyListJoinToString(replaceOnChangesProps,
			func(x string) string { return x })
		fmt.Fprintf(w, `        replace_on_changes = pulumi.ResourceOptions(replace_on_changes=["%s"])`, strings.Join(replaceOnChangesStrings, `", "`))
		fmt.Fprintf(w, "\n        opts = pulumi.ResourceOptions.merge(opts, replace_on_changes)\n")
	}

	// Finally, chain to the base constructor, which will actually register the resource.
	tok := res.Token
	if res.IsProvider {
		tok = mod.pkg.Name()
	}
	fmt.Fprintf(w, "        super(%s, __self__).__init__(\n", name)
	fmt.Fprintf(w, "            '%s',\n", tok)
	fmt.Fprintf(w, "            resource_name,\n")
	fmt.Fprintf(w, "            __props__,\n")
	if res.IsComponent {
		fmt.Fprintf(w, "            opts,\n")
		fmt.Fprintf(w, "            remote=True")
	} else {
		fmt.Fprintf(w, "            opts")
	}
	pkg, err := res.PackageReference.Definition()
	if err != nil {
		return "", err
	}
	if pkg.Parameterization != nil {
		fmt.Fprintf(w, ",\n            package_ref=_utilities.get_package()")
	}
	fmt.Fprintf(w, ")\n\n")

	if !res.IsProvider && !res.IsComponent {
		fmt.Fprintf(w, "    @staticmethod\n")
		fmt.Fprintf(w, "    def get(resource_name: str,\n")
		fmt.Fprintf(w, "            id: pulumi.Input[str],\n")
		fmt.Fprintf(w, "            opts: Optional[pulumi.ResourceOptions] = None")

		if hasStateInputs {
			for _, prop := range res.StateInputs.Properties {
				pname := InitParamName(prop.Name)
				ty := mod.typeString(codegen.OptionalType(prop), true, true /*acceptMapping*/, false /*forDict*/)
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
		ty := mod.typeString(prop.Type, false /*input*/, false /*acceptMapping*/, false /*forDict*/)
		return fmt.Sprintf("pulumi.Output[%s]", ty)
	})

	// Write out methods.
	mod.genMethods(w, res)

	return w.String(), nil
}

func (mod *modContext) genProperties(w io.Writer, properties []*schema.Property, setters bool, indent string,
	propType func(prop *schema.Property) string,
) {
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
		if prop.DeprecationMessage != "" {
			escaped := strings.ReplaceAll(prop.DeprecationMessage, `"`, `\"`)
			fmt.Fprintf(w, "%s    @_utilities.deprecated(\"\"\"%s\"\"\")\n", indent, escaped)
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

func (mod *modContext) genMethodReturnType(w io.Writer, method *schema.Method) string {
	var properties []*schema.Property
	var comment string

	if obj := returnTypeObject(method.Function); obj != nil {
		properties = obj.Properties
		comment = obj.Comment
	} else if method.Function.ReturnTypePlain {
		comment = ""
		properties = []*schema.Property{
			{
				Name:  "res",
				Type:  method.Function.ReturnType,
				Plain: true,
			},
		}
	}

	name := pyClassName(title(method.Name)) + "Result"

	// Produce a class definition with optional """ comment.
	fmt.Fprintf(w, "    @pulumi.output_type\n")
	fmt.Fprintf(w, "    class %s:\n", name)
	printComment(w, comment, "        ")

	// Now generate an initializer with properties for all inputs.
	fmt.Fprintf(w, "        def __init__(__self__")
	for _, prop := range properties {
		fmt.Fprintf(w, ", %s=None", PyName(prop.Name))
	}
	fmt.Fprintf(w, "):\n")
	for _, prop := range properties {
		// Check that required arguments are present.  Also check that types are as expected.
		pname := PyName(prop.Name)
		ptype := mod.pyType(prop.Type)
		fmt.Fprintf(w, "            if %s and not isinstance(%s, %s):\n", pname, pname, ptype)
		fmt.Fprintf(w, "                raise TypeError(\"Expected argument '%s' to be a %s\")\n", pname, ptype)

		// Now perform the assignment.
		fmt.Fprintf(w, "            pulumi.set(__self__, \"%[1]s\", %[1]s)\n", pname)
	}
	fmt.Fprintf(w, "\n")

	// Write out Python property getters for each property.
	// Note that deprecation messages will be emitted on access to the property, rather than initialization.
	// This avoids spamming end users with irrelevant deprecation messages.
	mod.genProperties(w, properties, false /*setters*/, "    ", func(prop *schema.Property) string {
		return mod.typeString(prop.Type, false /*input*/, false /*acceptMapping*/, false /*forDict*/)
	})

	return name
}

func (mod *modContext) genMethods(w io.Writer, res *schema.Resource) {
	genMethod := func(method *schema.Method) {
		methodName := PyName(method.Name)
		fun := method.Function

		returnType := returnTypeObject(fun)
		shouldLiftReturn := mod.liftSingleValueMethodReturns && returnType != nil && len(returnType.Properties) == 1

		// If there is a return type, emit it.
		var retTypeName, retTypeNameQualified, retTypeNameQualifiedOutput, methodRetType string
		if returnType != nil || fun.ReturnTypePlain {
			retTypeName = mod.genMethodReturnType(w, method)
			retTypeNameQualified = fmt.Sprintf("%s.%s", resourceName(res), retTypeName)
			retTypeNameQualifiedOutput = fmt.Sprintf("pulumi.Output['%s']", retTypeNameQualified)
			if shouldLiftReturn {
				methodRetType = fmt.Sprintf("pulumi.Output['%s']", mod.pyType(returnType.Properties[0].Type))
			} else if fun.ReturnTypePlain {
				if returnType != nil {
					methodRetType = retTypeName
				} else {
					methodRetType = mod.pyType(fun.ReturnType)
				}
			} else {
				methodRetType = retTypeNameQualifiedOutput
			}
		}

		var args []*schema.Property
		if fun.Inputs != nil {
			// Filter out the __self__ argument from the inputs.
			args = slice.Prealloc[*schema.Property](len(fun.Inputs.InputShape.Properties) - 1)
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
			ty := mod.typeString(arg.Type, true, false /*acceptMapping*/, false /*forDict*/)
			var defaultValue string
			if !arg.IsRequired() {
				defaultValue = " = None"
			}
			fmt.Fprintf(w, ",\n%s%s: %s%s", indent, pname, ty, defaultValue)
		}
		if retTypeNameQualifiedOutput != "" {
			fmt.Fprintf(w, ") -> %s:\n", methodRetType)
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
		trailingArgs := ""
		if retTypeNameQualified != "" {
			// Pass along the private output_type we generated, so any nested output classes are instantiated by
			// the call.
			trailingArgs = ", typ=" + retTypeNameQualified
		}

		// If the call is on a parameterized package, make sure we pass the parameter.
		pkg, err := fun.PackageReference.Definition()
		contract.AssertNoErrorf(err, "can not load package definition for %s: %s", pkg.Name, err)
		if pkg.Parameterization != nil {
			trailingArgs += ", package_ref=_utilities.get_package()"
		}

		if fun.ReturnTypePlain {
			property := ""
			// For non-object singleton return types, unwrap the magic property "res".
			if returnType == nil {
				property = "." + PyName("res")
			}
			fmt.Fprintf(w, "        return _utilities.call_plain('%s', __args__, res=__self__%s)%s\n",
				fun.Token, trailingArgs, property)
		} else if returnType == nil {
			fmt.Fprintf(w, "        pulumi.runtime.call('%s', __args__, res=__self__%s)\n", fun.Token, trailingArgs)
		} else if shouldLiftReturn {
			// Store the return in a variable and return the property output
			fmt.Fprintf(w, "        __result__ = pulumi.runtime.call('%s', __args__, res=__self__%s)\n", fun.Token, trailingArgs)
			fmt.Fprintf(w, "        return __result__.%s\n", PyName(returnType.Properties[0].Name))
		} else {
			// Otherwise return the call directly
			fmt.Fprintf(w, "        return pulumi.runtime.call('%s', __args__, res=__self__%s)\n", fun.Token, trailingArgs)
		}

		fmt.Fprintf(w, "\n")
	}

	for _, method := range res.Methods {
		genMethod(method)
	}
}

func (mod *modContext) genFunction(fun *schema.Function) (string, error) {
	w := &bytes.Buffer{}

	imports := imports{}
	if fun.Inputs != nil {
		mod.collectImports(fun.Inputs.Properties, imports, true)
	}

	var returnType *schema.ObjectType
	if fun.ReturnType != nil {
		if objectType, ok := fun.ReturnType.(*schema.ObjectType); ok {
			returnType = objectType
		} else {
			// TODO: remove when we add support for generalized return type for python
			return "", fmt.Errorf("python sdk-gen doesn't support non-Object return types for function %s", fun.Token)
		}
	}

	if returnType != nil {
		mod.collectImports(returnType.Properties, imports, false)
	}

	mod.genFunctionHeader(w, fun, imports)

	var baseName, awaitableName string
	if returnType != nil {
		baseName, awaitableName = awaitableTypeNames(returnType.Token)
	}
	name := PyName(tokenToName(fun.Token))

	// Export only the symbols we want exported.
	fmt.Fprintf(w, "__all__ = [\n")
	if returnType != nil {
		fmt.Fprintf(w, "    '%s',\n", baseName)
		fmt.Fprintf(w, "    '%s',\n", awaitableName)
	}
	fmt.Fprintf(w, "    '%s',\n", name)
	if fun.NeedsOutputVersion() {
		fmt.Fprintf(w, "    '%s_output',\n", name)
	}
	fmt.Fprintf(w, "]\n\n")

	if fun.DeprecationMessage != "" {
		escaped := strings.ReplaceAll(fun.DeprecationMessage, `"`, `\"`)
		fmt.Fprintf(w, "warnings.warn(\"\"\"%s\"\"\", DeprecationWarning)\n\n", escaped)
	}

	// If there is a return type, emit it.
	var retTypeName string
	var retTypeNameOutput string
	var rets []*schema.Property
	if returnType != nil {
		retTypeName, rets = mod.genAwaitableType(w, returnType), returnType.Properties
		originalOutputTypeName, _ := awaitableTypeNames(returnType.Token)
		retTypeNameOutput = fmt.Sprintf("pulumi.Output[%s]", originalOutputTypeName)
		fmt.Fprintf(w, "\n\n")
	} else {
		retTypeName = "Awaitable[None]"
		retTypeNameOutput = "pulumi.Output[None]"
	}

	var args []*schema.Property
	if fun.Inputs != nil {
		args = fun.Inputs.Properties
	}

	genFunctionDef := func(returnTypeName string, plain bool) error {
		fnName := name
		resultType := returnTypeName
		optionsClass := "InvokeOptions"
		if !plain {
			fnName += "_output"
			resultType, _ = awaitableTypeNames(returnType.Token)
			optionsClass = "InvokeOutputOptions"
		}

		mod.genFunDef(w, fnName, returnTypeName, args, !plain /* wrapInput */, plain)
		mod.genFunDocstring(w, fun)
		mod.genFunDeprecationMessage(w, fun)
		// Copy the function arguments into a dictionary.
		fmt.Fprintf(w, "    __args__ = dict()\n")
		for _, arg := range args {
			// TODO: args validation.
			fmt.Fprintf(w, "    __args__['%s'] = %s\n", arg.Name, PyName(arg.Name))
		}
		// If the caller explicitly specified a version, use it, otherwise inject this package's version.
		fmt.Fprintf(w, "    opts = pulumi.%s.merge(_utilities.get_invoke_opts_defaults(), opts)\n", optionsClass)

		// Now simply invoke the runtime function with the arguments.
		trailingArgs := ""
		if returnType != nil {
			// Pass along the private output_type we generated, so any nested outputs classes are instantiated by
			// the call to invoke.
			trailingArgs += ", typ=" + baseName
		}

		// If the invoke is on a parameterized package, make sure we pass the
		// parameter.
		pkg, err := fun.PackageReference.Definition()
		if err != nil {
			return err
		}
		if pkg.Parameterization != nil {
			trailingArgs += ", package_ref=_utilities.get_package()"
		}

		runtimeFunction := "invoke"
		if !plain {
			runtimeFunction = "invoke_output"
		}

		fmt.Fprintf(w, "    __ret__ = pulumi.runtime.%s('%s', __args__, opts=opts%s)",
			runtimeFunction,
			fun.Token,
			trailingArgs)

		if plain {
			// If the function is plain, we need to return the result directly.
			fmt.Fprint(w, ".value\n")
		}
		fmt.Fprintf(w, "\n")

		// And copy the results to an object, if there are indeed any expected returns.
		if returnType != nil {
			if plain {
				fmt.Fprintf(w, "    return %s(", resultType)
			} else {
				fmt.Fprintf(w, "    return __ret__.apply(lambda __response__: %s(", resultType)
			}

			getter := "__ret__"
			if !plain {
				getter = "__response__"
			}

			for i, ret := range rets {
				if i > 0 {
					fmt.Fprintf(w, ",")
				}
				// Use the `pulumi.get()` utility instead of calling `__ret__.field` directly.
				// This avoids calling getter functions which will print deprecation messages on unused
				// fields and should be hidden from the user during Result instantiation.
				fmt.Fprintf(w, "\n        %[1]s=pulumi.get(%[2]s, '%[1]s')", PyName(ret.Name), getter)
			}

			if plain {
				fmt.Fprintf(w, ")\n")
			} else {
				fmt.Fprintf(w, "))\n")
			}
		}

		return nil
	}

	// generate plain invoke
	if err := genFunctionDef(retTypeName, true /* plain */); err != nil {
		return "", err
	}

	if fun.NeedsOutputVersion() {
		// generate output-versioned invoke
		if err := genFunctionDef(retTypeNameOutput, false /* plain */); err != nil {
			return "", err
		}
	}

	return w.String(), nil
}

func (mod *modContext) genFunDocstring(w io.Writer, fun *schema.Function) {
	var args []*schema.Property
	if fun.Inputs != nil {
		args = fun.Inputs.Properties
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
}

func (mod *modContext) genFunDeprecationMessage(w io.Writer, fun *schema.Function) {
	if fun.DeprecationMessage == "" {
		return
	}
	name := PyName(tokenToName(fun.Token))
	fmt.Fprintf(w, "    pulumi.log.warn(\"\"\"%s is deprecated: %s\"\"\")\n", name, fun.DeprecationMessage)
}

// Generates the function signature line `def fn(...):` without the body.
func (mod *modContext) genFunDef(w io.Writer, name, retTypeName string, args []*schema.Property, wrapInput, plain bool) {
	def := fmt.Sprintf("def %s(", name)
	var indent string
	if len(args) > 0 {
		indent = strings.Repeat(" ", len(def))
	}
	fmt.Fprint(w, def)
	for i, arg := range args {
		var ind string
		if i != 0 {
			ind = indent
		}
		pname := PyName(arg.Name)

		var argType schema.Type
		if wrapInput {
			argType = &schema.OptionalType{
				ElementType: &schema.InputType{
					ElementType: arg.Type,
				},
			}
		} else {
			argType = codegen.OptionalType(arg)
		}

		ty := mod.typeString(argType, true /*input*/, true /*acceptMapping*/, false /*forDict*/)
		fmt.Fprintf(w, "%s%s: %s = None,\n", ind, pname, ty)
	}
	if plain {
		fmt.Fprintf(w, "%sopts: Optional[pulumi.InvokeOptions] = None", indent)
	} else {
		fmt.Fprintf(w, "%sopts: Optional[Union[pulumi.InvokeOptions, pulumi.InvokeOutputOptions]] = None", indent)
	}
	if retTypeName != "" {
		fmt.Fprintf(w, ") -> %s:\n", retTypeName)
	} else {
		fmt.Fprintf(w, "):\n")
	}
}

func returnTypeObject(fun *schema.Function) *schema.ObjectType {
	if fun.ReturnType != nil {
		if objectType, ok := fun.ReturnType.(*schema.ObjectType); ok && objectType != nil {
			return objectType
		}
	}
	return nil
}

func (mod *modContext) genEnums(w io.Writer, enums []*schema.EnumType) error {
	// Header
	mod.genHeader(w, false /*needsSDK*/, nil)

	// Enum import
	fmt.Fprintf(w, "import builtins\n")
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
	underlyingType := mod.typeString(enum.ElementType, false, false, false /*forDict*/)

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
				printComment(w, e.Comment, indent)
			}
		}
	default:
		return fmt.Errorf("enums of type %s are not yet implemented for this language", enum.ElementType.String())
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
	res *schema.Resource,
) {
	codegen.VisitTypeClosure(properties, func(t schema.Type) {
		switch t := t.(type) {
		case *schema.ObjectType:
			imports.addType(mod, t, input)
		case *schema.EnumType:
			imports.addEnum(mod, t)
		case *schema.ResourceType:
			// Don't import itself.
			if t.Resource != res {
				imports.addResource(mod, t)
			}
		}
	})
}

var (
	requirementRegex = regexp.MustCompile(`^>=([^,]+),<[^,]+$`)
	pep440AlphaRegex = regexp.MustCompile(`^(\d+\.\d+\.\d)+a(\d+)$`)
	pep440BetaRegex  = regexp.MustCompile(`^(\d+\.\d+\.\d+)b(\d+)$`)
	pep440RCRegex    = regexp.MustCompile(`^(\d+\.\d+\.\d+)rc(\d+)$`)
	pep440DevRegex   = regexp.MustCompile(`^(\d+\.\d+\.\d+)\.dev(\d+)$`)
)

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
	pulumiPlugin := &plugin.PulumiPluginJSON{
		Resource: true,
		Name:     pkg.Name,
		Server:   pkg.PluginDownloadURL,
	}

	if info, ok := pkg.Language["python"].(PackageInfo); pkg.Version != nil && ok && info.RespectSchemaVersion {
		pulumiPlugin.Version = pkg.Version.String()
	} else if pkg.SupportPack {
		if pkg.Version == nil {
			return nil, errors.New("package version is required")
		}
		pulumiPlugin.Version = pkg.Version.String()
	}
	if pkg.Parameterization != nil {
		// For a parameterized package the plugin name/version is from the base provider information, not the
		// top-level package name/version.
		pulumiPlugin.Parameterization = &plugin.PulumiParameterizationJSON{
			Name:    pulumiPlugin.Name,
			Version: pulumiPlugin.Version,
			Value:   pkg.Parameterization.Parameter,
		}
		pulumiPlugin.Name = pkg.Parameterization.BaseProvider.Name
		pulumiPlugin.Version = pkg.Parameterization.BaseProvider.Version.String()
	}

	return pulumiPlugin.JSON()
}

// genPackageMetadata generates all the non-code metadata required by a Pulumi package.
func genPackageMetadata(
	tool string, pkg *schema.Package, pyPkgName string, requires map[string]string,
) (string, error) {
	w := &bytes.Buffer{}
	(&modContext{tool: tool}).genHeader(w, false /*needsSDK*/, nil)

	// Now create a standard Python package from the metadata.
	fmt.Fprintf(w, "import errno\n")
	fmt.Fprintf(w, "from setuptools import setup, find_packages\n")
	fmt.Fprintf(w, "from setuptools.command.install import install\n")
	fmt.Fprintf(w, "from subprocess import check_call\n")
	fmt.Fprintf(w, "\n\n")

	// Create a constant for the version number to replace during build
	version := "\"0.0.0\""
	info, ok := pkg.Language["python"].(PackageInfo)
	if pkg.Version != nil && ok && info.RespectSchemaVersion {
		version = "\"" + PypiVersion(*pkg.Version) + "\""
	} else if pkg.SupportPack {
		// Parameterized schemas _always_ respect schema version
		if pkg.Version == nil {
			return "", errors.New("package version is required")
		}
		version = "\"" + PypiVersion(*pkg.Version) + "\""
	}
	fmt.Fprintf(w, "VERSION = %s\n", version)

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
	// Supply a default Python version if the schema does not provide one.
	pythonVersion := defaultMinPythonVersion
	if minPythonVersion, err := minimumPythonVersion(info); err == nil {
		pythonVersion = minPythonVersion
	}
	fmt.Fprintf(w, "      python_requires='%s',\n", pythonVersion)
	fmt.Fprintf(w, "      version=VERSION,\n")
	if pkg.Description != "" {
		fmt.Fprintf(w, "      description=%q,\n", sanitizePackageDescription(pkg.Description))
	}
	fmt.Fprintf(w, "      long_description=readme(),\n")
	fmt.Fprintf(w, "      long_description_content_type='text/markdown',\n")
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
	urls := mapURLs(pkg)

	if homepage, ok := urls["Homepage"]; ok {
		fmt.Fprintf(w, "      url='%s',\n", homepage)
	}
	if repo, ok := urls["Repository"]; ok {
		fmt.Fprintf(w, "      project_urls={\n")
		fmt.Fprintf(w, "          'Repository': '%s'\n", repo)
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
	fmt.Fprintf(w, "              'pulumi-plugin.json',\n")

	fmt.Fprintf(w, "          ]\n")
	fmt.Fprintf(w, "      },\n")

	// Collect the deps into a tuple, where the first
	// element is the dep name and the second element
	// is the version constraint.
	deps, err := calculateDeps(pkg.Parameterization != nil, requires)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(w, "      install_requires=[\n          ")
	// Concat the first and second element together,
	// and break each element apart with a comman and a newline.
	depStrings := slice.Prealloc[string](len(deps))
	for _, dep := range deps {
		concat := fmt.Sprintf("'%s%s'", dep[0], dep[1])
		depStrings = append(depStrings, concat)
	}
	allDeps := strings.Join(depStrings, ",\n          ")
	// Lastly, write the deps to the buffer.
	fmt.Fprintf(w, "%s\n      ],\n", allDeps)

	fmt.Fprintf(w, "      zip_safe=False)\n")
	return w.String(), nil
}

// errNoMinimumPythonVersion is a non-fatal error indicating that the schema
// did not provide a minimum version of Python for the Package.
var errNoMinimumPythonVersion = errors.New("the schema does not provide a minimum version of Python required for this provider. It's recommended to provide a minimum version so package users can understand the package's requirements")

// minimumPythonVersion returns a string containing the version
// constraint specifying the minimumal version of Python required
// by this package. For example, ">=3.8" is satified by all versions
// of Python greater than or equal to Python 3.8.
func minimumPythonVersion(info PackageInfo) (string, error) {
	if info.PythonRequires != "" {
		return info.PythonRequires, nil
	}
	return "", errNoMinimumPythonVersion
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
//  1. Parameters are introduced using the syntax ":param <type> <name>: <comment>". Sphinx parses this and uses it
//     to populate the list of parameters for this function.
//  2. The doc string of parameters is expected to be indented to the same indentation as the type of the parameter.
//     Sphinx will complain and make mistakes if this is not the case.
//  3. The doc string can't have random newlines in it, or Sphinx will complain.
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

	ty := mod.typeString(codegen.RequiredType(prop), true, acceptMapping, false /*forDict*/)

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

func (mod *modContext) typeString(t schema.Type, input, acceptMapping bool, forDict bool) string {
	switch t := t.(type) {
	case *schema.OptionalType:
		typ := mod.typeString(t.ElementType, input, acceptMapping, forDict)
		if forDict {
			return fmt.Sprintf("NotRequired[%s]", typ)
		}
		return fmt.Sprintf("Optional[%s]", typ)
	case *schema.InputType:
		typ := mod.typeString(codegen.SimplifyInputUnion(t.ElementType), input, acceptMapping, forDict)
		if typ == "Any" {
			return typ
		}
		return fmt.Sprintf("pulumi.Input[%s]", typ)
	case *schema.EnumType:
		return mod.enumType(t)
	case *schema.ArrayType:
		return fmt.Sprintf("Sequence[%s]", mod.typeString(t.ElementType, input, acceptMapping, forDict))
	case *schema.MapType:
		return fmt.Sprintf("Mapping[str, %s]", mod.typeString(t.ElementType, input, acceptMapping, forDict))
	case *schema.ObjectType:
		if forDict {
			return mod.objectType(t, input, true /*dictType*/)
		}
		typ := mod.objectType(t, input, false /*dictType*/)
		if !acceptMapping {
			return typ
		}
		// If the type is an input and the TypedDict generation is enabled for the type's package, we
		// we can emit `Union[type, dictType]` and avoid the `InputType[]` wrapper.
		// dictType covers the Mapping case in `InputType = Union[T, Mapping[str, Any]]`.
		pkg, err := t.PackageReference.Definition()
		contract.AssertNoErrorf(err, "error loading definition for package %q", t.PackageReference.Name())
		info, ok := pkg.Language["python"].(PackageInfo)
		// TODO[https://github.com/pulumi/pulumi/issues/16702]
		// We don't yet assume that external packages support TypedDicts by default.
		// Remove samePackage condition to enable TypedDicts for external packages by default.
		samePackage := codegen.PkgEquals(t.PackageReference, mod.pkg)
		typedDicts := ok && typedDictEnabled(info.InputTypes) && samePackage
		if typedDicts && input {
			return fmt.Sprintf("Union[%s, %s]", typ, mod.objectType(t, input, true /*dictType*/))
		}
		return fmt.Sprintf("pulumi.InputType[%s]", typ)
	case *schema.ResourceType:
		return fmt.Sprintf("'%s'", mod.resourceType(t))
	case *schema.TokenType:
		// Use the underlying type for now.
		if t.UnderlyingType != nil {
			return mod.typeString(t.UnderlyingType, input, acceptMapping, forDict)
		}
		return "Any"
	case *schema.UnionType:
		if !input {
			for _, e := range t.ElementTypes {
				// If this is an output and a "relaxed" enum, emit the type as the underlying primitive type rather than the union.
				// Eg. Output[str] rather than Output[Any]
				if typ, ok := e.(*schema.EnumType); ok {
					return mod.typeString(typ.ElementType, input, acceptMapping, forDict)
				}
			}
			if t.DefaultType != nil {
				return mod.typeString(t.DefaultType, input, acceptMapping, forDict)
			}
			return "Any"
		}

		elementTypeSet := codegen.NewStringSet()
		elements := slice.Prealloc[string](len(t.ElementTypes))
		for _, e := range t.ElementTypes {
			et := mod.typeString(e, input, acceptMapping, forDict)
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
			return "builtins.bool"
		case schema.IntType:
			return "builtins.int"
		case schema.NumberType:
			return "builtins.float"
		case schema.StringType:
			return "builtins.str"
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

// PyPack returns the suggested package name for the given string.
func PyPack(namespace, name string) string {
	if namespace == "" {
		namespace = "pulumi"
	} else {
		namespace = strings.ReplaceAll(namespace, "-", "_")
	}
	return namespace + "_" + strings.ReplaceAll(name, "-", "_")
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
	if input && typedDictEnabled(mod.inputTypes) {
		if err := mod.genDictType(w, name, obj.Comment, obj.Properties); err != nil {
			return err
		}
	}
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

	name = pythonCase(name)
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
		ty := mod.typeString(prop.Type, input, false /*acceptMapping*/, false /*forDict*/)
		if prop.DefaultValue != nil {
			ty = mod.typeString(codegen.OptionalType(prop), input, false /*acceptMapping*/, false /*forDict*/)
		}

		var defaultValue string
		if !prop.IsRequired() || prop.DefaultValue != nil {
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

		// Check that the property isn't deprecated.
		if input && prop.DeprecationMessage != "" {
			escaped := strings.ReplaceAll(prop.DeprecationMessage, `"`, `\"`)
			fmt.Fprintf(w, "        if %s is not None:\n", pname)
			fmt.Fprintf(w, "            warnings.warn(\"\"\"%s\"\"\", DeprecationWarning)\n", escaped)
			fmt.Fprintf(w, "            pulumi.log.warn(\"\"\"%s is deprecated: %s\"\"\")\n", pname, escaped)
		}

		// Fill in computed defaults for arguments.
		if prop.DefaultValue != nil {
			dv, err := getDefaultValue(prop.DefaultValue, codegen.UnwrapType(prop.Type))
			if err != nil {
				return err
			}
			fmt.Fprintf(w, "        if %s is None:\n", pname)
			fmt.Fprintf(w, "            %s = %s\n", pname, dv)
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
		return mod.typeString(prop.Type, input, false /*acceptMapping*/, false /*forDict*/)
	})

	fmt.Fprintf(w, "\n")
	return nil
}

func (mod *modContext) genDictType(w io.Writer, name, comment string, properties []*schema.Property) error {
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

	indent := "    "
	name = pythonCase(name)

	// TODO[pulumi/pulumi/issues/16408]
	// Running mypy gets very slow when there are a lot of TypedDicts.
	// https://github.com/python/mypy/issues/17231
	// For now we only use the TypedDict types when using a typechecker
	// other than mypy. For mypy we define the XXXArgsDict class as an alias
	// to the type `Mapping[str, Any]`.
	fmt.Fprintf(w, "if not MYPY:\n")
	fmt.Fprintf(w, "%sclass %sDict(TypedDict):\n", indent, name)

	indent += "    "

	if comment != "" {
		printComment(w, comment, indent)
	}

	for _, prop := range props {
		pname := PyName(prop.Name)
		ty := mod.typeString(prop.Type, true /*input*/, false /*acceptMapping*/, true /*forDict*/)
		fmt.Fprintf(w, "%s%s: %s\n", indent, pname, ty)
		if prop.Comment != "" {
			printComment(w, prop.Comment, indent)
		}
	}

	if len(props) == 0 {
		fmt.Fprintf(w, "%spass\n", indent)
	}

	indent = "    "
	fmt.Fprintf(w, "elif False:\n")
	fmt.Fprintf(w, "%s%sDict: TypeAlias = Mapping[str, Any]\n", indent, name)

	fmt.Fprintf(w, "\n")
	return nil
}

func getPrimitiveValue(value interface{}) (string, error) {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	//nolint:exhaustive // Only a subset of types can have default values
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
		return "", fmt.Errorf("unsupported default value of type %T", value)
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
	// determine whether to use the default Python package name
	pyPkgName := info.PackageName
	if pyPkgName == "" {
		pyPkgName = PyPack(pkg.Namespace, pkg.Name)
	}

	// group resources, types, and functions into modules
	// modules map will contain modContext entries for all modules in current package (pkg)
	modules := map[string]*modContext{}

	var getMod func(modName string, p schema.PackageReference) *modContext
	getMod = func(modName string, p schema.PackageReference) *modContext {
		mod, ok := modules[modName]
		if !ok {
			mod = &modContext{
				pkg:                          p,
				pyPkgName:                    pyPkgName,
				mod:                          modName,
				tool:                         tool,
				modNameOverrides:             info.ModuleNameOverrides,
				compatibility:                info.Compatibility,
				liftSingleValueMethodReturns: info.LiftSingleValueMethodReturns,
				inputTypes:                   info.InputTypes,
			}

			if modName != "" && codegen.PkgEquals(p, pkg.Reference()) {
				parentName := path.Dir(modName)
				if parentName == "." {
					parentName = ""
				}
				parent := getMod(parentName, p)
				parent.addChild(mod)
			}

			// Save the module only if it's for the current package.
			// This way, modules for external packages are not saved.

			if codegen.PkgEquals(p, pkg.Reference()) {
				modules[modName] = mod
			}
		}
		return mod
	}

	getModFromToken := func(tok string, p schema.PackageReference) *modContext {
		modName := tokenToModule(tok, p, info.ModuleNameOverrides)
		return getMod(modName, p)
	}

	// Create the config module if necessary.
	if len(pkg.Config) > 0 &&
		info.Compatibility != kubernetes20 { // k8s SDK doesn't use config.
		configMod := getMod("config", pkg.Reference())
		configMod.isConfig = true
	}

	visitObjectTypes(pkg.Config, func(t schema.Type) {
		if t, ok := t.(*schema.ObjectType); ok {
			getModFromToken(t.Token, t.PackageReference).details(t).outputType = true
		}
	})

	// Find input and output types referenced by resources.
	scanResource := func(r *schema.Resource) {
		mod := getModFromToken(r.Token, pkg.Reference())
		mod.resources = append(mod.resources, r)
		visitObjectTypes(r.Properties, func(t schema.Type) {
			switch T := t.(type) {
			case *schema.ObjectType:
				getModFromToken(T.Token, T.PackageReference).details(T).outputType = true
				getModFromToken(T.Token, T.PackageReference).details(T).resourceOutputType = true
			}
		})
		visitObjectTypes(r.InputProperties, func(t schema.Type) {
			switch T := t.(type) {
			case *schema.ObjectType:
				getModFromToken(T.Token, T.PackageReference).details(T).inputType = true
			}
		})
		if r.StateInputs != nil {
			visitObjectTypes(r.StateInputs.Properties, func(t schema.Type) {
				switch T := t.(type) {
				case *schema.ObjectType:
					getModFromToken(T.Token, T.PackageReference).details(T).inputType = true
				case *schema.ResourceType:
					getModFromToken(T.Token, T.Resource.PackageReference)
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
		mod := getModFromToken(f.Token, f.PackageReference)
		if !f.IsMethod {
			mod.functions = append(mod.functions, f)
		}
		if f.Inputs != nil {
			visitObjectTypes(f.Inputs.Properties, func(t schema.Type) {
				switch T := t.(type) {
				case *schema.ObjectType:
					getModFromToken(T.Token, T.PackageReference).details(T).inputType = true
					getModFromToken(T.Token, T.PackageReference).details(T).plainType = true
				case *schema.ResourceType:
					getModFromToken(T.Token, T.Resource.PackageReference)
				}
			})
		}

		var returnType *schema.ObjectType
		if f.ReturnType != nil {
			if objectType, ok := f.ReturnType.(*schema.ObjectType); ok && objectType != nil {
				returnType = objectType
			}
		}

		if returnType != nil {
			visitObjectTypes(returnType.Properties, func(t schema.Type) {
				switch T := t.(type) {
				case *schema.ObjectType:
					getModFromToken(T.Token, T.PackageReference).details(T).outputType = true
					getModFromToken(T.Token, T.PackageReference).details(T).plainType = true
				case *schema.ResourceType:
					getModFromToken(T.Token, T.Resource.PackageReference)
				}
			})
		}
	}

	// Find nested types.
	for _, t := range pkg.Types {
		switch typ := t.(type) {
		case *schema.ObjectType:
			mod := getModFromToken(typ.Token, typ.PackageReference)
			d := mod.details(typ)
			if d.inputType || d.outputType {
				mod.types = append(mod.types, typ)
			}
		case *schema.EnumType:
			if !typ.IsOverlay {
				mod := getModFromToken(typ.Token, pkg.Reference())
				mod.enums = append(mod.enums, typ)
			}
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
		mod := getMod(modName, pkg.Reference())
		mod.extraSourceFiles = append(mod.extraSourceFiles, p)
	}

	// Setup modLocator so that mod.typeDetails finds the right
	// modContext for every ObjectType.
	modLocator := &modLocator{
		objectTypeMod: func(t *schema.ObjectType) *modContext {
			if !codegen.PkgEquals(t.PackageReference, pkg.Reference()) {
				return nil
			}

			return getModFromToken(t.Token, t.PackageReference)
		},
	}

	for _, mod := range modules {
		mod.modLocator = modLocator
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
			if r.IsOverlay {
				// This resource code is generated by the provider, so no further action is required.
				continue
			}

			packagePath := strings.ReplaceAll(modName, "/", ".")
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

func GeneratePackage(
	tool string, pkg *schema.Package, extraFiles map[string][]byte, loader schema.ReferenceLoader,
) (map[string][]byte, error) {
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
		pkgName = PyPack(pkg.Namespace, pkg.Name)
	}

	files := codegen.Fs{}
	for p, f := range extraFiles {
		files.Add(filepath.Join(pkgName, p), f)
	}

	for _, mod := range modules {
		if err := mod.gen(files); err != nil {
			return nil, err
		}
	}

	// Generate pulumi-plugin.json
	plugin, err := genPulumiPluginFile(pkg)
	if err != nil {
		return nil, err
	}
	files.Add(filepath.Join(pkgName, "pulumi-plugin.json"), plugin)

	requires := map[string]string{}
	// Add package dependencies
	for _, dep := range pkg.Dependencies {
		ref, err := loader.LoadPackageReferenceV2(context.TODO(), &dep)
		if err != nil {
			return nil, err
		}
		namespace := "pulumi"
		if ref.Namespace() != "" {
			namespace = PyName(ref.Namespace())
		}
		requires[namespace+"_"+ref.Name()] = ">=" + dep.Version.String()
	}
	// Add language specific dependenceis
	for name, version := range info.Requires {
		requires[name] = version
	}
	// Add typing-extensions if we're using TypedDicts
	if typedDictEnabled(info.InputTypes) {
		requires["typing-extensions"] = ">=4.11,<5; python_version < \"3.11\""
	}

	// Next, emit the package metadata (setup.py).
	if !info.PyProject.Enabled {
		setup, err := genPackageMetadata(tool, pkg, pkgName, requires)
		if err != nil {
			return nil, err
		}
		files.Add("setup.py", []byte(setup))
	}

	// Finally, if pyproject.toml generation is enabled, generate
	// this file and emit it as well.
	if info.PyProject.Enabled {
		project, err := genPyprojectTOML(tool, pkg, pkgName, requires)
		if err != nil {
			return nil, err
		}
		files.Add("pyproject.toml", []byte(project))
	}

	return files, nil
}

func genPyprojectTOML(tool string,
	pkg *schema.Package,
	pyPkgName string,
	requires map[string]string,
) (string, error) {
	// First, create a Writer for everything in pyproject.toml
	w := &bytes.Buffer{}
	// Create an empty toml manifest.
	schema := new(PyprojectSchema)
	schema.Project = new(Project)
	// Populate the fields.
	schema.Project.Name = &pyPkgName
	schema.Project.Keywords = pkg.Keywords

	// Setting dependencies fails if the deps we provide specify
	// an invalid Pulumi package version as a dep.
	err := setDependencies(schema, pkg, requires)
	if err != nil {
		return "", err
	}

	// This sets the minimum version of Python.
	setPythonRequires(schema, pkg)

	// Set the project's URLs
	schema.Project.URLs = mapURLs(pkg)

	// • Description and License: These fields are populated the same
	//   way as in setup.py.
	description := sanitizePackageDescription(pkg.Description)
	schema.Project.Description = &description
	schema.Project.License = &License{
		Text: pkg.License,
	}

	// • Next, we set the version field.
	//   A Version of 0.0.0 is typically overridden elsewhere with sed
	//   or a similar tool.
	version := "0.0.0"
	info, ok := pkg.Language["python"].(PackageInfo)
	if pkg.Version != nil && ok && info.RespectSchemaVersion {
		version = PypiVersion(*pkg.Version)
	} else if pkg.SupportPack {
		if pkg.Version == nil {
			return "", errors.New("package version is required")
		}
		version = PypiVersion(*pkg.Version)
	}
	schema.Project.Version = &version

	// • Set the path to the README.
	readme := "README.md"
	schema.Project.README = &readme

	// Populate build extensions as follows:
	//
	// [build-system]
	// requires = ["setuptools>=61.0"]
	// build-backend = "setuptools.build_meta"
	// [tool.setuptools.package-data]
	// pulumi_kubernetes = ["py.typed", "pulumi-plugin.json"]
	//
	// This ensures that `python -m build` can proceed without `setup.py` present, while still
	// including the required files `py.typed` and `pulumi-plugin.json` in the distro.

	schema.BuildSystem = &BuildSystem{
		Requires:     []string{"setuptools>=61.0"},
		BuildBackend: "setuptools.build_meta",
	}

	schema.Tool = map[string]interface{}{
		"setuptools": map[string]interface{}{
			"package-data": map[string]interface{}{
				*schema.Project.Name: []string{
					"py.typed",
					"pulumi-plugin.json",
				},
			},
		},
	}

	// • Marshal the data into TOML format.
	err = toml.NewEncoder(w).Encode(schema)

	return w.String(), err
}

// mapURLs creates a map between the name of the URL and the URL itself.
// Currently, only two URLs are supported: the project "Homepage" and the
// project "Repository", which are the corresponding map keys.
func mapURLs(pkg *schema.Package) map[string]string {
	urls := map[string]string{}
	if homepage := pkg.Homepage; homepage != "" {
		urls["Homepage"] = homepage
	}
	if repo := pkg.Repository; repo != "" {
		urls["Repository"] = repo
	}
	return urls
}

// setPythonRequires adds a minimum version of Python required to run this package.
// It falls back to the default version supported by Pulumi if the user hasn't provided
// one in the schema.
func setPythonRequires(schema *PyprojectSchema, pkg *schema.Package) {
	info := pkg.Language["python"].(PackageInfo)

	// Start with the default, and replace it if the user provided
	// a specific version.
	minPython := defaultMinPythonVersion
	if userPythonVersion, err := minimumPythonVersion(info); err == nil {
		minPython = userPythonVersion
	}

	schema.Project.RequiresPython = &minPython
}

// setDependencies mutates the pyproject schema adding the dependencies to the
// list in lexical order.
func setDependencies(schema *PyprojectSchema, pkg *schema.Package, dependencies map[string]string) error {
	deps, err := calculateDeps(pkg.Parameterization != nil, dependencies)
	if err != nil {
		return err
	}
	for _, dep := range deps {
		// Append the dep constraint to the end of the dep name.
		// e.g. pulumi>=3.50.1
		depConstraint := fmt.Sprintf("%s%s", dep[0], dep[1])
		schema.Project.Dependencies = append(schema.Project.Dependencies, depConstraint)
	}

	return nil
}

// Require the SDK to fall within the same major version.
var MinimumValidSDKVersion = ">=3.142.0,<4.0.0"

// ensureValidPulumiVersion ensures that the Pulumi SDK has an entry.
// It accepts a list of dependencies
// as provided in the package schema, and validates whether
// this list correctly includes the Pulumi Python package.
// It returns a map that correctly specifies the dependency.
// This function does not modify the argument. Instead, it returns
// a copy of the original map, except that the `pulumi` key is guaranteed to have
// a valid value.
// This function returns an error if the provided Pulumi version fails to
// validate.
func ensureValidPulumiVersion(parameterized bool, requires map[string]string) (map[string]string, error) {
	deps := map[string]string{}
	// Special case: if the map is empty, we return just pulumi with the minimum version constraint.

	if len(requires) == 0 {
		result := map[string]string{
			"pulumi": MinimumValidSDKVersion,
		}
		return result, nil
	}
	// If the pulumi dep is missing, we require it to fall within
	// our major version constraint.
	if pulumiDep, ok := requires["pulumi"]; !ok {
		deps["pulumi"] = MinimumValidSDKVersion
	} else {
		// Since a value was provided, we check to make sure it's
		// within an acceptable version range.
		// We expect a specific pattern of ">=version,<version" here.
		matches := requirementRegex.FindStringSubmatch(pulumiDep)
		if len(matches) != 2 {
			return nil, fmt.Errorf("invalid requirement specifier \"%s\"; expected \">=version1,<version2\"", pulumiDep)
		}

		lowerBound, err := pep440VersionToSemver(matches[1])
		if err != nil {
			return nil, fmt.Errorf("invalid version for lower bound: %w", err)
		}
		if lowerBound.LT(oldestAllowedPulumi) {
			return nil, fmt.Errorf("lower version bound must be at least %v", oldestAllowedPulumi)
		}
		// The provided Pulumi version is valid, so we're copy it into
		// the new map.
		deps["pulumi"] = pulumiDep
	}

	// Copy the rest of the dependencies listed into deps.
	for k, v := range requires {
		if k == "pulumi" {
			continue
		}
		deps[k] = v
	}
	return deps, nil
}

// calculateDeps determines the dependencies of this project
// and orders them lexigraphical.
// This function returns a slice of tuples, where the first element
// of each tuple is the name of the dependency, and the second element
// is the dependency's version constraint.
// This function returns an error if the version of Pulumi listed as a
// dep fails to validate.
func calculateDeps(parameterized bool, requires map[string]string) ([][2]string, error) {
	var err error
	result := slice.Prealloc[[2]string](len(requires))
	if requires, err = ensureValidPulumiVersion(parameterized, requires); err != nil {
		return nil, err
	}
	// Collect all of the names into an array, including
	// two extras that we hardcode.
	// NB: I have no idea why we hardcode these values here. Because we
	// access the map later, they MUST already be in the map,
	// or else we'd be writing nil to the file, but since we append
	// them here, I'd expect them to show up twice in the output file.
	deps := []string{
		"semver>=2.8.1",
		"parver>=0.2.1",
	}
	for dep := range requires {
		deps = append(deps, dep)
	}
	sort.Strings(deps)

	for _, dep := range deps {
		next := [2]string{
			dep, requires[dep],
		}
		result = append(result, next)
	}

	return result, nil
}

//go:embed utilities.py
var utilitiesFile string
