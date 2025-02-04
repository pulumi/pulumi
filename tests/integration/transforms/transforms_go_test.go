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

//go:build (go || all)

package ints

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

func TestGoTransformations(t *testing.T) {
	t.Parallel()

	//nolint:paralleltest // ProgramTest calls t.Parallel()
	for _, dir := range Dirs {
		d := filepath.Join("go", dir)
		t.Run(d, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir: d,
				Dependencies: []string{
					"github.com/pulumi/pulumi/sdk/v3",
				},
				LocalProviders: []integration.LocalDependency{
					{Package: "testprovider", Path: filepath.Join("..", "..", "testprovider")},
				},
				Quick:                  true,
				ExtraRuntimeValidation: Validator,
			})
		})
	}
}
