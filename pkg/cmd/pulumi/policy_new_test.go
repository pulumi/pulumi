// Copyright 2016-2022, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !xplatform_acceptance

package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest // changes directory for process
func TestCreatingPolicyPackWithPromptedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	args := newPolicyArgs{
		templateNameOrURL: "aws-javascript",
	}

	err := runNewPolicyPack(context.Background(), args)
	assert.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempdir, "PulumiPolicy.yaml"))
	assert.FileExists(t, filepath.Join(tempdir, "index.js"))
}

//nolint:paralleltest // changes directory for process
func TestInvalidPolicyPackTemplateName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	// A template that will never exist.
	const nonExistantTemplate = "this-is-not-the-template-youre-looking-for"

	t.Run("RemoteTemplateNotFound", func(t *testing.T) {
		tempdir := tempProjectDir(t)
		chdir(t, tempdir)

		args := newPolicyArgs{
			templateNameOrURL: nonExistantTemplate,
		}

		err := runNewPolicyPack(context.Background(), args)
		assert.Error(t, err)
		assertNotFoundError(t, err)
	})

	t.Run("LocalTemplateNotFound", func(t *testing.T) {
		tempdir := tempProjectDir(t)
		chdir(t, tempdir)

		args := newPolicyArgs{
			generateOnly:      true,
			offline:           true,
			templateNameOrURL: nonExistantTemplate,
		}

		err := runNewPolicyPack(context.Background(), args)
		assert.Error(t, err)
		assertNotFoundError(t, err)
	})
}
