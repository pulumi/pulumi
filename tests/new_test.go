// Copyright 2016-2018, Pulumi Corporation.
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
	"testing"

	ptesting "github.com/pulumi/pulumi/pkg/testing"
	"github.com/stretchr/testify/assert"
)

// deleteIfNotFailed deletes the files in the testing environment if the testcase has
// not failed. (Otherwise they are left to aid debugging.)
func deleteIfNotFailed(e *ptesting.Environment) {
	if !e.T.Failed() {
		e.DeleteEnvironment()
	}
}

func TestPulumiNew(t *testing.T) {
	t.Run("NoTemplateSpecified", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		// Confirm this will result in an error since it isn't an
		// interactive terminal session.
		_, stderr := e.RunCommandExpectError("pulumi", "new")
		assert.NotEmpty(t, stderr)
	})

	t.Run("InvalidTemplateName", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		// An invalid template name.
		template := "/this/is\\not/a/valid/templatename"

		// Confirm this fails.
		_, stderr := e.RunCommandExpectError("pulumi", "new", template)
		assert.NotEmpty(t, stderr)
	})

	t.Run("LocalTemplateNotFound", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		// A template that will never exist remotely.
		template := "this-is-not-the-template-youre-looking-for"

		// Confirm this fails.
		_, stderr := e.RunCommandExpectError("pulumi", "new", template, "--offline", "--generate-only", "--yes")
		assert.NotEmpty(t, stderr)
	})

	t.Run("RemoteTemplateNotFound", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		// A template that will never exist remotely.
		template := "this-is-not-the-template-youre-looking-for"

		// Confirm this fails.
		_, stderr := e.RunCommandExpectError("pulumi", "new", template)
		assert.NotEmpty(t, stderr)
	})
}
