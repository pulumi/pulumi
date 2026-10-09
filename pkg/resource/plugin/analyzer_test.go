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

package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestAttributeStackDiagnostics(t *testing.T) {
	t.Parallel()

	const (
		rootURN = resource.URN("urn:pulumi:prod::web::pulumi:pulumi:Stack::web-prod")
		resURN  = resource.URN("urn:pulumi:prod::web::pkg:index:MyResource::res")
		goneURN = resource.URN("urn:pulumi:prod::web::pkg:index:MyResource::gone")
	)
	stackResource := func(urn resource.URN) AnalyzerStackResource {
		return AnalyzerStackResource{AnalyzerResource: AnalyzerResource{URN: urn, Type: urn.Type()}}
	}
	attribute := func(resources []AnalyzerStackResource, urns ...resource.URN) []resource.URN {
		diags := make([]AnalyzeDiagnostic, len(urns))
		for i, u := range urns {
			diags[i].URN = u
		}
		AttributeStackDiagnostics(resources, diags)
		result := make([]resource.URN, len(diags))
		for i, d := range diags {
			result[i] = d.URN
		}
		return result
	}

	withRoot := []AnalyzerStackResource{stackResource(rootURN), stackResource(resURN)}
	assert.Equal(t,
		[]resource.URN{resURN, rootURN, rootURN, rootURN},
		attribute(withRoot, resURN, "", goneURN, "not-a-urn"),
		"a URN in the stack is kept; anything else goes to the root stack")

	// The root stack URN is derived from the stack's project and name, whatever the root stack resource is called.
	oddRoot := resource.URN("urn:pulumi:prod::web::pulumi:pulumi:Stack::custom")
	assert.Equal(t,
		[]resource.URN{rootURN},
		attribute([]AnalyzerStackResource{stackResource(oddRoot), stackResource(resURN)}, ""))
	assert.Equal(t,
		[]resource.URN{rootURN},
		attribute([]AnalyzerStackResource{stackResource(resURN)}, ""))

	// Without resources, there's nothing to attribute to.
	assert.Equal(t, []resource.URN{""}, attribute(nil, ""))
}
