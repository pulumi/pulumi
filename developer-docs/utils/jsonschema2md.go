package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

var punctuationRegexp = regexp.MustCompile(`[^\w\- ]`)

// ref: https://github.com/gjtorikian/html-pipeline/blob/main/lib/html/pipeline/toc_filter.rb
func gfmHeaderAnchor(header string) string {
	header = strings.ToLower(header)
	header = punctuationRegexp.ReplaceAllString(header, "")
	return "#" + strings.ReplaceAll(header, " ", "-")
}

func fprintf(w io.Writer, f string, args ...interface{}) {
	_, err := fmt.Fprintf(w, f, args...)
	if err != nil {
		log.Fatal(err)
	}
}

func toJSON(v interface{}) string {
	bytes, err := json.Marshal(v)
	if err != nil {
		log.Fatal(err)
	}
	return string(bytes)
}

func schemaItems(schema *jsonschema.Schema) *jsonschema.Schema {
	if schema.Items2020 != nil {
		return schema.Items2020
	}
	if items, ok := schema.Items.(*jsonschema.Schema); ok {
		return items
	}
	return nil
}

type converter struct {
	multiSchema bool

	w            io.Writer
	rootLocation string

	defs map[string]*jsonschema.Schema
}

func (c *converter) printf(f string, args ...interface{}) {
	fprintf(c.w, f, args...)
}

func (c *converter) inlineDef(schema *jsonschema.Schema) bool {
	return schema.Description == "" &&
		schema.Title == "" &&
		schema.Format == "" &&
		len(schema.Properties) == 0 &&
		len(schema.AllOf) == 0 &&
		len(schema.AnyOf) == 0 &&
		len(schema.OneOf) == 0 &&
		schema.If == nil &&
		schema.PropertyNames == nil &&
		len(schema.PatternProperties) == 0 &&
		schema.Items == nil &&
		schema.AdditionalItems == nil &&
		len(schema.PrefixItems) == 0 &&
		schema.Items2020 == nil &&
		schema.Contains == nil &&
		schema.Pattern == nil
}

func (c *converter) recordDef(schema *jsonschema.Schema) {
	if schema != nil && strings.HasPrefix(schema.Location, c.rootLocation) {
		if _, has := c.defs[schema.Location]; !has {
			c.defs[schema.Location] = schema
			c.collectDefs(schema)
		}
	}
}

func (c *converter) recordDefs(schemas []*jsonschema.Schema) {
	for _, schema := range schemas {
		c.recordDef(schema)
	}
}

func (c *converter) collectDefs(schema *jsonschema.Schema) {
	c.recordDef(schema.Ref)
	c.recordDef(schema.RecursiveRef)
	c.recordDef(schema.DynamicRef)
	c.recordDef(schema.Not)
	c.recordDefs(schema.AllOf)
	c.recordDefs(schema.AnyOf)
	c.recordDefs(schema.OneOf)
	c.recordDef(schema.If)
	c.recordDef(schema.Then)
	c.recordDef(schema.Else)
	for _, schema := range schema.Properties {
		c.collectDefs(schema)
	}
	c.recordDef(schema.PropertyNames)
	for _, schema := range schema.PatternProperties {
		c.collectDefs(schema)
	}
	if child, ok := schema.AdditionalProperties.(*jsonschema.Schema); ok {
		c.recordDef(child)
	}
	for _, dep := range schema.Dependencies {
		if schema, ok := dep.(*jsonschema.Schema); ok {
			c.recordDef(schema)
		}
	}
	for _, schema := range schema.DependentSchemas {
		c.recordDef(schema)
	}
	c.recordDef(schema.UnevaluatedProperties)
	switch items := schema.Items.(type) {
	case *jsonschema.Schema:
		c.recordDef(items)
	case []*jsonschema.Schema:
		c.recordDefs(items)
	}
	if child, ok := schema.AdditionalItems.(*jsonschema.Schema); ok {
		c.recordDef(child)
	}
	c.recordDefs(schema.PrefixItems)
	c.recordDef(schema.Items2020)
	c.recordDef(schema.Contains)
	c.recordDef(schema.UnevaluatedItems)
}

func (c *converter) schemaTitle(schema *jsonschema.Schema) string {
	if schema.Title != "" {
		return schema.Title
	}
	return "`" + schema.Location + "`"
}

