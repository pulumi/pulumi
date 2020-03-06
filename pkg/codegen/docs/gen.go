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
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"unicode"

	"github.com/pulumi/pulumi/pkg/codegen/python"
	"github.com/pulumi/pulumi/pkg/codegen/schema"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type stringSet map[string]struct{}

func (ss stringSet) add(s string) {
	ss[s] = struct{}{}
}

type typeDetails struct {
	outputType   bool
	inputType    bool
	functionType bool
}

// wbr inserts HTML <wbr> in between case changes, e.g. "fooBar" becomes "foo<wbr>Bar".
func wbr(s string) string {
	var runes []rune
	var prev rune
	for i, r := range s {
		if i != 0 && unicode.IsLower(prev) && unicode.IsUpper(r) {
			runes = append(runes, []rune("<wbr>")...)
		}
		runes = append(runes, r)
		prev = r
	}
	return string(runes)
}

func title(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	return string(append([]rune{unicode.ToUpper(runes[0])}, runes[1:]...))
}

func lower(s string) string {
	return strings.ToLower(s)
}

type modContext struct {
	pkg         *schema.Package
	mod         string
	resources   []*schema.Resource
	functions   []*schema.Function
	typeDetails map[*schema.ObjectType]*typeDetails
	children    []*modContext
	tool        string
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

func tokenToName(tok string) string {
	components := strings.Split(tok, ":")
	contract.Assertf(len(components) == 3, "malformed token %v", tok)
	return title(components[2])
}

func resourceName(r *schema.Resource) string {
	if r.IsProvider {
		return "Provider"
	}
	return tokenToName(r.Token)
}

func (mod *modContext) typeStringPulumi(t schema.Type, link bool) string {
	var br string
	lt := "<"
	gt := ">"

	// If we're linking, we're including HTML, so also include word breaks,
	// and escape < and >.
	if link {
		br = "<wbr>"
		lt = "&lt;<wbr>"
		gt = "<wbr>&gt;"
	}

	var typ string
	switch t := t.(type) {
	case *schema.ArrayType:

		typ = fmt.Sprintf("Array%s%s%s", lt, mod.typeStringPulumi(t.ElementType, link), gt)
	case *schema.MapType:
		typ = fmt.Sprintf("Map%s%s%s", lt, mod.typeStringPulumi(t.ElementType, link), gt)
	case *schema.ObjectType:
		if link {
			typ = fmt.Sprintf("<a href=\"#%s\">%s</a>", lower(tokenToName(t.Token)), wbr(tokenToName(t.Token)))
		} else {
			typ = tokenToName(t.Token)
		}
	case *schema.TokenType:
		typ = tokenToName(t.Token)
	case *schema.UnionType:
		var elements []string
		for _, e := range t.ElementTypes {
			elements = append(elements, mod.typeStringPulumi(e, link))
		}
		sep := fmt.Sprintf(", %s", br)
		return fmt.Sprintf("Union%s%s%s", lt, strings.Join(elements, sep), gt)
	default:
		switch t {
		case schema.BoolType:
			typ = "boolean"
		case schema.IntType, schema.NumberType:
			typ = "number"
		case schema.StringType:
			typ = "string"
		case schema.ArchiveType:
			typ = "Archive"
		case schema.AssetType:
			typ = fmt.Sprintf("Union%sAsset, %sArchive%s", lt, br, gt)
		case schema.AnyType:
			typ = "any"
		}
	}
	return typ
}

func (mod *modContext) genConstructorTS(w io.Writer, r *schema.Resource) {
	name := resourceName(r)

	allOptionalInputs := true
	for _, prop := range r.InputProperties {
		allOptionalInputs = allOptionalInputs && !prop.IsRequired
	}

	var argsFlags string
	if allOptionalInputs {
		// If the number of required input properties was zero, we can make the args object optional.
		argsFlags = "?"
	}
	argsType := name + "Args"

	// TODO: The link to the class name and args type needs to factor in the package and module. Right now it's hardcoded to aws and s3.
	fmt.Fprintf(w, "<span class=\"k\">new</span> <span class=\"nx\"><a href=/docs/reference/pkg/nodejs/pulumi/aws/s3/#%s>%s</a></span><span class=\"p\">(</span><span class=\"nx\">name</span>: <span class=\"kt\"><a href=https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/String>string</a></span><span class=\"p\">,</span> <span class=\"nx\">args%s</span>: <span class=\"kt\"><a href=/docs/reference/pkg/nodejs/pulumi/aws/s3/#%s>%s</a></span><span class=\"p\">,</span> <span class=\"nx\">opts?</span>: <span class=\"kt\"><a href=/docs/reference/pkg/nodejs/pulumi/pulumi/#CustomResourceOptions>pulumi.CustomResourceOptions</a></span><span class=\"p\">);</span>", name, name, argsFlags, argsType, argsType)
}

func (mod *modContext) genConstructorPython(w io.Writer, r *schema.Resource) {
	fmt.Fprintf(w, "def __init__(__self__, resource_name, opts=None")
	for _, prop := range r.InputProperties {
		fmt.Fprintf(w, ", %s=None", python.PyName(prop.Name))
	}
	// Note: We're excluding __name__ and __opts__ as those are only there for backwards compatibility and are
	// deliberately not included in doc strings.
	fmt.Fprintf(w, ", __props__=None)")
}

func (mod *modContext) genConstructorGo(w io.Writer, r *schema.Resource) {
	name := resourceName(r)
	argsType := name + "Args"
	fmt.Fprintf(w, "func New%s(ctx *pulumi.Context, name string, args *%s, opts ...pulumi.ResourceOption) (*%s, error)\n", name, argsType, name)
}

func (mod *modContext) genConstructorCS(w io.Writer, r *schema.Resource) {
	name := resourceName(r)
	argsType := name + "Args"

	var argsDefault string
	allOptionalInputs := true
	for _, prop := range r.InputProperties {
		allOptionalInputs = allOptionalInputs && !prop.IsRequired
	}
	if allOptionalInputs {
		// If the number of required input properties was zero, we can make the args object optional.
		argsDefault = " = null"
		argsType += "?"
	}

	optionsType := "CustomResourceOptions"
	if r.IsProvider {
		optionsType = "ResourceOptions"
	}

	fmt.Fprintf(w, "public %s(string name, %s args%s, %s? options = null)\n", name, argsType, argsDefault, optionsType)
}

func (mod *modContext) genProperties(w io.Writer, properties []*schema.Property, input bool) {
	if len(properties) == 0 {
		return
	}

	fmt.Fprintf(w, "<table class=\"ml-6\">\n")
	fmt.Fprintf(w, "    <thead>\n")
	fmt.Fprintf(w, "        <tr>\n")
	fmt.Fprintf(w, "            <th>Argument</th>\n")
	fmt.Fprintf(w, "            <th>Type</th>\n")
	fmt.Fprintf(w, "            <th>Description</th>\n")
	fmt.Fprintf(w, "        </tr>\n")
	fmt.Fprintf(w, "    </thead>\n")
	fmt.Fprintf(w, "    <tbody>\n")

	for _, prop := range properties {
		var required string
		if input {
			required = "(Optional) "
			if prop.IsRequired {
				required = "(Required) "
			}
		}

		// The comment contains markdown, so we must wrap it in our `{{% md %}}`` shortcode, which enables markdown
		// to be rendered inside HTML tags (otherwise, Hugo's markdown renderer won't render it as markdown).
		// Unfortunately, this injects an extra `<p>...</p>` around the rendered markdown content, which adds some margin
		// to the top and bottom of the content which we don't want. So we inject some styles to remove the margins from
		// those `p` tags.
		fmt.Fprintf(w, "        <tr>\n")
		fmt.Fprintf(w, "            <td class=\"align-top\">%s</td>\n", wbr(prop.Name))
		fmt.Fprintf(w, "            <td class=\"align-top\"><code>%s</code></td>\n", mod.typeStringPulumi(prop.Type, true))
		fmt.Fprintf(w, "            <td class=\"align-top\">{{%% md %%}}\n%s%s\n{{%% /md %%}}</td>\n", required, prop.Comment)
		fmt.Fprintf(w, "        </tr>\n")
	}

	fmt.Fprintf(w, "    </tbody>\n")
	fmt.Fprintf(w, "</table>\n\n")
}

func (mod *modContext) genNestedTypes(w io.Writer, properties []*schema.Property, input bool) {
	tokens := stringSet{}
	mod.getTypes(properties, tokens)
	var objs []*schema.ObjectType
	for token := range tokens {
		for _, t := range mod.pkg.Types {
			if obj, ok := t.(*schema.ObjectType); ok && obj.Token == token {
				objs = append(objs, obj)
			}
		}
	}
	sort.Slice(objs, func(i, j int) bool {
		return tokenToName(objs[i].Token) < tokenToName(objs[j].Token)
	})
	for _, obj := range objs {
		fmt.Fprintf(w, "#### %s\n\n", tokenToName(obj.Token))

		mod.genProperties(w, obj.Properties, input)
	}
}

func (mod *modContext) genGet(w io.Writer, r *schema.Resource) {
	name := resourceName(r)

	stateType := name + "State"

	var stateParam string
	if r.StateInputs != nil {
		stateParam = fmt.Sprintf("state?: %s, ", stateType)
	}

	fmt.Fprintf(w, "{{< langchoose csharp >}}\n\n")

	fmt.Fprintf(w, "```typescript\n")
	fmt.Fprintf(w, "public static get(name: string, id: pulumi.Input<pulumi.ID>, %sopts?: pulumi.CustomResourceOptions): %s;\n", stateParam, name)
	fmt.Fprintf(w, "```\n\n")

	// TODO: This is currently hard coded for Bucket. Need to generalize for all resources.
	fmt.Fprintf(w, "```python\n")
	fmt.Fprintf(w, "def get(resource_name, id, opts=None, acceleration_status=None, acl=None, arn=None, bucket=None, bucket_domain_name=None, bucket_prefix=None, bucket_regional_domain_name=None, cors_rules=None, force_destroy=None, hosted_zone_id=None, lifecycle_rules=None, loggings=None, object_lock_configuration=None, policy=None, region=None, replication_configuration=None, request_payer=None, server_side_encryption_configuration=None, tags=None, versioning=None, website=None, website_domain=None, website_endpoint=None)\n")
	fmt.Fprintf(w, "```\n\n")

	// TODO: This is currently hard coded for Bucket. Need to generalize for all resources.
	fmt.Fprintf(w, "```go\n")
	fmt.Fprintf(w, "func GetBucket(ctx *pulumi.Context, name string, id pulumi.IDInput, state *BucketState, opts ...pulumi.ResourceOption) (*Bucket, error)\n")
	fmt.Fprintf(w, "```\n\n")

	// TODO: This is currently hard coded for Bucket. Need to generalize for all resources.
	fmt.Fprintf(w, "```csharp\n")
	fmt.Fprintf(w, "public static Bucket Get(string name, Input<string> id, BucketState? state = null, CustomResourceOptions? options = null);\n")
	fmt.Fprintf(w, "```\n\n")

	fmt.Fprintf(w, "Get an existing %s resource's state with the given name, ID, and optional extra\n", name)
	fmt.Fprintf(w, "properties used to qualify the lookup.\n\n")

	for _, lang := range []string{"nodejs", "go", "csharp"} {
		fmt.Fprintf(w, "{{%% lang %s %%}}\n", lang)
		fmt.Fprintf(w, "<ul class=\"pl-10\">\n")
		fmt.Fprintf(w, "    <li><strong>name</strong> &ndash; (Required) The unique name of the resulting resource.</li>\n")
		fmt.Fprintf(w, "    <li><strong>id</strong> &ndash; (Required) The _unique_ provider ID of the resource to lookup.</li>\n")
		if stateParam != "" {
			fmt.Fprintf(w, "    <li><strong>state</strong> &ndash; (Optional) Any extra arguments used during the lookup.</li>\n")
		}
		fmt.Fprintf(w, "    <li><strong>opts</strong> &ndash; (Optional) A bag of options that control this resource's behavior.</li>\n")
		fmt.Fprintf(w, "</ul>\n")
		fmt.Fprintf(w, "{{%% /lang %%}}\n\n")
	}

	// TODO: Unlike the other languages, Python does not have a separate state object. The state args are all just
	// named parameters of the get function. Consider injecting `resource_name`, `id`, and `opts` as the first three
	// items in the table of state input properties.

	if r.StateInputs != nil {
		fmt.Fprintf(w, "The following state arguments are supported:\n\n")

		mod.genProperties(w, r.StateInputs.Properties, true)
	}
}

func (mod *modContext) genResource(w io.Writer, r *schema.Resource) {
	// Create a resource module file into which all of this resource's types will go.
	name := resourceName(r)

	fmt.Fprintf(w, "%s\n\n", r.Comment)

	// TODO: Remove this - it's just temporary to include some data we don't have available yet.
	mod.genMockupExamples(w, r)

	fmt.Fprintf(w, "## Create a %s Resource\n\n", name)

	// TODO: In the examples on the page, we only want to show TypeScript and Python tabs for now, as initially
	// we'll only have examples in those languages.
	// However, lower on the page, we will be showing declarations and types in all of the supported languages.
	// The default behavior of the lang chooser is to switch all lang tabs on the page when a tab is selected.
	// This means, if Go is selected lower in the page, then the chooser tabs for the examples will try to show
	// Go content, which won't be present. We should fix this somehow such that selecting Go lower in the page
	// doesn't cause the example tabs to change. But if Python is selected, the example tabs should change since
	// Python is available there.
	fmt.Fprintf(w, "{{< langchoose csharp >}}\n\n")

	fmt.Fprintf(w, "<div class=\"highlight\"><pre class=\"chroma\"><code class=\"language-typescript\" data-lang=\"typescript\">")
	mod.genConstructorTS(w, r)
	fmt.Fprintf(w, "</code></pre></div>\n\n")

	fmt.Fprintf(w, "```python\n")
	mod.genConstructorPython(w, r)
	fmt.Fprintf(w, "\n```\n\n")

	fmt.Fprintf(w, "```go\n")
	mod.genConstructorGo(w, r)
	fmt.Fprintf(w, "\n```\n\n")

	fmt.Fprintf(w, "```csharp\n")
	mod.genConstructorCS(w, r)
	fmt.Fprintf(w, "\n```\n\n")

	fmt.Fprintf(w, "Creates a %s resource with the given unique name, arguments, and options.\n\n", name)

	allOptionalInputs := true
	for _, prop := range r.InputProperties {
		allOptionalInputs = allOptionalInputs && !prop.IsRequired
	}

	argsRequired := "Required"
	if allOptionalInputs {
		argsRequired = "Optional"
	}

	for _, lang := range []string{"nodejs", "go", "csharp"} {
		fmt.Fprintf(w, "{{%% lang %s %%}}\n", lang)
		fmt.Fprintf(w, "<ul class=\"pl-10\">\n")
		fmt.Fprintf(w, "    <li><strong>name</strong> &ndash; (Required) The unique name of the resulting resource.</li>\n")
		fmt.Fprintf(w, "    <li><strong>args</strong> &ndash; (%s) The arguments to use to populate this resource's properties.</li>\n", argsRequired)
		fmt.Fprintf(w, "    <li><strong>opts</strong> &ndash; (Optional) A bag of options that control this resource's behavior.</li>\n")
		fmt.Fprintf(w, "</ul>\n")
		fmt.Fprintf(w, "{{%% /lang %%}}\n\n")
	}

	fmt.Fprintf(w, "The following arguments are supported:\n\n")

	// TODO: Unlike the other languages, Python does not have a separate Args object. The args are all just
	// named parameters of the constructor. Consider injecting `resource_name` and `opts` as the first two items
	// in the table of properties.

	mod.genProperties(w, r.InputProperties, true)

	fmt.Fprintf(w, "## %s Output Properties\n\n", name)

	fmt.Fprintf(w, "The following output properties are available:\n\n")

	mod.genProperties(w, r.Properties, false)

	fmt.Fprintf(w, "## Look up an Existing %s Resource\n\n", name)

	mod.genGet(w, r)

	fmt.Fprintf(w, "## Import an Existing %s Resource\n\n", name)

	// TODO: How do we want to show import? It will take a paragraph or two of explanation plus example, similar
	// to the content at https://www.pulumi.com/docs/intro/concepts/programming-model/#import
	fmt.Fprintf(w, "TODO\n\n")

	fmt.Fprintf(w, "## Support Types\n\n")

	mod.genNestedTypes(w, r.InputProperties, true)
}

func (mod *modContext) genFunction(w io.Writer, fun *schema.Function) {
	fmt.Fprintf(w, "%s\n\n", fun.Comment)

	// TODO: Emit the page for functions, similar to the page for resources.
	fmt.Fprintf(w, "TODO\n\n")
}

func visitObjectTypes(t schema.Type, visitor func(*schema.ObjectType)) {
	switch t := t.(type) {
	case *schema.ArrayType:
		visitObjectTypes(t.ElementType, visitor)
	case *schema.MapType:
		visitObjectTypes(t.ElementType, visitor)
	case *schema.ObjectType:
		for _, p := range t.Properties {
			visitObjectTypes(p.Type, visitor)
		}
		visitor(t)
	case *schema.UnionType:
		for _, e := range t.ElementTypes {
			visitObjectTypes(e, visitor)
		}
	}
}

func (mod *modContext) getNestedTypes(t schema.Type, types stringSet) {
	switch t := t.(type) {
	case *schema.ArrayType:
		mod.getNestedTypes(t.ElementType, types)
	case *schema.MapType:
		mod.getNestedTypes(t.ElementType, types)
	case *schema.ObjectType:
		types.add(t.Token)
		mod.getTypes(t.Properties, types)
	case *schema.UnionType:
		for _, e := range t.ElementTypes {
			mod.getNestedTypes(e, types)
		}
	}
}

func (mod *modContext) getTypes(member interface{}, types stringSet) {
	switch member := member.(type) {
	case *schema.ObjectType:
		for _, p := range member.Properties {
			mod.getNestedTypes(p.Type, types)
		}
	case *schema.Resource:
		for _, p := range member.Properties {
			mod.getNestedTypes(p.Type, types)
		}
		for _, p := range member.InputProperties {
			mod.getNestedTypes(p.Type, types)
		}
	case *schema.Function:
		if member.Inputs != nil {
			mod.getNestedTypes(member.Inputs, types)
		}
		if member.Outputs != nil {
			mod.getNestedTypes(member.Outputs, types)
		}
	case []*schema.Property:
		for _, p := range member {
			mod.getNestedTypes(p.Type, types)
		}
	}
}

func (mod *modContext) genHeader(w io.Writer, title string) {
	// TODO: Generate the actual front matter we want for these pages.
	// Example:
	// title: "Package @pulumi/aws"
	// title_tag: "Package @pulumi/aws | Node.js SDK"
	// linktitle: "@pulumi/aws"
	// meta_desc: "Explore members of the @pulumi/aws package."

	fmt.Fprintf(w, "---\n")
	fmt.Fprintf(w, "title: %q\n", title)
	fmt.Fprintf(w, "---\n\n")

	fmt.Fprintf(w, "<!-- WARNING: this file was generated by %v. -->\n", mod.tool)
	fmt.Fprintf(w, "<!-- Do not edit by hand unless you're certain you know what you are doing! -->\n\n")

	// TODO: Move styles into a .scss file in the docs repo instead of emitting it inline here.
	// Note: In general, we should prefer using TailwindCSS classes whenever possible.
	// These styles are only for elements that we can't easily add a class to.
	fmt.Fprintf(w, "<style>\n")
	fmt.Fprintf(w, "  table td p { margin-top: 0; margin-bottom: 0; }\n")
	fmt.Fprintf(w, "</style>\n\n")
}

type fs map[string][]byte

func (fs fs) add(path string, contents []byte) {
	_, has := fs[path]
	contract.Assertf(!has, "duplicate file: %s", path)
	fs[path] = contents
}

func (mod *modContext) gen(fs fs) error {
	var files []string
	for p := range fs {
		d := path.Dir(p)
		if d == "." {
			d = ""
		}
		if d == mod.mod {
			files = append(files, p)
		}
	}

	addFile := func(name, contents string) {
		p := path.Join(mod.mod, name)
		files = append(files, p)
		fs.add(p, []byte(contents))
	}

	// Resources
	for _, r := range mod.resources {
		buffer := &bytes.Buffer{}
		mod.genHeader(buffer, resourceName(r))

		mod.genResource(buffer, r)

		addFile(lower(resourceName(r))+".md", buffer.String())
	}

	// Functions
	for _, f := range mod.functions {
		buffer := &bytes.Buffer{}
		mod.genHeader(buffer, tokenToName(f.Token))

		mod.genFunction(buffer, f)

		addFile(lower(tokenToName(f.Token))+".md", buffer.String())
	}

	// Index
	fs.add(path.Join(mod.mod, "_index.md"), []byte(mod.genIndex(files)))
	return nil
}

// genIndex emits an _index.md file for the module.
func (mod *modContext) genIndex(exports []string) string {
	w := &bytes.Buffer{}

	name := mod.mod
	if name == "" {
		name = mod.pkg.Name
	}

	mod.genHeader(w, name)

	// If this is the root module, write out the package description.
	if mod.mod == "" {
		description := mod.pkg.Description
		if description != "" {
			description += "\n\n"
		}
		fmt.Fprint(w, description)
	}

	// If there are submodules, list them.
	var children []string
	for _, mod := range mod.children {
		children = append(children, mod.mod)
	}
	if len(children) > 0 {
		sort.Strings(children)
		fmt.Fprintf(w, "<h3>Modules</h3>\n")
		fmt.Fprintf(w, "<ul class=\"api\">\n")
		for _, mod := range children {
			fmt.Fprintf(w, "    <li><a href=\"%s/\"><span class=\"symbol module\"></span>%s</a></li>\n", mod, mod)
		}
		fmt.Fprintf(w, "</ul>\n\n")
	}

	// If there are resources in the root, list them.
	var resources []string
	for _, r := range mod.resources {
		resources = append(resources, resourceName(r))
	}
	if len(resources) > 0 {
		sort.Strings(resources)
		fmt.Fprintf(w, "<h3>Resources</h3>\n")
		fmt.Fprintf(w, "<ul class=\"api\">\n")
		for _, r := range resources {
			fmt.Fprintf(w, "    <li><a href=\"%s\"><span class=\"symbol resource\"></span>%s</a></li>\n", lower(r), r)
		}
		fmt.Fprintf(w, "</ul>\n\n")
	}

	// If there are functions in the root, list them.
	var functions []string
	for _, f := range mod.functions {
		functions = append(functions, tokenToName(f.Token))
	}
	if len(functions) > 0 {
		sort.Strings(functions)
		fmt.Fprintf(w, "<h3>Functions</h3>\n")
		fmt.Fprintf(w, "<ul class=\"api\">\n")
		for _, f := range functions {
			// TODO: We want to use "function" rather than "data source" terminology. Need to add a
			// "function" class in the docs repo to replace "datasource".
			fmt.Fprintf(w, "    <li><a href=\"%s\"><span class=\"symbol datasource\"></span>%s</a></li>\n", lower(f), f)
		}
		fmt.Fprintf(w, "</ul>\n\n")
	}

	return w.String()
}

func GeneratePackage(tool string, pkg *schema.Package) (map[string][]byte, error) {
	// group resources, types, and functions into modules
	modules := map[string]*modContext{}

	var getMod func(token string) *modContext
	getMod = func(token string) *modContext {
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
				if parentName == "." || parentName == "" {
					parentName = ":index:"
				}
				parent := getMod(parentName)
				parent.children = append(parent.children, mod)
			}

			modules[modName] = mod
		}
		return mod
	}

	types := &modContext{pkg: pkg, mod: "types", tool: tool}

	for _, v := range pkg.Config {
		visitObjectTypes(v.Type, func(t *schema.ObjectType) { types.details(t).outputType = true })
	}

	scanResource := func(r *schema.Resource) {
		mod := getMod(r.Token)
		mod.resources = append(mod.resources, r)
		for _, p := range r.Properties {
			visitObjectTypes(p.Type, func(t *schema.ObjectType) { types.details(t).outputType = true })
		}
		for _, p := range r.InputProperties {
			visitObjectTypes(p.Type, func(t *schema.ObjectType) {
				if r.IsProvider {
					types.details(t).outputType = true
				}
				types.details(t).inputType = true
			})
		}
		if r.StateInputs != nil {
			visitObjectTypes(r.StateInputs, func(t *schema.ObjectType) { types.details(t).inputType = true })
		}
	}

	scanResource(pkg.Provider)
	for _, r := range pkg.Resources {
		scanResource(r)
	}

	for _, f := range pkg.Functions {
		mod := getMod(f.Token)
		mod.functions = append(mod.functions, f)
		if f.Inputs != nil {
			visitObjectTypes(f.Inputs, func(t *schema.ObjectType) {
				types.details(t).inputType = true
				types.details(t).functionType = true
			})
		}
		if f.Outputs != nil {
			visitObjectTypes(f.Outputs, func(t *schema.ObjectType) {
				types.details(t).outputType = true
				types.details(t).functionType = true
			})
		}
	}

	files := fs{}
	for _, mod := range modules {
		if err := mod.gen(files); err != nil {
			return nil, err
		}
	}

	return files, nil
}

