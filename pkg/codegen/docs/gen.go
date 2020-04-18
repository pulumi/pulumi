//go:generate go run bundler.go

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
package docs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"path"
	"sort"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/v2/codegen"
	"github.com/pulumi/pulumi/pkg/v2/codegen/dotnet"
	go_gen "github.com/pulumi/pulumi/pkg/v2/codegen/go"
	"github.com/pulumi/pulumi/pkg/v2/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v2/codegen/python"
	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

var (
	supportedLanguages = []string{"csharp", "go", "nodejs", "python"}
	templates          *template.Template
	packagedTemplates  map[string][]byte
	docHelpers         map[string]codegen.DocLanguageHelper

	// The following property case maps are for rendering property
	// names of nested properties in Python language with the correct
	// casing.
	snakeCaseToCamelCase map[string]string
	camelCaseToSnakeCase map[string]string

	// The language-specific info objects for a certain package (provider).
	goPkgInfo     go_gen.GoInfo
	csharpPkgInfo dotnet.CSharpPackageInfo
	nodePkgInfo   nodejs.NodePackageInfo

	// langModuleNameLookup is a map of module name to its language-specific
	// name.
	langModuleNameLookup map[string]string
	// titleLookup is a map to map module package name to the desired display name
	// for display in the TOC menu under API Reference.
	titleLookup = map[string]string{
		"aiven":        "Aiven",
		"alicloud":     "AliCloud",
		"aws":          "AWS",
		"azure":        "Azure",
		"azuread":      "Azure AD",
		"cloudamqp":    "CloudAMQP",
		"cloudflare":   "Cloudflare",
		"consul":       "Consul",
		"datadog":      "Datadog",
		"digitalocean": "Digital Ocean",
		"dnsimple":     "DNSimple",
		"f5bigip":      "f5 BIG-IP",
		"fastly":       "Fastly",
		"gcp":          "GCP",
		"gitlab":       "GitLab",
		"kafka":        "Kafka",
		"keycloak":     "Keycloak",
		"kubernetes":   "Kubernetes",
		"linode":       "Linode",
		"mailgun":      "Mailgun",
		"mysql":        "MySQL",
		"newrelic":     "New Relic",
		"okta":         "Okta",
		"openstack":    "Open Stack",
		"packet":       "Packet",
		"postgresql":   "PostgreSQL",
		"rabbitmq":     "RabbitMQ",
		"rancher2":     "Rancher 2",
		"signalfx":     "SignalFx",
		"spotinst":     "Spotinst",
		"tls":          "TLS",
		"vault":        "Vault",
		"vsphere":      "vSphere",
	}
)

func init() {
	docHelpers = make(map[string]codegen.DocLanguageHelper)
	for _, lang := range supportedLanguages {
		switch lang {
		case "csharp":
			docHelpers[lang] = &dotnet.DocLanguageHelper{}
		case "go":
			docHelpers[lang] = &go_gen.DocLanguageHelper{}
		case "nodejs":
			docHelpers[lang] = &nodejs.DocLanguageHelper{}
		case "python":
			docHelpers[lang] = &python.DocLanguageHelper{}
		}
	}

	snakeCaseToCamelCase = map[string]string{}
	camelCaseToSnakeCase = map[string]string{}
	langModuleNameLookup = map[string]string{}
}

// header represents the header of each resource markdown file.
type header struct {
	Title string
}

// property represents an input or an output property.
type property struct {
	// DisplayName is the property name with word-breaks.
	DisplayName        string
	Name               string
	Comment            string
	Type               propertyType
	DeprecationMessage string

	IsRequired bool
	IsInput    bool
}

// apiTypeDocLinks represents the links for a type's input and output API doc.
type apiTypeDocLinks struct {
	InputType  string
	OutputType string
}

// docNestedType represents a complex type.
type docNestedType struct {
	Name        string
	AnchorID    string
	APIDocLinks map[string]apiTypeDocLinks
	Properties  map[string][]property
}

// propertyType represents the type of a property.
type propertyType struct {
	DisplayName string
	Name        string
	// Link can be a link to an anchor tag on the same
	// page, or to another page/site.
	Link string
}

// formalParam represents the formal parameters of a constructor
// or a lookup function.
type formalParam struct {
	Name string
	Type propertyType

	// This is the language specific optional type indicator.
	// For example, in nodejs this is the character "?" and in Go
	// it's "*".
	OptionalFlag string

	DefaultValue string

	// Comment is an optional description of the parameter.
	Comment string
}

type packageDetails struct {
	Repository string
	License    string
	Notes      string
	Version    string
}

type resourceDocArgs struct {
	Header header

	Tool string
	// LangChooserLanguages is a comma-separated list of languages to pass to the
	// language chooser shortcode.
	LangChooserLanguages string

	// Comment represents the introductory resource comment.
	Comment            string
	DeprecationMessage string

	// ConstructorParams is a map from language to the rendered HTML for the constructor's
	// arguments.
	ConstructorParams map[string]string
	// ConstructorParamsTyped is the typed set of parameters for the constructor, in order.
	ConstructorParamsTyped map[string][]formalParam
	// ConstructorResource is the resource that is being constructed or
	// is the result of a constructor-like function.
	ConstructorResource map[string]propertyType
	// ArgsRequired is a flag indicating if the args param is required
	// when creating a new resource.
	ArgsRequired bool

	// InputProperties is a map per language and a corresponding slice of
	// input properties accepted as args while creating a new resource.
	InputProperties map[string][]property
	// OutputProperties is a map per language and a corresponding slice of
	// output properties returned when a new instance of the resource is
	// created.
	OutputProperties map[string][]property

	// LookupParams is a map of the param string to be rendered per language
	// for looking-up a resource.
	LookupParams map[string]string
	// StateInputs is a map per language and the corresponding slice of
	// state input properties required while looking-up an existing resource.
	StateInputs map[string][]property
	// StateParam is the type name of the state param, if any.
	StateParam string

	// NestedTypes is a slice of the nested types used in the input and
	// output properties.
	NestedTypes []docNestedType

	PackageDetails packageDetails
}

// typeUsage represents a nested type's usage.
type typeUsage struct {
	Input  bool
	Output bool
}

// nestedTypeUsageInfo is a type-alias for a map of Pulumi type-tokens
// and whether or not the type is used as an input and/or output
// properties.
type nestedTypeUsageInfo map[string]typeUsage

