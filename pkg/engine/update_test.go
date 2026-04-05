// Copyright 2020, Pulumi Corporation.
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

package engine

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/util/cancel"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAbbreviateFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     string
		expected string
	}{
		{
			path:     "/Users/username/test-policy",
			expected: "/Users/username/test-policy",
		},
		{
			path:     "./..//test-policy",
			expected: "../test-policy",
		},
		{
			path: `/Users/username/averylongpath/one/two/three/four/` +
				`five/six/seven/eight/nine/ten/eleven/twelve/test-policy`,
			expected: "/Users/.../twelve/test-policy",
		},
		{
			path: `nonrootdir/username/averylongpath/one/two/three/four/` +
				`five/six/seven/eight/nine/ten/eleven/twelve/test-policy`,
			expected: "nonrootdir/username/.../twelve/test-policy",
		},
		{
			path: `C:/Documents and Settings/username/My Documents/averylongpath/` +
				`one/two/three/four/five/six/seven/eight/test-policy`,
			expected: "C:/Documents and Settings/.../eight/test-policy",
		},
		{
			path: `C:\Documents and Settings\username\My Documents\averylongpath\` +
				`one\two\three\four\five\six\seven\eight\test-policy`,
			expected: `C:\Documents and Settings\...\eight\test-policy`,
		},
	}

	for _, tt := range tests {
		actual := abbreviateFilePath(tt.path)
		assert.Equal(t, filepath.ToSlash(tt.expected), filepath.ToSlash(actual))
	}
}

func TestDeletingComponentResourceProducesResourceOutputsEvent(t *testing.T) {
	t.Parallel()

	cancelCtx, _ := cancel.NewContext(t.Context())

	acts := newUpdateActions(&Context{
		Cancel: cancelCtx,
	}, UpdateInfo{}, &deploymentOptions{})
	eventsChan := make(chan Event, 10)
	acts.Opts.Events.ch = eventsChan

	step := deploy.NewDeleteStep(&deploy.Deployment{}, map[resource.URN]bool{}, &resource.State{
		URN:      resource.URN("urn:pulumi:stack::project::my:example:Foo::foo"),
		ID:       "foo",
		Custom:   false,
		Provider: "unimportant",
	}, nil)
	acts.Seen[resource.URN("urn:pulumi:stack::project::my:example:Foo::foo")] = step

	err := acts.OnResourceStepPost(
		&mockSnapshotMutation{}, step, resource.StatusOK,
		nil, /* err */
	)
	require.NoError(t, err)

	//nolint:exhaustive // the default case is for test failures
	switch e := <-eventsChan; e.Type {
	case ResourceOutputsEvent:
		e, ok := e.Payload().(ResourceOutputsEventPayload)
		assert.True(t, ok)
		assert.True(t, e.Metadata.URN == "urn:pulumi:stack::project::my:example:Foo::foo")
	default:
		assert.Fail(t, "unexpected event type")
	}
}

type mockSnapshotMutation struct{}

func (msm *mockSnapshotMutation) End(step deploy.Step, successful bool) error { return nil }

//nolint:paralleltest // subtests use t.Setenv
func TestLoadPolicyAnalyzer(t *testing.T) {
	t.Run("successful load", func(t *testing.T) {
		t.Parallel()

		host := &plugin.MockHost{
			PolicyAnalyzerF: func(name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
				return &mockAnalyzer{name: name}, nil
			},
		}
		plugctx, err := plugin.NewContextWithRoot(
			t.Context(), nil, nil, host, "", "", nil, false, nil, nil, nil, nil, nil, nil)
		require.NoError(t, err)
		defer plugctx.Close()

		analyzer, err := loadPolicyAnalyzer(t.Context(), plugctx, "my-policy", "/path", nil)
		require.NoError(t, err)
		assert.Equal(t, tokens.QName("my-policy"), analyzer.Name())
	})

	t.Run("non-MissingError passes through", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("some other error")
		host := &plugin.MockHost{
			PolicyAnalyzerF: func(tokens.QName, string, *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
				return nil, expectedErr
			},
		}
		plugctx, err := plugin.NewContextWithRoot(
			t.Context(), nil, nil, host, "", "", nil, false, nil, nil, nil, nil, nil, nil)
		require.NoError(t, err)
		defer plugctx.Close()

		_, err = loadPolicyAnalyzer(t.Context(), plugctx, "my-policy", "/path", nil)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("MissingError with auto-install disabled", func(t *testing.T) {
		t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true")

		host := &plugin.MockHost{
			PolicyAnalyzerF: func(tokens.QName, string, *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
				return nil, workspace.NewMissingError(workspace.PluginDescriptor{
					Name: "policy-opa",
					Kind: apitype.AnalyzerPlugin,
				}, false)
			},
		}
		plugctx, err := plugin.NewContextWithRoot(
			t.Context(), nil, nil, host, "", "", nil, false, nil, nil, nil, nil, nil, nil)
		require.NoError(t, err)
		defer plugctx.Close()

		_, err = loadPolicyAnalyzer(t.Context(), plugctx, "my-policy", "/path", nil)
		assert.ErrorContains(t, err,
			`could not start policy pack "my-policy" because the built-in analyzer `+
				`plugin that runs policy plugins is missing`)
		assert.ErrorContains(t, err, "required analyzer plugin has not been installed")

		// The original MissingError should be wrapped, not replaced.
		var me *workspace.MissingError
		assert.True(t, errors.As(err, &me))
	})

	t.Run("MissingError with auto-install retries and succeeds", func(t *testing.T) {
		t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")
		// auto-install is enabled by default. The first call to PolicyAnalyzer
		// returns MissingError; after the install attempt the second call succeeds.
		origInstall := installPluginFunc
		installPluginFunc = func(
			_ context.Context, _ workspace.PluginDescriptor,
			_ func(diag.Severity, string), _ plugin.NewLoaderFunc,
		) (*semver.Version, error) {
			return nil, nil
		}
		t.Cleanup(func() { installPluginFunc = origInstall })

		calls := 0
		host := &plugin.MockHost{
			PolicyAnalyzerF: func(name tokens.QName, _ string, _ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
				calls++
				if calls == 1 {
					return nil, workspace.NewMissingError(workspace.PluginDescriptor{
						Name: "policy-opa",
						Kind: apitype.AnalyzerPlugin,
					}, false)
				}
				return &mockAnalyzer{name: name}, nil
			},
		}
		plugctx, err := plugin.NewContextWithRoot(
			t.Context(), nil, nil, host, "", "", nil, false, nil, nil, nil, nil, nil, nil)
		require.NoError(t, err)
		defer plugctx.Close()

		analyzer, err := loadPolicyAnalyzer(t.Context(), plugctx, "my-policy", "/path", nil)
		require.NoError(t, err)
		assert.Equal(t, tokens.QName("my-policy"), analyzer.Name())
		assert.Equal(t, 2, calls, "expected two calls: first fails, second succeeds after install")
	})

	t.Run("MissingError with auto-install failure includes install error", func(t *testing.T) {
		t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")
		origInstall := installPluginFunc
		installErr := errors.New("network timeout")
		installPluginFunc = func(
			_ context.Context, _ workspace.PluginDescriptor,
			_ func(diag.Severity, string), _ plugin.NewLoaderFunc,
		) (*semver.Version, error) {
			return nil, installErr
		}
		t.Cleanup(func() { installPluginFunc = origInstall })

		host := &plugin.MockHost{
			PolicyAnalyzerF: func(tokens.QName, string, *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
				return nil, workspace.NewMissingError(workspace.PluginDescriptor{
					Name: "policy-opa",
					Kind: apitype.AnalyzerPlugin,
				}, false)
			},
		}
		plugctx, err := plugin.NewContextWithRoot(
			t.Context(), nil, nil, host, "", "", nil, false, nil, nil, nil, nil, nil, nil)
		require.NoError(t, err)
		defer plugctx.Close()

		_, err = loadPolicyAnalyzer(t.Context(), plugctx, "my-policy", "/path", nil)
		assert.ErrorContains(t, err, "network timeout")
		assert.ErrorContains(t, err, "failed to automatically install analyzer plugin")

		// The original MissingError should be wrapped.
		var me *workspace.MissingError
		assert.True(t, errors.As(err, &me))
	})

	t.Run("MissingError after successful install wraps retry error", func(t *testing.T) {
		t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")
		origInstall := installPluginFunc
		installPluginFunc = func(
			_ context.Context, _ workspace.PluginDescriptor,
			_ func(diag.Severity, string), _ plugin.NewLoaderFunc,
		) (*semver.Version, error) {
			return nil, nil
		}
		t.Cleanup(func() { installPluginFunc = origInstall })

		// Even after install, PolicyAnalyzer still returns MissingError.
		host := &plugin.MockHost{
			PolicyAnalyzerF: func(tokens.QName, string, *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
				return nil, workspace.NewMissingError(workspace.PluginDescriptor{
					Name: "policy-opa",
					Kind: apitype.AnalyzerPlugin,
				}, false)
			},
		}
		plugctx, err := plugin.NewContextWithRoot(
			t.Context(), nil, nil, host, "", "", nil, false, nil, nil, nil, nil, nil, nil)
		require.NoError(t, err)
		defer plugctx.Close()

		_, err = loadPolicyAnalyzer(t.Context(), plugctx, "my-policy", "/path", nil)
		assert.ErrorContains(t, err,
			`could not start policy pack "my-policy" because the built-in analyzer `+
				`plugin that runs policy plugins is missing`)

		var me *workspace.MissingError
		assert.True(t, errors.As(err, &me))
	})
}

type mockAnalyzer struct {
	name tokens.QName
}

func (a *mockAnalyzer) Close() error       { return nil }
func (a *mockAnalyzer) Name() tokens.QName { return a.name }
func (a *mockAnalyzer) Analyze(plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
	return plugin.AnalyzeResponse{}, nil
}

func (a *mockAnalyzer) AnalyzeStack([]plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
	return plugin.AnalyzeResponse{}, nil
}

func (a *mockAnalyzer) Remediate(plugin.AnalyzerResource) (plugin.RemediateResponse, error) {
	return plugin.RemediateResponse{}, nil
}

func (a *mockAnalyzer) GetAnalyzerInfo() (plugin.AnalyzerInfo, error) {
	return plugin.AnalyzerInfo{}, nil
}

func (a *mockAnalyzer) GetPluginInfo() (plugin.PluginInfo, error) {
	return plugin.PluginInfo{}, nil
}
func (a *mockAnalyzer) Configure(map[string]plugin.AnalyzerPolicyConfig) error { return nil }
func (a *mockAnalyzer) Cancel(context.Context) error                           { return nil }

func TestParsePolicyConfigKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key        string
		wantPack   string
		wantPolicy string
	}{
		{"cost-policy", "", "cost-policy"},
		{"my-pack:cost-policy", "my-pack", "cost-policy"},
		{":cost-policy", "", "cost-policy"},
		{"pack:a:b", "pack", "a:b"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			gotPack, gotPolicy := parsePolicyConfigKey(tt.key)
			assert.Equal(t, tt.wantPack, gotPack)
			assert.Equal(t, tt.wantPolicy, gotPolicy)
		})
	}
}

func TestMergePolicyConfig(t *testing.T) {
	t.Parallel()

	raw := func(s string) *json.RawMessage {
		r := json.RawMessage(s)
		return &r
	}

	t.Run("nil base and nil esc returns nil", func(t *testing.T) {
		t.Parallel()
		got := mergePolicyConfig(nil, nil, "pack")
		assert.Nil(t, got)
	})

	t.Run("non-nil base with nil esc returns base", func(t *testing.T) {
		t.Parallel()
		base := map[string]*json.RawMessage{"p": raw(`"a"`)}
		got := mergePolicyConfig(base, nil, "pack")
		assert.Equal(t, base, got)
	})

	t.Run("nil base with esc entries", func(t *testing.T) {
		t.Parallel()
		esc := map[string]*json.RawMessage{"p": raw(`"x"`)}
		got := mergePolicyConfig(nil, esc, "pack")
		assert.Equal(t, map[string]*json.RawMessage{"p": raw(`"x"`)}, got)
	})

	t.Run("no conflict merges both", func(t *testing.T) {
		t.Parallel()
		base := map[string]*json.RawMessage{"a": raw(`1`)}
		esc := map[string]*json.RawMessage{"b": raw(`2`)}
		got := mergePolicyConfig(base, esc, "pack")
		assert.Equal(t, map[string]*json.RawMessage{
			"a": raw(`1`),
			"b": raw(`2`),
		}, got)
	})

	t.Run("base wins on conflict for scalar values", func(t *testing.T) {
		t.Parallel()
		base := map[string]*json.RawMessage{"p": raw(`"base"`)}
		esc := map[string]*json.RawMessage{"p": raw(`"esc"`)}
		got := mergePolicyConfig(base, esc, "pack")
		assert.Equal(t, map[string]*json.RawMessage{"p": raw(`"base"`)}, got)
	})

	t.Run("deep merge objects with API winning on conflict", func(t *testing.T) {
		t.Parallel()
		base := map[string]*json.RawMessage{
			"cost-policy": raw(`{"enforcement":"advisory","maxCost":100}`),
		}
		esc := map[string]*json.RawMessage{
			"cost-policy": raw(`{"enforcement":"mandatory","minCost":10}`),
		}
		got := mergePolicyConfig(base, esc, "pack")
		require.Contains(t, got, "cost-policy")
		var m map[string]any
		require.NoError(t, json.Unmarshal(*got["cost-policy"], &m))
		// API wins on conflict.
		assert.Equal(t, "advisory", m["enforcement"])
		assert.Equal(t, float64(100), m["maxCost"])
		// ESC-only property is preserved.
		assert.Equal(t, float64(10), m["minCost"])
	})

	t.Run("deep merge nested objects recursively", func(t *testing.T) {
		t.Parallel()
		base := map[string]*json.RawMessage{
			"cost-policy": raw(`{"rules":{"a":1,"shared":"base"}}`),
		}
		esc := map[string]*json.RawMessage{
			"cost-policy": raw(`{"rules":{"b":2,"shared":"esc"}}`),
		}
		got := mergePolicyConfig(base, esc, "pack")
		require.Contains(t, got, "cost-policy")
		var m map[string]any
		require.NoError(t, json.Unmarshal(*got["cost-policy"], &m))
		rules := m["rules"].(map[string]any)
		// Both sides' unique keys are present.
		assert.Equal(t, float64(1), rules["a"])
		assert.Equal(t, float64(2), rules["b"])
		// Base (API) wins on conflict.
		assert.Equal(t, "base", rules["shared"])
	})

	t.Run("namespaced key matching pack is included", func(t *testing.T) {
		t.Parallel()
		esc := map[string]*json.RawMessage{"my-pack:cost-policy": raw(`true`)}
		got := mergePolicyConfig(nil, esc, "my-pack")
		assert.Equal(t, map[string]*json.RawMessage{"cost-policy": raw(`true`)}, got)
	})

	t.Run("namespaced key not matching pack is excluded", func(t *testing.T) {
		t.Parallel()
		esc := map[string]*json.RawMessage{"other-pack:cost-policy": raw(`true`)}
		got := mergePolicyConfig(nil, esc, "my-pack")
		assert.Empty(t, got)
	})

	t.Run("does not mutate original base map", func(t *testing.T) {
		t.Parallel()
		base := map[string]*json.RawMessage{"a": raw(`1`)}
		esc := map[string]*json.RawMessage{"b": raw(`2`)}
		got := mergePolicyConfig(base, esc, "pack")
		require.Contains(t, got, "b")
		_, inBase := base["b"]
		assert.False(t, inBase, "original base map should not be mutated")
	})
}
