// Copyright 2026, Pulumi Corporation.
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

package cloud

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
	"go.yaml.in/yaml/v4"
)

// schemaRenderer renders libopenapi schemas as Restish-style human-readable text.
// Stateless: per-call state flows through renderCtx.
type schemaRenderer struct {
	components *orderedmap.Map[string, *base.SchemaProxy]
}

// renderCtx carries the per-call state threaded through every render step.
type renderCtx struct {
	indent  int
	visited map[uint64]bool
	// stripDiscrimProps is copy-on-write — extend via withStripProp, never mutate in place.
	// Stripped props are removed from nested allOf merges so inherited discriminator
	// properties don't re-appear inside variant refs of a discriminated union.
	stripDiscrimProps map[string]bool
	// One-shot hint: tells the *immediate* renderObject call to show an inherited
	// discriminator property as an enum. Child renders must not see it.
	allOfDiscrim *allOfDiscrimInfo
}

type allOfDiscrimInfo struct {
	propName string
	values   []string
}

// indented returns ctx with indent increased by delta.
func (c renderCtx) indented(delta int) renderCtx {
	c.indent += delta
	return c
}

// withStripProp returns ctx with prop added to stripDiscrimProps, cloning
// the map so callers up the stack keep their original view.
func (c renderCtx) withStripProp(prop string) renderCtx {
	if prop == "" {
		return c
	}
	next := make(map[string]bool, len(c.stripDiscrimProps)+1)
	for k, v := range c.stripDiscrimProps {
		next[k] = v
	}
	next[prop] = true
	c.stripDiscrimProps = next
	return c
}

func newSchemaRenderer(components *orderedmap.Map[string, *base.SchemaProxy]) *schemaRenderer {
	return &schemaRenderer{components: components}
}

func rootCtx(indent int) renderCtx {
	return renderCtx{indent: indent, visited: make(map[uint64]bool)}
}

// renderBodySchema renders a request body schema with heading.
func (r *schemaRenderer) renderBodySchema(s *base.Schema) string {
	return "Request Body:\n" + r.renderValue(s, rootCtx(2))
}

// renderResponseSchema renders a response schema with heading.
func (r *schemaRenderer) renderResponseSchema(s *base.Schema) string {
	return "Response:\n" + r.renderValue(s, rootCtx(2))
}

// schemaType extracts the primary type from a schema's Type slice, skipping "null".
func schemaType(s *base.Schema) string {
	for _, t := range s.Type {
		if t != "null" {
			return t
		}
	}
	if len(s.Type) > 0 {
		return s.Type[0]
	}
	return ""
}

// schemaHash returns a hash for circular reference detection.
func schemaHash(s *base.Schema) uint64 {
	if low := s.GoLow(); low != nil {
		return low.Hash()
	}
	return 0
}

