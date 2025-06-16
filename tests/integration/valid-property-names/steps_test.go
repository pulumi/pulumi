// Copyright 2023-2024, Pulumi Corporation.
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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func getResource(stackInfo integration.RuntimeValidationStackInfo, name string) (apitype.ResourceV3, error) {
	for _, res := range stackInfo.Deployment.Resources {
		if res.URN.Name() == name {
			return res, nil
		}
	}
	return apitype.ResourceV3{}, fmt.Errorf("resource with name `%s` not found", name)
}

// TestPropertyNameDiffs checks that property names that look like invalid property paths
// do not break diff generation.
func TestPropertyNameDiffs(t *testing.T) {
	t.Parallel()

	validPropertyNames := []string{
		"foo",
		"example.com",
		".",

		".[0]",                      // Regression         v3.90.1
		"foo.[0].bar",               // Regression         v3.90.1
		`.["Hello, World!"]`,        // Regression         v3.90.1
		"[",                         // Regression v3.89.0 v3.90.1
		".[]",                       // Regression v3.89.0 v3.90.1
		"[]",                        // Regression v3.89.0 v3.90.1
		".[Hello, Unquoted World!]", // Regression v3.89.0 v3.90.1
		`.H[ello, World!"]`,         // Regression v3.89.0 v3.90.1
	}
	//nolint:paralleltest // ProgramTest calls t.Parallel()
	for _, propName := range validPropertyNames {
		propName := propName
		t.Run("validate path "+propName, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:          "step1",
				Dependencies: []string{"@pulumi/pulumi"},
				Config: map[string]string{
					"propertyName": propName,
				},
				Quick: true,
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					require.NotNil(t, stackInfo.Deployment)
					res, err := getResource(stackInfo, "a")
					assert.NoError(t, err)
					state := res.Outputs["state"].(map[string]interface{})
					assert.Equal(t, "foo", state[propName])
				},
				EditDirs: []integration.EditDir{
					{
						Dir:      "step2",
						Additive: true,
						ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
							require.NotNil(t, stackInfo.Deployment)
							_, err := getResource(stackInfo, "a")
							assert.NoError(t, err)
						},
					},
				},
			})
		})
	}
}