func (ss nestedTypeUsageInfo) add(s string, input bool) {
	if v, ok := ss[s]; ok {
		if input {
			v.Input = true
		} else {
			v.Output = true
		}
		ss[s] = v
		return
	}

	ss[s] = typeUsage{
		Input:  input,
		Output: !input,
	}
}

// contains returns true if the token already exists and matches the
// input or output flag of the token.
func (ss nestedTypeUsageInfo) contains(token string, input bool) bool {
	a, ok := ss[token]
	if !ok {
		return false
	}

	if input && a.Input {
		return true
	} else if !input && a.Output {
		return true
	}
	return false
}

type modContext struct {
	pkg       *schema.Package
	mod       string
	resources []*schema.Resource
	functions []*schema.Function
	children  []*modContext
	tool      string
}

func resourceName(r *schema.Resource) string {
	if r.IsProvider {
		return "Provider"
	}
	return tokenToName(r.Token)
}

func getLanguageDocHelper(lang string) codegen.DocLanguageHelper {
	if h, ok := docHelpers[lang]; ok {
		return h
	}
	panic(errors.Errorf("could not find a doc lang helper for %s", lang))
}

type propertyCharacteristics struct {
	// input is a flag indicating if the property is an input type.
	input bool
	// optional is a flag indicating if the property is optional.
	optional bool
}

// getLanguageModuleName returns the module name mapped to its language-specific
// equivalent if the schema for this provider has any overrides for that language.
// Otherwise, returns the module name as-is.
func getLanguageModuleName(pkg *schema.Package, mod, lang string) string {
	modName := mod
	lookupKey := lang + "_" + modName
	if v, ok := langModuleNameLookup[lookupKey]; ok {
		return v
	}

	switch lang {
	case "go":
		if override, ok := goPkgInfo.ModuleToPackage[modName]; ok {
			modName = override
		}
	case "csharp":
		// For k8s, the C# ModuleToPackage map uses the normalized Go package names as the key.
		// Despite that being the case, we don't apply that correction here since the schema-based
		// codegen for dotnet languages is not aware of this "issue" and will cause other issues in
		// the docs generator.
		if override, ok := csharpPkgInfo.Namespaces[modName]; ok {
			modName = override
		}
	case "nodejs":
		if override, ok := nodePkgInfo.ModuleToPackage[modName]; ok {
			modName = override
		}
	}

	langModuleNameLookup[lookupKey] = modName
	return modName
}

// cleanTypeString removes any namespaces from the generated type string for all languages.
// The result of this function should be used display purposes only.
func (mod *modContext) cleanTypeString(t schema.Type, langTypeString, lang, modName string, isInput bool) string {
	switch lang {
	case "go":
		parts := strings.Split(langTypeString, ".")
		return parts[len(parts)-1]
	case "python":
		return langTypeString
	}

	cleanCSharpName := func(pkgName, objModName string) string {
		// C# types can be wrapped in enumerable types such as List<> or Dictionary<>, so we have to
		// only replace the namespace between the < and the > characters.
		qualifier := "Inputs"
		if !isInput {
			qualifier = "Outputs"
		}

		var csharpNS string
		// This type could be at the package-level, so it won't have a module name.
		if objModName != "" {
			csharpNS = fmt.Sprintf("Pulumi.%s.%s.%s.", title(pkgName, lang), title(objModName, lang), qualifier)
		} else {
			csharpNS = fmt.Sprintf("Pulumi.%s.%s.", title(pkgName, lang), qualifier)
		}
		return strings.ReplaceAll(langTypeString, csharpNS, "")
	}

	cleanNodeJSName := func(objModName string) string {
		// The nodejs codegen currently doesn't use the ModuleToPackage override available
		// in the k8s package's schema. So we'll manually strip some known module names for k8s.
		// TODO[pulumi/pulumi#4325]: Remove this block once the nodejs code gen is able to use the
		// package name overrides for modules.
		if isKubernetesPackage(mod.pkg) {
			langTypeString = strings.ReplaceAll(langTypeString, "k8s.io.", "")
			langTypeString = strings.ReplaceAll(langTypeString, "apiserver.", "")
			langTypeString = strings.ReplaceAll(langTypeString, "rbac.authorization.v1.", "")
			langTypeString = strings.ReplaceAll(langTypeString, "rbac.authorization.v1alpha1.", "")
			langTypeString = strings.ReplaceAll(langTypeString, "rbac.authorization.v1beta1.", "")
		}
		objModName = strings.ReplaceAll(objModName, "/", ".") + "."
		return strings.ReplaceAll(langTypeString, objModName, "")
	}

	switch t := t.(type) {
	case *schema.ArrayType:
		if schema.IsPrimitiveType(t.ElementType) {
			break
		}
		return mod.cleanTypeString(t.ElementType, langTypeString, lang, modName, isInput)
	case *schema.UnionType:
		for _, e := range t.ElementTypes {
			if schema.IsPrimitiveType(e) {
				continue
			}
			return mod.cleanTypeString(e, langTypeString, lang, modName, isInput)
		}
	case *schema.ObjectType:
		objTypeModName := mod.pkg.TokenToModule(t.Token)
		if objTypeModName != mod.mod {
			modName = getLanguageModuleName(mod.pkg, objTypeModName, lang)
		}
	}

	if lang == "nodejs" {
		return cleanNodeJSName(modName)
	} else if lang == "csharp" {
		return cleanCSharpName(mod.pkg.Name, modName)
	}
	return strings.ReplaceAll(langTypeString, modName, "")
}

