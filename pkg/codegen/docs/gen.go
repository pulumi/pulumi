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
//nolint:lll, goconst
package docs

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html"
	"html/template"
	"path"
	"sort"
	"strings"

	"github.com/golang/glog"

	"github.com/pgavlin/goldmark"

	"github.com/pulumi/pulumi-java/pkg/codegen/java"
	yaml "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	go_gen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

//go:embed templates/*.tmpl
var packagedTemplates embed.FS

// NOTE: This lookup map can be removed when all Pulumi-managed packages
// have a DisplayName in their schema. See pulumi/pulumi#7813.
// This lookup table no longer needs to be updated for new providers
// and is considered stale.
//
// titleLookup is a map of package name to the desired display name.
func titleLookup(shortName string) (string, bool) {
	v, ok := map[string]string{
		"aiven":                                "Aiven",
		"akamai":                               "Akamai",
		"alicloud":                             "Alibaba Cloud",
		"auth0":                                "Auth0",
		"aws":                                  "AWS Classic",
		"awsx":                                 "AWSx (Pulumi Crosswalk for AWS)",
		"aws-apigateway":                       "AWS API Gateway",
		"aws-miniflux":                         "Miniflux",
		"aws-native":                           "AWS Native",
		"aws-quickstart-aurora-mysql":          "AWS QuickStart Aurora MySQL",
		"aws-quickstart-aurora-postgres":       "AWS QuickStart Aurora PostgreSQL",
		"aws-quickstart-redshift":              "AWS QuickStart Redshift",
		"aws-serverless":                       "AWS Serverless",
		"aws-quickstart-vpc":                   "AWS QuickStart VPC",
		"aws-s3-replicated-bucket":             "AWS S3 Replicated Bucket",
		"azure":                                "Azure Classic",
		"azure-justrun":                        "Azure Justrun",
		"azure-native":                         "Azure Native",
		"azure-quickstart-acr-geo-replication": "Azure QuickStart ACR Geo Replication",
		"azure-quickstart-aks":                 "Azure QuickStart AKS",
		"azure-quickstart-compute":             "Azure QuickStart Compute",
		"azure-quickstart-sql":                 "Azure QuickStart SQL",
		"azuread":                              "Azure Active Directory (Azure AD)",
		"azuredevops":                          "Azure DevOps",
		"azuresel":                             "Azure",
		"civo":                                 "Civo",
		"cloudamqp":                            "CloudAMQP",
		"cloudflare":                           "Cloudflare",
		"cloudinit":                            "cloud-init",
		"confluentcloud":                       "Confluent Cloud",
		"confluent":                            "Confluent Cloud (Deprecated)",
		"consul":                               "HashiCorp Consul",
		"coredns-helm":                         "CoreDNS (Helm)",
		"datadog":                              "Datadog",
		"digitalocean":                         "DigitalOcean",
		"dnsimple":                             "DNSimple",
		"docker":                               "Docker",
		"docker-buildkit":                      "Docker BuildKit",
		"eks":                                  "Amazon EKS",
		"equinix-metal":                        "Equinix Metal",
		"f5bigip":                              "f5 BIG-IP",
		"fastly":                               "Fastly",
		"gcp":                                  "Google Cloud (GCP) Classic",
		"gcp-global-cloudrun":                  "Google Global Cloud Run",
		"gcp-project-scaffold":                 "Google Project Scaffolding",
		"google-native":                        "Google Cloud Native",
		"github":                               "GitHub",
		"github-serverless-webhook":            "GitHub Serverless Webhook",
		"gitlab":                               "GitLab",
		"hcloud":                               "Hetzner Cloud",
		"istio-helm":                           "Istio (Helm)",
		"jaeger-helm":                          "Jaeger (Helm)",
		"kafka":                                "Kafka",
		"keycloak":                             "Keycloak",
		"kong":                                 "Kong",
		"kubernetes":                           "Kubernetes",
		"libvirt":                              "libvirt",
		"linode":                               "Linode",
		"mailgun":                              "Mailgun",
		"minio":                                "MinIO",
		"mongodbatlas":                         "MongoDB Atlas",
		"mysql":                                "MySQL",
		"newrelic":                             "New Relic",
		"kubernetes-ingress-nginx":             "NGINX Ingress Controller (Helm)",
		"kubernetes-coredns":                   "CoreDNS (Helm)",
		"kubernetes-cert-manager":              "Jetstack Cert Manager (Helm)",
		"nomad":                                "HashiCorp Nomad",
		"ns1":                                  "NS1",
		"okta":                                 "Okta",
		"openstack":                            "OpenStack",
		"opsgenie":                             "Opsgenie",
		"packet":                               "Packet",
		"pagerduty":                            "PagerDuty",
		"pulumi-std":                           "Pulumi Standard Library",
		"postgresql":                           "PostgreSQL",
		"prometheus-helm":                      "Prometheus (Helm)",
		"rabbitmq":                             "RabbitMQ",
		"rancher2":                             "Rancher2",
		"random":                               "random",
		"rke":                                  "Rancher Kubernetes Engine (RKE)",
		"run-my-darn-container":                "Run My Darn Container",
		"shipa":                                "Shipa",
		"signalfx":                             "SignalFx",
		"snowflake":                            "Snowflake",
		"splunk":                               "Splunk",
		"spotinst":                             "Spotinst",
		"sumologic":                            "Sumo Logic",
		"tls":                                  "TLS",
		"vault":                                "Vault",
		"venafi":                               "Venafi",
		"vsphere":                              "vSphere",
		"wavefront":                            "Wavefront",
		"yandex":                               "Yandex",
	}[shortName]
	return v, ok
}

// Property anchor tag separator, used in a property anchor tag id to separate the
// property and language (e.g. property~lang).
const propertyLangSeparator = "_"

type docGenContext struct {
	internalModMap map[string]*modContext

	supportedLanguages []string
	snippetLanguages   []string
	templates          *template.Template
	docHelpers         map[string]codegen.DocLanguageHelper

	// The language-specific info objects for a certain package (provider).
	goPkgInfo     go_gen.GoPackageInfo
	csharpPkgInfo dotnet.CSharpPackageInfo
	nodePkgInfo   nodejs.NodePackageInfo
	pythonPkgInfo python.PackageInfo

	// langModuleNameLookup is a map of module name to its language-specific
	// name.
	langModuleNameLookup map[string]string
}

// modules is a map of a module name and information
// about it. This is crux of all API docs generation
// as the modContext carries information about the resources,
// functions, as well other modules within each module.
func (dctx *docGenContext) modules() map[string]*modContext {
	return dctx.internalModMap
}

func (dctx *docGenContext) setModules(modules map[string]*modContext) {
	m := map[string]*modContext{}
	for k, v := range modules {
		m[k] = v.withDocGenContext(dctx)
	}
	dctx.internalModMap = m
}