// renderValue renders a schema value with indentation and depth tracking.
func (r *schemaRenderer) renderValue(s *base.Schema, ctx renderCtx) string {
	if s == nil {
		return spad(ctx.indent) + "(<any>)"
	}

	h := schemaHash(s)
	if h != 0 {
		if ctx.visited[h] {
			return spad(ctx.indent) + "(circular)"
		}
		ctx.visited[h] = true
		defer delete(ctx.visited, h)
	}

	// Handle allOf — merge into a single object.
	if len(s.AllOf) > 0 {
		merged := r.mergeAllOf(s.AllOf, ctx.visited)
		if merged != nil {
			mergedCtx := ctx
			// If an allOf member defines a discriminator, surface its allowed values inline.
			for _, proxy := range s.AllOf {
				if proxy == nil {
					continue
				}
				sub := proxy.Schema()
				if sub == nil {
					continue
				}
				if sub.Discriminator != nil && sub.Discriminator.PropertyName != "" && sub.Discriminator.Mapping != nil {
					var vals []string
					for key := range sub.Discriminator.Mapping.KeysFromOldest() {
						vals = append(vals, key)
					}
					sort.Strings(vals)
					mergedCtx.allOfDiscrim = &allOfDiscrimInfo{
						propName: sub.Discriminator.PropertyName,
						values:   vals,
					}
					break
				}
			}
			// Strip inherited discriminator properties inside a renderDiscriminator context
			// (e.g. repo inside pull_request variant shouldn't show "type" from the base union).
			if len(ctx.stripDiscrimProps) > 0 && merged.Properties != nil {
				for prop := range ctx.stripDiscrimProps {
					merged.Properties.Delete(prop)
				}
			}
			return r.renderValue(merged, mergedCtx)
		}
	}

	// Handle oneOf / anyOf — render each variant.
	if len(s.OneOf) > 0 {
		return r.renderPolymorph("oneOf", s.OneOf, ctx)
	}
	if len(s.AnyOf) > 0 {
		return r.renderPolymorph("anyOf", s.AnyOf, ctx)
	}

	// Discriminated union with mapping but no oneOf: resolve mapping refs and render as oneOf.
	if s.Discriminator != nil && s.Discriminator.Mapping != nil && len(s.OneOf) == 0 && r.components != nil {
		return r.renderDiscriminator(s, ctx)
	}

	typ := schemaType(s)

	if typ == "array" {
		return r.renderArray(s, ctx)
	}

	if typ == "object" || orderedmap.Len(s.Properties) > 0 {
		if orderedmap.Len(s.Properties) == 0 {
			if s.AdditionalProperties != nil && s.AdditionalProperties.IsA() && s.AdditionalProperties.A != nil {
				addSchema := s.AdditionalProperties.A.Schema()
				if addSchema != nil {
					val := r.renderValue(addSchema, ctx.indented(2))
					return spad(ctx.indent) + "{\n" + spad(ctx.indent+2) + "<string>: " +
						strings.TrimLeft(val, " ") + "\n" + spad(ctx.indent) + "}"
				}
			}
			return spad(ctx.indent) + "(object)"
		}
		return r.renderObject(s, ctx)
	}

	return spad(ctx.indent) + r.typeTag(s)
}

// renderArray renders an array schema.
func (r *schemaRenderer) renderArray(s *base.Schema, ctx renderCtx) string {
	if s.Items != nil && s.Items.IsA() && s.Items.A != nil {
		itemSchema := s.Items.A.Schema()
		if itemSchema != nil {
			item := r.renderValue(itemSchema, ctx.indented(2))
			return spad(ctx.indent) + "[\n" + item + "\n" + spad(ctx.indent) + "]"
		}
	}
	return spad(ctx.indent) + "[<any>]"
}

// renderPolymorph renders oneOf/anyOf variants.
func (r *schemaRenderer) renderPolymorph(label string, proxies []*base.SchemaProxy, ctx renderCtx) string {
	var lines []string
	lines = append(lines, spad(ctx.indent)+label+" {")
	for _, proxy := range proxies {
		if proxy == nil {
			continue
		}
		resolved := proxy.Schema()
		if resolved == nil {
			continue
		}
		val := r.renderValue(resolved, ctx.indented(2))
		lines = append(lines, val)
	}
	lines = append(lines, spad(ctx.indent)+"}")
	return strings.Join(lines, "\n")
}

