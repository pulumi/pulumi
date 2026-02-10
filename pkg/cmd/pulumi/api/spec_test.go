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
)

func TestDeduplicatePreview(t *testing.T) {
	ops := []OperationSpec{
		{OperationID: "a", Method: "GET", Path: "/api/esc/env/{org}/{env}"},
		{OperationID: "a-preview", Method: "GET", Path: "/api/preview/esc/env/{org}/{env}"},
		{OperationID: "b", Method: "POST", Path: "/api/preview/insights/{org}/resources"},
	}
	result := deduplicatePreview(ops)
	if len(result) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(result))
	}
	if result[0].OperationID != "a" {
		t.Errorf("expected first op 'a', got %q", result[0].OperationID)
	}
	// "b" has no non-preview equivalent, so it's kept.
	if result[1].OperationID != "b" {
		t.Errorf("expected second op 'b', got %q", result[1].OperationID)
	}
}

func TestTryLiteralMerge(t *testing.T) {
	ops := []OperationSpec{
		{
			OperationID: "op-destroy", Method: "POST",
			Path: "/api/stacks/{org}/{stack}/destroy/{updateID}/cancel",
			Params: []ParamSpec{
				{Name: "org", In: "path", Required: true},
				{Name: "stack", In: "path", Required: true},
				{Name: "updateID", In: "path", Required: true},
			},
		},
		{
			OperationID: "op-preview", Method: "POST",
			Path: "/api/stacks/{org}/{stack}/preview/{updateID}/cancel",
			Params: []ParamSpec{
				{Name: "org", In: "path", Required: true},
				{Name: "stack", In: "path", Required: true},
				{Name: "updateID", In: "path", Required: true},
			},
		},
		{
			OperationID: "op-update", Method: "POST",
			Path: "/api/stacks/{org}/{stack}/update/{updateID}/cancel",
			Params: []ParamSpec{
				{Name: "org", In: "path", Required: true},
				{Name: "stack", In: "path", Required: true},
				{Name: "updateID", In: "path", Required: true},
			},
		},
	}

	segs := make([][]string, len(ops))
	for i, op := range ops {
		segs[i] = strings.Split(op.Path, "/")
	}

	merged, ok := tryLiteralMerge(ops, segs)
	if !ok {
		t.Fatal("tryLiteralMerge should succeed")
	}
	if !strings.Contains(merged.Path, "{update-kind}") {
		t.Errorf("merged path should contain {update-kind}, got %q", merged.Path)
	}

	var enumParam *ParamSpec
	for i, p := range merged.Params {
		if p.Name == "update-kind" {
			enumParam = &merged.Params[i]
			break
		}
	}
	if enumParam == nil {
		t.Fatal("merged should have update-kind param")
	}
	if len(enumParam.Values) != 3 {
		t.Errorf("expected 3 values, got %v", enumParam.Values)
	}
	if enumParam.Values[0] != "destroy" || enumParam.Values[1] != "preview" || enumParam.Values[2] != "update" {
		t.Errorf("unexpected values: %v", enumParam.Values)
	}
}

func TestTryParamMerge(t *testing.T) {
	ops := []OperationSpec{
		{
			OperationID: "op-base", Method: "GET",
			Path: "/api/env/{org}/{env}",
			Params: []ParamSpec{
				{Name: "org", In: "path", Required: true},
				{Name: "env", In: "path", Required: true},
			},
			ResponseContentType: "application/x-yaml",
		},
		{
			OperationID: "op-versioned", Method: "GET",
			Path: "/api/env/{org}/{env}/versions/{version}",
			Params: []ParamSpec{
				{Name: "org", In: "path", Required: true},
				{Name: "env", In: "path", Required: true},
				{Name: "version", In: "path", Required: true},
			},
			ResponseContentType: "application/x-yaml",
		},
	}

	segs := make([][]string, len(ops))
	for i, op := range ops {
		segs[i] = strings.Split(op.Path, "/")
	}

	merged, ok := tryParamMerge(ops, segs)
	if !ok {
		t.Fatal("tryParamMerge should succeed")
	}
	if len(merged.PathVariants) != 2 {
		t.Fatalf("expected 2 PathVariants, got %d", len(merged.PathVariants))
	}
	// Longest first
	if !strings.Contains(merged.PathVariants[0].Path, "{version}") {
		t.Errorf("first variant should be longer path, got %q", merged.PathVariants[0].Path)
	}

	// version param should be optional
	var versionParam *ParamSpec
	for i, p := range merged.Params {
		if p.Name == "version" {
			versionParam = &merged.Params[i]
			break
		}
	}
	if versionParam == nil {
		t.Fatal("merged should have version param")
	}
	if versionParam.Required {
		t.Error("version param should not be required")
	}
}

