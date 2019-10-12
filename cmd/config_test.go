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

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func TestPrettyKeyForProject(t *testing.T) {
	t.Parallel()
	proj := &workspace.Project{
		Name:    tokens.PackageName("test-package"),
		Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil),
	}

	assert.Equal(t, "foo", prettyKeyForProject(config.MustMakeKey("test-package", "foo"), proj))
	assert.Equal(t, "other-package:bar", prettyKeyForProject(config.MustMakeKey("other-package", "bar"), proj))
}

func TestSecretDetection(t *testing.T) {
	t.Parallel()
	assert.True(t, looksLikeSecret(config.MustMakeKey("test", "token"), "1415fc1f4eaeb5e096ee58c1480016638fff29bf"))
	assert.True(t, looksLikeSecret(config.MustMakeKey("test", "apiToken"), "1415fc1f4eaeb5e096ee58c1480016638fff29bf"))

	// The key name does not match the, so even though this "looks like" a secret, we say it is not.
	assert.False(t, looksLikeSecret(config.MustMakeKey("test", "okay"), "1415fc1f4eaeb5e096ee58c1480016638fff29bf"))
}