// renderDiscriminator resolves a discriminator mapping and renders each variant.
// This handles the case where a schema has a discriminator with mapping but no oneOf.
func (r *schemaRenderer) renderDiscriminator(s *base.Schema, ctx renderCtx) string {
	var lines []string
	lines = append(lines, spad(ctx.indent)+"oneOf {")

	propName := s.Discriminator.PropertyName
	variantCtx := ctx.withStripProp(propName).indented(2)
	for discVal, ref := range s.Discriminator.Mapping.FromOldest() {
		const prefix = "#/components/schemas/"
		if !strings.HasPrefix(ref, prefix) {
			continue
		}
		name := ref[len(prefix):]
		proxy, ok := r.components.Get(name)
		if !ok || proxy == nil {
			continue
		}
		variantSchema := proxy.Schema()
		if variantSchema == nil {
			continue
		}

		if len(variantSchema.AllOf) > 0 {
			variantSchema = r.mergeAllOf(variantSchema.AllOf, ctx.visited)
			if variantSchema == nil {
				continue
			}
		}

		// Shallow copy clears Discriminator to prevent infinite re-resolution.
		// Deep-copy Properties (orderedmap is pointer-backed) and Required
		// (slice-aliased via [:0] would stomp the shared backing array) so
		// repeat renders of the same component schema stay idempotent.
		schemaCopy := *variantSchema
		schemaCopy.Discriminator = nil
		if propName != "" && schemaCopy.Properties != nil {
			newProps := orderedmap.New[string, *base.SchemaProxy]()
			for k, v := range schemaCopy.Properties.FromOldest() {
				if k == propName {
					continue
				}
				newProps.Set(k, v)
			}
			schemaCopy.Properties = newProps
			filtered := make([]string, 0, len(schemaCopy.Required))
			for _, req := range schemaCopy.Required {
				if req != propName {
					filtered = append(filtered, req)
				}
			}
			schemaCopy.Required = filtered
		}

		val := r.renderValue(&schemaCopy, variantCtx)

		if propName != "" && discVal != "" {
			discLine := spad(ctx.indent+4) + propName + "*: \"" + discVal + "\""
			if idx := strings.Index(val, "{\n"); idx >= 0 {
				val = val[:idx+2] + discLine + "\n" + val[idx+2:]
			}
		}

		lines = append(lines, val)
	}

	lines = append(lines, spad(ctx.indent)+"}")
	return strings.Join(lines, "\n")
}

// renderObject renders an object schema with its properties.
func (r *schemaRenderer) renderObject(s *base.Schema, ctx renderCtx) string {
	reqSet := make(map[string]bool)
	for _, req := range s.Required {
		reqSet[req] = true
	}

	// Collect discriminator values if present (from schema itself or inherited via allOf).
	var discrimProp string
	var discrimValues []string
	if s.Discriminator != nil && s.Discriminator.PropertyName != "" {
		discrimProp = s.Discriminator.PropertyName
		if s.Discriminator.Mapping != nil {
			for val := range s.Discriminator.Mapping.KeysFromOldest() {
				discrimValues = append(discrimValues, val)
			}
		}
		sort.Strings(discrimValues)
	} else if ctx.allOfDiscrim != nil {
		discrimProp = ctx.allOfDiscrim.propName
		discrimValues = ctx.allOfDiscrim.values
	}

	// Child renders must not see the one-shot allOfDiscrim hint.
	childCtx := ctx.indented(2)
	childCtx.allOfDiscrim = nil

	props := sortedProps(s)
	var lines []string

	for _, p := range props {
		if p.schema != nil {
			if p.schema.ReadOnly != nil && *p.schema.ReadOnly {
				continue
			}
		}

		mark := ""
		if reqSet[p.name] {
			mark = "*"
		}

		if p.name == discrimProp && len(discrimValues) > 0 {
			vals := discrimValues
			if len(vals) > 6 {
				vals = append(vals[:6:6], "...")
			}
			valueStr := "(string) one of: " + strings.Join(vals, ", ")
			line := spad(childCtx.indent) + p.name + mark + ": " + valueStr
			if p.schema != nil && p.schema.Description != "" {
				line += " " + truncateDesc(p.schema.Description)
			}
			lines = append(lines, line)
			continue
		}

		valueStr := r.renderValue(p.schema, childCtx)
		valueStr = strings.TrimLeft(valueStr, " ")

		hasDesc := p.schema != nil && p.schema.Description != ""
		isMultiLine := strings.Contains(valueStr, "\n")

		var line string
		if isMultiLine && hasDesc {
			line = spad(childCtx.indent) + p.name + mark + ": " + truncateDesc(p.schema.Description) + " " + valueStr
		} else {
			line = spad(childCtx.indent) + p.name + mark + ": " + valueStr
			if hasDesc {
				line += " " + truncateDesc(p.schema.Description)
			}
		}

		lines = append(lines, line)
	}

	return spad(ctx.indent) + "{\n" + strings.Join(lines, "\n") + "\n" + spad(ctx.indent) + "}"
}

