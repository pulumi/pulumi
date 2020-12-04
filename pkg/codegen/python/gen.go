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
	"github.com/pulumi/pulumi/pkg/v2/codegen"
	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

type typeDetails struct {
	outputType   bool
	inputType    bool
	functionType bool
}

type stringSet map[string]struct{}

func (ss stringSet) add(s string) {
	ss[s] = struct{}{}
}

func (ss stringSet) has(s string) bool {
	_, ok := ss[s]
	return ok
}

type imports stringSet

func (imports imports) addType(mod *modContext, tok string, input bool) {
	imports.addTypeIf(mod, tok, input, nil /*predicate*/)
}

func (imports imports) addTypeIf(mod *modContext, tok string, input bool, predicate func(imp string) bool) {
	if imp := mod.importTypeFromToken(tok, input); imp != "" && (predicate == nil || predicate(imp)) {
		stringSet(imports).add(imp)
	}
}

func (imports imports) addEnum(mod *modContext, tok string) {
	if imp := mod.importEnumFromToken(tok); imp != "" {
		stringSet(imports).add(imp)
	}
}

func (imports imports) addResource(mod *modContext, tok string) {
	if imp := mod.importResourceFromToken(tok); imp != "" {
		stringSet(imports).add(imp)
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
	types                []*schema.ObjectType
	enums                []*schema.EnumType
	resources            []*schema.Resource
	functions            []*schema.Function
	typeDetails          map[*schema.ObjectType]*typeDetails
	children             []*modContext
	snakeCaseToCamelCase map[string]string
	camelCaseToSnakeCase map[string]string
	tool                 string
	extraSourceFiles     []string
	isConfig             bool

	// Name overrides set in PackageInfo
	modNameOverrides map[string]string // Optional overrides for Pulumi module names
	compatibility    string            // Toggle compatibility mode for a specified target.
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

func (mod *modContext) tokenToType(tok string, input, functionType bool) string {
	modName, name := mod.tokenToModule(tok), tokenToName(tok)

	var suffix string
	switch {
	case input:
		suffix = "Args"
	case functionType:
		suffix = "Result"
	}

	if modName == "" && modName != mod.mod {
		rootModName := "_root_outputs."
		if input {
			rootModName = "_root_inputs."
		}
		return fmt.Sprintf("'%s%s%s'", rootModName, name, suffix)
	}

	if modName == mod.mod {
		modName = ""
	}
	if modName != "" {
		modName = "_" + strings.ReplaceAll(modName, "/", ".") + "."
	}

	var prefix string
	if !input {
		prefix = "outputs."
	}

	return fmt.Sprintf("'%s%s%s%s'", modName, prefix, name, suffix)
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
	canonicalModName := pkg.TokenToModule(tok)
	modName := PyName(strings.ToLower(canonicalModName))
	if override, ok := moduleNameOverrides[canonicalModName]; ok {
		modName = override
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

	replacer := strings.NewReplacer(`"""`, `\"\"\"`, `\x`, `\\x`)
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
		fmt.Fprintf(w, "from typing import Any, Mapping, Optional, Sequence, Union\n")
		fmt.Fprintf(w, "from %s import _utilities, _tables\n", relImport)
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
	dir := path.Join(pyPack(mod.pkg.Name), mod.mod)

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
		exports = append(exports, name[:len(name)-len(".py")])
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

// genInit emits an __init__.py module, optionally re-exporting other members or submodules.
func (mod *modContext) genInit(exports []string) string {
	w := &bytes.Buffer{}
	mod.genHeader(w, false /*needsSDK*/, nil)

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
		sort.Slice(mod.children, func(i, j int) bool {
			return PyName(mod.children[i].mod) < PyName(mod.children[j].mod)
		})

		fmt.Fprintf(w, "\n# Make subpackages available:\n")
		fmt.Fprintf(w, "from . import (\n")
		for _, mod := range mod.children {
			if mod.isEmpty() {
				continue
			}

			child := mod.mod
			// Extract version suffix from child modules. Nested versions will have their own __init__.py file.
			// Example: apps/v1beta1 -> v1beta1
			parts := strings.SplitN(child, "/", 2)
			if len(parts) == 2 {
				child = parts[1]
			}
			fmt.Fprintf(w, "    %s,\n", PyName(child))
		}
		fmt.Fprintf(w, ")\n")
	}

	// If there are resources in this module, register the module with the runtime.
	if len(mod.resources) != 0 {
		mod.genResourceModule(w)
	}

	return w.String()
}

func (mod *modContext) getRelImportFromRoot() string {
	rel, err := filepath.Rel(mod.mod, "")
	contract.Assert(err == nil)
	relRoot := path.Dir(rel)
	return relPathToRelImport(relRoot)
}

// genResourceModule generates a ResourceModule definition and the code to register an instance thereof with the
// Pulumi runtime. The generated ResourceModule supports the deserialization of resource references into fully-
// hydrated Resource instances. If this is the root module, this function also generates a ResourcePackage
// definition and its registration to support rehydrating providers.
func (mod *modContext) genResourceModule(w io.Writer) {
	contract.Assert(len(mod.resources) != 0)

	rel, err := filepath.Rel(mod.mod, "")
	contract.Assert(err == nil)
	relRoot := path.Dir(rel)
	relImport := relPathToRelImport(relRoot)

	fmt.Fprintf(w, "\ndef _register_module():\n")
	fmt.Fprintf(w, "    import pulumi\n")
	fmt.Fprintf(w, "    from %s import _utilities\n", relImport)

	// Check for provider-only modules.
	var provider *schema.Resource
	if providerOnly := len(mod.resources) == 1 && mod.resources[0].IsProvider; providerOnly {
		provider = mod.resources[0]
	} else {
		fmt.Fprintf(w, "\n\n    class Module(pulumi.runtime.ResourceModule):\n")
		fmt.Fprintf(w, "        _version = _utilities.get_semver_version()\n")
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "        def version(self):\n")
		fmt.Fprintf(w, "            return Module._version\n")
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "        def construct(self, name: str, typ: str, urn: str) -> pulumi.Resource:\n")

		registrations, first := codegen.StringSet{}, true
		for _, r := range mod.resources {
			if r.IsProvider {
				contract.Assert(provider == nil)
				provider = r
				continue
			}

			registrations.Add(mod.pkg.TokenToRuntimeModule(r.Token))

			conditional := "elif"
			if first {
				conditional, first = "if", false
			}
			fmt.Fprintf(w, "            %v typ == \"%v\":\n", conditional, r.Token)
			fmt.Fprintf(w, "                return %v(name, pulumi.ResourceOptions(urn=urn))\n", tokenToName(r.Token))
		}
		fmt.Fprintf(w, "            else:\n")
		fmt.Fprintf(w, "                raise Exception(f\"unknown resource type {typ}\")\n")
		fmt.Fprintf(w, "\n\n")
		fmt.Fprintf(w, "    _module_instance = Module()\n")
		for _, name := range registrations.SortedValues() {
			fmt.Fprintf(w, "    pulumi.runtime.register_resource_module(\"%v\", \"%v\", _module_instance)\n", mod.pkg.Name, name)
		}
	}

	if provider != nil {
		fmt.Fprintf(w, "\n\n    class Package(pulumi.runtime.ResourcePackage):\n")
		fmt.Fprintf(w, "        _version = _utilities.get_semver_version()\n")
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "        def version(self):\n")
		fmt.Fprintf(w, "            return Package._version\n")
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "        def construct_provider(self, name: str, typ: str, urn: str) -> pulumi.ProviderResource:\n")
		fmt.Fprintf(w, "            if typ != \"%v\":\n", provider.Token)
		fmt.Fprintf(w, "                raise Exception(f\"unknown provider type {typ}\")\n")
		fmt.Fprintf(w, "            return Provider(name, pulumi.ResourceOptions(urn=urn))\n")
		fmt.Fprintf(w, "\n\n")
		fmt.Fprintf(w, "    pulumi.runtime.register_resource_package(\"%v\", Package())\n", mod.pkg.Name)
	}

	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "_register_module()\n")
}

