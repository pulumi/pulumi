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

package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// TestCanonicalCheckoutHashStableAcrossLoad locks in the no-op detection guarantee: the hash computed at
// checkout (from the in-memory ProjectStack, no banner) must equal the hash computed by commit/discard
// after writing the banner-prefixed file and loading it back. A raw byte hash would diverge here because
// a loaded ProjectStack retains the file's trivia for comment-preserving saves.
//
//nolint:paralleltest // mutates the working directory via t.Chdir
func TestCanonicalCheckoutHashStableAcrossLoad(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: proj\nruntime: nodejs\n"), 0o600))
	t.Chdir(dir)

	ps := &workspace.ProjectStack{Config: config.Map{}}
	require.NoError(t, ps.Config.Set(config.MustMakeKey("proj", "a"), config.NewValue("b"), false))
	ps.Environment = workspace.NewEnvironment([]string{"proj/base"})

	checkoutHash, err := canonicalCheckoutHash(ps)
	require.NoError(t, err)

	path := filepath.Join(dir, "Pulumi.dev.local.yaml")
	banner := "# created by checkout\n# do not commit\n"
	require.NoError(t, writeWorkingCopy(ps, path, banner))

	loaded, err := cmdStack.LoadProjectStack(t.Context(),
		diag.DefaultSink(&bytes.Buffer{}, &bytes.Buffer{}, diag.FormatOptions{Color: colors.Never}),
		&workspace.Project{Name: "proj"}, nil, path)
	require.NoError(t, err)

	loadedHash, err := canonicalCheckoutHash(loaded)
	require.NoError(t, err)

	require.Equal(t, checkoutHash, loadedHash,
		"hash must survive checkout-write (with banner) then load, or no-op detection breaks")
}
