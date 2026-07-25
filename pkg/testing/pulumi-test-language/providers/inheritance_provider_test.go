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

package providers

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInheritanceAbstractDirectConstructRejected verifies the host-level abstract guard: a direct Construct of an
// abstract component type fails with the pinned error, regardless of any language-level guard. The host check is the
// source of truth because old consumer SDKs and Go embedding can bypass the language-level guard, and generated
// conformance programs cannot reach this path (codegen makes the abstract type non-constructable), so it is asserted
// here directly against the provider.
func TestInheritanceAbstractDirectConstructRejected(t *testing.T) {
	t.Parallel()

	p := &InheritanceAbstractProvider{}
	_, err := p.Construct(t.Context(), plugin.ConstructRequest{
		Type: "inheritabstract:index:AbstractBase",
		Name: "abstract",
		Inputs: resource.PropertyMap{
			"seed": resource.NewProperty("x"),
		},
	})
	require.Error(t, err)
	assert.EqualError(t, err,
		"type 'inheritabstract:index:AbstractBase' is abstract and cannot be instantiated directly")
}