func newDocGenContext() *docGenContext {
	supportedLanguages := []string{"csharp", "go", "nodejs", "python", "yaml", "java"}
	docHelpers := make(map[string]codegen.DocLanguageHelper)
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
		case "yaml":
			docHelpers[lang] = &yaml.DocLanguageHelper{}
		case "java":
			docHelpers[lang] = &java.DocLanguageHelper{}
		}
	}

	return &docGenContext{
		supportedLanguages:   supportedLanguages,
		snippetLanguages:     []string{"csharp", "go", "python", "typescript", "yaml", "java"},
		langModuleNameLookup: map[string]string{},
		docHelpers:           docHelpers,
	}
}

type typeDetails struct {
	inputType bool
}

// header represents the header of each resource markdown file.
type header struct {
	Title    string
	TitleTag string
	MetaDesc string
}

// property represents an input or an output property.
type property struct {
	// ID is the `id` attribute that will be attached to the DOM element containing the property.
	ID string
	// DisplayName is the property name with word-breaks.
	DisplayName        string
	Name               string
	Comment            string
	Types              []propertyType
	DeprecationMessage string
	Link               string

	IsRequired         bool
	IsInput            bool
	IsReplaceOnChanges bool
}

// enum represents an enum.
type enum struct {
	ID                 string // ID is the `id` attribute attached to the DOM element containing the enum.
	DisplayName        string // DisplayName is the enum name with word-breaks.
	Name               string // Name is the language-specific enum name.
	Value              string
	Comment            string
	DeprecationMessage string
}

// docNestedType represents a complex type.
type docNestedType struct {
	Name       string
	AnchorID   string
	Properties map[string][]property
	EnumValues map[string][]enum
}

// propertyType represents the type of a property.
type propertyType struct {
	DisplayName     string
	DescriptionName string // Name used in description list.
	Name            string
	// Link can be a link to an anchor tag on the same
	// page, or to another page/site.
	Link string
}

// paramSeparator is for data passed to the separator template.
type paramSeparator struct {
	Indent string
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
	DisplayName    string
	Repository     string
	RepositoryName string
	License        string
	Notes          string
	Version        string
}

type resourceDocArgs struct {
	Header header

	Tool string
	// LangChooserLanguages is a comma-separated list of languages to pass to the
	// language chooser shortcode. Use this to customize the languages shown for a
	// resource. By default, the language chooser will show all languages supported
	// by Pulumi for all resources.
	LangChooserLanguages string

	// Comment represents the introductory resource comment.
	Comment            string
	ExamplesSection    []exampleSection
	DeprecationMessage string

	// Import
	ImportDocs string

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

	// A list of methods associated with the resource.
	Methods []methodDocArgs

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
	pkg           schema.PackageReference
	mod           string
	inputTypes    []*schema.ObjectType
	resources     []*schema.Resource
	functions     []*schema.Function
	typeDetails   map[*schema.ObjectType]*typeDetails
	children      []*modContext
	tool          string
	docGenContext *docGenContext
}

func (mod *modContext) withDocGenContext(dctx *docGenContext) *modContext {
	if mod == nil {
		return nil
	}
	copy := *mod
	copy.docGenContext = dctx
	var children = make([]*modContext, 0, len(copy.children))
	for _, c := range copy.children {
		children = append(children, c.withDocGenContext(dctx))
	}
	copy.children = children
	return &copy
}

func resourceName(r *schema.Resource) string {
	if r.IsProvider {
		return "Provider"
	}
	return strings.Title(tokenToName(r.Token))
}

func (dctx *docGenContext) getLanguageDocHelper(lang string) codegen.DocLanguageHelper {
	if h, ok := dctx.docHelpers[lang]; ok {
		return h
	}
	panic(fmt.Errorf("could not find a doc lang helper for %s", lang))
}

type propertyCharacteristics struct {
	// input is a flag indicating if the property is an input type.
	input bool
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

// getLanguageModuleName transforms the current module's name to a
// language-specific name using the language info, if any, for the
// current package.
func (mod *modContext) getLanguageModuleName(lang string) string {
	dctx := mod.docGenContext
	modName := mod.mod
	lookupKey := lang + "_" + modName
	if v, ok := mod.docGenContext.langModuleNameLookup[lookupKey]; ok {
		return v
	}

	switch lang {
	case "go":
		// Go module names use lowercase.
		modName = strings.ToLower(modName)
		if override, ok := dctx.goPkgInfo.ModuleToPackage[modName]; ok {
			modName = override
		}
	case "csharp":
		if override, ok := dctx.csharpPkgInfo.Namespaces[modName]; ok {
			modName = override
		}
	case "nodejs":
		if override, ok := dctx.nodePkgInfo.ModuleToPackage[modName]; ok {
			modName = override
		}
	case "python":
		if override, ok := dctx.pythonPkgInfo.ModuleNameOverrides[modName]; ok {
			modName = override
		}
	}

	mod.docGenContext.langModuleNameLookup[lookupKey] = modName
	return modName
}

// cleanTypeString removes any namespaces from the generated type string for all languages.
// The result of this function should be used display purposes only.
func (mod *modContext) cleanTypeString(t schema.Type, langTypeString, lang, modName string, isInput bool) string {
	switch lang {
	case "go", "python":
		langTypeString = cleanOptionalIdentifier(langTypeString, lang)
		parts := strings.Split(langTypeString, ".")
		return parts[len(parts)-1]
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
			modName = mod.getLanguageModuleName(lang)
		}
	}

	if lang == "nodejs" {
		return cleanNodeJSName(modName)
	} else if lang == "csharp" {
		return cleanCSharpName(mod.pkg.Name(), modName)
	}
	return strings.ReplaceAll(langTypeString, modName, "")
}

