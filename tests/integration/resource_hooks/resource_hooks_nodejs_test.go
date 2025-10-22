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

//go:build nodejs || all

package ints

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNodejsResourceHooks(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("nodejs", "step-1"),
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
		},
		Quick: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			requirePrinted(t, stack, "info", "beforeCreate was called with length = 10")
			requirePrinted(t, stack, "info", "funComp was called with child")
		},
		EditDirs: []integration.EditDir{{
			Additive: true,
			Dir:      filepath.Join("nodejs", "step-2"),
			ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
				requirePrinted(t, stack, "info", "beforeDelete was called with length = 10")
			},
		}},
	})
}

// Test that a transform can modify resource hooks
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNodejsResourceHooksTransform(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "nodejs_transform",
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
		},
		Quick: true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			text := "fun was called with length = 10"
			found := false
			textComp := "fun_comp was called with child"
			foundComp := false
			for _, event := range stackInfo.Events {
				if event.DiagnosticEvent != nil {
					if strings.Contains(event.DiagnosticEvent.Message, text) {
						found = true
					}
					if strings.Contains(event.DiagnosticEvent.Message, textComp) {
						foundComp = true
					}
				}
			}
			b, err := json.Marshal(stackInfo.Events)
			require.NoError(t, err)
			require.True(t, found, "expected hook to print a message for the resource, got: %s", b)
			require.True(t, foundComp, "expected hook to print a message for the component, got: %s", b)
		},
	})
}

// Test that hooks receives secrets as secret outputs
//
//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestNodejsResourceHooksSecrets(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "nodejs_secret",
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
		},
		Quick: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			requirePrinted(t, stack, "info", "hook called")
		},
	})
}
