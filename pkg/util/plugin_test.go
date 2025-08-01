// Copyright 2016-2023, Pulumi Corporation.
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

//nolint:revive // Legacy package name we don't want to change
package util

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestUrlAlreadySet(t *testing.T) {
	t.Parallel()

	spec := workspace.PluginSpec{
		Name:              "acme",
		Kind:              apitype.ResourcePlugin,
		PluginDownloadURL: "github://api.github.com/pulumiverse",
	}
	res := SetKnownPluginDownloadURL(&spec)
	assert.False(t, res)
}

func TestKnownProvider(t *testing.T) {
	t.Parallel()

	spec := workspace.PluginSpec{
		Name: "acme",
		Kind: apitype.ResourcePlugin,
	}
	res := SetKnownPluginDownloadURL(&spec)
	assert.True(t, res)
	assert.Equal(t, "github://api.github.com/pulumiverse", spec.PluginDownloadURL)
}