// typeTag renders the inline type annotation like "(string format:date-time) one of: a, b, c".
func (r *schemaRenderer) typeTag(s *base.Schema) string {
	typ := schemaType(s)
	if typ == "" {
		typ = "<any>"
	}

	parts := []string{typ}

	if s.Format != "" {
		parts = append(parts, "format:"+s.Format)
	}

	for _, t := range s.Type {
		if t == "null" {
			parts = append(parts, "nullable")
			break
		}
	}
	if s.Nullable != nil && *s.Nullable {
		parts = append(parts, "nullable")
	}

	if s.MinLength != nil {
		parts = append(parts, fmt.Sprintf("minLength:%d", *s.MinLength))
	}
	if s.MaxLength != nil {
		parts = append(parts, fmt.Sprintf("maxLength:%d", *s.MaxLength))
	}
	if s.Pattern != "" {
		parts = append(parts, "pattern:"+s.Pattern)
	}
	if s.Minimum != nil {
		parts = append(parts, fmt.Sprintf("min:%g", *s.Minimum))
	}
	if s.Maximum != nil {
		parts = append(parts, fmt.Sprintf("max:%g", *s.Maximum))
	}
	if s.MultipleOf != nil {
		parts = append(parts, fmt.Sprintf("multipleOf:%g", *s.MultipleOf))
	}
	if s.Default != nil {
		parts = append(parts, "default:"+s.Default.Value)
	}

	tag := "(" + strings.Join(parts, " ") + ")"

	if len(s.Enum) > 0 {
		vals := enumStringsYAML(s.Enum)
		if len(vals) > 6 {
			vals = append(vals[:6:6], "...")
		}
		tag += " one of: " + strings.Join(vals, ", ")
	}

	return tag
}

// mergeAllOf merges allOf sub-schemas into a single flattened object schema.
func (r *schemaRenderer) mergeAllOf(proxies []*base.SchemaProxy, visited map[uint64]bool) *base.Schema {
	merged := &base.Schema{
		Type:       []string{"object"},
		Properties: orderedmap.New[string, *base.SchemaProxy](),
	}
	reqSet := make(map[string]bool)

	for _, proxy := range proxies {
		if proxy == nil {
			continue
		}
		resolved := proxy.Schema()
		if resolved == nil {
			continue
		}

		if len(resolved.AllOf) > 0 {
			resolved = r.mergeAllOf(resolved.AllOf, visited)
			if resolved == nil {
				continue
			}
		}

		if resolved.Properties != nil {
			for k, v := range resolved.Properties.FromOldest() {
				merged.Properties.Set(k, v)
			}
		}
		for _, req := range resolved.Required {
			reqSet[req] = true
		}
		if resolved.Description != "" && merged.Description == "" {
			merged.Description = resolved.Description
		}
		// Note: we intentionally do NOT copy Discriminator from sub-schemas.
		// The discriminator belongs on the base schema that defines the union,
		// not on concrete variants that inherit from it via allOf.
	}

	for req := range reqSet {
		merged.Required = append(merged.Required, req)
	}
	sort.Strings(merged.Required)

	return merged
}

type propEntry struct {
	name   string
	schema *base.Schema
	order  int
}

