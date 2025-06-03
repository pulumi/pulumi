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

//go:build all

package automation

import (
	"path/filepath"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

// TestShimlessInline tests that shimless can be used with the automation API.
func TestShimlessInline(t *testing.T) {
	t.Parallel()
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	e.CWD = filepath.Join("testdata", "goauto")
	e.RunCommand("go", "run", ".")
}
