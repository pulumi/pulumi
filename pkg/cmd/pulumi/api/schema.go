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

package api

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
	"go.yaml.in/yaml/v4"
)

// schemaRenderer renders libopenapi schemas as Restish-style human-readable text.
type schemaRenderer struct {
	// components holds the model's component schemas for resolving discriminator mappings.
	components *orderedmap.Map[string, *base.SchemaProxy]
	// stripDiscrimProps tracks discriminator property names to strip from nested allOf merges.
	// Set by renderDiscriminator so that nested variant refs (e.g. repo → AgentEntityRepository)
	// don't show the inherited discriminator property.
	stripDiscrimProps map[string]bool
	// allOfDiscrim is set temporarily when processing allOf schemas that inherit from a
	// discriminator base. renderObject uses it to show allowed values inline.
	allOfDiscrim *allOfDiscrimInfo
}

type allOfDiscrimInfo struct {
	propName string
	values   []string
}

func newSchemaRenderer(components *orderedmap.Map[string, *base.SchemaProxy]) *schemaRenderer {
	return &schemaRenderer{components: components}
}

// renderBodySchema renders a request body schema with heading.
func (r *schemaRenderer) renderBodySchema(s *base.Schema) string {
	visited := make(map[uint64]bool)
	body := r.renderValue(s, 2, 0, visited)
	return "Request Body:\n" + body
}

// renderResponseSchema renders a response schema with heading.
func (r *schemaRenderer) renderResponseSchema(s *base.Schema) string {
	visited := make(map[uint64]bool)
	body := r.renderValue(s, 2, 0, visited)
	return "Response:\n" + body
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
func (r *schemaRenderer) renderValue(s *base.Schema, indent, depth int, visited map[uint64]bool) string {
	if s == nil {
		return spad(indent) + "(<any>)"
	}

	// Circular reference detection via low-level hash.
	h := schemaHash(s)
	if h != 0 {
		if visited[h] {
			return spad(indent) + "(circular)"
		}
		visited[h] = true
		defer delete(visited, h)
	}

	// Handle allOf — merge into a single object.
	if len(s.AllOf) > 0 {
		merged := r.mergeAllOf(s.AllOf, visited)
		if merged != nil {
			// If an allOf member defines a discriminator, extract mapping keys so
			// renderObject can show allowed values for the inherited property.
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
					prev := r.allOfDiscrim
					r.allOfDiscrim = &allOfDiscrimInfo{
						propName: sub.Discriminator.PropertyName,
						values:   vals,
					}
					defer func() { r.allOfDiscrim = prev }()
					break
				}
			}
			// Strip inherited discriminator properties when rendering inside
			// a renderDiscriminator context (e.g. repo inside pull_request variant
			// should not show the "type" property inherited from the base union).
			if len(r.stripDiscrimProps) > 0 && merged.Properties != nil {
				for prop := range r.stripDiscrimProps {
					merged.Properties.Delete(prop)
				}
			}
			return r.renderValue(merged, indent, depth, visited)
		}
	}

	// Handle oneOf / anyOf — render each variant.
	if len(s.OneOf) > 0 {
		return r.renderPolymorph("oneOf", s.OneOf, indent, depth, visited)
	}
	if len(s.AnyOf) > 0 {
		return r.renderPolymorph("anyOf", s.AnyOf, indent, depth, visited)
	}

	// Handle discriminated union: object with discriminator mapping but no oneOf.
	// Resolve the mapping refs from component schemas and render as oneOf.
	if s.Discriminator != nil && s.Discriminator.Mapping != nil && len(s.OneOf) == 0 && r.components != nil {
		return r.renderDiscriminator(s, indent, depth, visited)
	}

	typ := schemaType(s)

	// Handle array.
	if typ == "array" {
		return r.renderArray(s, indent, depth, visited)
	}

	// Handle object.
	if typ == "object" || orderedmap.Len(s.Properties) > 0 {
		if orderedmap.Len(s.Properties) == 0 {
			if s.AdditionalProperties != nil && s.AdditionalProperties.IsA() && s.AdditionalProperties.A != nil {
				addSchema := s.AdditionalProperties.A.Schema()
				if addSchema != nil {
					val := r.renderValue(addSchema, indent+2, depth+1, visited)
					return spad(indent) + "{\n" + spad(indent+2) + "<string>: " +
						strings.TrimLeft(val, " ") + "\n" + spad(indent) + "}"
				}
			}
			return spad(indent) + "(object)"
		}
		return r.renderObject(s, indent, depth, visited)
	}

	// Scalar type.
	return spad(indent) + r.typeTag(s)
}

// renderArray renders an array schema.
func (r *schemaRenderer) renderArray(s *base.Schema, indent, depth int, visited map[uint64]bool) string {
	if s.Items != nil && s.Items.IsA() && s.Items.A != nil {
		itemSchema := s.Items.A.Schema()
		if itemSchema != nil {
			item := r.renderValue(itemSchema, indent+2, depth, visited)
			return spad(indent) + "[\n" + item + "\n" + spad(indent) + "]"
		}
	}
	return spad(indent) + "[<any>]"
}

// renderPolymorph renders oneOf/anyOf variants.
func (r *schemaRenderer) renderPolymorph(label string, proxies []*base.SchemaProxy, indent, depth int, visited map[uint64]bool) string {
	var lines []string
	lines = append(lines, spad(indent)+label+" {")
	for _, proxy := range proxies {
		if proxy == nil {
			continue
		}
		resolved := proxy.Schema()
		if resolved == nil {
			continue
		}
		val := r.renderValue(resolved, indent+2, depth+1, visited)
		lines = append(lines, val)
	}
	lines = append(lines, spad(indent)+"}")
	return strings.Join(lines, "\n")
}

