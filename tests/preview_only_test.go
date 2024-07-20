// Copyright 2016-2024, Pulumi Corporation.
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

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

func TestPreviewOnlyFlag(t *testing.T) {
	t.Run("PreviewOnlyRefresh", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

		integration.CreateBasicPulumiRepo(e)
		e.ImportDirectory("integration/single_resource")
		e.RunCommand("yarn", "link", "@pulumi/pulumi")
		e.RunCommand("yarn", "install")
		e.SetBackend(e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "foo")
		e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

		// Try some invalid flag combinations.
		_, stderr := e.RunCommandExpectError("pulumi", "refresh", "--preview-only", "--yes")
		assert.Equal(t,
			"error: --yes and --preview-only cannot be used together",
			strings.Trim(stderr, "\r\n"))
		_, stderr = e.RunCommandExpectError("pulumi", "refresh", "--skip-preview", "--preview-only")
		assert.Equal(t,
			"error: --skip-preview and --preview-only cannot be used together",
			strings.Trim(stderr, "\r\n"))
		_, stderr = e.RunCommandExpectError("pulumi", "refresh", "--non-interactive")
		assert.Equal(t,
			"error: --yes or --skip-preview or --preview-only must be passed in to proceed when "+
				"running in non-interactive mode",
			strings.Trim(stderr, "\r\n"))

		// Now try just the flag.
		stdout, _ := e.RunCommand("pulumi", "refresh", "--preview-only")
		assert.NotContains(t, stdout, "Do you want to perform this refresh?")
		// Make sure it works with --non-interactive too.
		e.RunCommand("pulumi", "refresh", "--preview-only", "--non-interactive")

		e.RunCommand("pulumi", "destroy", "--skip-preview", "--yes")
		// Remove the stack.
		e.RunCommand("pulumi", "stack", "rm", "foo", "--yes")
	})

	t.Run("PreviewOnlyDestroy", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

		integration.CreateBasicPulumiRepo(e)
		e.ImportDirectory("integration/single_resource")
		e.RunCommand("yarn", "link", "@pulumi/pulumi")
		e.RunCommand("yarn", "install")
		e.SetBackend(e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "foo")
		e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

		// Try some invalid flag combinations.
		_, stderr := e.RunCommandExpectError("pulumi", "destroy", "--preview-only", "--yes")
		assert.Equal(t,
			"error: --yes and --preview-only cannot be used together",
			strings.Trim(stderr, "\r\n"))
		_, stderr = e.RunCommandExpectError("pulumi", "destroy", "--skip-preview", "--preview-only")
		assert.Equal(t,
			"error: --skip-preview and --preview-only cannot be used together",
			strings.Trim(stderr, "\r\n"))
		_, stderr = e.RunCommandExpectError("pulumi", "destroy", "--non-interactive")
		assert.Equal(t,
			"error: --yes or --skip-preview or --preview-only must be passed in to proceed when running in non-interactive mode",
			strings.Trim(stderr, "\r\n"))

		// Now try just the flag.
		stdout, _ := e.RunCommand("pulumi", "destroy", "--preview-only")
		assert.NotContains(t, stdout, "Do you want to perform this destroy?")
		assert.NotContains(t, stdout, "The resources in the stack have been deleted")
		// Make sure it works with --non-interactive too.
		e.RunCommand("pulumi", "destroy", "--preview-only", "--non-interactive")

		e.RunCommand("pulumi", "destroy", "--skip-preview", "--yes")
		// Remove the stack.
		e.RunCommand("pulumi", "stack", "rm", "foo", "--yes")
	})

	t.Run("PreviewOnlyImport", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

		integration.CreateBasicPulumiRepo(e)
		e.SetBackend(e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "foo")

		// Make sure random is installed
		e.RunCommand("pulumi", "plugin", "install", "resource", "random", "4.13.0")

		// Try some invalid flag combinations.
		_, stderr := e.RunCommandExpectError("pulumi", "import", "random:index/randomId:RandomId",
			"identifier", "p-9hUg", "--preview-only", "--yes")
		assert.Equal(t,
			"error: --yes and --preview-only cannot be used together",
			strings.Trim(stderr, "\r\n"))
		_, stderr = e.RunCommandExpectError("pulumi", "import", "random:index/randomId:RandomId",
			"identifier", "p-9hUg", "--skip-preview", "--preview-only")
		assert.Equal(t,
			"error: --skip-preview and --preview-only cannot be used together",
			strings.Trim(stderr, "\r\n"))
		_, stderr = e.RunCommandExpectError("pulumi", "import", "random:index/randomId:RandomId",
			"identifier", "p-9hUg", "--non-interactive")
		assert.Equal(t,
			"error: --yes or --skip-preview or --preview-only must be passed in to proceed when running in non-interactive mode",
			strings.Trim(stderr, "\r\n"))

		// Now try just the flag.
		stdout, _ := e.RunCommand("pulumi", "import", "random:index/randomId:RandomId",
			"identifier", "p-9hUg", "--preview-only")
		assert.NotContains(t, stdout, "Do you want to perform this import?")
		// Make sure it works with --non-interactive too.
		e.RunCommand("pulumi", "import", "random:index/randomId:RandomId",
			"identifier", "p-9hUg", "--preview-only", "--non-interactive")

		// Remove the stack.
		e.RunCommand("pulumi", "stack", "rm", "foo", "--yes")
	})
}