// typeString returns a property type suitable for docs with its display name and the anchor link to
// a type if the type of the property is an array or an object.
func (mod *modContext) typeString(t schema.Type, lang string, characteristics propertyCharacteristics, insertWordBreaks bool) propertyType {
	docLanguageHelper := getLanguageDocHelper(lang)
	modName := getLanguageModuleName(mod.pkg, mod.mod, lang)
	langTypeString := docLanguageHelper.GetLanguageTypeString(mod.pkg, modName, t, characteristics.input, characteristics.optional)

	// If the type is an object type, let's also wrap it with a link to the supporting type
	// on the same page using an anchor tag.
	var href string
	switch t := t.(type) {
	case *schema.ArrayType:
		elementLangType := mod.typeString(t.ElementType, lang, characteristics, false)
		href = elementLangType.Link
	case *schema.ObjectType:
		tokenName := tokenToName(t.Token)
		// Links to anchor tags on the same page must be lower-cased.
		href = "#" + strings.ToLower(tokenName)
	default:
		// Check if type is primitive/built-in type if no match for cases listed above.
		if schema.IsPrimitiveType(t) {
			href = docLanguageHelper.GetDocLinkForBuiltInType(t.String())
		}
	}

	// Strip the namespace/module prefix for the type's display name.
	displayName := langTypeString
	if !schema.IsPrimitiveType(t) {
		displayName = mod.cleanTypeString(t, langTypeString, lang, modName, characteristics.input)
	}

	// If word-breaks need to be inserted, then the type string
	// should be html-encoded first if the language is C# in order
	// to avoid confusing the Hugo rendering where the word-break
	// tags are inserted.
	if insertWordBreaks {
		if lang == "csharp" {
			displayName = html.EscapeString(displayName)
		}
		displayName = wbr(displayName)
	}

	displayName = cleanOptionalIdentifier(displayName, lang)
	langTypeString = cleanOptionalIdentifier(langTypeString, lang)

	return propertyType{
		Name:        langTypeString,
		DisplayName: displayName,
		Link:        href,
	}
}

// cleanOptionalIdentifier removes the type identifier (i.e. "?" in "string?").
func cleanOptionalIdentifier(s, lang string) string {
	switch lang {
	case "nodejs":
		return strings.TrimSuffix(s, "?")
	case "go":
		return strings.TrimPrefix(s, "*")
	case "csharp":
		return strings.TrimSuffix(s, "?")
	case "python":
		return s
	}
	return s
}

// Resources typically take the same set of parameters to their constructors, and these
// are the default comments/descriptions for them.
const (
	ctorNameArgComment = "The unique name of the resource."
	ctorArgsArgComment = "The arguments to resource properties."
	ctorOptsArgComment = "Bag of options to control resource's behavior."
)

func (mod *modContext) genConstructorTS(r *schema.Resource, argsOptional bool) []formalParam {
	name := resourceName(r)
	docLangHelper := getLanguageDocHelper("nodejs")
	// Use the NodeJS module to package lookup to transform the module name to its normalized package name.
	modName := getLanguageModuleName(mod.pkg, mod.mod, "nodejs")

	var argsType string
	var argsDocLink string
	// The non-schema-based k8s codegen does not apply a suffix to the input types.
	if isKubernetesPackage(mod.pkg) {
		argsType = name
		argsDocLink = docLangHelper.GetDocLinkForResourceInputOrOutputType(mod.pkg, modName, argsType, true)
	} else {
		argsType = name + "Args"
		argsDocLink = docLangHelper.GetDocLinkForResourceType(mod.pkg, modName, argsType)
	}

	argsFlag := ""
	if argsOptional {
		argsFlag = "?"
	}

	return []formalParam{
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
				Link: docLangHelper.GetDocLinkForBuiltInType("string"),
			},
			Comment: ctorNameArgComment,
		},
		{
			Name:         "args",
			OptionalFlag: argsFlag,
			Type: propertyType{
				Name: argsType,
				Link: argsDocLink,
			},
			Comment: ctorArgsArgComment,
		},
		{
			Name:         "opts",
			OptionalFlag: "?",
			Type: propertyType{
				Name: "CustomResourceOptions",
				Link: docLangHelper.GetDocLinkForPulumiType(mod.pkg, "CustomResourceOptions"),
			},
			Comment: ctorOptsArgComment,
		},
	}
}

func (mod *modContext) genConstructorGo(r *schema.Resource, argsOptional bool) []formalParam {
	name := resourceName(r)
	argsType := name + "Args"
	argsFlag := ""
	if argsOptional {
		argsFlag = "*"
	}

	docLangHelper := getLanguageDocHelper("go")
	// Use the Go module to package lookup to transform the module name to its normalized package name.
	modName := getLanguageModuleName(mod.pkg, mod.mod, "go")
	return []formalParam{
		{
			Name:         "ctx",
			OptionalFlag: "*",
			Type: propertyType{
				Name: "Context",
				Link: docLangHelper.GetDocLinkForPulumiType(mod.pkg, "Context"),
			},
			Comment: "Context object for the current deployment",
		},
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
				Link: docLangHelper.GetDocLinkForBuiltInType("string"),
			},
			Comment: ctorNameArgComment,
		},
		{
			Name:         "args",
			OptionalFlag: argsFlag,
			Type: propertyType{
				Name: argsType,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg, modName, argsType),
			},
			Comment: ctorArgsArgComment,
		},
		{
			Name:         "opts",
			OptionalFlag: "...",
			Type: propertyType{
				Name: "ResourceOption",
				Link: docLangHelper.GetDocLinkForPulumiType(mod.pkg, "ResourceOption"),
			},
			Comment: ctorOptsArgComment,
		},
	}
}

func (mod *modContext) genConstructorCS(r *schema.Resource, argsOptional bool) []formalParam {
	name := resourceName(r)
	argsSchemaType := &schema.ObjectType{
		Token: r.Token,
	}
	// Get the C#-specific name for the args type, which will be the fully-qualified name.
	characteristics := propertyCharacteristics{
		input:    true,
		optional: argsOptional,
	}

	var argLangTypeName string
	// Top-level argument types in the k8s package for C# use different namespace path.
	// Additionally, overlay resources in k8s have argument types in the top-level module path,
	// but don't use the suffix "Args" like the other modules.
	if isKubernetesPackage(mod.pkg) {
		if mod.mod != "" && !mod.isKubernetesOverlayModule() {
			// Find the normalize package name for the current module from the "Go" moduleToPackage language info map.
			normalizedModName := getLanguageModuleName(mod.pkg, mod.mod, "go")
			correctModName := getLanguageModuleName(mod.pkg, normalizedModName, "csharp")
			// For k8s, the args type for a resource is part of the `Types.Inputs` namespace.
			argLangTypeName = "Pulumi.Kubernetes.Types.Inputs." + correctModName + "." + name + "Args"
		} else if mod.isKubernetesOverlayModule() {
			argLangTypeName = "Pulumi.Kubernetes." + name
		} else {
			argLangTypeName = "Pulumi.Kubernetes." + name + "Args"
		}
	} else {
		argLangType := mod.typeString(argsSchemaType, "csharp", characteristics, false)
		argLangTypeName = strings.ReplaceAll(argLangType.Name, "Inputs.", "")
	}

	var argsFlag string
	var argsDefault string
	if argsOptional {
		// If the number of required input properties was zero, we can make the args object optional.
		argsDefault = " = null"
		argsFlag = "?"
	}

	docLangHelper := getLanguageDocHelper("csharp")
	return []formalParam{
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
				Link: docLangHelper.GetDocLinkForBuiltInType("string"),
			},
			Comment: ctorNameArgComment,
		},
		{
			Name:         "args",
			OptionalFlag: argsFlag,
			DefaultValue: argsDefault,
			Type: propertyType{
				Name: name + "Args",
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg, "", argLangTypeName),
			},
			Comment: ctorArgsArgComment,
		},
		{
			Name:         "opts",
			OptionalFlag: "?",
			DefaultValue: " = null",
			Type: propertyType{
				Name: "CustomResourceOptions",
				Link: docLangHelper.GetDocLinkForPulumiType(mod.pkg, "Pulumi.CustomResourceOptions"),
			},
			Comment: ctorOptsArgComment,
		},
	}
}

