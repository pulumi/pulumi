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
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	sdkconfig "github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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
	assert.EqualError(t, err, `unsupported remote URL ""`)
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
	require.NoError(t, registerOutcome(nil))
	require.NoError(t, registerOutcome(result.BailError(&plugin.AwaitError{})))

	boom := errors.New("boom")
	assert.Equal(t, boom, registerOutcome(boom))
	assert.EqualError(t, registerOutcome(result.BailError(boom)), "BAIL: boom")
}

func TestDetectSourceNotAGitRepository(t *testing.T) {
	t.Parallel()
	_, err := detectSource(t.TempDir())
	require.ErrorContains(t, err, "delivery:source")
	assert.ErrorContains(t, err, "pulumi config set --path")
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

func TestWriteSourceIfMissing(t *testing.T) {
	t.Parallel()
	detected := pipelineSource{URL: "https://github.com/acme/shop", Directory: "pipeline"}
	detect := func() (pipelineSource, error) { return detected, nil }

	t.Run("set", func(t *testing.T) {
		t.Parallel()
		ps := &workspace.ProjectStack{Config: sdkconfig.Map{}}
		require.NoError(t, setSource(ps.Config, pipelineSource{URL: "https://github.com/acme/other"}))
		saved := false
		src, err := writeSourceIfMissing(ps, detect, func(*workspace.ProjectStack) error { saved = true; return nil })
		require.NoError(t, err)
		assert.Nil(t, src)
		assert.False(t, saved)
	})

	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		ps := &workspace.ProjectStack{Config: sdkconfig.Map{}}
		file := filepath.Join(t.TempDir(), "x.yaml")
		src, err := writeSourceIfMissing(ps, detect, func(ps *workspace.ProjectStack) error { return ps.Save(file) })
		require.NoError(t, err)
		require.NotNil(t, src)
		assert.Equal(t, detected, *src)
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		assert.Contains(t, string(data), "https://github.com/acme/shop")
	})

	t.Run("detection fails", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("boom")
		saved := false
		_, err := writeSourceIfMissing(&workspace.ProjectStack{Config: sdkconfig.Map{}},
			func() (pipelineSource, error) { return pipelineSource{}, boom },
			func(*workspace.ProjectStack) error { saved = true; return nil })
		require.ErrorIs(t, err, boom)
		assert.False(t, saved)
	})
}

func noPrepare(*cobra.Command, string, string) (func(io.Writer), error) {
	return func(io.Writer) {}, nil
}

func allFlags(cmd *cobra.Command) map[string]*pflag.Flag {
	flags := map[string]*pflag.Flag{}
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) { flags[f.Name] = f })
	return flags
}

func TestRegisterHasAllFlagsOfUp(t *testing.T) {
	t.Parallel()
	register := allFlags(newRegisterCmd(operations.NewUpCmd(), noPrepare))
	up := allFlags(operations.NewUpCmd())
	require.NotEmpty(t, up)
	for name, f := range up {
		got, ok := register[name]
		require.True(t, ok, "missing flag %q", name)
		assert.Equal(t, f.Value.Type(), got.Value.Type(), name)
		assert.Equal(t, f.Shorthand, got.Shorthand, name)
	}
}

func TestRegisterPassesFlagsToUp(t *testing.T) {
	t.Parallel()
	up := operations.NewUpCmd()
	var stack string
	var configs []string
	var yes, skipPreview, prepared bool
	var innerArgs []string
	up.RunE = func(cmd *cobra.Command, args []string) error {
		innerArgs = args
		var err error
		if stack, err = cmd.Flags().GetString("stack"); err != nil {
			return err
		}
		if configs, err = cmd.Flags().GetStringArray("config"); err != nil {
			return err
		}
		if yes, err = cmd.Flags().GetBool("yes"); err != nil {
			return err
		}
		skipPreview, err = cmd.Flags().GetBool("skip-preview")
		return err
	}
	var prepareStack, prepareConfigFile string
	cmd := newRegisterCmd(up, func(_ *cobra.Command, s, cf string) (func(io.Writer), error) {
		prepared = true
		prepareStack = s
		prepareConfigFile = cf
		return func(w io.Writer) { _, _ = w.Write([]byte("done")) }, nil
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"--stack", "org/proj/dev", "--config", "a=b", "--config", "c=d", "--yes", "--skip-preview", "--config-file", "x.yaml",
	})
	require.NoError(t, cmd.Execute())

	assert.True(t, prepared)
	assert.Equal(t, "org/proj/dev", prepareStack)
	assert.Equal(t, "x.yaml", prepareConfigFile)
	assert.Equal(t, "org/proj/dev", stack)
	assert.Equal(t, []string{"a=b", "c=d"}, configs)
	assert.True(t, yes)
	assert.True(t, skipPreview)
	assert.Empty(t, innerArgs)
	assert.Equal(t, "done", out.String())
}

func TestRegisterStopsWhenPrepareFails(t *testing.T) {
	t.Parallel()
	up := operations.NewUpCmd()
	up.RunE = func(*cobra.Command, []string) error {
		return errors.New("update ran")
	}
	boom := errors.New("boom")
	cmd := newRegisterCmd(up, func(*cobra.Command, string, string) (func(io.Writer), error) { return nil, boom })
	cmd.SetArgs(nil)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	require.ErrorIs(t, cmd.Execute(), boom)
}

func TestRegisterRefusesArguments(t *testing.T) {
	t.Parallel()
	cmd := newRegisterCmd(operations.NewUpCmd(), noPrepare)
	cmd.RunE = func(*cobra.Command, []string) error { return nil }
	cmd.SetArgs([]string{"some-arg"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	require.EqualError(t, cmd.Execute(), "accepts 0 arg(s), received 1")
	assert.Equal(t, "register", cmd.Use)
}

func TestRegisterHiddenFlags(t *testing.T) {
	t.Parallel()
	up := operations.NewUpCmd()
	var strict bool
	up.RunE = func(cmd *cobra.Command, _ []string) error {
		var err error
		strict, err = cmd.Flags().GetBool("strict")
		return err
	}
	cmd := newRegisterCmd(up, noPrepare)
	flags := allFlags(cmd)
	for _, name := range registerHiddenFlags {
		f, ok := flags[name]
		require.True(t, ok, name)
		assert.True(t, f.Hidden, name)
	}
	for name, f := range flags {
		if strings.HasPrefix(name, "remote") {
			assert.True(t, f.Hidden, name)
		}
	}
	assert.False(t, flags["stack"].Hidden)
	assert.False(t, flags["config"].Hidden)
	assert.False(t, flags["expect-no-changes"].Hidden)
	assert.False(t, flags["secrets-provider"].Hidden)

	cmd.SetArgs([]string{"--strict"})
	require.NoError(t, cmd.Execute())
	assert.True(t, strict)
}