// typeString returns a property type suitable for docs with its display name and the anchor link to
// a type if the type of the property is an array or an object.
func (mod *modContext) typeString(t schema.Type, lang string, characteristics propertyCharacteristics, insertWordBreaks bool) propertyType {
	t = codegen.PlainType(t)

	docLanguageHelper := mod.docGenContext.getLanguageDocHelper(lang)
	modName := mod.getLanguageModuleName(lang)
	def, err := mod.pkg.Definition()
	contract.AssertNoError(err)
	langTypeString := docLanguageHelper.GetLanguageTypeString(def, modName, t, characteristics.input)

	if optional, ok := t.(*schema.OptionalType); ok {
		t = optional.ElementType
	}

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
	case *schema.EnumType:
		tokenName := tokenToName(t.Token)
		// Links to anchor tags on the same page must be lower-cased.
		href = "#" + strings.ToLower(tokenName)
	case *schema.UnionType:
		var elements []string
		for _, e := range t.ElementTypes {
			elementLangType := mod.typeString(e, lang, characteristics, false)
			elements = append(elements, elementLangType.DisplayName)
		}
		langTypeString = strings.Join(elements, " | ")
	}

	// Strip the namespace/module prefix for the type's display name.
	displayName := langTypeString
	if !schema.IsPrimitiveType(t) {
		displayName = mod.cleanTypeString(t, langTypeString, lang, modName, characteristics.input)
	}

	displayName = cleanOptionalIdentifier(displayName, lang)
	langTypeString = cleanOptionalIdentifier(langTypeString, lang)

	// Name and DisplayName should be html-escaped to avoid throwing off rendering for template types in languages like
	// csharp, Java etc. If word-breaks need to be inserted, then the type string should be html-escaped first.
	displayName = html.EscapeString(displayName)
	if insertWordBreaks {
		displayName = wbr(displayName)
	}

	return propertyType{
		Name:        html.EscapeString(langTypeString),
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
		if strings.HasPrefix(s, "Optional[") && strings.HasSuffix(s, "]") {
			s = strings.TrimPrefix(s, "Optional[")
			s = strings.TrimSuffix(s, "]")
			return s
		}
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
	docLangHelper := mod.docGenContext.getLanguageDocHelper("nodejs")

	var argsType string
	optsType := "CustomResourceOptions"
	// The args type for k8s package differs from the rest depending on whether we are dealing with
	// overlay resources or regular k8s resources.
	if isKubernetesPackage(mod.pkg) {
		if mod.isKubernetesOverlayModule() {
			if name == "CustomResource" {
				argsType = name + "Args"
			} else {
				argsType = name + "Opts"
			}
		} else {
			// The non-schema-based k8s codegen does not apply a suffix to the input types.
			argsType = name
		}

		if mod.isComponentResource() {
			optsType = "ComponentResourceOptions"
		}
	} else {
		argsType = name + "Args"
	}

	argsFlag := ""
	if argsOptional {
		argsFlag = "?"
	}

	def, err := mod.pkg.Definition()
	contract.AssertNoError(err)

	return []formalParam{
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
			},
			Comment: ctorNameArgComment,
		},
		{
			Name:         "args",
			OptionalFlag: argsFlag,
			Type: propertyType{
				Name: argsType,
				Link: "#inputs",
			},
			Comment: ctorArgsArgComment,
		},
		{
			Name:         "opts",
			OptionalFlag: "?",
			Type: propertyType{
				Name: optsType,
				Link: docLangHelper.GetDocLinkForPulumiType(def, optsType),
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

	docLangHelper := mod.docGenContext.getLanguageDocHelper("go")

	def, err := mod.pkg.Definition()
	contract.AssertNoError(err)

	return []formalParam{
		{
			Name:         "ctx",
			OptionalFlag: "*",
			Type: propertyType{
				Name: "Context",
				Link: docLangHelper.GetDocLinkForPulumiType(def, "Context"),
			},
			Comment: "Context object for the current deployment.",
		},
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
			},
			Comment: ctorNameArgComment,
		},
		{
			Name:         "args",
			OptionalFlag: argsFlag,
			Type: propertyType{
				Name: argsType,
				Link: "#inputs",
			},
			Comment: ctorArgsArgComment,
		},
		{
			Name:         "opts",
			OptionalFlag: "...",
			Type: propertyType{
				Name: "ResourceOption",
				Link: docLangHelper.GetDocLinkForPulumiType(def, "ResourceOption"),
			},
			Comment: ctorOptsArgComment,
		},
	}
}

func (mod *modContext) genConstructorCS(r *schema.Resource, argsOptional bool) []formalParam {
	name := resourceName(r)
	optsType := "CustomResourceOptions"

	if isKubernetesPackage(mod.pkg) && mod.isComponentResource() {
		optsType = "ComponentResourceOptions"
	}

	var argsFlag string
	var argsDefault string
	if argsOptional {
		// If the number of required input properties was zero, we can make the args object optional.
		argsDefault = " = null"
		argsFlag = "?"
	}

	docLangHelper := mod.docGenContext.getLanguageDocHelper("csharp")

	def, err := mod.pkg.Definition()
	contract.AssertNoError(err)

	return []formalParam{
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
			},
			Comment: ctorNameArgComment,
		},
		{
			Name:         "args",
			OptionalFlag: argsFlag,
			DefaultValue: argsDefault,
			Type: propertyType{
				Name: name + "Args",
				Link: "#inputs",
			},
			Comment: ctorArgsArgComment,
		},
		{
			Name:         "opts",
			OptionalFlag: "?",
			DefaultValue: " = null",
			Type: propertyType{
				Name: optsType,
				Link: docLangHelper.GetDocLinkForPulumiType(def, fmt.Sprintf("Pulumi.%s", optsType)),
			},
			Comment: ctorOptsArgComment,
		},
	}
}

func (mod *modContext) genConstructorYaml() []formalParam {
	return []formalParam{
		{
			Name:    "properties",
			Comment: ctorArgsArgComment,
		},
		{
			Name:    "options",
			Comment: ctorOptsArgComment,
		},
	}
}

func (mod *modContext) genConstructorJava(r *schema.Resource, argsOverload bool) []formalParam {
	name := resourceName(r)
	optsType := "CustomResourceOptions"

	if mod.isComponentResource() {
		optsType = "ComponentResourceOptions"
	}

	docLangHelper := mod.docGenContext.getLanguageDocHelper("java")

	def, err := mod.pkg.Definition()
	contract.AssertNoError(err)

	result := []formalParam{
		{
			Name: "name",
			Type: propertyType{
				Name: "String",
			},
			Comment: ctorNameArgComment,
		},
		{
			Name: "args",
			Type: propertyType{
				Name: name + "Args",
				Link: "#inputs",
			},
			Comment: ctorArgsArgComment,
		},
	}
	if !argsOverload {
		result = append(result, formalParam{
			Name:         "options",
			OptionalFlag: "@Nullable",
			Type: propertyType{
				Name: optsType,
				Link: docLangHelper.GetDocLinkForPulumiType(def, optsType),
			},
			Comment: ctorOptsArgComment,
		})
	}
	return result
}

