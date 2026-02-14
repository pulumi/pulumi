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
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
)

// loadTestModel parses the embedded spec and returns the renderer and component schemas.
func loadTestModel(t *testing.T) *schemaRenderer {
	t.Helper()
	doc, err := libopenapi.NewDocument(openAPISpecJSON)
	if err != nil {
		t.Fatalf("parsing OpenAPI doc: %v", err)
	}
	v3Model, errs := doc.BuildV3Model()
	if errs != nil && v3Model == nil {
		t.Fatalf("building v3 model: %v", errs)
	}
	return newSchemaRenderer(v3Model.Model.Components.Schemas)
}

// getComponentSchema resolves a named component schema from the embedded spec.
func getComponentSchema(t *testing.T, renderer *schemaRenderer, name string) string {
	t.Helper()
	proxy, ok := renderer.components.Get(name)
	if !ok || proxy == nil {
		t.Fatalf("schema %q not found in components", name)
	}
	schema := proxy.Schema()
	if schema == nil {
		t.Fatalf("schema %q resolved to nil", name)
	}
	return renderer.renderBodySchema(schema)
}

// TestDiscriminatorVariantsShowLiteralType tests that discriminated union variants
// rendered via renderDiscriminator show literal type values (e.g. type*: "policy_issue")
// instead of generic type*: (string).
func TestDiscriminatorVariantsShowLiteralType(t *testing.T) {
	renderer := loadTestModel(t)

	// AgentEntity has discriminator mapping: policy_issue, pull_request, repository, stack.
	// When rendered, each variant should show type*: "variant_name".
	output := getComponentSchema(t, renderer, "AgentEntity")
	t.Log(output)

	for _, variant := range []string{"policy_issue", "pull_request", "repository", "stack"} {
		expected := `type*: "` + variant + `"`
		if !strings.Contains(output, expected) {
			t.Errorf("expected discriminator variant %q in output, got:\n%s", expected, output)
		}
	}

	// Should NOT contain a generic type*: (string) at the variant property level.
	// (The only type*: (string) should be inside nested objects, not at the variant level.)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "type*: (string)" {
			t.Errorf("found generic 'type*: (string)' at variant level, expected literal values")
		}
	}
}

// TestDiscriminatorNestedRefOmitsType tests that when a variant (e.g. AgentEntityPR)
// has a nested reference to another variant (e.g. repo → AgentEntityRepository),
// the nested object does NOT show the inherited discriminator "type" property.
func TestDiscriminatorNestedRefOmitsType(t *testing.T) {
	renderer := loadTestModel(t)
	output := getComponentSchema(t, renderer, "AgentEntity")
	t.Log(output)

	// Find the pull_request variant section and check that "repo" doesn't contain type.
	inPR := false
	inRepo := false
	repoDepth := 0
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, `type*: "pull_request"`) {
			inPR = true
			continue
		}
		if inPR && strings.HasPrefix(trimmed, "repo") && strings.Contains(trimmed, "{") {
			inRepo = true
			repoDepth = 1
			continue
		}
		if inRepo {
			if strings.Contains(trimmed, "{") {
				repoDepth++
			}
			if strings.Contains(trimmed, "}") {
				repoDepth--
				if repoDepth == 0 {
					inRepo = false
					inPR = false
					continue
				}
			}
			if strings.HasPrefix(trimmed, "type") {
				t.Errorf("nested repo in pull_request variant should not have 'type' property, found: %s", trimmed)
			}
		}
	}
}

// TestAllOfMergePreservesDiscriminatorProperty tests that when a schema uses allOf
// to extend a discriminator base (like AgentUserEventMessage extends AgentUserEvent),
// the discriminator property ("type") is preserved in the rendered output.
// This is the "root" case — the schema is used directly, not as a variant in renderDiscriminator.
func TestAllOfMergePreservesDiscriminatorProperty(t *testing.T) {
	renderer := loadTestModel(t)

	// AgentUserEventMessage extends AgentUserEvent (which has discriminator on "type"
	// with mapping keys: user_cancel, user_confirmation, user_message).
	// When rendered as a standalone schema (e.g. as a request body field),
	// the "type" property should be visible at the root level AND should show
	// the allowed discriminator values.
	output := getComponentSchema(t, renderer, "AgentUserEventMessage")
	t.Log(output)

	// Check that "type" appears as a root-level property (at 4-space indent,
	// since renderBodySchema uses indent=2 and properties are at indent+2=4)
	// with the discriminator's allowed values.
	found := false
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "    type") && !strings.HasPrefix(line, "      ") {
			found = true
			// Should show the discriminator mapping keys as allowed values.
			if !strings.Contains(line, "one of:") {
				t.Errorf("root-level 'type' should show discriminator values, got: %s", line)
			}
			for _, val := range []string{"user_cancel", "user_confirmation", "user_message"} {
				if !strings.Contains(line, val) {
					t.Errorf("root-level 'type' should include value %q, got: %s", val, line)
				}
			}
			break
		}
	}
	if !found {
		t.Errorf("expected root-level 'type' property in AgentUserEventMessage output, got:\n%s", output)
	}
}