func (mod *modContext) genNestedTypes(member interface{}, resourceType bool) []docNestedType {
	tokens := nestedTypeUsageInfo{}
	// Collect all of the types for this "member" as a map of resource names
	// and if it appears in an input object and/or output object.
	mod.getTypes(member, tokens)

	var objs []docNestedType
	for token, tyUsage := range tokens {
		for _, t := range mod.pkg.Types {
			obj, ok := t.(*schema.ObjectType)
			if !ok || obj.Token != token {
				continue
			}
			if len(obj.Properties) == 0 {
				continue
			}

			// Create maps to hold the per-language properties of this object and links to
			// the API doc for each language.
			props := make(map[string][]property)
			apiDocLinks := make(map[string]apiTypeDocLinks)
			for _, lang := range supportedLanguages {
				// The nested type may be under a different package in a language.
				// For example, in k8s, common types are in the core/v1 module and can appear in
				// nested types elsewhere. So we use the appropriate name of that type,
				// as well as its language-specific name. For example, module name for use as a C# namespace
				// or as a Go package name.
				modName := getLanguageModuleName(mod.pkg, mod.mod, lang)
				nestedTypeModName := mod.pkg.TokenToModule(token)
				if nestedTypeModName != mod.mod {
					modName = getLanguageModuleName(mod.pkg, nestedTypeModName, lang)
				}

				docLangHelper := getLanguageDocHelper(lang)
				inputCharacteristics := propertyCharacteristics{
					input:    true,
					optional: true,
				}
				outputCharacteristics := propertyCharacteristics{
					input:    false,
					optional: true,
				}
				inputObjLangType := mod.typeString(t, lang, inputCharacteristics, false /*insertWordBreaks*/)
				outputObjLangType := mod.typeString(t, lang, outputCharacteristics, false /*insertWordBreaks*/)

				// Get the doc link for this nested type based on whether the type is for a Function or a Resource.
				var inputTypeDocLink string
				var outputTypeDocLink string
				if resourceType {
					if tyUsage.Input {
						inputTypeDocLink = docLangHelper.GetDocLinkForResourceInputOrOutputType(mod.pkg, modName, inputObjLangType.Name, true)
					}
					if tyUsage.Output {
						outputTypeDocLink = docLangHelper.GetDocLinkForResourceInputOrOutputType(mod.pkg, modName, outputObjLangType.Name, false)
					}
				} else {
					if tyUsage.Input {
						inputTypeDocLink = docLangHelper.GetDocLinkForFunctionInputOrOutputType(mod.pkg, modName, inputObjLangType.Name, true)
					}
					if tyUsage.Output {
						outputTypeDocLink = docLangHelper.GetDocLinkForFunctionInputOrOutputType(mod.pkg, modName, outputObjLangType.Name, false)
					}
				}
				apiDocLinks[lang] = apiTypeDocLinks{
					InputType:  inputTypeDocLink,
					OutputType: outputTypeDocLink,
				}
				props[lang] = mod.getProperties(obj.Properties, lang, true, true)
			}

			name := tokenToName(obj.Token)
			objs = append(objs, docNestedType{
				Name:        wbr(name),
				AnchorID:    strings.ToLower(name),
				APIDocLinks: apiDocLinks,
				Properties:  props,
			})
		}
	}

	sort.Slice(objs, func(i, j int) bool {
		return objs[i].Name < objs[j].Name
	})

	return objs
}

// getProperties returns a slice of properties that can be rendered for docs for
// the provided slice of properties in the schema.
func (mod *modContext) getProperties(properties []*schema.Property, lang string, input, nested bool) []property {
	if len(properties) == 0 {
		return nil
	}
	isK8s := isKubernetesPackage(mod.pkg)
	docProperties := make([]property, 0, len(properties))
	for _, prop := range properties {
		if prop == nil {
			continue
		}

		// If the property has a const value, then don't show it as an input property.
		// Even though it is a valid property, it is used by the language code gen to
		// generate the appropriate defaults for it. These cannot be overridden by users.
		if prop.ConstValue != nil {
			continue
		}

		characteristics := propertyCharacteristics{
			input:    input,
			optional: !prop.IsRequired,
		}

		langDocHelper := getLanguageDocHelper(lang)
		var propLangName string
		switch lang {
		case "python":
			pyName := python.PyName(prop.Name)
			// The default casing for a Python property name is snake_case unless
			// it is a property of a nested object, in which case, we should check the property
			// case maps.
			propLangName = pyName

			// We don't use the property-case maps for k8s since input properties are accessible
			// using either case, where output properties are only accessible using snake_case.
			if nested && !isK8s {
				if snakeCase, ok := camelCaseToSnakeCase[prop.Name]; ok {
					propLangName = snakeCase
				} else if camelCase, ok := snakeCaseToCamelCase[pyName]; ok {
					propLangName = camelCase
				} else {
					// If neither of the property case maps have the property
					// then use the default name of the property.
					propLangName = prop.Name
				}
			}
		default:
			name, err := langDocHelper.GetPropertyName(prop)
			if err != nil {
				panic(err)
			}

			propLangName = name
		}

		docProperties = append(docProperties, property{
			DisplayName:        wbr(propLangName),
			Name:               propLangName,
			Comment:            prop.Comment,
			DeprecationMessage: prop.DeprecationMessage,
			IsRequired:         prop.IsRequired,
			IsInput:            input,
			Type:               mod.typeString(prop.Type, lang, characteristics, true),
		})
	}

	// Sort required props to move them to the top of the properties list.
	sort.SliceStable(docProperties, func(i, j int) bool {
		return docProperties[i].IsRequired && !docProperties[j].IsRequired
	})

	return docProperties
}