func sortedProps(s *base.Schema) []propEntry {
	var props []propEntry
	if s.Properties == nil {
		return props
	}
	for name, proxy := range s.Properties.FromOldest() {
		order := 9999
		var resolved *base.Schema
		if proxy != nil {
			resolved = proxy.Schema()
		}
		if resolved != nil && resolved.Extensions != nil {
			if node := resolved.Extensions.GetOrZero("x-order"); node != nil {
				var o int
				if err := node.Decode(&o); err == nil && o > 0 {
					order = o
				}
			}
		}
		props = append(props, propEntry{name: name, schema: resolved, order: order})
	}
	sort.Slice(props, func(i, j int) bool {
		if props[i].order != props[j].order {
			return props[i].order < props[j].order
		}
		return props[i].name < props[j].name
	})
	return props
}

func spad(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}

func enumStringsYAML(nodes []*yaml.Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		if n != nil {
			out[i] = n.Value
		}
	}
	return out
}

// Collapse newlines so each field stays on one line; no hard char cap — truncating
// mid-sentence would hide information the user came here to read.
func truncateDesc(desc string) string {
	return strings.ReplaceAll(desc, "\n", " ")
}

// --- Markdown rendering ---
//
// The markdown renderer produces a nested bullet tree per property, matching
// the layout of pulumi/docs' property-rows.html. Depth is capped at mdMaxDepth
// so recursive schemas don't blow up the output; past that the renderer emits
// "(nested schema)" and stops.

const mdMaxDepth = 4

// renderSchemaMarkdown renders the schema as a markdown document. For objects
// it emits a bullet list of properties; for arrays it emits an "array of" header
// followed by the item schema; for primitives/unions it emits a single-line
// descriptor.
func (r *schemaRenderer) renderSchemaMarkdown(s *base.Schema) string {
	var b strings.Builder
	r.writeSchemaMarkdown(&b, s, 0, mdRenderState{visited: make(map[uint64]bool)})
	return strings.TrimRight(b.String(), "\n")
}

type mdRenderState struct {
	visited           map[uint64]bool
	stripDiscrimProps map[string]bool
}

func (st mdRenderState) withStripProp(prop string) mdRenderState {
	if prop == "" {
		return st
	}
	next := make(map[string]bool, len(st.stripDiscrimProps)+1)
	for k, v := range st.stripDiscrimProps {
		next[k] = v
	}
	next[prop] = true
	st.stripDiscrimProps = next
	return st
}

// writeSchemaMarkdown walks a schema at the given depth and writes a markdown
// description to b. The caller controls what wraps the output (the describe
// renderer places it under "## Request body" / per-response headings).
func (r *schemaRenderer) writeSchemaMarkdown(b *strings.Builder, s *base.Schema, depth int, st mdRenderState) {
	if s == nil {
		fmt.Fprintf(b, "%s- `any`\n", strings.Repeat("  ", depth))
		return
	}

	if h := schemaHash(s); h != 0 {
		if st.visited[h] {
			fmt.Fprintf(b, "%s- _(circular reference)_\n", strings.Repeat("  ", depth))
			return
		}
		st.visited[h] = true
		defer delete(st.visited, h)
	}

	if len(s.AllOf) > 0 {
		if merged := r.mergeAllOf(s.AllOf, st.visited); merged != nil {
			if len(st.stripDiscrimProps) > 0 && merged.Properties != nil {
				for prop := range st.stripDiscrimProps {
					merged.Properties.Delete(prop)
				}
			}
			r.writeSchemaMarkdown(b, merged, depth, st)
			return
		}
	}

	if len(s.OneOf) > 0 {
		r.writeUnionMarkdown(b, "oneOf", s.OneOf, depth, st)
		return
	}
	if len(s.AnyOf) > 0 {
		r.writeUnionMarkdown(b, "anyOf", s.AnyOf, depth, st)
		return
	}

	if s.Discriminator != nil && s.Discriminator.Mapping != nil && len(s.OneOf) == 0 && r.components != nil {
		r.writeDiscriminatorMarkdown(b, s, depth, st)
		return
	}

	typ := schemaType(s)

	if typ == "object" || orderedmap.Len(s.Properties) > 0 {
		r.writeObjectMarkdown(b, s, depth, st)
		return
	}

	if typ == "array" {
		r.writeArrayMarkdown(b, s, depth, st)
		return
	}

	// Leaf: primitive, map (object with only additionalProperties), or any.
	fmt.Fprintf(b, "%s- %s", strings.Repeat("  ", depth), r.markdownTypeBadge(s))
	if len(s.Enum) > 0 {
		fmt.Fprint(b, " "+markdownEnumTail(enumStringsYAML(s.Enum)))
	}
	if s.Description != "" {
		fmt.Fprintf(b, " — %s", mdInline(s.Description))
	}
	b.WriteByte('\n')
}