// TODO: Remove this when we have real examples available.
func (mod *modContext) genMockupExamples(w io.Writer, r *schema.Resource) {

	if resourceName(r) != "Bucket" {
		return
	}

	fmt.Fprintf(w, "## Example Usage\n\n")

	examples := []struct {
		Heading string
		Code    string
	}{
		{
			Heading: "Private Bucket w/ Tags",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const bucket = new aws.s3.Bucket("b", {
	acl: "private",
	tags: {
		Environment: "Dev",
		Name: "My bucket",
	},
});
`,
		},
		{
			Heading: "Static Website Hosting",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";
import * as fs from "fs";

const bucket = new aws.s3.Bucket("b", {
	acl: "public-read",
	policy: fs.readFileSync("policy.json", "utf-8"),
	website: {
		errorDocument: "error.html",
		indexDocument: "index.html",
		routingRules: ` + "`" + `[{
	"Condition": {
		"KeyPrefixEquals": "docs/"
	},
	"Redirect": {
		"ReplaceKeyPrefixWith": "documents/"
	}
}]
` + "`" + `,
	},
});
`,
		},
		{
			Heading: "Using CORS",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const bucket = new aws.s3.Bucket("b", {
	acl: "public-read",
	corsRules: [{
		allowedHeaders: ["*"],
		allowedMethods: [
			"PUT",
			"POST",
		],
		allowedOrigins: ["https://s3-website-test.mydomain.com"],
		exposeHeaders: ["ETag"],
		maxAgeSeconds: 3000,
	}],
});
`,
		},
		{
			Heading: "Using versioning",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const bucket = new aws.s3.Bucket("b", {
	acl: "private",
	versioning: {
		enabled: true,
	},
});
`,
		},
		{
			Heading: "Enable Logging",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const logBucket = new aws.s3.Bucket("logBucket", {
	acl: "log-delivery-write",
});
const bucket = new aws.s3.Bucket("b", {
	acl: "private",
	loggings: [{
		targetBucket: logBucket.id,
		targetPrefix: "log/",
	}],
});
`,
		},
		{
			Heading: "Using object lifecycle",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const bucket = new aws.s3.Bucket("bucket", {
	acl: "private",
	lifecycleRules: [
		{
			enabled: true,
			expiration: {
				days: 90,
			},
			id: "log",
			prefix: "log/",
			tags: {
				autoclean: "true",
				rule: "log",
			},
			transitions: [
				{
					days: 30,
					storageClass: "STANDARD_IA", // or "ONEZONE_IA"
				},
				{
					days: 60,
					storageClass: "GLACIER",
				},
			],
		},
		{
			enabled: true,
			expiration: {
				date: "2016-01-12",
			},
			id: "tmp",
			prefix: "tmp/",
		},
	],
});
const versioningBucket = new aws.s3.Bucket("versioningBucket", {
	acl: "private",
	lifecycleRules: [{
		enabled: true,
		noncurrentVersionExpiration: {
			days: 90,
		},
		noncurrentVersionTransitions: [
			{
				days: 30,
				storageClass: "STANDARD_IA",
			},
			{
				days: 60,
				storageClass: "GLACIER",
			},
		],
		prefix: "config/",
	}],
	versioning: {
		enabled: true,
	},
});
`,
		},
		{
			Heading: "Using replication configuration",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const central = new aws.Provider("central", {
	region: "eu-central-1",
});
const replicationRole = new aws.iam.Role("replication", {
	assumeRolePolicy: ` + "`" + `{
	"Version": "2012-10-17",
	"Statement": [
	{
		"Action": "sts:AssumeRole",
		"Principal": {
		"Service": "s3.amazonaws.com"
		},
		"Effect": "Allow",
		"Sid": ""
	}
	]
}
` + "`" + `,
});
const destination = new aws.s3.Bucket("destination", {
	region: "eu-west-1",
	versioning: {
		enabled: true,
	},
});
const bucket = new aws.s3.Bucket("bucket", {
	acl: "private",
	region: "eu-central-1",
	replicationConfiguration: {
		role: replicationRole.arn,
		rules: [{
			destination: {
				bucket: destination.arn,
				storageClass: "STANDARD",
			},
			id: "foobar",
			prefix: "foo",
			status: "Enabled",
		}],
	},
	versioning: {
		enabled: true,
	},
}, {provider: central});
const replicationPolicy = new aws.iam.Policy("replication", {
	policy: pulumi.interpolate` + "`" + `{
	"Version": "2012-10-17",
	"Statement": [
	{
		"Action": [
		"s3:GetReplicationConfiguration",
		"s3:ListBucket"
		],
		"Effect": "Allow",
		"Resource": [
		"${bucket.arn}"
		]
	},
	{
		"Action": [
		"s3:GetObjectVersion",
		"s3:GetObjectVersionAcl"
		],
		"Effect": "Allow",
		"Resource": [
		"${bucket.arn}/*"
		]
	},
	{
		"Action": [
		"s3:ReplicateObject",
		"s3:ReplicateDelete"
		],
		"Effect": "Allow",
		"Resource": "${destination.arn}/*"
	}
	]
}
` + "`" + `,
});
const replicationRolePolicyAttachment = new aws.iam.RolePolicyAttachment("replication", {
	policyArn: replicationPolicy.arn,
	role: replicationRole.name,
});
`,
		},
		{
			Heading: "Enable Default Server Side Encryption",
			Code: `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const mykey = new aws.kms.Key("mykey", {
	deletionWindowInDays: 10,
	description: "This key is used to encrypt bucket objects",
});
const mybucket = new aws.s3.Bucket("mybucket", {
	serverSideEncryptionConfiguration: {
		rule: {
			applyServerSideEncryptionByDefault: {
				kmsMasterKeyId: mykey.arn,
				sseAlgorithm: "aws:kms",
			},
		},
	},
});
`,
		},
	}

	for _, example := range examples {
		fmt.Fprintf(w, "### %s\n\n", example.Heading)

		fmt.Fprintf(w, "{{< langchoose nojavascript nogo >}}\n\n")

		fmt.Fprintf(w, "```typescript\n")
		fmt.Fprintf(w, example.Code)
		fmt.Fprintf(w, "```\n\n")

		fmt.Fprintf(w, "```python\nComing soon\n```\n\n")
	}
}