// Returns the rendered HTML for the resource's constructor, as well as the specific arguments.
func (mod *modContext) genConstructors(r *schema.Resource, allOptionalInputs bool) (map[string]string, map[string][]formalParam) {
	renderedParams := make(map[string]string)
	formalParams := make(map[string][]formalParam)
	for _, lang := range supportedLanguages {
		var (
			paramTemplate string
			params        []formalParam
		)
		b := &bytes.Buffer{}

		switch lang {
		case "nodejs":
			params = mod.genConstructorTS(r, allOptionalInputs)
			paramTemplate = "ts_formal_param"
		case "go":
			params = mod.genConstructorGo(r, allOptionalInputs)
			paramTemplate = "go_formal_param"
		case "csharp":
			params = mod.genConstructorCS(r, allOptionalInputs)
			paramTemplate = "csharp_formal_param"
		case "python":
			paramTemplate = "py_formal_param"
			// The Pulumi Python SDK does not have types for constructor args.
			// The input properties for a resource needs to be exploded as
			// individual constructor params.
			params = make([]formalParam, 0, len(r.InputProperties))
			for _, p := range r.InputProperties {
				// If the property defines a const value, then skip it.
				// For example, in k8s, `apiVersion` and `kind` are often hard-coded
				// in the SDK and are not really user-provided input properties.
				if p.ConstValue != nil {
					continue
				}
				params = append(params, formalParam{
					Name:         python.PyName(p.Name),
					DefaultValue: "=None",
				})
			}
		}

		n := len(params)
		for i, p := range params {
			if err := templates.ExecuteTemplate(b, paramTemplate, p); err != nil {
				panic(err)
			}
			if i != n-1 {
				if err := templates.ExecuteTemplate(b, "param_separator", nil); err != nil {
					panic(err)
				}
			}
		}
		renderedParams[lang] = b.String()
		formalParams[lang] = params
	}

	return renderedParams, formalParams
}

// getConstructorResourceInfo returns a map of per-language information about
// the resource being constructed.
func (mod *modContext) getConstructorResourceInfo(resourceTypeName string) map[string]propertyType {
	resourceMap := make(map[string]propertyType)
	resourceDisplayName := resourceTypeName

	for _, lang := range supportedLanguages {
		// Use the module to package lookup to transform the module name to its normalized package name.
		modName := getLanguageModuleName(mod.pkg, mod.mod, lang)
		// Reset the type name back to the display name.
		resourceTypeName = resourceDisplayName

		docLangHelper := getLanguageDocHelper(lang)
		switch lang {
		case "nodejs", "go":
			// Intentionally left blank.
		case "csharp":
			if mod.mod == "" {
				resourceTypeName = fmt.Sprintf("Pulumi.%s.%s", title(mod.pkg.Name, lang), resourceTypeName)
				break
			}

			// For k8s, the C# ModuleToPackage map uses the normalized Go package names as the key.
			// So we first lookup that name and then use that to lookup the C# namespace.
			if isKubernetesPackage(mod.pkg) {
				goPkgName := getLanguageModuleName(mod.pkg, mod.mod, "go")
				modName = getLanguageModuleName(mod.pkg, goPkgName, "csharp")
			}
			resourceTypeName = fmt.Sprintf("Pulumi.%s.%s.%s", title(mod.pkg.Name, lang), modName, resourceTypeName)
		case "python":
			// Pulumi's Python language SDK does not have "types" yet, so we will skip it for now.
			continue
		default:
			panic(errors.Errorf("cannot generate constructor info for unhandled language %q", lang))
		}

		parts := strings.Split(resourceTypeName, ".")
		displayName := parts[len(parts)-1]
		resourceMap[lang] = propertyType{
			Name:        resourceDisplayName,
			DisplayName: displayName,
			Link:        docLangHelper.GetDocLinkForResourceType(mod.pkg, modName, resourceTypeName),
		}
	}

	return resourceMap
}

func (mod *modContext) getTSLookupParams(r *schema.Resource, stateParam string) []formalParam {
	docLangHelper := getLanguageDocHelper("nodejs")
	// Use the NodeJS module to package lookup to transform the module name to its normalized package name.
	modName := getLanguageModuleName(mod.pkg, mod.mod, "nodejs")
	return []formalParam{
		{
			Name: "name",

			Type: propertyType{
				Name: "string",
				Link: docLangHelper.GetDocLinkForBuiltInType("string"),
			},
		},
		{
			Name: "id",
			Type: propertyType{
				Name: "Input<ID>",
				Link: docLangHelper.GetDocLinkForPulumiType(mod.pkg, "ID"),
			},
		},
		{
			Name:         "state",
			OptionalFlag: "?",
			Type: propertyType{
				Name: stateParam,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg, modName, stateParam),
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "?",
			Type: propertyType{
				Name: "CustomResourceOptions",
				Link: docLangHelper.GetDocLinkForPulumiType(mod.pkg, "CustomResourceOptions"),
			},
		},
	}
}

func (mod *modContext) getGoLookupParams(r *schema.Resource, stateParam string) []formalParam {
	docLangHelper := getLanguageDocHelper("go")
	// Use the Go module to package lookup to transform the module name to its normalized package name.
	modName := getLanguageModuleName(mod.pkg, mod.mod, "go")
	return []formalParam{
		{
			Name:         "ctx",
			OptionalFlag: "*",
			Type: propertyType{
				Name: "Context",
				Link: docLangHelper.GetDocLinkForPulumiType(mod.pkg, "Context"),
			},
		},
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
				Link: docLangHelper.GetDocLinkForBuiltInType("string"),
			},
		},
		{
			Name: "id",
			Type: propertyType{
				Name: "IDInput",
				Link: docLangHelper.GetDocLinkForPulumiType(mod.pkg, "IDInput"),
			},
		},
		{
			Name:         "state",
			OptionalFlag: "*",
			Type: propertyType{
				Name: stateParam,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg, modName, stateParam),
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "...",
			Type: propertyType{
				Name: "ResourceOption",
				Link: docLangHelper.GetDocLinkForPulumiType(mod.pkg, "ResourceOption"),
			},
		},
	}
}

