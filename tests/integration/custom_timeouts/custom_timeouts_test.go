// Copyright 2020-2024, Pulumi Corporation.
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

//go:build (python || all)

package ints

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
)

func TestCustomTimeouts(t *testing.T) {
	t.Parallel()

	opts := &integration.ProgramTestOptions{
		Dir: filepath.Join(".", "python", "success"),
		Dependencies: []string{
			filepath.Join("..", "..", "..", "sdk", "python"),
		},
		Quick:            true,
		DestroyOnCleanup: true,
	}
	integration.ProgramTest(t, opts)

	opts = &integration.ProgramTestOptions{
		Dir: filepath.Join(".", "python", "failure"),
		Dependencies: []string{
			filepath.Join("..", "..", "..", "sdk", "python"),
		},
		Quick:            true,
		DestroyOnCleanup: true,
		ExpectFailure:    true,
	}
	integration.ProgramTest(t, opts)
}
