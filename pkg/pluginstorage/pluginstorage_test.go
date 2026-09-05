// Copyright 2026, Pulumi Corporation.
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

package pluginstorage

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// An attached resource provider counts as installed; the attachment applies to
// resource plugins only, and to nothing else in the empty plugin cache.
func TestHasPluginAttachedProvider(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_DEBUG_PROVIDERS", "attached:12345")

	has := func(name string, kind apitype.PluginKind) bool {
		return Instance.HasPlugin(t.Context(), workspace.PluginDescriptor{Name: name, Kind: kind})
	}
	assert.Equal(t, map[string]bool{
		"attached resource": true,
		"attached language": false,
		"other resource":    false,
	}, map[string]bool{
		"attached resource": has("attached", apitype.ResourcePlugin),
		"attached language": has("attached", apitype.LanguagePlugin),
		"other resource":    has("other", apitype.ResourcePlugin),
	})
}