func (mod *modContext) getCSLookupParams(r *schema.Resource, stateParam string) []formalParam {
	modName := getLanguageModuleName(mod.pkg, mod.mod, "csharp")
	stateParamFQDN := fmt.Sprintf("Pulumi.%s.%s.%s", title(mod.pkg.Name, "csharp"), modName, stateParam)

	docLangHelper := getLanguageDocHelper("csharp")
	return []formalParam{
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
				Link: docLangHelper.GetDocLinkForBuiltInType("string"),
			},
		},
		{
			Name: "id",
			Type: propertyType{
				Name: "Input<string>",
				Link: docLangHelper.GetDocLinkForPulumiType(mod.pkg, "Pulumi.Input"),
			},
		},
		{
			Name:         "state",
			OptionalFlag: "?",
			Type: propertyType{
				Name: stateParam,
				Link: docLangHelper.GetDocLinkForResourceType(mod.pkg, "", stateParamFQDN),
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "?",
			DefaultValue: " = null",
			Type: propertyType{
				Name: "CustomResourceOptions",
				Link: docLangHelper.GetDocLinkForPulumiType(mod.pkg, "Pulumi.CustomResourceOptions"),
			},
		},
	}
}

// genLookupParams generates a map of per-language way of rendering the formal parameters of the lookup function
// used to lookup an existing resource.
func (mod *modContext) genLookupParams(r *schema.Resource, stateParam string) map[string]string {
	lookupParams := make(map[string]string)
	if r.StateInputs == nil {
		return lookupParams
	}

	for _, lang := range supportedLanguages {
		var (
			paramTemplate string
			params        []formalParam
		)
		b := &bytes.Buffer{}

		switch lang {
		case "nodejs":
			params = mod.getTSLookupParams(r, stateParam)
			paramTemplate = "ts_formal_param"
		case "go":
			params = mod.getGoLookupParams(r, stateParam)
			paramTemplate = "go_formal_param"
		case "csharp":
			params = mod.getCSLookupParams(r, stateParam)
			paramTemplate = "csharp_formal_param"
		case "python":
			paramTemplate = "py_formal_param"
			// The Pulumi Python SDK does not yet have types for formal parameters.
			// The input properties for a resource needs to be exploded as
			// individual constructor params.
			params = make([]formalParam, 0, len(r.StateInputs.Properties))
			for _, p := range r.StateInputs.Properties {
				params = append(params, formalParam{
					Name:         python.PyName(p.Name),
					DefaultValue: "=None",
				})
			}
		}

		n := len(params)
		for i, p := range params {
			if err := templates.ExecuteTemplate(b, paramTemplate, p); err != nil {
				panic(err)
			}
			if i != n-1 {
				if err := templates.ExecuteTemplate(b, "param_separator", nil); err != nil {
					panic(err)
				}
			}
		}
		lookupParams[lang] = b.String()
	}
	return lookupParams
}

// filterOutputProperties removes the input properties from the output properties list
// (since input props are implicitly output props), returning only "output" props.
func filterOutputProperties(inputProps []*schema.Property, props []*schema.Property) []*schema.Property {
	var outputProps []*schema.Property
	inputMap := make(map[string]bool, len(inputProps))
	for _, p := range inputProps {
		inputMap[p.Name] = true
	}
	for _, p := range props {
		if _, found := inputMap[p.Name]; !found {
			outputProps = append(outputProps, p)
		}
	}
	return outputProps
}

// genResource is the entrypoint for generating a doc for a resource
// from its Pulumi schema.
func (mod *modContext) genResource(r *schema.Resource) resourceDocArgs {
	// Create a resource module file into which all of this resource's types will go.
	name := resourceName(r)

	inputProps := make(map[string][]property)
	outputProps := make(map[string][]property)
	stateInputs := make(map[string][]property)

	var filteredOutputProps []*schema.Property
	// Provider resources do not have output properties, so there won't be anything to filter.
	if !r.IsProvider {
		filteredOutputProps = filterOutputProperties(r.InputProperties, r.Properties)
	}
	hasFilteredOutputProps := len(filteredOutputProps) > 0
	for _, lang := range supportedLanguages {
		inputProps[lang] = mod.getProperties(r.InputProperties, lang, true, false)
		if r.IsProvider {
			continue
		}
		if hasFilteredOutputProps {
			outputProps[lang] = mod.getProperties(filteredOutputProps, lang, false, false)
		}
		if r.StateInputs != nil {
			stateInputs[lang] = mod.getProperties(r.StateInputs.Properties, lang, true, false)
		}
	}

	allOptionalInputs := true
	for _, prop := range r.InputProperties {
		// If at least one prop is required, then break.
		if prop.IsRequired {
			allOptionalInputs = false
			break
		}
	}

	packageDetails := packageDetails{
		Repository: mod.pkg.Repository,
		License:    mod.pkg.License,
		Notes:      mod.pkg.Attribution,
	}

	renderedCtorParams, typedCtorParams := mod.genConstructors(r, allOptionalInputs)

	stateParam := name + "State"
	data := resourceDocArgs{
		Header: header{
			Title: name,
		},

		Tool: mod.tool,

		Comment:            r.Comment,
		DeprecationMessage: r.DeprecationMessage,

		ConstructorParams:      renderedCtorParams,
		ConstructorParamsTyped: typedCtorParams,

		ConstructorResource: mod.getConstructorResourceInfo(name),
		ArgsRequired:        !allOptionalInputs,

		InputProperties:  inputProps,
		OutputProperties: outputProps,
		LookupParams:     mod.genLookupParams(r, stateParam),
		StateInputs:      stateInputs,
		StateParam:       stateParam,
		NestedTypes:      mod.genNestedTypes(r, true /*resourceType*/),

		PackageDetails: packageDetails,
	}

	return data
}

