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

package ints

import (
	"math/rand/v2"
	"os"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/require"
)

func TestPackagePublishLifecycle(t *testing.T) {
	t.Parallel()

	name := "test-publish" + randomSuffix()
	e := ptesting.NewEnvironment(t)
	e.WriteTestFile("schema.json", `{
    "name": "`+name+`",
    "version": "1.2.3"
}`)
	org := os.Getenv("PULUMI_TEST_ORG")
	require.NotEmpty(t, org, "Missing PULUMI_TEST_ORG")
	e.WriteTestFile("README.md", "# test-publish\n")
	e.RunCommand("pulumi", "package", "publish", "./schema.json", "--readme", "./README.md", "--publisher", org)
	e.RunCommand("pulumi", "package", "delete", "--yes", // non-interactive mode requires --yes flag
		org+"/"+name+"@1.2.3")
}

func randomSuffix() string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, 7)
	result[0] = '-'
	for i := range result[1:] {
		result[i+1] = letters[rand.IntN(len(letters))] //nolint:gosec
	}
	return string(result)
}
