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
		Dir:          "nodejs",
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
		},
		Quick: true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			text := "fun was called with length = 10"
			found := false
			textComp := "funComp was called with child"
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
			require.True(t, found, "expected 'hook_fun' to print a message, got: %s", b)
			require.True(t, foundComp, "expected 'hook_fun_comp' to print a message, got: %s", b)
		},
	})
}