func (mod *modContext) getNestedTypes(t schema.Type, types nestedTypeUsageInfo, input bool) {
	switch t := t.(type) {
	case *schema.ArrayType:
		glog.V(4).Infof("visiting array %s\n", t.ElementType.String())
		skip := false
		if o, ok := t.ElementType.(*schema.ObjectType); ok && types.contains(o.Token, input) {
			glog.V(4).Infof("already added %s. skipping...\n", o.Token)
			skip = true
		}

		if !skip {
			mod.getNestedTypes(t.ElementType, types, input)
		}
	case *schema.MapType:
		glog.V(4).Infof("visiting map %s\n", t.ElementType.String())
		skip := false
		if o, ok := t.ElementType.(*schema.ObjectType); ok && types.contains(o.Token, input) {
			glog.V(4).Infof("already added %s. skipping...\n", o.Token)
			skip = true
		}

		if !skip {
			mod.getNestedTypes(t.ElementType, types, input)
		}
	case *schema.ObjectType:
		glog.V(4).Infof("visiting object %s\n", t.Token)
		types.add(t.Token, input)
		for _, p := range t.Properties {
			if o, ok := p.Type.(*schema.ObjectType); ok && types.contains(o.Token, input) {
				glog.V(4).Infof("already added %s. skipping...\n", o.Token)
				continue
			}
			glog.V(4).Infof("visiting object property %s\n", p.Type.String())
			mod.getNestedTypes(p.Type, types, input)
		}
	case *schema.UnionType:
		glog.V(4).Infof("visiting union type %s\n", t.String())
		for _, e := range t.ElementTypes {
			if o, ok := e.(*schema.ObjectType); ok && types.contains(o.Token, input) {
				glog.V(4).Infof("already added %s. skipping...\n", o.Token)
				continue
			}
			glog.V(4).Infof("visiting union element type %s\n", e.String())
			mod.getNestedTypes(e, types, input)
		}
	}
}

func (mod *modContext) getTypes(member interface{}, types nestedTypeUsageInfo) {
	glog.V(3).Infoln("getting nested types for module", mod.mod)

	switch t := member.(type) {
	case *schema.ObjectType:
		for _, p := range t.Properties {
			mod.getNestedTypes(p.Type, types, false)
		}
	case *schema.Resource:
		for _, p := range t.Properties {
			mod.getNestedTypes(p.Type, types, false)
		}
		for _, p := range t.InputProperties {
			mod.getNestedTypes(p.Type, types, true)
		}
	case *schema.Function:
		if t.Inputs != nil {
			mod.getNestedTypes(t.Inputs, types, true)
		}
		if t.Outputs != nil {
			mod.getNestedTypes(t.Outputs, types, false)
		}
	}
}

type fs map[string][]byte

func (fs fs) add(path string, contents []byte) {
	_, has := fs[path]
	contract.Assertf(!has, "duplicate file: %s", path)
	fs[path] = contents
}

// getModuleFileName returns the normalized package name to use
// for k8s. Otherwise, returns the current module's name as-is.
func (mod *modContext) getModuleFileName() string {
	if !isKubernetesPackage(mod.pkg) {
		return mod.mod
	}

	if override, ok := goPkgInfo.ModuleToPackage[mod.mod]; ok {
		return override
	}
	return mod.mod
}

func (mod *modContext) gen(fs fs) error {
	modName := mod.getModuleFileName()
	var files []string
	for p := range fs {
		d := path.Dir(p)
		if d == "." {
			d = ""
		}
		if d == modName {
			files = append(files, p)
		}
	}

	addFile := func(name, contents string) {
		p := path.Join(modName, name)
		files = append(files, p)
		fs.add(p, []byte(contents))
	}

	// Resources
	isK8s := isKubernetesPackage(mod.pkg)
	for _, r := range mod.resources {
		data := mod.genResource(r)

		title := resourceName(r)
		buffer := &bytes.Buffer{}
		// Don't include the "go" language in the language chooser
		// for the helm/v2 and yaml modules in the k8s package. These
		// are "overlay" modules. The resources under those modules are
		// not available in Go.
		if isK8s && mod.isKubernetesOverlayModule() {
			data.LangChooserLanguages = "javascript,typescript,python,csharp"
		}

		err := templates.ExecuteTemplate(buffer, "resource.tmpl", data)
		if err != nil {
			return err
		}

		addFile(strings.ToLower(title)+".md", buffer.String())
	}

	// Functions
	for _, f := range mod.functions {
		data := mod.genFunction(f)

		buffer := &bytes.Buffer{}
		err := templates.ExecuteTemplate(buffer, "function.tmpl", data)
		if err != nil {
			return err
		}

		addFile(strings.ToLower(tokenToName(f.Token))+".md", buffer.String())
	}

	// Generate the index files.
	idxData := mod.genIndex()
	buffer := &bytes.Buffer{}
	err := templates.ExecuteTemplate(buffer, "index.tmpl", idxData)
	if err != nil {
		return err
	}

	fs.add(path.Join(modName, "_index.md"), []byte(buffer.String()))
	return nil
}

// indexEntry represents an individual entry on an index page.
type indexEntry struct {
	Link        string
	DisplayName string
}

// indexData represents the index file data to be rendered as _index.md.
type indexData struct {
	Tool string

	Title              string
	PackageDescription string
	// Menu indicates if an index page should be part of the TOC menu.
	Menu bool

	Functions      []indexEntry
	Resources      []indexEntry
	Modules        []indexEntry
	PackageDetails packageDetails
}

// indexEntrySorter implements the sort.Interface for sorting
// a slice of indexEntry struct types.
type indexEntrySorter struct {
	entries []indexEntry
}

// Len is part of sort.Interface. Returns the length of the
// entries slice.
func (s *indexEntrySorter) Len() int {
	return len(s.entries)
}

// Swap is part of sort.Interface.
func (s *indexEntrySorter) Swap(i, j int) {
	s.entries[i], s.entries[j] = s.entries[j], s.entries[i]
}

// Less is part of sort.Interface. It sorts the entries by their
// display name in an ascending order.
func (s *indexEntrySorter) Less(i, j int) bool {
	return s.entries[i].DisplayName < s.entries[j].DisplayName
}

func sortIndexEntries(entries []indexEntry) {
	if len(entries) == 0 {
		return
	}

	sorter := &indexEntrySorter{
		entries: entries,
	}

	sort.Sort(sorter)
}

