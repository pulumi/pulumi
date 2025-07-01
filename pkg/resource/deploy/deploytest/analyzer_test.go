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

package deploytest

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()
	t.Run("Close", func(t *testing.T) {
		t.Parallel()
		a := &Analyzer{}
		require.NoError(t, a.Close())
		// Ensure Idempotent.
		require.NoError(t, a.Close())
	})
	t.Run("Name", func(t *testing.T) {
		t.Parallel()
		a := &Analyzer{
			Info: plugin.AnalyzerInfo{
				Name: "my-analyzer",
			},
		}
		assert.Equal(t, tokens.QName("my-analyzer"), a.Name())
	})
	t.Run("Analyze", func(t *testing.T) {
		t.Parallel()
		t.Run("has AnalyzeF", func(t *testing.T) {
			t.Parallel()

			var called bool
			a := &Analyzer{
				AnalyzeF: func(r plugin.AnalyzerResource) ([]plugin.AnalyzeDiagnostic, error) {
					called = true
					return nil, nil
				},
			}
			res, err := a.Analyze(plugin.AnalyzerResource{})
			assert.True(t, called)
			require.NoError(t, err)
			assert.Nil(t, res)
		})
		t.Run("no AnalyzeF", func(t *testing.T) {
			t.Parallel()

			a := &Analyzer{}
			res, err := a.Analyze(plugin.AnalyzerResource{})
			require.NoError(t, err)
			assert.Nil(t, res)
		})
	})
	t.Run("AnalyzeStack", func(t *testing.T) {
		t.Parallel()
		t.Run("has AnalyzeStackF", func(t *testing.T) {
			t.Parallel()

			var called bool
			a := &Analyzer{
				AnalyzeStackF: func(resources []plugin.AnalyzerStackResource) ([]plugin.AnalyzeDiagnostic, error) {
					called = true
					return nil, nil
				},
			}
			res, err := a.AnalyzeStack(nil)
			assert.True(t, called)
			require.NoError(t, err)
			assert.Nil(t, res)
		})
		t.Run("no AnalyzeStackF", func(t *testing.T) {
			t.Parallel()

			a := &Analyzer{}
			res, err := a.AnalyzeStack(nil)
			require.NoError(t, err)
			assert.Nil(t, res)
		})
	})
	t.Run("Remediate", func(t *testing.T) {
		t.Parallel()
		t.Run("has RemediateF", func(t *testing.T) {
			t.Parallel()

			var called bool
			a := &Analyzer{
				RemediateF: func(r plugin.AnalyzerResource) ([]plugin.Remediation, error) {
					called = true
					return nil, nil
				},
			}
			res, err := a.Remediate(plugin.AnalyzerResource{})
			assert.True(t, called)
			require.NoError(t, err)
			assert.Nil(t, res)
		})
		t.Run("no RemediateF", func(t *testing.T) {
			t.Parallel()

			a := &Analyzer{}
			res, err := a.Remediate(plugin.AnalyzerResource{})
			require.NoError(t, err)
			assert.Nil(t, res)
		})
	})
	t.Run("GetPluginInfo", func(t *testing.T) {
		t.Parallel()
		a := &Analyzer{
			Info: plugin.AnalyzerInfo{
				Name: "my-analyzer",
			},
		}
		info, err := a.GetPluginInfo()
		require.NoError(t, err)
		assert.Equal(t, "my-analyzer", info.Name)
		assert.Equal(t, apitype.AnalyzerPlugin, info.Kind)
	})
	t.Run("Configure", func(t *testing.T) {
		t.Parallel()
		t.Run("has ConfigureF", func(t *testing.T) {
			t.Parallel()

			var called bool
			a := &Analyzer{
				ConfigureF: func(policyConfig map[string]plugin.AnalyzerPolicyConfig) error {
					called = true
					return nil
				},
			}
			require.NoError(t, a.Configure(nil))
			assert.True(t, called)
		})
		t.Run("no ConfigureF", func(t *testing.T) {
			t.Parallel()

			a := &Analyzer{}
			require.NoError(t, a.Configure(nil))
		})
	})
}
