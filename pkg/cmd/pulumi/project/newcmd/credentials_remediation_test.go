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

package newcmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/credentialcheck"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const awsSchema = `{"name":"aws","displayName":"AWS","validateCredentialsOnNew":true}`

var awsSignIn = plugin.CommandRemediation{
	Description: "Sign in to AWS",
	Command:     []string{"aws", "login", "--profile", "my proj"},
}

// remediationProvider returns a provider whose CheckConfig2 fails with the given remediations until fixed reports true.
func remediationProvider(remediations []plugin.CommandRemediation, fixed func() bool) *plugin.MockProvider {
	prov := schemaProvider(awsSchema)
	prov.CheckConfig2F = func(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfig2Response, error) {
		if fixed() {
			return plugin.CheckConfig2Response{}, nil
		}
		return plugin.CheckConfig2Response{Failures: []plugin.CheckConfigFailure{{
			Reason: "No valid credential sources found.", Remediations: remediations,
		}}}, nil
	}
	prov.ConfigureF = func(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
		return plugin.ConfigureResponse{}, nil
	}
	return prov
}

// problemWith returns the Problem that the credentials check reports for a provider suggesting remediations.
func problemWith(t *testing.T, remediations ...plugin.CommandRemediation) *credentialcheck.Problem {
	t.Helper()
	prov := remediationProvider(remediations, func() bool { return false })
	_, err := credentialcheck.Check(t.Context(), prov, plugin.GetSchemaRequest{}, nil)
	problem, ok := errors.AsType[*credentialcheck.Problem](err)
	require.True(t, ok, "expected a credentials problem, got %v", err)
	return problem
}

func TestOfferRemediation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		remediations []plugin.CommandRemediation
		noConfirm    bool // as under --yes
		decline      bool
		runErr       error
		prompt       string // "" when there is no offer
		ran          bool
		ok           bool
		output       string
	}{
		{
			name:         "accepted",
			remediations: []plugin.CommandRemediation{awsSignIn, {Command: []string{"aws", "sso", "login"}}},
			prompt:       "Sign in to AWS now? This runs `aws login --profile 'my proj'`.",
			ran:          true,
			ok:           true,
		},
		{
			name:         "no description",
			remediations: []plugin.CommandRemediation{{Command: []string{"gcloud", "auth", "login"}}},
			prompt:       "Run `gcloud auth login` now?",
			ran:          true,
			ok:           true,
		},
		{
			name:         "declined",
			remediations: []plugin.CommandRemediation{awsSignIn},
			decline:      true,
			prompt:       "Sign in to AWS now? This runs `aws login --profile 'my proj'`.",
			output:       "\n",
		},
		{
			name:         "command fails",
			remediations: []plugin.CommandRemediation{awsSignIn},
			runErr:       errors.New("exit status 255"),
			prompt:       "Sign in to AWS now? This runs `aws login --profile 'my proj'`.",
			ran:          true,
			output:       "`aws login --profile 'my proj'` did not complete; run it yourself, then `pulumi up`.\n\n",
		},
		{name: "nothing suggested"},
		{name: "not offered under --yes", remediations: []plugin.CommandRemediation{awsSignIn}, noConfirm: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			var prompted string
			var ran []plugin.CommandRemediation
			pf := credentialsPreflight{
				stdout: &buf,
				run: func(_ context.Context, r plugin.CommandRemediation) error {
					ran = append(ran, r)
					return tt.runErr
				},
			}
			if !tt.noConfirm {
				pf.confirm = func(prompt string) bool {
					prompted = prompt
					return !tt.decline
				}
			}

			ok := pf.offerRemediation(t.Context(), problemWith(t, tt.remediations...))
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.prompt, prompted)
			assert.Equal(t, tt.output, buf.String())
			if tt.ran {
				assert.Equal(t, []plugin.CommandRemediation{tt.remediations[0]}, ran)
			} else {
				assert.Empty(t, ran)
			}
		})
	}
}

func TestCheckPackageRemediation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		decline  bool
		succeeds bool // whether the remediation command succeeds
		fixes    bool // whether running it fixes the credentials
		cancels  bool // whether the user interrupts pulumi while it runs
		loads    int
		warnings int
		verified bool
		asked    bool
	}{
		{name: "declined", decline: true, loads: 1, warnings: 1, asked: true},
		{name: "command fails", loads: 1, warnings: 1, asked: true},
		{name: "fixed", succeeds: true, fixes: true, loads: 2, warnings: 1, verified: true, asked: true},
		{name: "still failing", succeeds: true, loads: 2, warnings: 2, asked: true},
		{name: "interrupted", succeeds: true, fixes: true, cancels: true, loads: 2, warnings: 1, asked: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			loads := 0
			fixed := false
			host := &plugin.MockHost{
				ProviderF: func(*plugin.Context, workspace.PluginDescriptor, env.Env) (plugin.Provider, error) {
					loads++
					return remediationProvider([]plugin.CommandRemediation{awsSignIn}, func() bool { return fixed }), nil
				},
			}
			sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
			pctx, err := plugin.NewContextWithHost(t.Context(), sink, sink, host, "", "", nil, nil)
			require.NoError(t, err)
			defer pctx.Close()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			asked := false
			var buf bytes.Buffer
			pf := credentialsPreflight{
				host: host, pctx: pctx, stdout: &buf, opts: display.Options{Color: colors.Never},
				confirm: func(string) bool {
					asked = true
					return !tt.decline
				},
				run: func(_ context.Context, r plugin.CommandRemediation) error {
					assert.Equal(t, awsSignIn, r)
					fixed = tt.fixes
					if tt.cancels {
						cancel()
					}
					if !tt.succeeds {
						return errors.New("exit status 1")
					}
					return nil
				},
			}
			pf.checkPackage(ctx, resourcePackage("aws", "7.0.0"))

			out := buf.String()
			assert.Equal(t, tt.asked, asked)
			assert.Equal(t, tt.loads, loads)
			assert.Equal(t, tt.warnings, strings.Count(out, "warning:"), out)
			assert.Equal(t, tt.verified, strings.Contains(out, "AWS credentials verified."), out)
		})
	}
}