func (r *schemaRenderer) writeObjectMarkdown(b *strings.Builder, s *base.Schema, depth int, st mdRenderState) {
	// Map shape: object with no named properties, only additionalProperties.
	if orderedmap.Len(s.Properties) == 0 {
		if s.AdditionalProperties != nil && s.AdditionalProperties.IsA() && s.AdditionalProperties.A != nil {
			add := s.AdditionalProperties.A.Schema()
			fmt.Fprintf(b, "%s- `map[string, %s]`\n", strings.Repeat("  ", depth), r.markdownTypeName(add))
			if depth+1 <= mdMaxDepth {
				r.writeSchemaMarkdown(b, add, depth+1, st)
			}
			return
		}
		if s.Description != "" {
			fmt.Fprintf(b, "%s- `object` — %s\n", strings.Repeat("  ", depth), mdInline(s.Description))
		} else {
			fmt.Fprintf(b, "%s- `object`\n", strings.Repeat("  ", depth))
		}
		return
	}

	reqSet := make(map[string]bool, len(s.Required))
	for _, req := range s.Required {
		reqSet[req] = true
	}

	var discrimProp string
	var discrimValues []string
	if s.Discriminator != nil && s.Discriminator.PropertyName != "" {
		discrimProp = s.Discriminator.PropertyName
		if s.Discriminator.Mapping != nil {
			for val := range s.Discriminator.Mapping.KeysFromOldest() {
				discrimValues = append(discrimValues, val)
			}
			sort.Strings(discrimValues)
		}
	}

	indent := strings.Repeat("  ", depth)
	for _, p := range sortedProps(s) {
		if p.schema != nil && p.schema.ReadOnly != nil && *p.schema.ReadOnly {
			continue
		}
		r.writePropertyMarkdown(b, indent, p, reqSet[p.name], depth, st, discrimProp, discrimValues)
	}
}