func TestSelectVariant(t *testing.T) {
	variants := []PathVariant{
		{Path: "/api/env/{org}/{env}/versions/{version}"},
		{Path: "/api/env/{org}/{env}"},
	}

	// With version provided: should select longer path.
	v, err := selectVariant(variants, map[string]string{
		"org": "myorg", "env": "myenv", "version": "3",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Path != variants[0].Path {
		t.Errorf("expected longer variant, got %q", v.Path)
	}

	// Without version: should select shorter path.
	v, err = selectVariant(variants, map[string]string{
		"org": "myorg", "env": "myenv",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Path != variants[1].Path {
		t.Errorf("expected shorter variant, got %q", v.Path)
	}

	// Without required params: should error.
	_, err = selectVariant(variants, map[string]string{"org": "myorg"})
	if err == nil {
		t.Error("expected error when no variant resolves")
	}
}

func TestToKebab(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ListGates", "list-gates"},
		{"GetAITemplate", "get-ai-template"},
		{"AITemplate", "ai-template"},
		{"AWSSSOListAccounts", "aws-sso-list-accounts"},
		{"AWSSetup", "aws-setup"},
		{"GetSAMLOrganization", "get-saml-organization"},
		{"CreateScheduledTTLDeployment", "create-scheduled-ttl-deployment"},
		{"AddOrganizationMember", "add-organization-member"},
		{"CancelUpdate_destroy", "cancel-update-destroy"},
		{"Capabilities", "capabilities"},
		{"Apply", "apply"},
		{"CheckYAML", "check-yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toKebab(tt.input)
			if got != tt.want {
				t.Errorf("toKebab(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTagToSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"AI", "ai"},
		{"AI Agents", "ai-agents"},
		{"CloudSetup", "cloud-setup"},
		{"Deployments", "deployments"},
		{"Environments", "environments"},
		{"Insights", "insights"},
		{"Miscellaneous", "miscellaneous"},
		{"Organizations", "organizations"},
		{"Registry", "registry"},
		{"RegistryPreview", "registry-preview"},
		{"Stacks", "stacks"},
		{"Users", "users"},
		{"Workflows", "workflows"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := tagToSlug(tt.input)
			if got != tt.want {
				t.Errorf("tagToSlug(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseEmbeddedSpec(t *testing.T) {
	groups, err := parseEmbeddedSpec()
	if err != nil {
		t.Fatalf("parseEmbeddedSpec failed: %v", err)
	}

	if got := len(groups); got != 13 {
		t.Errorf("expected 13 tag groups, got %d", got)
		for _, g := range groups {
			t.Logf("  tag: %q (slug: %q, ops: %d)", g.Name, g.Slug, len(g.Operations))
		}
	}

	totalOps := 0
	for _, g := range groups {
		totalOps += len(g.Operations)
	}
	if totalOps != 431 {
		t.Errorf("expected 431 operations, got %d", totalOps)
	}

	// Verify tags are sorted.
	for i := 1; i < len(groups); i++ {
		if groups[i-1].Name >= groups[i].Name {
			t.Errorf("tags not sorted: %q >= %q", groups[i-1].Name, groups[i].Name)
		}
	}

	// Spot-check: find an operation with a body and verify schema rendering.
	var found bool
	for _, g := range groups {
		for _, op := range g.Operations {
			if op.OperationID == "ai-template" {
				found = true
				if op.CommandName != "ai-template" {
					t.Errorf("ai-template CommandName = %q, want %q", op.CommandName, "ai-template")
				}
				if !op.HasBody {
					t.Error("ai-template should have HasBody=true")
				}
				if op.BodyContentType != "application/json" {
					t.Errorf("ai-template BodyContentType = %q, want %q", op.BodyContentType, "application/json")
				}
				if !strings.HasPrefix(op.BodySchemaText, "Request Body:") {
					t.Errorf("ai-template BodySchemaText should start with 'Request Body:', got %q", op.BodySchemaText)
				}
				if !strings.Contains(op.BodySchemaText, "instructions") {
					t.Errorf("ai-template BodySchemaText should show 'instructions' field, got %q", op.BodySchemaText)
				}
			}
		}
	}
	if !found {
		t.Error("ai-template operation not found")
	}

	// Spot-check: find an operation with a response schema.
	for _, g := range groups {
		for _, op := range g.Operations {
			if op.OperationID == "list-change-gates" {
				if op.CommandName != "list-gates" {
					t.Errorf("list-change-gates CommandName = %q, want %q", op.CommandName, "list-gates")
				}
				if op.ResponseSchemaText == "" {
					t.Error("list-gates should have ResponseSchemaText")
				}
				if !strings.Contains(op.ResponseSchemaText, "gates") {
					t.Errorf("list-gates ResponseSchemaText should show 'gates' field, got %q", op.ResponseSchemaText)
				}
				break
			}
		}
	}

	// Verify command names are unique within each tag group.
	for _, g := range groups {
		seen := make(map[string]int)
		for _, op := range g.Operations {
			seen[op.CommandName]++
		}
		for name, count := range seen {
			if count > 1 {
				t.Errorf("tag %q has %d operations with CommandName %q", g.Name, count, name)
			}
		}
	}

	// Spot-check: summary-based names for unique operations.
	for _, g := range groups {
		for _, op := range g.Operations {
			if op.OperationID == "delete-environment-esc-environments" {
				if op.CommandName != "delete-environment" {
					t.Errorf("delete-environment-esc-environments CommandName = %q, want %q",
						op.CommandName, "delete-environment")
				}
			}
		}
	}

	// Every operation with a body should have a BodyContentType.
	for _, g := range groups {
		for _, op := range g.Operations {
			if op.HasBody && op.BodyContentType == "" {
				t.Errorf("operation %q has body but no BodyContentType", op.OperationID)
			}
		}
	}

	// Collect non-JSON content types for visibility.
	var nonJSONBody, nonJSONResponse int
	for _, g := range groups {
		for _, op := range g.Operations {
			if op.BodyContentType != "" && op.BodyContentType != "application/json" {
				nonJSONBody++
				t.Logf("non-JSON body: %s %s (%s)", op.Method, op.Path, op.BodyContentType)
			}
			if op.ResponseContentType != "" && op.ResponseContentType != "application/json" {
				nonJSONResponse++
				t.Logf("non-JSON response: %s %s (%s)", op.Method, op.Path, op.ResponseContentType)
			}
		}
	}
	t.Logf("non-JSON bodies: %d, non-JSON responses: %d", nonJSONBody, nonJSONResponse)

	// Spot-check: literal merge (stacks update-kind operations).
	for _, g := range groups {
		for _, op := range g.Operations {
			if op.CommandName == "cancel-update" {
				if !strings.Contains(op.Path, "{update-kind}") {
					t.Errorf("cancel-update should have {update-kind} in path, got %q", op.Path)
				}
				// Should have the synthetic enum param.
				var enumParam *ParamSpec
				for i, p := range op.Params {
					if p.Name == "update-kind" {
						enumParam = &op.Params[i]
						break
					}
				}
				if enumParam == nil {
					t.Error("cancel-update should have update-kind param")
				} else {
					if len(enumParam.Values) != 4 {
						t.Errorf("cancel-update update-kind should have 4 values, got %d", len(enumParam.Values))
					}
					if !enumParam.Required {
						t.Error("cancel-update update-kind should be required")
					}
				}
				if len(op.PathVariants) != 0 {
					t.Error("literal-merged cancel-update should not have PathVariants")
				}
				break
			}
		}
	}

	// Spot-check: param merge (environment version variants).
	for _, g := range groups {
		for _, op := range g.Operations {
			if op.CommandName == "read-environment" {
				if len(op.PathVariants) != 2 {
					t.Errorf("read-environment should have 2 PathVariants, got %d", len(op.PathVariants))
				}
				// First variant should be the longer path (with version).
				if len(op.PathVariants) == 2 {
					if !strings.Contains(op.PathVariants[0].Path, "{version}") {
						t.Errorf("read-environment first variant should contain {version}, got %q",
							op.PathVariants[0].Path)
					}
				}
				// version param should be optional.
				for _, p := range op.Params {
					if p.Name == "version" {
						if p.Required {
							t.Error("read-environment version param should not be required")
						}
					}
				}
				break
			}
		}
	}

	// Spot-check: param merge (get-deployment outlier with mutually exclusive params).
	for _, g := range groups {
		for _, op := range g.Operations {
			if op.CommandName == "get-deployment" {
				if len(op.PathVariants) != 2 {
					t.Errorf("get-deployment should have 2 PathVariants, got %d", len(op.PathVariants))
				}
				// Both deploymentId and version should be optional.
				var hasDeploymentId, hasVersion bool
				for _, p := range op.Params {
					if p.Name == "deploymentId" {
						hasDeploymentId = true
						if p.Required {
							t.Error("get-deployment deploymentId should not be required")
						}
					}
					if p.Name == "version" {
						hasVersion = true
						if p.Required {
							t.Error("get-deployment version should not be required")
						}
					}
				}
				if !hasDeploymentId || !hasVersion {
					t.Errorf("get-deployment should have both deploymentId and version params, got deploymentId=%v version=%v",
						hasDeploymentId, hasVersion)
				}
				break
			}
		}
	}

	// Spot-check: preview dedup (environment drafts).
	// After dedup, there should be no /api/preview/esc/ paths in Environments.
	for _, g := range groups {
		if g.Name != "Environments" {
			continue
		}
		for _, op := range g.Operations {
			if strings.HasPrefix(op.Path, "/api/preview/esc/") {
				t.Errorf("Environments should not have preview path %q after dedup", op.Path)
			}
		}
	}
}
