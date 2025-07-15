// Copyright 2025, Pulumi Corporation.
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

//go:build go || all

package ints

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/require"
)

// This test ensures that we do not proceed to deletions if a program throws an error.
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestProgramErrorGo(t *testing.T) {
	d := "go"

	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join(d, "step1"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v3",
		},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
		},
		Quick: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			require.Len(t, stack.Deployment.Resources, 4)
			require.Equal(t, stack.Deployment.Resources[0].Type, tokens.Type("pulumi:pulumi:Stack"))
			require.Equal(t, stack.Deployment.Resources[1].Type, tokens.Type("pulumi:providers:testprovider"))
			require.Equal(t, stack.Deployment.Resources[2].Type, tokens.Type("testprovider:index:Random"))
			require.Equal(t, stack.Deployment.Resources[3].Type, tokens.Type("testprovider:index:Random"))
		},
		EditDirs: []integration.EditDir{
			{
				Dir:           filepath.Join(d, "step2"),
				Additive:      true,
				ExpectFailure: true,
				ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
					require.Len(t, stack.Deployment.Resources, 4)
					require.Equal(t, stack.Deployment.Resources[0].Type, tokens.Type("pulumi:pulumi:Stack"))
					require.Equal(t, stack.Deployment.Resources[1].Type, tokens.Type("pulumi:providers:testprovider"))
					require.Equal(t, stack.Deployment.Resources[2].Type, tokens.Type("testprovider:index:Random"))
					require.Equal(t, stack.Deployment.Resources[3].Type, tokens.Type("testprovider:index:Random"))
				},
			},
		},
	})
}
