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

package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// fakeAnalyzer implements Analyzer with no-op methods and does NOT implement
// StackConfigurableAnalyzer.
type fakeAnalyzer struct{}

func (f *fakeAnalyzer) Close() error       { return nil }
func (f *fakeAnalyzer) Name() tokens.QName { return "fake" }
func (f *fakeAnalyzer) Analyze(context.Context, AnalyzerResource) (AnalyzeResponse, error) {
	return AnalyzeResponse{}, nil
}

func (f *fakeAnalyzer) AnalyzeStack(context.Context, []AnalyzerStackResource) (AnalyzeResponse, error) {
	return AnalyzeResponse{}, nil
}

func (f *fakeAnalyzer) Remediate(context.Context, AnalyzerResource) (RemediateResponse, error) {
	return RemediateResponse{}, nil
}

func (f *fakeAnalyzer) GetAnalyzerInfo(context.Context) (AnalyzerInfo, error) {
	return AnalyzerInfo{}, nil
}

func (f *fakeAnalyzer) GetPluginInfo(context.Context) (PluginInfo, error) {
	return PluginInfo{}, nil
}

func (f *fakeAnalyzer) Configure(context.Context, map[string]AnalyzerPolicyConfig) error {
	return nil
}
func (f *fakeAnalyzer) Cancel(context.Context) error { return nil }

// fakeStackConfigurableAnalyzer additionally implements StackConfigurableAnalyzer.
type fakeStackConfigurableAnalyzer struct {
	fakeAnalyzer
	gotArgs *AnalyzerStackConfigureArgs
}

func (f *fakeStackConfigurableAnalyzer) ConfigureStack(
	_ context.Context, args AnalyzerStackConfigureArgs,
) error {
	f.gotArgs = &args
	return nil
}

func TestConfigureStackForwardsToStackConfigurableAnalyzer(t *testing.T) {
	t.Parallel()

	fake := &fakeStackConfigurableAnalyzer{}
	srv := NewAnalyzerServer(fake)

	req := &pulumirpc.AnalyzerStackConfigureRequest{
		Stack:            "dev",
		Project:          "proj",
		Organization:     "org",
		DryRun:           true,
		Tags:             map[string]string{"k": "v"},
		Config:           map[string]string{"proj:key": "val"},
		ConfigSecretKeys: []string{"proj:secret"},
	}

	_, err := srv.ConfigureStack(t.Context(), req)
	require.NoError(t, err)
	require.NotNil(t, fake.gotArgs)
	assert.Equal(t, "dev", fake.gotArgs.Stack)
	assert.Equal(t, "proj", fake.gotArgs.Project)
	assert.Equal(t, "org", fake.gotArgs.Organization)
	assert.True(t, fake.gotArgs.DryRun)
	assert.Equal(t, map[string]string{"k": "v"}, fake.gotArgs.Tags)
	assert.Equal(t, map[string]string{"proj:key": "val"}, fake.gotArgs.Config)
	assert.Equal(t, []string{"proj:secret"}, fake.gotArgs.ConfigSecretKeys)
}

func TestConfigureStackNoOpForPlainAnalyzer(t *testing.T) {
	t.Parallel()

	srv := NewAnalyzerServer(&fakeAnalyzer{})
	_, err := srv.ConfigureStack(
		t.Context(),
		&pulumirpc.AnalyzerStackConfigureRequest{Stack: "dev"},
	)
	require.NoError(t, err)
}
