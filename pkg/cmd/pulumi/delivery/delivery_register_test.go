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

package delivery

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	sdkconfig "github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

func TestNormalizeRemoteURL(t *testing.T) {
	t.Parallel()
	const want = "https://github.com/acme/shop"
	for _, in := range []string{
		"https://github.com/acme/shop",
		"https://github.com/acme/shop.git",
		"https://user:token@github.com/acme/shop.git",
		"git@github.com:acme/shop.git",
		"git@github.com:acme/shop",
		"ssh://git@github.com/acme/shop.git",
		"ssh://git@github.com:22/acme/shop",
	} {
		got, err := normalizeRemoteURL(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	_, err := normalizeRemoteURL("")
	assert.Error(t, err)
}

func TestRelativeDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sub := filepath.Join(root, "infra", "pipeline")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	got, err := relativeDirectory(root, root)
	require.NoError(t, err)
	assert.Equal(t, "", got)

	got, err = relativeDirectory(root, sub)
	require.NoError(t, err)
	assert.Equal(t, "infra/pipeline", got)
}

func TestRegisterOutcome(t *testing.T) {
	t.Parallel()
	assert.NoError(t, registerOutcome(nil))
	assert.NoError(t, registerOutcome(result.BailError(&plugin.AwaitError{})))

	boom := errors.New("boom")
	assert.Equal(t, boom, registerOutcome(boom))
	assert.Error(t, registerOutcome(result.BailError(boom)))
}

func TestDetectSourceNotAGitRepository(t *testing.T) {
	t.Parallel()
	_, err := detectSource(t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delivery:source")
	assert.Contains(t, err.Error(), "pulumi config set --path")
}

func TestSetSource(t *testing.T) {
	t.Parallel()

	cfg := sdkconfig.Map{}
	require.NoError(t, setSource(cfg, pipelineSource{URL: "https://github.com/acme/shop", Directory: "pipeline"}))
	v, ok, err := cfg.Get(sdkconfig.MustMakeKey("delivery", "source"), false)
	require.NoError(t, err)
	require.True(t, ok)
	raw, err := v.Value(nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"url":"https://github.com/acme/shop","directory":"pipeline"}`, raw)
}
