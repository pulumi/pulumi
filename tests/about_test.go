// Copyright 2016-2021, Pulumi Corporation.
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

package tests

import (
	"encoding/json"
	"regexp"
	"runtime"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/assert"
)

func TestAboutCommands(t *testing.T) {
	t.Parallel()

	// pulumi about --json
	t.Run("json", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		integration.CreateBasicPulumiRepo(e)
		e.SetBackend(e.LocalURL())
		stdout, _ := e.RunCommand("pulumi", "about", "--json")
		var res interface{}
		assert.NoError(t, json.Unmarshal([]byte(stdout), &res), "Should be valid json")
		assert.Contains(t, stdout, runtimeMajorMinor())
		assert.Contains(t, stdout, runtime.Compiler)
		assert.Contains(t, stdout, "Failed to get information about the current stack:")
	})

	// pulumi about
	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		integration.CreateBasicPulumiRepo(e)
		e.SetBackend(e.LocalURL())
		stdout, _ := e.RunCommand("pulumi", "about")
		assert.Contains(t, stdout, runtimeMajorMinor())
		assert.Contains(t, stdout, runtime.Compiler)
	})
}

// Given a runtime version like "go1.17.123", returns "go1.17.", trimming patch and prerelease
// values.
func runtimeMajorMinor() string {
	re := regexp.MustCompile(`go\d+.\d+.`)
	return re.FindString(runtime.Version())
}
