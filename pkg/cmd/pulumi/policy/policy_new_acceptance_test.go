// Copyright 2016-2024, Pulumi Corporation.
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

package policy

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest // changes directory for process
func TestCreatingPolicyPackWithArgsSpecifiedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)

	args := newPolicyArgs{
		templateNameOrURL: "aws-typescript",
	}

	err := runNewPolicyPack(context.Background(), args)
	assert.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempdir, "PulumiPolicy.yaml"))
	assert.FileExists(t, filepath.Join(tempdir, "index.ts"))
}

func assertNotFoundError(t *testing.T, err error) {
	msg := err.Error()
	if strings.Contains(msg, "not found") || strings.Contains(msg, "no such file or directory") {
		return
	}
	assert.Failf(t, "Error message does not contain \"not found\" or \"no such file or directory\": %s", msg)
}
