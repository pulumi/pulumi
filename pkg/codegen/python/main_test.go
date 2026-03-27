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

package python

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Skip all tests in Bazel environment - these tests require Python toolchain
	// for setting up virtual environments which isn't available in Bazel's sandbox
	if os.Getenv("BAZEL_TEST") != "" || os.Getenv("TEST_SRCDIR") != "" {
		fmt.Println("Skipping python codegen tests in Bazel environment")
		os.Exit(0)
	}
	os.Exit(m.Run())
}
