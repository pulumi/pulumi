// Copyright 2024, Pulumi Corporation.
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

//go:build nodejs || all

package ints

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNodejsSimpleTransforms(t *testing.T) {
	d := filepath.Join("nodejs", "simple")
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          d,
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
		},
		Quick:                  true,
		ExtraRuntimeValidation: Validator,
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNodejsSingleTransforms(t *testing.T) {
	d := filepath.Join("nodejs", "single")
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          d,
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
		},
		Quick: true,
	})
}

// Test that transforms work for a resource that is creted after we return from `stack.runInPulumiStack`.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNodejsTransformsAsyncResource(t *testing.T) {
	d := filepath.Join("nodejs", "async")
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          d,
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
		},
		Quick: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			res := stack.Deployment.Resources[2]
			require.Equal(t, res.URN.Name(), "res")
			length := res.Inputs["length"]
			require.Equal(t, 12.0, length)
		},
	})
}