func (r *schemaRenderer) writePropertyMarkdown(
	b *strings.Builder, indent string, p propEntry, required bool,
	depth int, st mdRenderState, discrimProp string, discrimValues []string,
) {
	fmt.Fprintf(b, "%s- `%s`", indent, p.name)

	child := p.schema
	if child != nil && len(child.AllOf) > 0 {
		if merged := r.mergeAllOf(child.AllOf, st.visited); merged != nil {
			child = merged
		}
	}

	fmt.Fprint(b, " "+r.markdownTypeBadge(child))

	if required {
		fmt.Fprint(b, " **required**")
	}

	enumVals := enumStringsYAML(nonNil(child).Enum)
	if p.name == discrimProp && len(discrimValues) > 0 {
		enumVals = discrimValues
	}
	if len(enumVals) > 0 {
		fmt.Fprint(b, " "+markdownEnumTail(enumVals))
	}

	if child != nil && child.Description != "" {
		fmt.Fprintf(b, " — %s", mdInline(child.Description))
	}
	b.WriteByte('\n')

	if child == nil {
		return
	}
	if depth+1 > mdMaxDepth {
		return
	}

	typ := schemaType(child)
	// A map-shaped object (no named properties, only additionalProperties)
	// already conveys its shape in the type badge — don't recurse unless the
	// value type is itself structured.
	if typ == "object" && orderedmap.Len(child.Properties) == 0 &&
		child.AdditionalProperties != nil && child.AdditionalProperties.IsA() &&
		child.AdditionalProperties.A != nil {
		val := child.AdditionalProperties.A.Schema()
		if val != nil && (schemaType(val) == "object" || orderedmap.Len(val.Properties) > 0) {
			if depth+1 <= mdMaxDepth {
				r.writeSchemaMarkdown(b, val, depth+1, st)
			}
		}
		return
	}

	hasChildren := typ == "object" ||
		orderedmap.Len(child.Properties) > 0 ||
		len(child.OneOf) > 0 || len(child.AnyOf) > 0 ||
		(child.Discriminator != nil && child.Discriminator.Mapping != nil)
	if !hasChildren && typ != "array" {
		return
	}

	nextSt := st
	if p.name == discrimProp {
		nextSt = nextSt.withStripProp(discrimProp)
	}

	if typ == "array" {
		if child.Items != nil && child.Items.IsA() && child.Items.A != nil {
			if it := child.Items.A.Schema(); it != nil {
				r.writeSchemaMarkdown(b, it, depth+1, nextSt)
			}
		}
		return
	}

	r.writeSchemaMarkdown(b, child, depth+1, nextSt)
}

func (r *schemaRenderer) writeArrayMarkdown(b *strings.Builder, s *base.Schema, depth int, st mdRenderState) {
	indent := strings.Repeat("  ", depth)
	if s.Items != nil && s.Items.IsA() && s.Items.A != nil {
		it := s.Items.A.Schema()
		fmt.Fprintf(b, "%s- `array[%s]`", indent, r.markdownTypeName(it))
		if s.Description != "" {
			fmt.Fprintf(b, " — %s", mdInline(s.Description))
		}
		b.WriteByte('\n')
		if depth+1 <= mdMaxDepth && it != nil {
			itTyp := schemaType(it)
			if itTyp == "object" || orderedmap.Len(it.Properties) > 0 || len(it.OneOf) > 0 {
				r.writeSchemaMarkdown(b, it, depth+1, st)
			}
		}
		return
	}
	fmt.Fprintf(b, "%s- `array[any]`\n", indent)
}

func (r *schemaRenderer) writeUnionMarkdown(
	b *strings.Builder, label string, proxies []*base.SchemaProxy, depth int, st mdRenderState,
) {
	fmt.Fprintf(b, "%s- _%s of:_\n", strings.Repeat("  ", depth), label)
	if depth+1 > mdMaxDepth {
		return
	}
	for _, proxy := range proxies {
		if proxy == nil {
			continue
		}
		variant := proxy.Schema()
		if variant == nil {
			continue
		}
		r.writeSchemaMarkdown(b, variant, depth+1, st)
	}
}