// genIndex emits an _index.md file for the module.
func (mod *modContext) genIndex() indexData {
	glog.V(4).Infoln("genIndex for", mod.mod)
	modules := make([]indexEntry, 0, len(mod.children))
	resources := make([]indexEntry, 0, len(mod.resources))
	functions := make([]indexEntry, 0, len(mod.functions))

	modName := mod.getModuleFileName()
	title := modName
	menu := false
	if title == "" {
		// If title not found in titleLookup map, default back to mod.pkg.Name.
		if val, ok := titleLookup[mod.pkg.Name]; ok {
			title = val
		} else {
			title = mod.pkg.Name
		}
		// Flag top-level entries for inclusion in the table-of-contents menu.
		menu = true
	}

	// If there are submodules, list them.
	for _, mod := range mod.children {
		modName := mod.getModuleFileName()
		parts := strings.Split(modName, "/")
		modName = parts[len(parts)-1]
		modules = append(modules, indexEntry{
			Link:        strings.ToLower(modName) + "/",
			DisplayName: modName,
		})
	}
	sortIndexEntries(modules)

	// If there are resources in the root, list them.
	for _, r := range mod.resources {
		name := resourceName(r)
		resources = append(resources, indexEntry{
			Link:        strings.ToLower(name),
			DisplayName: name,
		})
	}
	sortIndexEntries(resources)

	// If there are functions in the root, list them.
	for _, f := range mod.functions {
		name := tokenToName(f.Token)
		functions = append(functions, indexEntry{
			Link:        strings.ToLower(name),
			DisplayName: name,
		})
	}
	sortIndexEntries(functions)

	packageDetails := packageDetails{
		Repository: mod.pkg.Repository,
		License:    mod.pkg.License,
		Notes:      mod.pkg.Attribution,
		Version:    mod.pkg.Version.String(),
	}

	data := indexData{
		Tool: mod.tool,

		Title: title,
		Menu:  menu,

		Resources:      resources,
		Functions:      functions,
		Modules:        modules,
		PackageDetails: packageDetails,
	}

	// If this is the root module, write out the package description.
	if mod.mod == "" {
		data.PackageDescription = mod.pkg.Description
	}

	return data
}

func decodeLangSpecificInfo(pkg *schema.Package, lang string, obj interface{}) error {
	if csharp, ok := pkg.Language[lang]; ok {
		if err := json.Unmarshal([]byte(csharp), &obj); err != nil {
			return errors.Wrap(err, "decoding csharp package info")
		}
	}
	return nil
}

func getMod(pkg *schema.Package, token string, modules map[string]*modContext, tool string) *modContext {
	modName := pkg.TokenToModule(token)
	mod, ok := modules[modName]
	if !ok {
		mod = &modContext{
			pkg:  pkg,
			mod:  modName,
			tool: tool,
		}

		if modName != "" {
			parentName := path.Dir(modName)
			// If the parent name is blank, it means this is the package-level.
			if parentName == "." || parentName == "" {
				parentName = ":index:"
			}
			parent := getMod(pkg, parentName, modules, tool)
			parent.children = append(parent.children, mod)
		}

		modules[modName] = mod
	}
	return mod
}

func generatePythonPropertyCaseMaps(mod *modContext, r *schema.Resource) {
	pyLangHelper := getLanguageDocHelper("python").(*python.DocLanguageHelper)
	for _, p := range r.Properties {
		pyLangHelper.GenPropertyCaseMap(mod.pkg, mod.mod, mod.tool, p, snakeCaseToCamelCase, camelCaseToSnakeCase)
	}

	for _, p := range r.InputProperties {
		pyLangHelper.GenPropertyCaseMap(mod.pkg, mod.mod, mod.tool, p, snakeCaseToCamelCase, camelCaseToSnakeCase)
	}
}

func generateModulesFromSchemaPackage(tool string, pkg *schema.Package) map[string]*modContext {
	// Group resources, types, and functions into modules.
	modules := map[string]*modContext{}

	// Decode Go-specific language info.
	if err := decodeLangSpecificInfo(pkg, "go", &goPkgInfo); err != nil {
		panic(errors.Wrap(err, "error decoding go language info"))
	}
	goLangHelper := getLanguageDocHelper("go").(*go_gen.DocLanguageHelper)
	// Generate the Go package map info now, so we can use that to get the type string
	// names later.
	goLangHelper.GeneratePackagesMap(pkg, tool, goPkgInfo)

	// Decode C#-specific language info.
	if err := decodeLangSpecificInfo(pkg, "csharp", &csharpPkgInfo); err != nil {
		panic(errors.Wrap(err, "error decoding c# language info"))
	}
	csharpLangHelper := getLanguageDocHelper("csharp").(*dotnet.DocLanguageHelper)
	csharpLangHelper.Namespaces = csharpPkgInfo.Namespaces

	// Decode NodeJS-specific language info.
	if err := decodeLangSpecificInfo(pkg, "nodejs", &nodePkgInfo); err != nil {
		panic(errors.Wrap(err, "error decoding nodejs language info"))
	}

	scanResource := func(r *schema.Resource) {
		mod := getMod(pkg, r.Token, modules, tool)
		mod.resources = append(mod.resources, r)

		generatePythonPropertyCaseMaps(mod, r)
	}

	scanK8SResource := func(r *schema.Resource) {
		mod := getKubernetesMod(pkg, r.Token, modules, tool)
		mod.resources = append(mod.resources, r)
	}

	glog.V(3).Infoln("scanning resources")
	if isKubernetesPackage(pkg) {
		scanK8SResource(pkg.Provider)
		for _, r := range pkg.Resources {
			scanK8SResource(r)
		}
	} else {
		scanResource(pkg.Provider)
		for _, r := range pkg.Resources {
			scanResource(r)
		}
	}
	glog.V(3).Infoln("done scanning resources")

	for _, f := range pkg.Functions {
		mod := getMod(pkg, f.Token, modules, tool)
		mod.functions = append(mod.functions, f)
	}
	return modules
}

// GeneratePackage generates the docs package with docs for each resource given the Pulumi
// schema.
func GeneratePackage(tool string, pkg *schema.Package) (map[string][]byte, error) {
	templates = template.New("").Funcs(template.FuncMap{
		"htmlSafe": func(html string) template.HTML {
			// Markdown fragments in the templates need to be rendered as-is,
			// so that html/template package doesn't try to inject data into it,
			// which will most certainly fail.
			// nolint gosec
			return template.HTML(html)
		},
		"pyName": func(str string) string {
			return python.PyName(str)
		},
	})

	for name, b := range packagedTemplates {
		template.Must(templates.New(name).Parse(string(b)))
	}

	defer glog.Flush()

	// Generate the modules from the schema, and for every module
	// run the generator functions to generate markdown files.
	modules := generateModulesFromSchemaPackage(tool, pkg)
	glog.V(3).Infoln("generating package now...")
	files := fs{}
	for _, mod := range modules {
		if err := mod.gen(files); err != nil {
			return nil, err
		}
	}

	return files, nil
}