func (c *converter) refLink(ref *jsonschema.Schema) string {
	dest := ref.Location
	if strings.HasPrefix(ref.Location, c.rootLocation) {
		dest = gfmHeaderAnchor(c.schemaTitle(ref))
	}

	return fmt.Sprintf("[%v](%v)", c.schemaTitle(ref), dest)
}

func (c *converter) ref(ref *jsonschema.Schema) string {
	if !c.inlineDef(ref) {
		return c.refLink(ref)
	}

	if ref.Ref != nil {
		return c.ref(ref.Ref)
	}

	if len(ref.Constant) != 0 {
		return c.schemaConstant(ref)
	}
	if len(ref.Enum) != 0 {
		return c.schemaEnum(ref)
	}

	return c.schemaTypes(ref)
}

func (c *converter) schemaTypes(schema *jsonschema.Schema) string {
	types := schema.Types
	if len(types) == 1 {
		return fmt.Sprintf("`%v`", types[0])
	}

	var sb strings.Builder
	for i, t := range types {
		if i != 0 {
			fprintf(&sb, " | ")
		}
		fprintf(&sb, "`%v`", t)
	}
	return sb.String()
}

func (c *converter) convertSchemaTypes(schema *jsonschema.Schema) {
	types := schema.Types
	switch len(types) {
	case 0:
		// Nothing to do
	case 1:
		c.printf("\n%v\n", c.schemaTypes(schema))
	default:
		c.printf("\n%v\n", c.schemaTypes(schema))
	}
}

func (c *converter) convertSchemaStringValidators(schema *jsonschema.Schema) {
	if schema.Format != "" {
		c.printf("\nFormat: `%v`\n", schema.Format)
	}
	if schema.Pattern != nil {
		c.printf("\nPattern: `%v`\n", schema.Pattern)
	}
}

func (c *converter) convertSchemaRef(schema *jsonschema.Schema) {
	if schema.Ref != nil {
		c.printf("\n%v\n", c.refLink(schema.Ref))
	}
}

func (c *converter) schemaConstant(schema *jsonschema.Schema) string {
	return fmt.Sprintf("`%s`", toJSON(schema.Constant[0]))
}

func (c *converter) convertSchemaConstant(schema *jsonschema.Schema) {
	if len(schema.Constant) != 0 {
		c.printf("\nConstant: %v\n", c.schemaConstant(schema))
	}
}

func (c *converter) schemaEnum(schema *jsonschema.Schema) string {
	var sb strings.Builder
	for i, v := range schema.Enum {
		if i != 0 {
			sb.WriteString(" | ")
		}
		fprintf(&sb, "`%s`", toJSON(v))
	}
	return sb.String()
}

func (c *converter) convertSchemaEnum(schema *jsonschema.Schema) {
	if len(schema.Enum) != 0 {
		c.printf("\nEnum: %v\n", c.schemaEnum(schema))
	}
}

func (c *converter) convertSchemaLogic(schema *jsonschema.Schema) {
	if len(schema.AllOf) != 0 {
		c.printf("\nAll of:\n")
		for _, ref := range schema.AllOf {
			c.printf("- %v\n", c.ref(ref))
		}
	}
	if len(schema.AnyOf) != 0 {
		c.printf("\nAny of:\n")
		for _, ref := range schema.AllOf {
			c.printf("- %v\n", c.ref(ref))
		}
	}
	if len(schema.OneOf) != 0 {
		c.printf("\nOne of:\n")
		for _, ref := range schema.AllOf {
			c.printf("- %v\n", c.ref(ref))
		}
	}
	if schema.If != nil {
		c.printf("\nIf %v", c.ref(schema.If))
		if schema.Then != nil {
			c.printf(", then %v", c.ref(schema.Then))
		}
		if schema.Else != nil {
			c.printf(", else %v", c.ref(schema.Else))
		}
		c.printf("\n")
	}
}