func (r *schemaRenderer) writeDiscriminatorMarkdown(b *strings.Builder, s *base.Schema, depth int, st mdRenderState) {
	indent := strings.Repeat("  ", depth)
	propName := s.Discriminator.PropertyName
	fmt.Fprintf(b, "%s- _oneOf (discriminated by `%s`):_\n", indent, propName)
	if depth+1 > mdMaxDepth {
		return
	}
	variantSt := st.withStripProp(propName)
	for discVal, ref := range s.Discriminator.Mapping.FromOldest() {
		const prefix = "#/components/schemas/"
		if !strings.HasPrefix(ref, prefix) {
			continue
		}
		name := ref[len(prefix):]
		proxy, ok := r.components.Get(name)
		if !ok || proxy == nil {
			continue
		}
		variant := proxy.Schema()
		if variant == nil {
			continue
		}
		if len(variant.AllOf) > 0 {
			if merged := r.mergeAllOf(variant.AllOf, variantSt.visited); merged != nil {
				variant = merged
			}
		}
		// Remove the discriminator property from the variant; we display it
		// as the header. Deep-copy Properties and Required so we don't mutate
		// the shared cached schema on repeat renders.
		variantCopy := *variant
		variantCopy.Discriminator = nil
		if propName != "" && variantCopy.Properties != nil {
			newProps := orderedmap.New[string, *base.SchemaProxy]()
			for k, v := range variantCopy.Properties.FromOldest() {
				if k == propName {
					continue
				}
				newProps.Set(k, v)
			}
			variantCopy.Properties = newProps
			filtered := make([]string, 0, len(variantCopy.Required))
			for _, req := range variantCopy.Required {
				if req != propName {
					filtered = append(filtered, req)
				}
			}
			variantCopy.Required = filtered
		}
		fmt.Fprintf(b, "%s- `%s` = `\"%s\"`\n", strings.Repeat("  ", depth+1), propName, discVal)
		r.writeSchemaMarkdown(b, &variantCopy, depth+2, variantSt)
	}
}

// markdownTypeBadge formats the type as a backtick-wrapped badge. Keeps it
// compact: `string`, `array[User]`, `map[string, int]`, `User | null`.
func (r *schemaRenderer) markdownTypeBadge(s *base.Schema) string {
	name := r.markdownTypeName(s)
	if s != nil {
		for _, t := range s.Type {
			if t == "null" {
				return "`" + name + " | null`"
			}
		}
		if s.Nullable != nil && *s.Nullable {
			return "`" + name + " | null`"
		}
	}
	return "`" + name + "`"
}

// markdownTypeName returns the inner type name without backticks, suitable
// for nesting inside `array[...]` or `map[string, ...]`.
func (r *schemaRenderer) markdownTypeName(s *base.Schema) string {
	if s == nil {
		return "any"
	}
	t := schemaType(s)
	switch t {
	case "array":
		if s.Items != nil && s.Items.IsA() && s.Items.A != nil {
			return "array[" + r.markdownTypeName(s.Items.A.Schema()) + "]"
		}
		return "array[any]"
	case "object":
		if orderedmap.Len(s.Properties) == 0 && s.AdditionalProperties != nil &&
			s.AdditionalProperties.IsA() && s.AdditionalProperties.A != nil {
			return "map[string, " + r.markdownTypeName(s.AdditionalProperties.A.Schema()) + "]"
		}
		return "object"
	case "":
		if len(s.OneOf) > 0 || len(s.AnyOf) > 0 {
			return "union"
		}
		if len(s.AllOf) > 0 || orderedmap.Len(s.Properties) > 0 {
			return "object"
		}
		return "any"
	}
	if s.Format != "" {
		return t + " (" + s.Format + ")"
	}
	if len(s.Enum) > 0 {
		return t + " enum"
	}
	return t
}

// markdownEnumTail returns the italicized enum values list: _enum:_ `A`, `B`, `C`.
// Truncates after 6 values to stay readable.
func markdownEnumTail(vals []string) string {
	shown := vals
	if len(shown) > 6 {
		shown = append(shown[:6:6], "…")
	}
	quoted := make([]string, len(shown))
	for i, v := range shown {
		if v == "…" {
			quoted[i] = "…"
			continue
		}
		quoted[i] = "`" + v + "`"
	}
	return "_enum:_ " + strings.Join(quoted, ", ")
}

// mdInline flattens a description to a single line so markdown bullet entries
// don't break into multiple paragraphs mid-list.
func mdInline(s string) string {
	s = strings.TrimSpace(s)
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// nonNil returns s or a zero-value schema so callers can safely deref fields
// (Enum, OneOf, Properties) without a nil check every time.
func nonNil(s *base.Schema) *base.Schema {
	if s == nil {
		return &base.Schema{}
	}
	return s
}