func (mod *modContext) genConstructorPython(r *schema.Resource, argsOptional, argsOverload bool) []formalParam {
	docLanguageHelper := mod.docGenContext.getLanguageDocHelper("python")
	isK8sOverlayMod := mod.isKubernetesOverlayModule()
	isDockerImageResource := mod.pkg.Name() == "docker" && resourceName(r) == "Image"

	// Kubernetes overlay resources use a different ordering of formal params in Python.
	if isK8sOverlayMod && r.IsOverlay {
		return getKubernetesOverlayPythonFormalParams(mod.mod)
	} else if isDockerImageResource {
		return getDockerImagePythonFormalParams()
	}

	// We perform at least three appends before iterating over input types.
	params := make([]formalParam, 0, 3+len(r.InputProperties))

	params = append(params, formalParam{
		Name: "resource_name",
		Type: propertyType{
			Name: "str",
		},
		Comment: ctorNameArgComment,
	})

	if argsOverload {
		// Determine whether we need to use the alternate args class name (e.g. `<Resource>InitArgs` instead of
		// `<Resource>Args`) due to an input type with the same name as the resource in the same module.
		resName := resourceName(r)
		resArgsName := fmt.Sprintf("%sArgs", resName)
		for _, inputType := range mod.inputTypes {
			inputTypeName := strings.Title(tokenToName(inputType.Token))
			if resName == inputTypeName {
				resArgsName = fmt.Sprintf("%sInitArgs", resName)
			}
		}

		optionalFlag, defaultVal, descriptionName := "", "", resArgsName
		typeName := descriptionName
		if argsOptional {
			optionalFlag, defaultVal, typeName = "optional", " = None", fmt.Sprintf("Optional[%s]", typeName)
		}
		params = append(params, formalParam{
			Name:         "args",
			OptionalFlag: optionalFlag,
			DefaultValue: defaultVal,
			Type: propertyType{
				Name:            typeName,
				DescriptionName: descriptionName,
				Link:            "#inputs",
			},
			Comment: ctorArgsArgComment,
		})
	}

	params = append(params, formalParam{
		Name:         "opts",
		OptionalFlag: "optional",
		DefaultValue: " = None",
		Type: propertyType{
			Name:            "Optional[ResourceOptions]",
			DescriptionName: "ResourceOptions",
			Link:            "/docs/reference/pkg/python/pulumi/#pulumi.ResourceOptions",
		},
		Comment: ctorOptsArgComment,
	})

	if argsOverload {
		return params
	}

	for _, p := range r.InputProperties {
		// If the property defines a const value, then skip it.
		// For example, in k8s, `apiVersion` and `kind` are often hard-coded
		// in the SDK and are not really user-provided input properties.
		if p.ConstValue != nil {
			continue
		}
		def, err := mod.pkg.Definition()
		contract.AssertNoError(err)
		typ := docLanguageHelper.GetLanguageTypeString(def, mod.mod, codegen.PlainType(codegen.OptionalType(p)), true /*input*/)
		params = append(params, formalParam{
			Name:         python.InitParamName(p.Name),
			DefaultValue: " = None",
			Type: propertyType{
				Name: typ,
			},
		})
	}
	return params
}

func (mod *modContext) genNestedTypes(member interface{}, resourceType bool) []docNestedType {
	dctx := mod.docGenContext
	tokens := nestedTypeUsageInfo{}
	// Collect all of the types for this "member" as a map of resource names
	// and if it appears in an input object and/or output object.
	mod.getTypes(member, tokens)

	sortedTokens := make([]string, 0, len(tokens))
	for token := range tokens {
		sortedTokens = append(sortedTokens, token)
	}
	sort.Strings(sortedTokens)

	var typs []docNestedType
	for _, token := range sortedTokens {
		for iter := mod.pkg.Types().Range(); iter.Next(); {
			t, err := iter.Type()
			contract.AssertNoError(err)
			switch typ := t.(type) {
			case *schema.ObjectType:
				if typ.Token != token || len(typ.Properties) == 0 || typ.IsInputShape() {
					continue
				}

				// Create a map to hold the per-language properties of this object.
				props := make(map[string][]property)
				for _, lang := range dctx.supportedLanguages {
					props[lang] = mod.getProperties(typ.Properties, lang, true, true, false)
				}

				name := strings.Title(tokenToName(typ.Token))
				typs = append(typs, docNestedType{
					Name:       wbr(name),
					AnchorID:   strings.ToLower(name),
					Properties: props,
				})
			case *schema.EnumType:
				if typ.Token != token || len(typ.Elements) == 0 {
					continue
				}
				name := strings.Title(tokenToName(typ.Token))

				enums := make(map[string][]enum)
				for _, lang := range dctx.supportedLanguages {
					docLangHelper := dctx.getLanguageDocHelper(lang)

					var langEnumValues []enum
					for _, e := range typ.Elements {
						enumName, err := docLangHelper.GetEnumName(e, name)
						if err != nil {
							panic(err)
						}
						enumID := strings.ToLower(name + propertyLangSeparator + lang)
						langEnumValues = append(langEnumValues, enum{
							ID:                 enumID,
							DisplayName:        wbr(enumName),
							Name:               enumName,
							Value:              fmt.Sprintf("%v", e.Value),
							Comment:            e.Comment,
							DeprecationMessage: e.DeprecationMessage,
						})
					}
					enums[lang] = langEnumValues
				}

				typs = append(typs, docNestedType{
					Name:       wbr(name),
					AnchorID:   strings.ToLower(name),
					EnumValues: enums,
				})
			}
		}
	}

	sort.Slice(typs, func(i, j int) bool {
		return typs[i].Name < typs[j].Name
	})

	return typs
}

// getProperties returns a slice of properties that can be rendered for docs for
// the provided slice of properties in the schema.
func (mod *modContext) getProperties(properties []*schema.Property, lang string, input, nested, isProvider bool,
) []property {
	return mod.getPropertiesWithIDPrefixAndExclude(properties, lang, input, nested, isProvider, "", nil)
}

func (mod *modContext) getPropertiesWithIDPrefixAndExclude(properties []*schema.Property, lang string, input, nested,
	isProvider bool, idPrefix string, exclude func(name string) bool) []property {

	dctx := mod.docGenContext
	if len(properties) == 0 {
		return nil
	}
	docProperties := make([]property, 0, len(properties))
	for _, prop := range properties {
		if prop == nil {
			continue
		}

		if exclude != nil && exclude(prop.Name) {
			continue
		}

		// If the property has a const value, then don't show it as an input property.
		// Even though it is a valid property, it is used by the language code gen to
		// generate the appropriate defaults for it. These cannot be overridden by users.
		if prop.ConstValue != nil {
			continue
		}

		characteristics := propertyCharacteristics{input: input}

		langDocHelper := dctx.getLanguageDocHelper(lang)
		name, err := langDocHelper.GetPropertyName(prop)
		if err != nil {
			panic(err)
		}
		propLangName := name

		propID := idPrefix + strings.ToLower(propLangName+propertyLangSeparator+lang)

		propTypes := make([]propertyType, 0)
		if typ, isUnion := codegen.UnwrapType(prop.Type).(*schema.UnionType); isUnion {
			for _, elementType := range typ.ElementTypes {
				propTypes = append(propTypes, mod.typeString(elementType, lang, characteristics, true))
			}
		} else {
			propTypes = append(propTypes, mod.typeString(prop.Type, lang, characteristics, true))
		}

		comment := prop.Comment
		// Default values for Provider inputs correspond to environment variables, so add that info to the docs.
		if isProvider && input && prop.DefaultValue != nil && len(prop.DefaultValue.Environment) > 0 {
			var suffix string
			if len(prop.DefaultValue.Environment) > 1 {
				suffix = "s"
			}
			comment += fmt.Sprintf(" It can also be sourced from the following environment variable%s: ", suffix)
			for i, v := range prop.DefaultValue.Environment {
				comment += fmt.Sprintf("`%s`", v)
				if i != len(prop.DefaultValue.Environment)-1 {
					comment += ", "
				}
			}
		}

		docProperties = append(docProperties, property{
			ID:                 propID,
			DisplayName:        wbr(propLangName),
			Name:               propLangName,
			Comment:            comment,
			DeprecationMessage: prop.DeprecationMessage,
			IsRequired:         prop.IsRequired(),
			IsInput:            input,
			// We indicate that a property will replace if either
			// a) we will force the replace at the engine level
			// b) we are told that the provider will require a replace
			IsReplaceOnChanges: prop.ReplaceOnChanges || prop.WillReplaceOnChanges,
			Link:               "#" + propID,
			Types:              propTypes,
		})
	}

	// Sort required props to move them to the top of the properties list, then by name.
	sort.SliceStable(docProperties, func(i, j int) bool {
		pi, pj := docProperties[i], docProperties[j]
		switch {
		case pi.IsRequired != pj.IsRequired:
			return pi.IsRequired && !pj.IsRequired
		default:
			return pi.Name < pj.Name
		}
	})

	return docProperties
}