func (c *converter) convertSchemaObject(schema *jsonschema.Schema, level int) {
	if schema.PropertyNames != nil {
		c.printf("\nProperty names: %v\n", c.ref(schema.PropertyNames))
	}
	if additionalProperties, ok := schema.AdditionalProperties.(*jsonschema.Schema); ok {
		c.printf("\nAdditional properties: %v\n", c.ref(additionalProperties))
	}

	required := map[string]bool{}
	for _, name := range schema.Required {
		required[name] = true
	}

	properties := slice.Prealloc[string](len(schema.Properties))
	for name, schema := range schema.Properties {
		if schema.Always != nil && !*schema.Always {
			continue
		}
		properties = append(properties, name)
	}
	sort.Strings(properties)

	if len(properties) != 0 {
		c.printf("\n%v Properties\n", strings.Repeat("#", level+1))
		c.printf("\n---\n")
		for _, name := range properties {
			c.printf("\n%s `%s`", strings.Repeat("#", level+2), name)
			if required[name] {
				c.printf(" (_required_)")
			}
			c.printf("\n")

			c.convertSchema(schema.Properties[name], level+2)
			c.printf("\n---\n")
		}
	}
}

func (c *converter) convertSchemaArray(schema *jsonschema.Schema) {
	if items := schemaItems(schema); items != nil {
		c.printf("\nItems: %v\n", c.ref(items))
	}
}

func (c *converter) convertSchema(schema *jsonschema.Schema, level int) {
	if schema.Description != "" {
		c.printf("\n%s\n", schema.Description)
	}

	c.convertSchemaTypes(schema)
	c.convertSchemaConstant(schema)
	c.convertSchemaEnum(schema)
	c.convertSchemaStringValidators(schema)
	c.convertSchemaRef(schema)
	c.convertSchemaLogic(schema)
	c.convertSchemaArray(schema)
	c.convertSchemaObject(schema, level)
}

func (c *converter) convertRootSchema(schema *jsonschema.Schema) {
	c.collectDefs(schema)

	level := 1
	if c.multiSchema {
		level = 2
	}

	c.printf("%s %s\n", strings.Repeat("#", level), c.schemaTitle(schema))

	c.convertSchema(schema, level)

	defs := slice.Prealloc[*jsonschema.Schema](len(c.defs))
	for _, def := range c.defs {
		defs = append(defs, def)
	}
	sort.Slice(defs, func(i, j int) bool {
		return c.schemaTitle(defs[i]) < c.schemaTitle(defs[j])
	})

	for _, def := range defs {
		if !c.inlineDef(def) {
			c.printf("\n%s %s\n", strings.Repeat("#", level+1), c.schemaTitle(def))
			c.convertSchema(def, level+1)
		}
	}
}

func main() {
	title := flag.String("title", "", "the top-level title for the output, if any")
	idString := flag.String("ids", "", "a comma-separated list of 'id=path' mappings")
	flag.Parse()

	const rootID = "blob://stdin"
	ids := map[string]string{
		rootID: "-",
	}
	if *idString != "" {
		for _, idm := range strings.Split(*idString, ",") {
			eq := strings.IndexByte(idm, '=')
			if eq == -1 {
				log.Fatalf("invalid 'id=path' mapping '%v'", idm)
			}
			id, path := idm[:eq], idm[eq+1:]
			if id == "" || path == "" {
				log.Fatalf("invalid 'id=path' mapping '%v'", idm)
			}
			ids[id] = path

			if path == "-" {
				delete(ids, rootID)
			}
		}
		if len(ids) > 1 && *title == "" {
			log.Fatal("-title is required if more than one ID is mapped")
		}
	}

	compiler := jsonschema.NewCompiler()
	compiler.ExtractAnnotations = true
	compiler.LoadURL = func(s string) (io.ReadCloser, error) {
		if path, ok := ids[s]; ok {
			if path == "-" {
				return os.Stdin, nil
			}
			return os.Open(path)
		}
		return jsonschema.LoadURL(s)
	}

	schemas := slice.Prealloc[*jsonschema.Schema](len(ids))
	for id := range ids {
		schema, err := compiler.Compile(id)
		if err != nil {
			log.Fatal(err)
		}
		schemas = append(schemas, schema)
	}
	sort.Slice(schemas, func(i, j int) bool { return schemas[i].Location < schemas[j].Location })

	if *title != "" {
		fprintf(os.Stdout, "# %v\n", *title)
	}

	for _, schema := range schemas {
		fprintf(os.Stdout, "\n")

		converter := converter{
			multiSchema:  len(ids) > 1,
			w:            os.Stdout,
			rootLocation: schema.Location,
			defs:         map[string]*jsonschema.Schema{},
		}
		converter.convertRootSchema(schema)
	}
}
