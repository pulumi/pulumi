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

package deploytest

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLanguageRuntime(t *testing.T) {
	t.Parallel()
	t.Run("Close", func(t *testing.T) {
		t.Parallel()
		p := &languageRuntime{}
		require.NoError(t, p.Close())
		assert.True(t, p.closed)
		// Ensure idempotent.
		require.NoError(t, p.Close())
		assert.True(t, p.closed)
	})
	t.Run("error: language runtime is shutting down", func(t *testing.T) {
		t.Parallel()
		t.Run("Run", func(t *testing.T) {
			t.Parallel()
			p := &languageRuntime{closed: true}
			_, _, err := p.Run(plugin.RunInfo{})
			assert.ErrorIs(t, err, ErrLanguageRuntimeIsClosed)
		})
		t.Run("GetRequiredPackages", func(t *testing.T) {
			t.Parallel()
			p := &languageRuntime{closed: true}
			_, err := p.GetRequiredPackages(plugin.ProgramInfo{})
			assert.ErrorIs(t, err, ErrLanguageRuntimeIsClosed)
		})
		t.Run("GetPluginInfo", func(t *testing.T) {
			t.Parallel()
			p := &languageRuntime{closed: true}
			_, err := p.GetPluginInfo()
			assert.ErrorIs(t, err, ErrLanguageRuntimeIsClosed)
		})
		t.Run("InstallDependencies", func(t *testing.T) {
			t.Parallel()
			p := &languageRuntime{closed: true}
			_, _, _, err := p.InstallDependencies(plugin.InstallDependenciesRequest{})
			assert.ErrorIs(t, err, ErrLanguageRuntimeIsClosed)
		})
		t.Run("RuntimeOptionsPrompts", func(t *testing.T) {
			t.Parallel()
			p := &languageRuntime{closed: true}
			_, err := p.RuntimeOptionsPrompts(plugin.ProgramInfo{})
			assert.ErrorIs(t, err, ErrLanguageRuntimeIsClosed)
		})
		t.Run("About", func(t *testing.T) {
			t.Parallel()
			p := &languageRuntime{closed: true}
			_, err := p.About(plugin.ProgramInfo{})
			assert.ErrorIs(t, err, ErrLanguageRuntimeIsClosed)
		})
		t.Run("GetProgramDependencies", func(t *testing.T) {
			t.Parallel()
			p := &languageRuntime{closed: true}
			_, err := p.GetProgramDependencies(plugin.ProgramInfo{}, false)
			assert.ErrorIs(t, err, ErrLanguageRuntimeIsClosed)
		})
	})
	t.Run("error: could not determine whether secrets are supported", func(t *testing.T) {
		t.Parallel()
		t.Run("Run", func(t *testing.T) {
			t.Parallel()
			p := &languageRuntime{}
			_, _, err := p.Run(plugin.RunInfo{})
			assert.ErrorContains(t, err, "could not determine whether secrets are supported")
		})
	})
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		t.Run("GetPluginInfo", func(t *testing.T) {
			t.Parallel()
			p := &languageRuntime{}
			res, err := p.GetPluginInfo()
			require.NoError(t, err)
			assert.Equal(t, workspace.PluginInfo{Name: "TestLanguage"}, res)
		})
		t.Run("InstallDependencies", func(t *testing.T) {
			t.Parallel()
			p := &languageRuntime{}
			_, _, _, err := p.InstallDependencies(plugin.InstallDependenciesRequest{})
			require.NoError(t, err)
		})
		t.Run("RuntimeOptionsPrompts", func(t *testing.T) {
			t.Parallel()
			p := &languageRuntime{}
			options, err := p.RuntimeOptionsPrompts(plugin.ProgramInfo{})
			require.NoError(t, err)
			assert.Equal(t, []plugin.RuntimeOptionPrompt{}, options)
		})
		t.Run("About", func(t *testing.T) {
			t.Parallel()
			p := &languageRuntime{}
			about, err := p.About(plugin.ProgramInfo{})
			require.NoError(t, err)
			assert.Equal(t, plugin.AboutInfo{}, about)
		})
		t.Run("GetProgramDependencies", func(t *testing.T) {
			t.Parallel()
			p := &languageRuntime{}
			res, err := p.GetProgramDependencies(plugin.ProgramInfo{}, false)
			require.NoError(t, err)
			assert.Nil(t, res)
		})
	})
	t.Run("unimplemented", func(t *testing.T) {
		p := &languageRuntime{}
		t.Run("RunPlugin", func(t *testing.T) {
			t.Parallel()
			_, _, _, err := p.RunPlugin(context.Background(), plugin.RunPluginInfo{})
			assert.ErrorContains(t, err, "are not currently supported")
		})
		t.Run("GenerateProject", func(t *testing.T) {
			t.Parallel()
			_, err := p.GenerateProject("", "", "", false, "", nil)
			assert.ErrorContains(t, err, "is not supported")
		})
		t.Run("GeneratePackage", func(t *testing.T) {
			t.Parallel()
			_, err := p.GeneratePackage("", "", nil, "", nil, false)
			assert.ErrorContains(t, err, "is not supported")
		})
		t.Run("GenerateProgram", func(t *testing.T) {
			t.Parallel()
			_, _, err := p.GenerateProgram(nil, "", false)
			assert.ErrorContains(t, err, "is not supported")
		})
		t.Run("Pack", func(t *testing.T) {
			t.Parallel()
			_, err := p.Pack("", "")
			assert.ErrorContains(t, err, "is not supported")
		})
	})
}