// renderDiscriminator resolves a discriminator mapping and renders each variant.
// This handles the case where a schema has a discriminator with mapping but no oneOf.
func (r *schemaRenderer) renderDiscriminator(s *base.Schema, indent, depth int, visited map[uint64]bool) string {
	var lines []string
	lines = append(lines, spad(indent)+"oneOf {")

	propName := s.Discriminator.PropertyName
	// Tell nested renderValue calls to strip this discriminator property from allOf merges.
	// This prevents inherited discriminator properties from showing up in nested variant refs
	// (e.g. repo → AgentEntityRepository should not show "type" from AgentEntity).
	if propName != "" {
		if r.stripDiscrimProps == nil {
			r.stripDiscrimProps = make(map[string]bool)
		}
		r.stripDiscrimProps[propName] = true
		defer delete(r.stripDiscrimProps, propName)
	}
	for discVal, ref := range s.Discriminator.Mapping.FromOldest() {
		// Extract schema name from $ref like "#/components/schemas/AgentEntityPR".
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

		// Merge allOf to get the complete variant schema.
		if len(variantSchema.AllOf) > 0 {
			variantSchema = r.mergeAllOf(variantSchema.AllOf, visited)
			if variantSchema == nil {
				continue
			}
		}

		// Clear discriminator on a shallow copy to prevent infinite re-resolution,
		// and remove the discriminator property itself (we inject the literal value instead).
		schemaCopy := *variantSchema
		schemaCopy.Discriminator = nil
		if propName != "" && schemaCopy.Properties != nil {
			schemaCopy.Properties.Delete(propName)
			// Also remove from Required since we show it via the injected line.
			filtered := schemaCopy.Required[:0]
			for _, req := range schemaCopy.Required {
				if req != propName {
					filtered = append(filtered, req)
				}
			}
			schemaCopy.Required = filtered
		}

		val := r.renderValue(&schemaCopy, indent+2, depth+1, visited)

		// Inject the discriminator value as the first property line.
		if propName != "" && discVal != "" {
			discLine := spad(indent+4) + propName + "*: \"" + discVal + "\""
			if idx := strings.Index(val, "{\n"); idx >= 0 {
				val = val[:idx+2] + discLine + "\n" + val[idx+2:]
			}
		}

		lines = append(lines, val)
	}

	lines = append(lines, spad(indent)+"}")
	return strings.Join(lines, "\n")
}

// renderObject renders an object schema with its properties.
func (r *schemaRenderer) renderObject(s *base.Schema, indent, depth int, visited map[uint64]bool) string {
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
	} else if r.allOfDiscrim != nil {
		discrimProp = r.allOfDiscrim.propName
		discrimValues = r.allOfDiscrim.values
		r.allOfDiscrim = nil // consume: only applies to the immediate merged schema
	}

	props := sortedProps(s)
	var lines []string

	for _, p := range props {
		if p.schema != nil {
			// Skip readOnly fields in request context, writeOnly in response context.
			if p.schema.ReadOnly != nil && *p.schema.ReadOnly {
				continue
			}
		}

		mark := ""
		if reqSet[p.name] {
			mark = "*"
		}

		// If this property is the discriminator, override with enum of mapping keys.
		if p.name == discrimProp && len(discrimValues) > 0 {
			vals := discrimValues
			if len(vals) > 6 {
				vals = append(vals[:6:6], "...")
			}
			valueStr := "(string) one of: " + strings.Join(vals, ", ")
			line := spad(indent+2) + p.name + mark + ": " + valueStr
			if p.schema != nil && p.schema.Description != "" {
				line += " " + truncateDesc(p.schema.Description)
			}
			lines = append(lines, line)
			continue
		}

		// Render the property value, then strip leading indent so it follows ": "
		valueStr := r.renderValue(p.schema, indent+2, depth+1, visited)
		valueStr = strings.TrimLeft(valueStr, " ")

		hasDesc := p.schema != nil && p.schema.Description != ""
		isMultiLine := strings.Contains(valueStr, "\n")

		var line string
		if isMultiLine && hasDesc {
			// For multi-line values (objects, arrays), show description before the value.
			line = spad(indent+2) + p.name + mark + ": " + truncateDesc(p.schema.Description) + " " + valueStr
		} else {
			line = spad(indent+2) + p.name + mark + ": " + valueStr
			if hasDesc {
				line += " " + truncateDesc(p.schema.Description)
			}
		}

		lines = append(lines, line)
	}

	return spad(indent) + "{\n" + strings.Join(lines, "\n") + "\n" + spad(indent) + "}"
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

	// Nullable annotation.
	for _, t := range s.Type {
		if t == "null" {
			parts = append(parts, "nullable")
			break
		}
	}
	if s.Nullable != nil && *s.Nullable {
		parts = append(parts, "nullable")
	}

	// Validation constraints.
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

		// Recursively merge nested allOf.
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

// Helper types and functions.

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

func truncateDesc(desc string) string {
	// Remove newlines.
	desc = strings.ReplaceAll(desc, "\n", " ")
	// Take first sentence if short enough.
	if idx := strings.Index(desc, ". "); idx > 0 && idx < 80 {
		return desc[:idx+1]
	}
	if len(desc) > 100 {
		return desc[:97] + "..."
	}
	return desc
}