func (mod *modContext) importTypeFromToken(tok string, input bool) string {
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

func (mod *modContext) importResourceFromToken(tok string) string {
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

	components := strings.Split(modName, "/")
	return fmt.Sprintf("from %s import %s", importPath, components[0])
}

// emitConfigVariables emits all config variables in the given module, returning the resulting file.
func (mod *modContext) genConfig(variables []*schema.Property) (string, error) {
	w := &bytes.Buffer{}

	imports, seen := imports{}, codegen.Set{}
	visitObjectTypesFromProperties(variables, seen, func(t interface{}) {
		switch T := t.(type) {
		case *schema.ObjectType:
			imports.addType(mod, T.Token, false /*input*/)
		case *schema.EnumType:
			imports.addEnum(mod, T.Token)
		case *schema.ResourceType:
			imports.addResource(mod, T.Token)
		}
	})

	mod.genHeader(w, true /*needsSDK*/, imports)

	// Export only the symbols we want exported.
	if len(variables) > 0 {
		fmt.Fprintf(w, "__all__ = [\n")
		for _, p := range variables {
			fmt.Fprintf(w, "    '%s',\n", PyName(p.Name))
		}
		fmt.Fprintf(w, "]\n\n")
	}

	// Create a config bag for the variables to pull from.
	fmt.Fprintf(w, "__config__ = pulumi.Config('%s')\n", mod.pkg.Name)
	fmt.Fprintf(w, "\n")

	// Emit an entry for all config variables.
	for _, p := range variables {
		configFetch := fmt.Sprintf("__config__.get('%s')", p.Name)
		if p.DefaultValue != nil {
			v, err := getDefaultValue(p.DefaultValue, p.Type)
			if err != nil {
				return "", err
			}
			configFetch += " or " + v
		}

		fmt.Fprintf(w, "%s = %s\n", PyName(p.Name), configFetch)
		printComment(w, p.Comment, "")
		fmt.Fprintf(w, "\n")
	}

	return w.String(), nil
}

func (mod *modContext) genTypes(dir string, fs fs) error {
	genTypes := func(file string, input bool) error {
		w := &bytes.Buffer{}

		imports, inputSeen, outputSeen := imports{}, codegen.Set{}, codegen.Set{}
		for _, t := range mod.types {
			if input && mod.details(t).inputType {
				visitObjectTypesFromProperties(t.Properties, inputSeen, func(t interface{}) {
					switch T := t.(type) {
					case *schema.ObjectType:
						imports.addTypeIf(mod, T.Token, true /*input*/, func(imp string) bool {
							// No need to import `._inputs` inside _inputs.py.
							return imp != "from ._inputs import *"
						})
					case *schema.EnumType:
						imports.addEnum(mod, T.Token)
					case *schema.ResourceType:
						imports.addResource(mod, T.Token)
					}
				})
			}
			if !input && mod.details(t).outputType {
				visitObjectTypesFromProperties(t.Properties, outputSeen, func(t interface{}) {
					switch T := t.(type) {
					case *schema.ObjectType:
						imports.addType(mod, T.Token, false /*input*/)
					case *schema.EnumType:
						imports.addEnum(mod, T.Token)
					case *schema.ResourceType:
						imports.addResource(mod, T.Token)
					}
				})
			}
		}
		for _, e := range mod.enums {
			imports.addEnum(mod, e.Token)
		}

		mod.genHeader(w, true /*needsSDK*/, imports)

		// Export only the symbols we want exported.
		fmt.Fprintf(w, "__all__ = [\n")
		for _, t := range mod.types {
			if (input && mod.details(t).inputType) || (!input && mod.details(t).outputType) {
				name := tokenToName(t.Token)
				if input {
					name += "Args"
				} else if mod.details(t).functionType {
					name += "Result"
				}
				fmt.Fprintf(w, "    '%s',\n", name)
			}
		}
		fmt.Fprintf(w, "]\n\n")

		var hasTypes bool
		for _, t := range mod.types {
			if input && mod.details(t).inputType {
				wrapInput := !mod.details(t).functionType
				if err := mod.genType(w, t, true, wrapInput); err != nil {
					return err
				}
				hasTypes = true
			}
			if !input && mod.details(t).outputType {
				if err := mod.genType(w, t, false, false); err != nil {
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
			fmt.Fprintf(w, "            pulumi.log.warn(\"%s is deprecated: %s\")\n\n", pname, escaped)
		}

		// Now perform the assignment.
		fmt.Fprintf(w, "        pulumi.set(__self__, \"%[1]s\", %[1]s)\n", pname)
	}
	fmt.Fprintf(w, "\n")

	// Write out Python property getters for each property.
	mod.genProperties(w, obj.Properties, false /*setters*/, func(prop *schema.Property) string {
		return mod.typeString(prop.Type, false /*input*/, false /*wrapInput*/, !prop.IsRequired,
			false /*acceptMapping*/)
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

func (mod *modContext) genResource(res *schema.Resource) (string, error) {
	w := &bytes.Buffer{}

	imports, inputSeen, outputSeen := imports{}, codegen.Set{}, codegen.Set{}
	visitObjectTypesFromProperties(res.Properties, outputSeen, func(t interface{}) {
		switch T := t.(type) {
		case *schema.ObjectType:
			imports.addType(mod, T.Token, false /*input*/)
		case *schema.EnumType:
			imports.addEnum(mod, T.Token)
		case *schema.ResourceType:
			imports.addResource(mod, T.Token)
		}
	})
	visitObjectTypesFromProperties(res.InputProperties, inputSeen, func(t interface{}) {
		switch T := t.(type) {
		case *schema.ObjectType:
			imports.addType(mod, T.Token, true /*input*/)
		case *schema.EnumType:
			imports.addEnum(mod, T.Token)
		case *schema.ResourceType:
			imports.addResource(mod, T.Token)
		}
	})
	if res.StateInputs != nil {
		visitObjectTypesFromProperties(res.StateInputs.Properties, inputSeen, func(t interface{}) {
			switch T := t.(type) {
			case *schema.ObjectType:
				imports.addType(mod, T.Token, true /*input*/)
			case *schema.EnumType:
				imports.addEnum(mod, T.Token)
			case *schema.ResourceType:
				imports.addResource(mod, T.Token)
			}
		})
	}

	mod.genHeader(w, true /*needsSDK*/, imports)

	name := pyClassName(tokenToName(res.Token))
	if res.IsProvider {
		name = "Provider"
	}

	// Export only the symbols we want exported.
	fmt.Fprintf(w, "__all__ = ['%s']\n\n", name)

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
		fmt.Fprintf(w, "warnings.warn(\"\"\"%s\"\"\", DeprecationWarning)\n\n", escaped)
	}

	// Produce a class definition with optional """ comment.
	fmt.Fprint(w, "\n")
	fmt.Fprintf(w, "class %s(%s):\n", name, baseType)
	if res.DeprecationMessage != "" && mod.compatibility != kubernetes20 {
		escaped := strings.ReplaceAll(res.DeprecationMessage, `"`, `\"`)
		fmt.Fprintf(w, "    warnings.warn(\"\"\"%s\"\"\", DeprecationWarning)\n\n", escaped)
	}
	// Now generate an initializer with arguments for all input properties.
	fmt.Fprintf(w, "    def __init__(__self__,\n")
	fmt.Fprintf(w, "                 resource_name: str,\n")
	fmt.Fprintf(w, "                 opts: Optional[pulumi.ResourceOptions] = None")

	// If there's an argument type, emit it.
	for _, prop := range res.InputProperties {
		ty := mod.typeString(prop.Type, true, true, true /*optional*/, true /*acceptMapping*/)
		fmt.Fprintf(w, ",\n                 %s: %s = None", InitParamName(prop.Name), ty)
	}

	// Old versions of TFGen emitted parameters named __name__ and __opts__. In order to preserve backwards
	// compatibility, we still emit them, but we don't emit documentation for them.
	fmt.Fprintf(w, ",\n                 __props__=None")
	fmt.Fprintf(w, ",\n                 __name__=None")
	fmt.Fprintf(w, ",\n                 __opts__=None):\n")
	mod.genInitDocstring(w, res)
	if res.DeprecationMessage != "" && mod.compatibility != kubernetes20 {
		fmt.Fprintf(w, "        pulumi.log.warn(\"%s is deprecated: %s\")\n", name, res.DeprecationMessage)
	}
	fmt.Fprintf(w, "        if __name__ is not None:\n")
	fmt.Fprintf(w, "            warnings.warn(\"explicit use of __name__ is deprecated\", DeprecationWarning)\n")
	fmt.Fprintf(w, "            resource_name = __name__\n")
	fmt.Fprintf(w, "        if __opts__ is not None:\n")
	fmt.Fprintf(w, "            warnings.warn(\"explicit use of __opts__ is deprecated, use 'opts' instead\", DeprecationWarning)\n")
	fmt.Fprintf(w, "            opts = __opts__\n")
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
	fmt.Fprintf(w, "            __props__ = dict()\n\n")
	fmt.Fprintf(w, "")

	ins := stringSet{}
	for _, prop := range res.InputProperties {
		pname := InitParamName(prop.Name)
		var arg interface{}
		var err error

		// Fill in computed defaults for arguments.
		if prop.DefaultValue != nil {
			dv, err := getDefaultValue(prop.DefaultValue, prop.Type)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(w, "            if %s is None:\n", pname)
			fmt.Fprintf(w, "                %s = %s\n", pname, dv)
		}

		// Check that required arguments are present.
		if prop.IsRequired {
			fmt.Fprintf(w, "            if %s is None and not opts.urn:\n", pname)
			fmt.Fprintf(w, "                raise TypeError(\"Missing required property '%s'\")\n", pname)
		}

		// Check that the property isn't deprecated
		if prop.DeprecationMessage != "" {
			escaped := strings.ReplaceAll(prop.DeprecationMessage, `"`, `\"`)
			fmt.Fprintf(w, "            if %s is not None and not opts.urn:\n", pname)
			fmt.Fprintf(w, "                warnings.warn(\"\"\"%s\"\"\", DeprecationWarning)\n", escaped)
			fmt.Fprintf(w, "                pulumi.log.warn(\"%s is deprecated: %s\")\n", pname, escaped)
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
		fmt.Fprintf(w, "            __props__['%s'] = %s\n", PyName(prop.Name), arg)

		ins.add(prop.Name)
	}

	var secretProps []string
	for _, prop := range res.Properties {
		// Default any pure output properties to None.  This ensures they are available as properties, even if
		// they don't ever get assigned a real value, and get documentation if available.
		if !ins.has(prop.Name) {
			fmt.Fprintf(w, "            __props__['%s'] = None\n", PyName(prop.Name))
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
		fmt.Fprintf(w, `        secret_opts = pulumi.ResourceOptions(additional_secret_outputs=[`)

		for i, sp := range secretProps {
			if i > 0 {
				fmt.Fprintf(w, ", ")
			}
			fmt.Fprintf(w, "%q", sp)
		}

		fmt.Fprintf(w, "])\n")
		fmt.Fprintf(w, "        opts = pulumi.ResourceOptions.merge(opts, secret_opts)\n")
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

		if res.StateInputs != nil {
			for _, prop := range res.StateInputs.Properties {
				pname := PyName(prop.Name)
				ty := mod.typeString(prop.Type, true, true, true /*optional*/, true /*acceptMapping*/)
				fmt.Fprintf(w, ",\n            %s: %s = None", pname, ty)
			}
		}
		fmt.Fprintf(w, ") -> '%s':\n", name)
		mod.genGetDocstring(w, res)
		fmt.Fprintf(w,
			"        opts = pulumi.ResourceOptions.merge(opts, pulumi.ResourceOptions(id=id))\n")
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "        __props__ = dict()\n\n")

		if res.StateInputs != nil {
			for _, prop := range res.StateInputs.Properties {
				fmt.Fprintf(w, "        __props__[\"%[1]s\"] = %[1]s\n", PyName(prop.Name))
			}
		}

		fmt.Fprintf(w, "        return %s(resource_name, opts=opts, __props__=__props__)\n\n", name)
	}

	// Write out Python property getters for each of the resource's properties.
	mod.genProperties(w, res.Properties, false /*setters*/, func(prop *schema.Property) string {
		ty := mod.typeString(prop.Type, false /*input*/, false /*wrapInput*/, !prop.IsRequired, false /*acceptMapping*/)
		return fmt.Sprintf("pulumi.Output[%s]", ty)
	})

	// Override translate_{input|output}_property on each resource to translate between snake case and
	// camel case when interacting with tfbridge.
	fmt.Fprintf(w,
		`    def translate_output_property(self, prop):
        return _tables.CAMEL_TO_SNAKE_CASE_TABLE.get(prop) or prop

    def translate_input_property(self, prop):
        return _tables.SNAKE_TO_CAMEL_CASE_TABLE.get(prop) or prop

`)

	return w.String(), nil
}

func (mod *modContext) genProperties(w io.Writer, properties []*schema.Property, setters bool,
	propType func(prop *schema.Property) string) {
	// Write out Python properties for each property. If there is a property named "property", it will
	// be emitted last to avoid conflicting with the built-in `@property` decorator function. We do
	// this instead of importing `builtins` and fully qualifying the decorator as `@builtins.property`
	// because that wouldn't address the problem if there was a property named "builtins".
	emitProp := func(pname string, prop *schema.Property) {
		ty := propType(prop)
		fmt.Fprintf(w, "    @property\n")
		if pname == prop.Name {
			fmt.Fprintf(w, "    @pulumi.getter\n")
		} else {
			fmt.Fprintf(w, "    @pulumi.getter(name=%q)\n", prop.Name)
		}
		fmt.Fprintf(w, "    def %s(self) -> %s:\n", pname, ty)
		if prop.Comment != "" {
			printComment(w, prop.Comment, "        ")
		}
		fmt.Fprintf(w, "        return pulumi.get(self, %q)\n\n", pname)

		if setters {
			fmt.Fprintf(w, "    @%s.setter\n", pname)
			fmt.Fprintf(w, "    def %s(self, value: %s):\n", pname, ty)
			fmt.Fprintf(w, "        pulumi.set(self, %q, value)\n\n", pname)
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

	imports, inputSeen, outputSeen := imports{}, codegen.Set{}, codegen.Set{}
	if fun.Inputs != nil {
		visitObjectTypesFromProperties(fun.Inputs.Properties, inputSeen, func(t interface{}) {
			switch T := t.(type) {
			case *schema.ObjectType:
				imports.addType(mod, T.Token, true /*input*/)
			case *schema.EnumType:
				imports.addEnum(mod, T.Token)
			case *schema.ResourceType:
				imports.addResource(mod, T.Token)
			}
		})
	}
	if fun.Outputs != nil {
		visitObjectTypesFromProperties(fun.Outputs.Properties, outputSeen, func(t interface{}) {
			switch T := t.(type) {
			case *schema.ObjectType:
				imports.addType(mod, T.Token, false /*input*/)
			case *schema.EnumType:
				imports.addEnum(mod, T.Token)
			case *schema.ResourceType:
				imports.addResource(mod, T.Token)
			}
		})
	}

	mod.genHeader(w, true /*needsSDK*/, imports)

	baseName, awaitableName := awaitableTypeNames(fun.Outputs.Token)
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
		ty := mod.typeString(arg.Type, true, false /*wrapInput*/, true /*optional*/, true /*acceptMapping*/)
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
			mod.genPropDocstring(docs, PyName(arg.Name), arg, false /*wrapInputs*/, true /*acceptMapping*/)
		}
	}
	printComment(w, docs.String(), "    ")

	if fun.DeprecationMessage != "" {
		fmt.Fprintf(w, "    pulumi.log.warn(\"%s is deprecated: %s\")\n", name, fun.DeprecationMessage)
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

func makeSafeEnumName(name string) (string, error) {
	// Replace common single character enum names.
	safeName := codegen.ExpandShortEnumName(name)

	// If the name is one illegal character, return an error.
	if len(safeName) == 1 && !isLegalIdentifierStart(rune(safeName[0])) {
		return "", errors.Errorf("enum name %s is not a valid identifier", safeName)
	}

	// If it's camelCase, change it to snake_case.
	safeName = pyName(safeName, false /*legacy*/)

	// Change to uppercase and make a valid identifier.
	safeName = makeValidIdentifier(strings.ToTitle(safeName))

	// If there are multiple underscores in a row, replace with one.
	regex := regexp.MustCompile(`_+`)
	safeName = regex.ReplaceAllString(safeName, "_")

	return safeName, nil
}

func (mod *modContext) genEnum(w io.Writer, enum *schema.EnumType) error {
	indent := "    "
	enumName := tokenToName(enum.Token)
	underlyingType := mod.typeString(enum.ElementType, false, false, false, false)

	switch enum.ElementType {
	case schema.StringType, schema.IntType, schema.NumberType:
		fmt.Fprintf(w, "class %s(%s, Enum):\n", enumName, underlyingType)
		printComment(w, enum.Comment, indent)
		for _, e := range enum.Elements {
			// If the enum doesn't have a name, set the value as the name.
			if e.Name == "" {
				e.Name = fmt.Sprintf("%v", e.Value)
			}

			name, err := makeSafeEnumName(e.Name)
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
		}
	default:
		return errors.Errorf("enums of type %s are not yet implemented for this language", enum.ElementType.String())
	}

	return nil
}

func visitObjectTypesFromProperties(properties []*schema.Property, seen codegen.Set, visitor func(objectOrResource interface{})) {
	for _, p := range properties {
		visitObjectTypes(p.Type, seen, visitor)
	}
}

func visitObjectTypes(t schema.Type, seen codegen.Set, visitor func(objectOrResource interface{})) {
	if seen.Has(t) {
		return
	}
	seen.Add(t)
	switch t := t.(type) {
	case *schema.EnumType:
		visitor(t)
	case *schema.ArrayType:
		visitObjectTypes(t.ElementType, seen, visitor)
	case *schema.MapType:
		visitObjectTypes(t.ElementType, seen, visitor)
	case *schema.ObjectType:
		for _, p := range t.Properties {
			visitObjectTypes(p.Type, seen, visitor)
		}
		visitor(t)
	case *schema.ResourceType:
		visitor(t)
	case *schema.UnionType:
		for _, e := range t.ElementTypes {
			visitObjectTypes(e, seen, visitor)
		}
	}
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
	tool string, pkg *schema.Package, emitPulumiPluginFile bool, requires map[string]string) (string, error) {

	w := &bytes.Buffer{}
	(&modContext{tool: tool}).genHeader(w, false /*needsSDK*/, nil)

	// Now create a standard Python package from the metadata.
	fmt.Fprintf(w, "import errno\n")
	fmt.Fprintf(w, "from setuptools import setup, find_packages\n")
	fmt.Fprintf(w, "from setuptools.command.install import install\n")
	fmt.Fprintf(w, "from subprocess import check_call\n")
	fmt.Fprintf(w, "\n\n")

	// Create a command that will install the Pulumi plugin for this resource provider.
	fmt.Fprintf(w, "class InstallPluginCommand(install):\n")
	fmt.Fprintf(w, "    def run(self):\n")
	fmt.Fprintf(w, "        install.run(self)\n")
	fmt.Fprintf(w, "        try:\n")
	if pkg.PluginDownloadURL == "" {
		fmt.Fprintf(w, "            check_call(['pulumi', 'plugin', 'install', 'resource', '%s', '${PLUGIN_VERSION}'])\n", pkg.Name)
	} else {
		fmt.Fprintf(w, "            check_call(['pulumi', 'plugin', 'install', 'resource', '%s', '${PLUGIN_VERSION}', '--server', '%s'])\n", pkg.Name, pkg.PluginDownloadURL)
	}
	fmt.Fprintf(w, "        except OSError as error:\n")
	fmt.Fprintf(w, "            if error.errno == errno.ENOENT:\n")
	fmt.Fprintf(w, "                print(\"\"\"\n")
	fmt.Fprintf(w, "                There was an error installing the %s resource provider plugin.\n", pkg.Name)
	fmt.Fprintf(w, "                It looks like `pulumi` is not installed on your system.\n")
	fmt.Fprintf(w, "                Please visit https://pulumi.com/ to install the Pulumi CLI.\n")
	fmt.Fprintf(w, "                You may try manually installing the plugin by running\n")
	fmt.Fprintf(w, "                `pulumi plugin install resource %s ${PLUGIN_VERSION}`\n", pkg.Name)
	fmt.Fprintf(w, "                \"\"\")\n")
	fmt.Fprintf(w, "            else:\n")
	fmt.Fprintf(w, "                raise\n")
	fmt.Fprintf(w, "\n\n")

	// Generate a readme method which will load README.rst, we use this to fill out the
	// long_description field in the setup call.
	fmt.Fprintf(w, "def readme():\n")
	fmt.Fprintf(w, "    with open('README.md', encoding='utf-8') as f:\n")
	fmt.Fprintf(w, "        return f.read()\n")
	fmt.Fprintf(w, "\n\n")

	// Finally, the actual setup part.
	fmt.Fprintf(w, "setup(name='%s',\n", pyPack(pkg.Name))
	fmt.Fprintf(w, "      version='${VERSION}',\n")
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
	fmt.Fprintf(w, "          '%s': [\n", pyPack(pkg.Name))
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

// Emits property conversion tables for all properties recorded using `recordProperty`. The two tables emitted here are
// used to convert to and from snake case and camel case.
func (mod *modContext) genPropertyConversionTables() string {
	w := &bytes.Buffer{}
	mod.genHeader(w, false /*needsSDK*/, nil)

	var allKeys []string
	for key := range mod.snakeCaseToCamelCase {
		allKeys = append(allKeys, key)
	}
	sort.Strings(allKeys)

	fmt.Fprintf(w, "SNAKE_TO_CAMEL_CASE_TABLE = {\n")
	for _, key := range allKeys {
		value := mod.snakeCaseToCamelCase[key]
		if key != value {
			fmt.Fprintf(w, "    %q: %q,\n", key, value)
		}
	}
	fmt.Fprintf(w, "}\n")
	fmt.Fprintf(w, "\nCAMEL_TO_SNAKE_CASE_TABLE = {\n")
	for _, value := range allKeys {
		key := mod.snakeCaseToCamelCase[value]
		if key != value {
			fmt.Fprintf(w, "    %q: %q,\n", key, value)
		}
	}
	fmt.Fprintf(w, "}\n")
	return w.String()
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
		mapCase = python.(PropertyInfo).MapCase
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
func (mod *modContext) genInitDocstring(w io.Writer, res *schema.Resource) {
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
	fmt.Fprintln(b, ":param pulumi.ResourceOptions opts: Options for the resource.")
	for _, prop := range res.InputProperties {
		mod.genPropDocstring(b, InitParamName(prop.Name), prop, true /*wrapInput*/, true /*acceptMapping*/)
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
			mod.genPropDocstring(b, PyName(prop.Name), prop, true /*wrapInput*/, true /*acceptMapping*/)
		}
	}

	// printComment handles the prefix and triple quotes.
	printComment(w, b.String(), "        ")
}

func (mod *modContext) genTypeDocstring(w io.Writer, comment string, properties []*schema.Property, wrapInput bool) {
	// b contains the full text of the docstring, without the leading and trailing triple quotes.
	b := &bytes.Buffer{}

	// If this type has documentation, write it at the top of the docstring.
	if comment != "" {
		fmt.Fprintln(b, comment)
	}

	for _, prop := range properties {
		mod.genPropDocstring(b, PyName(prop.Name), prop, wrapInput, false /*acceptMapping*/)
	}

	// printComment handles the prefix and triple quotes.
	printComment(w, b.String(), "        ")
}

func (mod *modContext) genPropDocstring(w io.Writer, name string, prop *schema.Property, wrapInput bool,
	acceptMapping bool) {

	if prop.Comment == "" {
		return
	}

	ty := mod.typeString(prop.Type, true, wrapInput, false /*optional*/, acceptMapping)

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

func (mod *modContext) typeString(t schema.Type, input, wrapInput, optional, acceptMapping bool) string {
	var typ string
	switch t := t.(type) {
	case *schema.EnumType:
		typ = mod.tokenToEnum(t.Token)
	case *schema.ArrayType:
		typ = fmt.Sprintf("Sequence[%s]", mod.typeString(t.ElementType, input, wrapInput, false, acceptMapping))
	case *schema.MapType:
		typ = fmt.Sprintf("Mapping[str, %s]", mod.typeString(t.ElementType, input, wrapInput, false, acceptMapping))
	case *schema.ObjectType:
		typ = mod.tokenToType(t.Token, input, mod.details(t).functionType)
		if acceptMapping {
			typ = fmt.Sprintf("pulumi.InputType[%s]", typ)
		}
	case *schema.ResourceType:
		typ = fmt.Sprintf("'%s'", mod.tokenToResource(t.Token))
	case *schema.TokenType:
		// Use the underlying type for now.
		if t.UnderlyingType != nil {
			return mod.typeString(t.UnderlyingType, input, wrapInput, optional, acceptMapping)
		}
		typ = "Any"
	case *schema.UnionType:
		if !input {
			for _, e := range t.ElementTypes {
				// If this is an output and a "relaxed" enum, emit the type as the underlying primitive type rather than the union.
				// Eg. Output[str] rather than Output[Any]
				if typ, ok := e.(*schema.EnumType); ok {
					return mod.typeString(typ.ElementType, input, wrapInput, optional, acceptMapping)
				}
			}
			if t.DefaultType != nil {
				return mod.typeString(t.DefaultType, input, wrapInput, optional, acceptMapping)
			}
			typ = "Any"
		} else {
			elementTypeSet := stringSet{}
			var elementTypes []schema.Type
			for _, e := range t.ElementTypes {
				et := mod.typeString(e, input, false, false, acceptMapping)
				if !elementTypeSet.has(et) {
					elementTypeSet.add(et)
					elementTypes = append(elementTypes, e)
				}
			}

			if len(elementTypes) == 1 {
				return mod.typeString(elementTypes[0], input, wrapInput, optional, acceptMapping)
			}

			var elements []string
			for _, e := range elementTypes {
				t := mod.typeString(e, input, wrapInput, false, acceptMapping)
				if wrapInput && strings.HasPrefix(t, "pulumi.Input[") {
					contract.Assert(t[len(t)-1] == ']')
					// Strip off the leading `pulumi.Input[` and the trailing `]`
					t = t[len("pulumi.Input[") : len(t)-1]
				}
				elements = append(elements, t)
			}
			typ = fmt.Sprintf("Union[%s]", strings.Join(elements, ", "))
		}
	default:
		switch t {
		case schema.BoolType:
			typ = "bool"
		case schema.IntType:
			typ = "int"
		case schema.NumberType:
			typ = "float"
		case schema.StringType:
			typ = "str"
		case schema.ArchiveType:
			typ = "pulumi.Archive"
		case schema.AssetType:
			typ = "Union[pulumi.Asset, pulumi.Archive]"
		case schema.JSONType:
			fallthrough
		case schema.AnyType:
			typ = "Any"
		}
	}

	if wrapInput && typ != "Any" {
		typ = fmt.Sprintf("pulumi.Input[%s]", typ)
	}
	if optional {
		return fmt.Sprintf("Optional[%s]", typ)
	}
	return typ
}

// pyType returns the expected runtime type for the given variable.  Of course, being a dynamic language, this
// check is not exhaustive, but it should be good enough to catch 80% of the cases early on.
func (mod *modContext) pyType(typ schema.Type) string {
	switch typ := typ.(type) {
	case *schema.EnumType:
		return mod.pyType(typ.ElementType)
	case *schema.ArrayType:
		return "list"
	case *schema.MapType, *schema.ObjectType, *schema.UnionType:
		return "dict"
	case *schema.ResourceType:
		return mod.tokenToResource(typ.Token)
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

func (mod *modContext) genType(w io.Writer, obj *schema.ObjectType, input, wrapInput bool) error {
	// Sort required props first.
	props := make([]*schema.Property, len(obj.Properties))
	copy(props, obj.Properties)
	sort.Slice(props, func(i, j int) bool {
		pi, pj := props[i], props[j]
		switch {
		case pi.IsRequired != pj.IsRequired:
			return pi.IsRequired && !pj.IsRequired
		default:
			return pi.Name < pj.Name
		}
	})

	decorator := "@pulumi.output_type"
	if input {
		decorator = "@pulumi.input_type"
	}

	name := tokenToName(obj.Token)
	switch {
	case input:
		name += "Args"
	case mod.details(obj).functionType:
		name += "Result"
	}

	var suffix string
	if !input {
		suffix = "(dict)"
	}

	fmt.Fprintf(w, "%s\n", decorator)
	fmt.Fprintf(w, "class %s%s:\n", name, suffix)
	if !input && obj.Comment != "" {
		printComment(w, obj.Comment, "    ")
	}

	// Generate an __init__ method.
	fmt.Fprintf(w, "    def __init__(__self__")
	// Bare `*` argument to force callers to use named arguments.
	if len(props) > 0 {
		fmt.Fprintf(w, ", *")
	}
	for _, prop := range props {
		pname := PyName(prop.Name)
		ty := mod.typeString(prop.Type, input, wrapInput, !prop.IsRequired, false /*acceptMapping*/)
		var defaultValue string
		if !prop.IsRequired {
			defaultValue = " = None"
		}
		fmt.Fprintf(w, ",\n                 %s: %s%s", pname, ty, defaultValue)
	}
	fmt.Fprintf(w, "):\n")
	mod.genTypeDocstring(w, obj.Comment, props, wrapInput)
	if len(props) == 0 {
		fmt.Fprintf(w, "        pass\n")
	}
	for _, prop := range props {
		pname := PyName(prop.Name)
		var arg interface{}
		var err error

		// Fill in computed defaults for arguments.
		if prop.DefaultValue != nil {
			dv, err := getDefaultValue(prop.DefaultValue, prop.Type)
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
			fmt.Fprintf(w, "            pulumi.log.warn(\"%s is deprecated: %s\")\n", pname, escaped)
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
		if !prop.IsRequired {
			fmt.Fprintf(w, "        if %s is not None:\n", pname)
			indent = "    "
		}

		fmt.Fprintf(w, "%s        pulumi.set(__self__, \"%s\", %s)\n", indent, pname, arg)
	}
	fmt.Fprintf(w, "\n")

	// Generate properties. Input types have getters and setters, output types only have getters.
	mod.genProperties(w, props, input /*setters*/, func(prop *schema.Property) string {
		return mod.typeString(prop.Type, input, wrapInput, !prop.IsRequired, false /*acceptMapping*/)
	})

	if !input && !mod.details(obj).functionType {
		// The generated output class is a subclass of dict and contains translated keys
		// to maintain backwards compatibility. When this function is present, property
		// getters will use it to translate the key from the Pulumi name before looking
		// up the value in the dictionary.
		fmt.Fprintf(w, "    def _translate_property(self, prop):\n")
		fmt.Fprintf(w, "        return _tables.CAMEL_TO_SNAKE_CASE_TABLE.get(prop) or prop\n\n")
	}

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

	// group resources, types, and functions into modules
	modules := map[string]*modContext{}

	var getMod func(modName string) *modContext
	getMod = func(modName string) *modContext {
		mod, ok := modules[modName]
		if !ok {
			mod = &modContext{
				pkg:                  pkg,
				mod:                  modName,
				tool:                 tool,
				snakeCaseToCamelCase: snakeCaseToCamelCase,
				camelCaseToSnakeCase: camelCaseToSnakeCase,
				modNameOverrides:     info.ModuleNameOverrides,
				compatibility:        info.Compatibility,
			}

			if modName != "" {
				parentName := path.Dir(modName)
				if parentName == "." {
					parentName = ""
				}
				parent := getMod(parentName)
				parent.children = append(parent.children, mod)
			}

			modules[modName] = mod
		}
		return mod
	}

	getModFromToken := func(tok string) *modContext {
		modName := tokenToModule(tok, pkg, info.ModuleNameOverrides)
		return getMod(modName)
	}

	// Create the config module if necessary.
	if len(pkg.Config) > 0 &&
		info.Compatibility != kubernetes20 { // k8s SDK doesn't use config.
		configMod := getMod("config")
		configMod.isConfig = true
	}

	inputSeen, outputSeen := codegen.Set{}, codegen.Set{}
	visitObjectTypesFromProperties(pkg.Config, outputSeen, func(t interface{}) {
		switch T := t.(type) {
		case *schema.ObjectType:
			getModFromToken(T.Token).details(T).outputType = true
		}
	})

	// Find input and output types referenced by resources.
	scanResource := func(r *schema.Resource) {
		mod := getModFromToken(r.Token)
		mod.resources = append(mod.resources, r)
		visitObjectTypesFromProperties(r.Properties, outputSeen, func(t interface{}) {
			switch T := t.(type) {
			case *schema.ObjectType:
				getModFromToken(T.Token).details(T).outputType = true
			}
		})
		visitObjectTypesFromProperties(r.InputProperties, inputSeen, func(t interface{}) {
			switch T := t.(type) {
			case *schema.ObjectType:
				if r.IsProvider {
					getModFromToken(T.Token).details(T).outputType = true
				}
				getModFromToken(T.Token).details(T).inputType = true
			}
		})
		if r.StateInputs != nil {
			visitObjectTypes(r.StateInputs, inputSeen, func(t interface{}) {
				switch T := t.(type) {
				case *schema.ObjectType:
					getModFromToken(T.Token).details(T).inputType = true
				case *schema.ResourceType:
					getModFromToken(T.Token)
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
		mod := getModFromToken(f.Token)
		mod.functions = append(mod.functions, f)
		if f.Inputs != nil {
			visitObjectTypes(f.Inputs, inputSeen, func(t interface{}) {
				switch T := t.(type) {
				case *schema.ObjectType:
					getModFromToken(T.Token).details(T).inputType = true
					getModFromToken(T.Token).details(T).functionType = true
				case *schema.ResourceType:
					getModFromToken(T.Token)
				}
			})
		}
		if f.Outputs != nil {
			visitObjectTypes(f.Outputs, outputSeen, func(t interface{}) {
				switch T := t.(type) {
				case *schema.ObjectType:
					getModFromToken(T.Token).details(T).outputType = true
					getModFromToken(T.Token).details(T).functionType = true
				case *schema.ResourceType:
					getModFromToken(T.Token)
				}
			})
		}
	}

	// Find nested types.
	for _, t := range pkg.Types {
		switch typ := t.(type) {
		case *schema.ObjectType:
			mod := getModFromToken(typ.Token)
			d := mod.details(typ)
			if d.inputType || d.outputType {
				mod.types = append(mod.types, typ)
			}
		case *schema.EnumType:
			mod := getModFromToken(typ.Token)
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
		mod := getMod(modName)
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

	files := fs{}
	for p, f := range extraFiles {
		files.add(filepath.Join(pyPack(pkg.Name), p), f)
	}

	for _, mod := range modules {
		if err := mod.gen(files); err != nil {
			return nil, err
		}
	}

	// Emit casing tables.
	files.add(filepath.Join(pyPack(pkg.Name), "_tables.py"), []byte(modules[""].genPropertyConversionTables()))

	// Generate pulumiplugin.json, if requested.
	if info.EmitPulumiPluginFile {
		plugin, err := genPulumiPluginFile(pkg)
		if err != nil {
			return nil, err
		}
		files.add("pulumiplugin.json", plugin)
	}

	// Finally emit the package metadata (setup.py).
	setup, err := genPackageMetadata(tool, pkg, info.EmitPulumiPluginFile, info.Requires)
	if err != nil {
		return nil, err
	}
	files.add("setup.py", []byte(setup))

	return files, nil
}

const utilitiesFile = `
import os
import pkg_resources

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


def get_semver_version():
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

def get_version():
	return str(get_semver_version())
`