func getDockerImagePythonFormalParams() []formalParam {
	return []formalParam{
		{
			Name: "image_name",
		},
		{
			Name: "build",
		},
		{
			Name:         "local_image_name",
			DefaultValue: "=None",
		},
		{
			Name:         "registry",
			DefaultValue: "=None",
		},
		{
			Name:         "skip_push",
			DefaultValue: "=None",
		},
		{
			Name:         "opts",
			DefaultValue: "=None",
		},
	}
}

// Returns the rendered HTML for the resource's constructor, as well as the specific arguments.
func (mod *modContext) genConstructors(r *schema.Resource, allOptionalInputs bool) (map[string]string, map[string][]formalParam) {
	dctx := mod.docGenContext
	renderedParams := make(map[string]string)
	formalParams := make(map[string][]formalParam)

	// Add an extra language for Python's ResourceArg __init__ overload.
	langs := append(dctx.supportedLanguages, "pythonargs")
	// Add an extra language for Java's ResourceArg overload.
	langs = append(langs, "javaargs")

	for _, lang := range langs {
		var (
			paramTemplate string
			params        []formalParam
		)
		b := &bytes.Buffer{}

		paramSeparatorTemplate := "param_separator"
		ps := paramSeparator{}

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
		case "java":
			fallthrough
		case "javaargs":
			argsOverload := lang == "javaargs"
			params = mod.genConstructorJava(r, argsOverload)
			paramTemplate = "java_formal_param"
		case "python":
			fallthrough
		case "pythonargs":
			argsOverload := lang == "pythonargs"
			params = mod.genConstructorPython(r, allOptionalInputs, argsOverload)
			paramTemplate = "py_formal_param"
			paramSeparatorTemplate = "py_param_separator"
			ps = paramSeparator{Indent: strings.Repeat(" ", len("def (")+len(resourceName(r)))}
		case "yaml":
			params = mod.genConstructorYaml()
		}

		if paramTemplate != "" {
			for i, p := range params {
				if i != 0 {
					if err := dctx.templates.ExecuteTemplate(b, paramSeparatorTemplate, ps); err != nil {
						panic(err)
					}
				}
				if err := dctx.templates.ExecuteTemplate(b, paramTemplate, p); err != nil {
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
func (mod *modContext) getConstructorResourceInfo(resourceTypeName, tok string) map[string]propertyType {

	dctx := mod.docGenContext
	docLangHelper := dctx.getLanguageDocHelper("yaml")
	resourceMap := make(map[string]propertyType)
	resourceDisplayName := resourceTypeName

	for _, lang := range dctx.supportedLanguages {
		// Use the module to package lookup to transform the module name to its normalized package name.
		modName := mod.getLanguageModuleName(lang)
		// Reset the type name back to the display name.
		resourceTypeName = resourceDisplayName

		switch lang {
		case "nodejs", "go", "python", "java":
			// Intentionally left blank.
		case "csharp":
			namespace := title(mod.pkg.Name(), lang)
			if ns, ok := dctx.csharpPkgInfo.Namespaces[mod.pkg.Name()]; ok {
				namespace = ns
			}
			if mod.mod == "" {
				resourceTypeName = fmt.Sprintf("Pulumi.%s.%s", namespace, resourceTypeName)
				break
			}

			resourceTypeName = fmt.Sprintf("Pulumi.%s.%s.%s", namespace, modName, resourceTypeName)
		case "yaml":
			def, err := mod.pkg.Definition()
			contract.AssertNoError(err)
			resourceMap[lang] = propertyType{
				Name:        resourceTypeName,
				DisplayName: docLangHelper.GetLanguageTypeString(def, mod.mod, &schema.ResourceType{Token: tok}, false),
			}
			continue
		default:
			panic(fmt.Errorf("cannot generate constructor info for unhandled language %q", lang))
		}

		parts := strings.Split(resourceTypeName, ".")
		displayName := parts[len(parts)-1]

		resourceMap[lang] = propertyType{
			Name:        resourceDisplayName,
			DisplayName: displayName,
		}
	}

	return resourceMap
}

func (mod *modContext) getTSLookupParams(r *schema.Resource, stateParam string) []formalParam {
	dctx := mod.docGenContext
	docLangHelper := dctx.getLanguageDocHelper("nodejs")
	def, err := mod.pkg.Definition()
	contract.AssertNoError(err)

	return []formalParam{
		{
			Name: "name",

			Type: propertyType{
				Name: "string",
			},
		},
		{
			Name: "id",
			Type: propertyType{
				Name: "Input<ID>",
				Link: docLangHelper.GetDocLinkForPulumiType(def, "ID"),
			},
		},
		{
			Name:         "state",
			OptionalFlag: "?",
			Type: propertyType{
				Name: stateParam,
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "?",
			Type: propertyType{
				Name: "CustomResourceOptions",
				Link: docLangHelper.GetDocLinkForPulumiType(def, "CustomResourceOptions"),
			},
		},
	}
}

func (mod *modContext) getGoLookupParams(r *schema.Resource, stateParam string) []formalParam {
	dctx := mod.docGenContext
	docLangHelper := dctx.getLanguageDocHelper("go")

	def, err := mod.pkg.Definition()
	contract.AssertNoError(err)

	return []formalParam{
		{
			Name:         "ctx",
			OptionalFlag: "*",
			Type: propertyType{
				Name: "Context",
				Link: docLangHelper.GetDocLinkForPulumiType(def, "Context"),
			},
		},
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
			},
		},
		{
			Name: "id",
			Type: propertyType{
				Name: "IDInput",
				Link: docLangHelper.GetDocLinkForPulumiType(def, "IDInput"),
			},
		},
		{
			Name:         "state",
			OptionalFlag: "*",
			Type: propertyType{
				Name: stateParam,
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "...",
			Type: propertyType{
				Name: "ResourceOption",
				Link: docLangHelper.GetDocLinkForPulumiType(def, "ResourceOption"),
			},
		},
	}
}

func (mod *modContext) getCSLookupParams(r *schema.Resource, stateParam string) []formalParam {
	dctx := mod.docGenContext
	docLangHelper := dctx.getLanguageDocHelper("csharp")

	def, err := mod.pkg.Definition()
	contract.AssertNoError(err)

	return []formalParam{
		{
			Name: "name",
			Type: propertyType{
				Name: "string",
			},
		},
		{
			Name: "id",
			Type: propertyType{
				Name: "Input<string>",
				Link: docLangHelper.GetDocLinkForPulumiType(def, "Pulumi.Input"),
			},
		},
		{
			Name:         "state",
			OptionalFlag: "?",
			Type: propertyType{
				Name: stateParam,
			},
		},
		{
			Name:         "opts",
			OptionalFlag: "?",
			DefaultValue: " = null",
			Type: propertyType{
				Name: "CustomResourceOptions",
				Link: docLangHelper.GetDocLinkForPulumiType(def, "Pulumi.CustomResourceOptions"),
			},
		},
	}
}

func (mod *modContext) getJavaLookupParams(r *schema.Resource, stateParam string) []formalParam {
	dctx := mod.docGenContext
	docLangHelper := dctx.getLanguageDocHelper("java")
	def, err := mod.pkg.Definition()
	contract.AssertNoError(err)

	return []formalParam{
		{
			Name: "name",
			Type: propertyType{
				Name: "String",
			},
		},
		{
			Name: "id",
			Type: propertyType{
				Name: "Output<String>",
				Link: docLangHelper.GetDocLinkForPulumiType(def, "Output"),
			},
		},
		{
			Name: "state",
			Type: propertyType{
				Name: stateParam,
			},
		},
		{
			Name: "options",
			Type: propertyType{
				Name: "CustomResourceOptions",
				Link: docLangHelper.GetDocLinkForPulumiType(def, "CustomResourceOptions"),
			},
		},
	}
}

func (mod *modContext) getPythonLookupParams(r *schema.Resource, stateParam string) []formalParam {
	dctx := mod.docGenContext
	// The input properties for a resource needs to be exploded as
	// individual constructor params.
	docLanguageHelper := dctx.getLanguageDocHelper("python")
	params := make([]formalParam, 0, len(r.StateInputs.Properties))
	for _, p := range r.StateInputs.Properties {
		def, err := mod.pkg.Definition()
		contract.AssertNoError(err)

		typ := docLanguageHelper.GetLanguageTypeString(def, mod.mod, codegen.PlainType(codegen.OptionalType(p)), true /*input*/)
		params = append(params, formalParam{
			Name:         python.PyName(p.Name),
			DefaultValue: " = None",
			Type: propertyType{
				Name: typ,
			},
		})
	}
	return params
}

// genLookupParams generates a map of per-language way of rendering the formal parameters of the lookup function
// used to lookup an existing resource.
func (mod *modContext) genLookupParams(r *schema.Resource, stateParam string) map[string]string {
	dctx := mod.docGenContext
	lookupParams := make(map[string]string)
	if r.StateInputs == nil {
		return lookupParams
	}

	for _, lang := range dctx.supportedLanguages {
		var (
			paramTemplate string
			params        []formalParam
		)
		b := &bytes.Buffer{}

		paramSeparatorTemplate := "param_separator"
		ps := paramSeparator{}

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
		case "java":
			params = mod.getJavaLookupParams(r, stateParam)
			paramTemplate = "java_formal_param"
		case "python":
			params = mod.getPythonLookupParams(r, stateParam)
			paramTemplate = "py_formal_param"
			paramSeparatorTemplate = "py_param_separator"
			ps = paramSeparator{Indent: strings.Repeat(" ", len("def get("))}
		}

		n := len(params)
		for i, p := range params {
			if err := dctx.templates.ExecuteTemplate(b, paramTemplate, p); err != nil {
				panic(err)
			}
			if i != n-1 {
				if err := dctx.templates.ExecuteTemplate(b, paramSeparatorTemplate, ps); err != nil {
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

func (mod *modContext) genResourceHeader(r *schema.Resource) header {
	resourceName := resourceName(r)
	var metaDescription string
	var titleTag string
	if mod.mod == "" {
		metaDescription = fmt.Sprintf("Documentation for the %s.%s resource "+
			"with examples, input properties, output properties, "+
			"lookup functions, and supporting types.", mod.pkg.Name(), resourceName)
		titleTag = fmt.Sprintf("%s.%s", mod.pkg.Name(), resourceName)
	} else {
		metaDescription = fmt.Sprintf("Documentation for the %s.%s.%s resource "+
			"with examples, input properties, output properties, "+
			"lookup functions, and supporting types.", mod.pkg.Name(), mod.mod, resourceName)
		titleTag = fmt.Sprintf("%s.%s.%s", mod.pkg.Name(), mod.mod, resourceName)
	}

	return header{
		Title:    resourceName,
		TitleTag: titleTag,
		MetaDesc: metaDescription,
	}
}

// genResource is the entrypoint for generating a doc for a resource
// from its Pulumi schema.
func (mod *modContext) genResource(r *schema.Resource) resourceDocArgs {
	dctx := mod.docGenContext
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

	// All custom resources have an implicit `id` output property, that we must inject into the docs.
	if !r.IsComponent {
		filteredOutputProps = append(filteredOutputProps, &schema.Property{
			Name:    "id",
			Comment: "The provider-assigned unique ID for this managed resource.",
			Type:    schema.StringType,
		})
	}

	for _, lang := range dctx.supportedLanguages {
		inputProps[lang] = mod.getProperties(r.InputProperties, lang, true, false, r.IsProvider)
		outputProps[lang] = mod.getProperties(filteredOutputProps, lang, false, false, r.IsProvider)
		if r.IsProvider {
			continue
		}
		if r.StateInputs != nil {
			stateProps := mod.getProperties(r.StateInputs.Properties, lang, true, false, r.IsProvider)
			for i := 0; i < len(stateProps); i++ {
				id := "state_" + stateProps[i].ID
				stateProps[i].ID = id
				stateProps[i].Link = "#" + id
			}
			stateInputs[lang] = stateProps
		}
	}

	allOptionalInputs := true
	for _, prop := range r.InputProperties {
		// If at least one prop is required, then break.
		if prop.IsRequired() {
			allOptionalInputs = false
			break
		}
	}

	def, err := mod.pkg.Definition()
	contract.AssertNoError(err)
	packageDetails := packageDetails{
		DisplayName:    getPackageDisplayName(def.Name),
		Repository:     def.Repository,
		RepositoryName: getRepositoryName(def.Repository),
		License:        def.License,
		Notes:          def.Attribution,
	}

	renderedCtorParams, typedCtorParams := mod.genConstructors(r, allOptionalInputs)

	stateParam := name + "State"

	docInfo := dctx.decomposeDocstring(r.Comment)
	data := resourceDocArgs{
		Header: mod.genResourceHeader(r),

		Tool: mod.tool,

		Comment:            docInfo.description,
		DeprecationMessage: r.DeprecationMessage,
		ExamplesSection:    docInfo.examples,
		ImportDocs:         docInfo.importDetails,

		ConstructorParams:      renderedCtorParams,
		ConstructorParamsTyped: typedCtorParams,

		ConstructorResource: mod.getConstructorResourceInfo(name, r.Token),
		ArgsRequired:        !allOptionalInputs,

		InputProperties:  inputProps,
		OutputProperties: outputProps,
		LookupParams:     mod.genLookupParams(r, stateParam),
		StateInputs:      stateInputs,
		StateParam:       stateParam,
		NestedTypes:      mod.genNestedTypes(r, true /*resourceType*/),

		Methods: mod.genMethods(r),

		PackageDetails: packageDetails,
	}

	return data
}

func (mod *modContext) getNestedTypes(t schema.Type, types nestedTypeUsageInfo, input bool) {
	switch t := t.(type) {
	case *schema.InputType:
		mod.getNestedTypes(t.ElementType, types, input)
	case *schema.OptionalType:
		mod.getNestedTypes(t.ElementType, types, input)
	case *schema.ArrayType:
		mod.getNestedTypes(t.ElementType, types, input)
	case *schema.MapType:
		mod.getNestedTypes(t.ElementType, types, input)
	case *schema.ObjectType:
		if types.contains(t.Token, input) {
			break
		}

		types.add(t.Token, input)
		for _, p := range t.Properties {
			mod.getNestedTypes(p.Type, types, input)
		}
	case *schema.EnumType:
		types.add(t.Token, false)
	case *schema.UnionType:
		for _, e := range t.ElementTypes {
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
		for _, m := range t.Methods {
			mod.getTypes(m.Function, types)
		}
	case *schema.Function:
		if t.Inputs != nil && !t.MultiArgumentInputs {
			mod.getNestedTypes(t.Inputs, types, true)
		}

		if t.ReturnType != nil {
			if objectType, ok := t.ReturnType.(*schema.ObjectType); ok && objectType != nil {
				mod.getNestedTypes(objectType, types, false)
			}
		}
	}
}

// getModuleFileName returns the file name to use for a module.
func (mod *modContext) getModuleFileName() string {
	dctx := mod.docGenContext
	if !isKubernetesPackage(mod.pkg) {
		return mod.mod
	}

	// For k8s packages, use the Go-language info to get the file name
	// for the module.
	if override, ok := dctx.goPkgInfo.ModuleToPackage[mod.mod]; ok {
		return override
	}
	return mod.mod
}

func (mod *modContext) gen(fs codegen.Fs) error {
	dctx := mod.docGenContext
	modName := mod.getModuleFileName()

	addFile := func(name, contents string) {
		p := path.Join(modName, name, "_index.md")
		fs.Add(p, []byte(contents))
	}

	// Resources
	for _, r := range mod.resources {
		data := mod.genResource(r)

		title := resourceName(r)
		buffer := &bytes.Buffer{}

		err := dctx.templates.ExecuteTemplate(buffer, "resource.tmpl", data)
		if err != nil {
			return err
		}

		resourceFileName := strings.ToLower(title)
		// Handle file generation for resources named `index`. We prepend a double underscore
		// here, since this ends up resulting in route of .../<module>/index which has trouble
		// resolving and returns a 404 in the browser, likely due to `index` being some sort
		// of reserved keyword.
		if resourceFileName == "index" {
			resourceFileName = "--index"
		}

		addFile(resourceFileName, buffer.String())
	}

	// Functions
	for _, f := range mod.functions {
		data := mod.genFunction(f)

		buffer := &bytes.Buffer{}
		err := dctx.templates.ExecuteTemplate(buffer, "function.tmpl", data)
		if err != nil {
			return err
		}

		addFile(strings.ToLower(tokenToName(f.Token)), buffer.String())
	}

	// Generate the index files.
	idxData := mod.genIndex()
	buffer := &bytes.Buffer{}
	err := dctx.templates.ExecuteTemplate(buffer, "index.tmpl", idxData)
	if err != nil {
		return err
	}

	fs.Add(path.Join(modName, "_index.md"), buffer.Bytes())
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
	TitleTag           string
	PackageDescription string

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

	def, err := mod.pkg.Definition()
	contract.AssertNoError(err)

	// An empty string indicates that this is the root module.
	if title == "" {
		if def.DisplayName != "" {
			title = def.DisplayName
		} else {
			title = getPackageDisplayName(mod.pkg.Name())
		}
	}

	// If there are submodules, list them.
	for _, mod := range mod.children {
		modName := mod.getModuleFileName()
		displayName := modFilenameToDisplayName(modName)
		modules = append(modules, indexEntry{
			Link:        getModuleLink(displayName),
			DisplayName: displayName,
		})
	}
	sortIndexEntries(modules)

	// If there are resources in the root, list them.
	for _, r := range mod.resources {
		name := resourceName(r)
		resources = append(resources, indexEntry{
			Link:        getResourceLink(name) + "/",
			DisplayName: name,
		})
	}
	sortIndexEntries(resources)

	// If there are functions in the root, list them.
	for _, f := range mod.functions {
		name := tokenToName(f.Token)
		functions = append(functions, indexEntry{
			Link:        getFunctionLink(name) + "/",
			DisplayName: strings.Title(name),
		})
	}
	sortIndexEntries(functions)

	version := ""
	if mod.pkg.Version() != nil {
		version = mod.pkg.Version().String()
	}

	packageDetails := packageDetails{
		DisplayName:    getPackageDisplayName(def.Name),
		Repository:     def.Repository,
		RepositoryName: getRepositoryName(def.Repository),
		License:        def.License,
		Notes:          def.Attribution,
		Version:        version,
	}

	var titleTag string
	var packageDescription string
	// The same index.tmpl template is used for both top level package and module pages, if modules not present,
	// assume top level package index page when formatting title tags otherwise, if contains modules, assume modules
	// top level page when generating title tags.
	if len(modules) > 0 {
		titleTag = fmt.Sprintf("%s Package", getPackageDisplayName(title))
	} else {
		titleTag = fmt.Sprintf("%s.%s", mod.pkg.Name(), title)
		packageDescription = fmt.Sprintf("Explore the resources and functions of the %s.%s module.",
			mod.pkg.Name(), title)
	}

	data := indexData{
		Tool:               mod.tool,
		PackageDescription: packageDescription,
		Title:              title,
		TitleTag:           titleTag,
		Resources:          resources,
		Functions:          functions,
		Modules:            modules,
		PackageDetails:     packageDetails,
	}

	// If this is the root module, write out the package description.
	if mod.mod == "" {
		data.PackageDescription = mod.pkg.Description()
	}

	return data
}

// getPackageDisplayName uses the title lookup map to look for a
// display name for the given title.
func getPackageDisplayName(title string) string {
	// If title not found in titleLookup map, default back to title given.
	if val, ok := titleLookup(title); ok {
		return val
	}
	return title
}

// getRepositoryName returns the repository name based on the repository's URL.
func getRepositoryName(repoURL string) string {
	return strings.TrimPrefix(repoURL, "https://github.com/")
}

func (dctx *docGenContext) getMod(
	pkg schema.PackageReference,
	token string,
	tokenPkg schema.PackageReference,
	modules map[string]*modContext,
	tool string,
	add bool) *modContext {

	modName := pkg.TokenToModule(token)
	mod, ok := modules[modName]
	if !ok {
		mod = &modContext{
			pkg:           pkg,
			mod:           modName,
			tool:          tool,
			docGenContext: dctx,
		}

		if modName != "" && codegen.PkgEquals(tokenPkg, pkg) {
			parentName := path.Dir(modName)
			// If the parent name is blank, it means this is the package-level.
			if parentName == "." || parentName == "" {
				parentName = ":index:"
			} else {
				parentName = ":" + parentName + ":"
			}
			parent := dctx.getMod(pkg, parentName, tokenPkg, modules, tool, add)
			if add {
				parent.children = append(parent.children, mod)
			}
		}

		// Save the module only if we're adding and it's for the current package.
		// This way, modules for external packages are not saved.
		if add && tokenPkg == pkg {
			modules[modName] = mod
		}
	}
	return mod
}

func (dctx *docGenContext) generateModulesFromSchemaPackage(tool string, pkg *schema.Package) map[string]*modContext {
	// Group resources, types, and functions into modules.
	modules := map[string]*modContext{}

	// Decode language-specific info.
	if err := pkg.ImportLanguages(map[string]schema.Language{
		"go":     go_gen.Importer,
		"python": python.Importer,
		"csharp": dotnet.Importer,
		"nodejs": nodejs.Importer,
	}); err != nil {
		panic(err)
	}
	dctx.goPkgInfo, _ = pkg.Language["go"].(go_gen.GoPackageInfo)
	dctx.csharpPkgInfo, _ = pkg.Language["csharp"].(dotnet.CSharpPackageInfo)
	dctx.nodePkgInfo, _ = pkg.Language["nodejs"].(nodejs.NodePackageInfo)
	dctx.pythonPkgInfo, _ = pkg.Language["python"].(python.PackageInfo)

	goLangHelper := dctx.getLanguageDocHelper("go").(*go_gen.DocLanguageHelper)
	// Generate the Go package map info now, so we can use that to get the type string
	// names later.
	goLangHelper.GeneratePackagesMap(pkg, tool, dctx.goPkgInfo)

	csharpLangHelper := dctx.getLanguageDocHelper("csharp").(*dotnet.DocLanguageHelper)
	csharpLangHelper.Namespaces = dctx.csharpPkgInfo.Namespaces

	visitObjects := func(r *schema.Resource) {
		visitObjectTypes(r.InputProperties, func(t schema.Type) {
			switch T := t.(type) {
			case *schema.ObjectType:
				dctx.getMod(pkg.Reference(), T.Token, T.PackageReference, modules, tool, true).details(T).inputType = true
			}
		})
		if r.StateInputs != nil {
			visitObjectTypes(r.StateInputs.Properties, func(t schema.Type) {
				switch T := t.(type) {
				case *schema.ObjectType:
					dctx.getMod(pkg.Reference(), T.Token, T.PackageReference, modules, tool, true).details(T).inputType = true
				}
			})
		}
	}

	scanResource := func(r *schema.Resource) {
		mod := dctx.getMod(pkg.Reference(), r.Token, r.PackageReference, modules, tool, true)
		mod.resources = append(mod.resources, r)
		visitObjects(r)
	}

	scanK8SResource := func(r *schema.Resource) {
		mod := getKubernetesMod(pkg, r.Token, modules, tool)
		mod.resources = append(mod.resources, r)
		visitObjects(r)
	}

	glog.V(3).Infoln("scanning resources")
	if isKubernetesPackage(pkg.Reference()) {
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
		if !f.IsMethod {
			mod := dctx.getMod(pkg.Reference(), f.Token, f.PackageReference, modules, tool, true)
			mod.functions = append(mod.functions, f)
		}
	}

	// Find nested types.
	for _, t := range pkg.Types {
		switch typ := t.(type) {
		case *schema.ObjectType:
			mod := dctx.getMod(pkg.Reference(), typ.Token, typ.PackageReference, modules, tool, false)
			if mod.details(typ).inputType {
				mod.inputTypes = append(mod.inputTypes, typ)
			}
		}
	}

	return modules
}

func (dctx *docGenContext) initialize(tool string, pkg *schema.Package) {
	dctx.templates = template.New("").Funcs(template.FuncMap{
		"htmlSafe": func(html string) template.HTML {
			// Markdown fragments in the templates need to be rendered as-is,
			// so that html/template package doesn't try to inject data into it,
			// which will most certainly fail.
			//nolint:gosec
			return template.HTML(html)
		},
		"markdownify": func(html string) template.HTML {
			// Convert a string of Markdown into HTML.
			var buf bytes.Buffer
			if err := goldmark.Convert([]byte(html), &buf); err != nil {
				glog.Fatalf("rendering Markdown: %v", err)
			}
			//nolint:gosec
			return template.HTML(buf.String())
		},
	})

	defer glog.Flush()

	if _, err := dctx.templates.ParseFS(packagedTemplates, "templates/*.tmpl"); err != nil {
		glog.Fatalf("initializing templates: %v", err)
	}

	// Generate the modules from the schema, and for every module
	// run the generator functions to generate markdown files.
	dctx.setModules(dctx.generateModulesFromSchemaPackage(tool, pkg))
}

func (dctx *docGenContext) generatePackage(tool string, pkg *schema.Package) (map[string][]byte, error) {
	if dctx.modules() == nil {
		return nil, errors.New("must call Initialize before generating the docs package")
	}

	defer glog.Flush()

	glog.V(3).Infoln("generating package docs now...")
	files := codegen.Fs{}
	modules := []string{}
	modMap := dctx.modules()
	for k := range modMap {
		modules = append(modules, k)
	}
	sort.Strings(modules)
	for _, mod := range modules {
		if err := modMap[mod].gen(files); err != nil {
			return nil, err
		}
	}

	return files, nil
}

// GeneratePackageTree returns a navigable structure starting from the top-most module.
func (dctx *docGenContext) generatePackageTree() ([]PackageTreeItem, error) {
	if dctx.modules() == nil {
		return nil, errors.New("must call Initialize before generating the docs package")
	}

	defer glog.Flush()

	var packageTree []PackageTreeItem
	// "" indicates the top-most module.
	if rootMod, ok := dctx.modules()[""]; ok {
		tree, err := generatePackageTree(*rootMod)
		if err != nil {
			glog.Errorf("Error generating the package tree for package: %v", err)
		}

		packageTree = tree
	} else {
		glog.Error("A root module entry was not found for the package. Cannot generate the package tree...")
	}

	return packageTree, nil
}

func visitObjectTypes(properties []*schema.Property, visitor func(t schema.Type)) {
	codegen.VisitTypeClosure(properties, func(t schema.Type) {
		switch st := t.(type) {
		case *schema.EnumType, *schema.ObjectType, *schema.ResourceType:
			visitor(st)
		}
	})
}

// Export a default static context so as not to break external
// consumers of this API; prefer *WithContext API internally to ensure
// tests can run in parallel.
var defaultContext = newDocGenContext()

func Initialize(tool string, pkg *schema.Package) {
	defaultContext.initialize(tool, pkg)
}

// GeneratePackage generates docs for each resource given the Pulumi
// schema. The returned map contains the filename with path as the key
// and the contents as its value.
func GeneratePackage(tool string, pkg *schema.Package) (map[string][]byte, error) {
	return defaultContext.generatePackage(tool, pkg)
}

// GeneratePackageTree returns a navigable structure starting from the top-most module.
func GeneratePackageTree() ([]PackageTreeItem, error) {
	return defaultContext.generatePackageTree()
}
