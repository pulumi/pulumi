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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v2/testing/integration"
	ptesting "github.com/pulumi/pulumi/sdk/v2/go/common/testing"
)

func TestConfigCommands(t *testing.T) {
	t.Run("SanityTest", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		integration.CreateBasicPulumiRepo(e)
		e.SetBackend(e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "test")

		// check config is empty
		stdout, _ := e.RunCommand("pulumi", "config")
		assert.Equal(t, "KEY  VALUE", strings.Trim(stdout, "\r\n"))

		// set a bunch of config
		e.RunCommand("pulumi", "config", "set-all",
			"--plaintext", "key1=value1",
			"--plaintext", "outer.inner=value2",
			"--secret", "my_token=my_secret_token",
			"--plaintext", "myList[0]=foo")

		// check that it all exists
		stdout, _ = e.RunCommand("pulumi", "config", "get", "key1")
		assert.Equal(t, "value1", strings.Trim(stdout, "\r\n"))

		stdout, _ = e.RunCommand("pulumi", "config", "get", "outer.inner")
		assert.Equal(t, "value2", strings.Trim(stdout, "\r\n"))

		stdout, _ = e.RunCommand("pulumi", "config", "get", "my_token")
		assert.Equal(t, "my_secret_token", strings.Trim(stdout, "\r\n"))

		stdout, _ = e.RunCommand("pulumi", "config", "get", "myList[0]")
		assert.Equal(t, "foo", strings.Trim(stdout, "\r\n"))

		// check that the nested config does not exist because we didn't use path
		_, stderr := e.RunCommandExpectError("pulumi", "config", "get", "outer")
		assert.Equal(t, "error: configuration key 'outer' not found for stack 'test'", strings.Trim(stderr, "\r\n"))

		_, stderr = e.RunCommandExpectError("pulumi", "config", "get", "myList")
		assert.Equal(t, "error: configuration key 'myList' not found for stack 'test'", strings.Trim(stderr, "\r\n"))

		// set the nested config using --path
		e.RunCommand("pulumi", "config", "set-all", "--path",
			"--plaintext", "outer.inner=value2",
			"--plaintext", "myList[0]=foo")

		// check that the nested config now exists
		stdout, _ = e.RunCommand("pulumi", "config", "get", "outer")
		assert.Equal(t, "{\"inner\":\"value2\"}", strings.Trim(stdout, "\r\n"))

		stdout, _ = e.RunCommand("pulumi", "config", "get", "myList")
		assert.Equal(t, "[\"foo\"]", strings.Trim(stdout, "\r\n"))

		// remove the nested config values
		e.RunCommand("pulumi", "config", "rm-all", "--path", "outer.inner", "myList[0]")

		// check that it worked
		stdout, _ = e.RunCommand("pulumi", "config", "get", "outer")
		assert.Equal(t, "{}", strings.Trim(stdout, "\r\n"))

		stdout, _ = e.RunCommand("pulumi", "config", "get", "myList")
		assert.Equal(t, "[]", strings.Trim(stdout, "\r\n"))

		// remove other config values
		e.RunCommand("pulumi", "config", "rm-all",
			"outer.inner", "myList[0]", "outer", "myList", "key1", "my_token")

		// check config is empty again
		stdout, _ = e.RunCommand("pulumi", "config")
		assert.Equal(t, "KEY  VALUE", strings.Trim(stdout, "\r\n"))

		e.RunCommand("pulumi", "stack", "rm", "--yes")
	})
}
