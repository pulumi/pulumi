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

	"github.com/kballard/go-shellquote"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Credentials errors as pulumi-aws reports them.
const (
	awsNoCredentials = "No valid credential sources found.\n" +
		"To sign in through your browser, run `aws login` (requires AWS CLI 2.32.0 or later) and try again.\n" +
		"Please see " + awsDocURL + " for more information about providing credentials."
	awsNoProfileSelected = "No valid credential sources found.\n" +
		"No AWS profile is selected, but these profiles are configured: myproj, corp.\n" +
		"To use one, run `pulumi config set aws:profile <name>` or set AWS_PROFILE.\n" +
		"To sign in through your browser, run `aws login` (requires AWS CLI 2.32.0 or later) and try again."
	awsExpiredLogin = "The AWS login session for profile \"myproj\" has expired or was not found.\n" +
		"Run `aws login --profile myproj` to sign in again, then retry."
	awsExpiredSSO = "The AWS SSO session for profile \"corp\" has expired or was not found.\n" +
		"Run `aws sso login --profile corp` to sign in again, then retry."
	awsInvalidCredentials = "The configured AWS credentials are invalid or have expired.\n" +
		"Please see " + awsDocURL + " for more information about providing credentials."
	awsProfileNotFound = "The AWS profile \"nope\" was not found.\n" +
		"Check the `aws:profile` configuration and the AWS_PROFILE environment variable."
	awsNoRegion = "Missing region information\n" +
		"Make sure you have set your AWS region, e.g. `pulumi config set aws:region us-west-2`."
	awsLegacyNoCredentials = "No valid credential sources found.\n" +
		"Please see " + awsDocURL + " for more information about providing credentials."
)

func TestAWSSignInCommandFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		reason  string
		command []string // nil when no sign-in is suggested
	}{
		{awsNoCredentials, []string{"login"}},
		{awsNoProfileSelected, []string{"login"}},
		{awsExpiredLogin, []string{"login"}},
		{awsExpiredSSO, []string{"sso", "login"}},
		{awsInvalidCredentials, nil},
		{awsProfileNotFound, nil},
		{awsNoRegion, nil},
		{awsLegacyNoCredentials, nil},
	}
	for _, tt := range tests {
		problem := &credentialsProblem{details: []string{"region: expected a valid region", tt.reason}}
		c, ok := awsSignInCommandFor(problem)
		assert.Equal(t, tt.command != nil, ok, tt.reason)
		assert.Equal(t, tt.command, c.command, tt.reason)
	}
}

func TestAWSCLIVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		output  string
		version string // empty when the output cannot be parsed
	}{
		{"aws-cli/2.32.0 Python/3.13.9 Darwin/24.6.0 source/arm64\n", "2.32.0"},
		{"aws-cli/2.38.2 Python/3.15.0 Darwin/24.6.0 source/arm64", "2.38.2"},
		{"aws-cli/1.40.0 Python/3.12.0 Linux/6.8.0 botocore/1.38.0", "1.40.0"},
		{"command not found", ""},
		{"", ""},
	}
	for _, tt := range tests {
		v, ok := awsCLIVersion([]byte(tt.output))
		assert.Equal(t, tt.version != "", ok, tt.output)
		if ok {
			assert.Equal(t, tt.version, v.String())
		}
	}
}

func TestAWSSignInArgs(t *testing.T) {
	t.Parallel()

	stackConfig := func(values map[string]string) config.Map {
		cfg := config.Map{}
		for k, v := range values {
			cfg[config.MustMakeKey("aws", k)] = config.NewValue(v)
		}
		return cfg
	}

	login, sso := awsSignInCommands[0], awsSignInCommands[1]
	tests := []struct {
		name   string
		signIn awsSignInCommand
		cfg    map[string]string
		env    map[string]string
		want   []string
	}{
		{name: "nothing configured", signIn: login, want: []string{"login"}},
		{
			name:   "stack config",
			signIn: login,
			cfg:    map[string]string{"profile": "myproj", "region": "us-west-2"},
			want:   []string{"login", "--profile", "myproj", "--region", "us-west-2"},
		},
		{
			name:   "environment",
			signIn: login,
			env:    map[string]string{"AWS_PROFILE": "corp", "AWS_DEFAULT_REGION": "eu-west-1"},
			want:   []string{"login", "--profile", "corp", "--region", "eu-west-1"},
		},
		{
			name:   "AWS_REGION takes precedence over AWS_DEFAULT_REGION",
			signIn: login,
			env:    map[string]string{"AWS_REGION": "eu-central-1", "AWS_DEFAULT_REGION": "eu-west-1"},
			want:   []string{"login", "--region", "eu-central-1"},
		},
		{
			name:   "stack config takes precedence over the environment",
			signIn: login,
			cfg:    map[string]string{"profile": "myproj", "region": "us-west-2"},
			env:    map[string]string{"AWS_PROFILE": "corp", "AWS_REGION": "eu-west-1"},
			want:   []string{"login", "--profile", "myproj", "--region", "us-west-2"},
		},
		{
			name:   "remote over SSH",
			signIn: login,
			env:    map[string]string{"SSH_CONNECTION": "10.0.0.1 51234 10.0.0.2 22"},
			want:   []string{"login", "--remote"},
		},
		{
			name:   "SSO takes the profile but not the region",
			signIn: sso,
			cfg:    map[string]string{"profile": "corp", "region": "us-west-2"},
			want:   []string{"sso", "login", "--profile", "corp"},
		},
		{
			name:   "SSO without a browser over SSH",
			signIn: sso,
			env:    map[string]string{"AWS_PROFILE": "corp", "SSH_TTY": "/dev/ttys001"},
			want:   []string{"sso", "login", "--profile", "corp", "--no-browser"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := awsSignIn{cfg: stackConfig(tt.cfg), getenv: func(key string) string { return tt.env[key] }}
			assert.Equal(t, tt.want, s.args(tt.signIn))
		})
	}
}

type fakeCLI struct {
	version string
	err     error // returned by `aws --version`, as when the AWS CLI is not installed
	runErr  error
	ran     [][]string
}

func (f *fakeCLI) Output(context.Context, string, ...string) ([]byte, error) {
	return []byte(f.version), f.err
}

func (f *fakeCLI) RunAttached(_ context.Context, name string, args ...string) error {
	f.ran = append(f.ran, append([]string{name}, args...))
	return f.runErr
}

func TestAWSSignInOffer(t *testing.T) {
	t.Parallel()

	const supported = "aws-cli/2.38.2 Python/3.15.0 Darwin/24.6.0 source/arm64"
	const tooOldForLogin = "aws-cli/2.31.0 Python/3.13.7"

	tests := []struct {
		name     string
		provider cloudProvider
		reason   string
		cfg      map[string]string
		cli      fakeCLI
		decline  bool
		command  string // the command offered, or "" when there is no offer
		signedIn bool
		output   string
	}{
		{
			name: "aws login", provider: awsProvider, reason: awsNoCredentials, cli: fakeCLI{version: supported},
			command: "aws login --region us-west-2", signedIn: true,
		},
		{
			name: "aws sso login", provider: awsProvider, reason: awsExpiredSSO, cli: fakeCLI{version: tooOldForLogin},
			command: "aws sso login", signedIn: true,
		},
		{
			name: "profile with a space", provider: awsProvider, reason: awsExpiredLogin, cli: fakeCLI{version: supported},
			cfg:     map[string]string{"profile": "my proj"},
			command: "aws login --profile 'my proj' --region us-west-2", signedIn: true,
		},
		{
			name: "sign in fails", provider: awsProvider, reason: awsNoCredentials,
			cli:     fakeCLI{version: supported, runErr: errors.New("exit status 255")},
			command: "aws login --region us-west-2",
			output:  "`aws login --region us-west-2` did not complete; run it yourself, then `pulumi up`.\n\n",
		},
		{
			name: "declined", provider: awsProvider, reason: awsNoCredentials, cli: fakeCLI{version: supported},
			decline: true, command: "aws login --region us-west-2", output: "\n",
		},
		{
			name: "AWS CLI not installed", provider: awsProvider, reason: awsNoCredentials,
			cli: fakeCLI{err: errors.New("executable file not found in $PATH")},
		},
		{
			name: "AWS CLI too old for aws login", provider: awsProvider, reason: awsNoCredentials,
			cli: fakeCLI{version: tooOldForLogin},
		},
		{
			name: "AWS CLI v1", provider: awsProvider, reason: awsExpiredSSO,
			cli: fakeCLI{version: "aws-cli/1.40.0 Python/3.12.0"},
		},
		{
			name: "no sign-in recommended", provider: awsProvider, reason: awsInvalidCredentials,
			cli: fakeCLI{version: supported},
		},
		{
			name: "other provider with the same hint", provider: gcpProvider, reason: awsNoCredentials,
			cli: fakeCLI{version: supported},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.Map{config.MustMakeKey("aws", "region"): config.NewValue("us-west-2")}
			for k, v := range tt.cfg {
				cfg[config.MustMakeKey("aws", k)] = config.NewValue(v)
			}
			cli := tt.cli
			var buf bytes.Buffer
			var offered string
			s := awsSignIn{
				cli: &cli, cfg: cfg, stdout: &buf, getenv: func(string) string { return "" },
				confirm: func(prompt string) bool {
					offered = prompt
					return !tt.decline
				},
			}

			signedIn := s.offer(t.Context(), tt.provider, &credentialsProblem{details: []string{tt.reason}})
			assert.Equal(t, tt.signedIn, signedIn)
			assert.Equal(t, tt.output, buf.String())
			if tt.command == "" {
				assert.Empty(t, offered)
				assert.Empty(t, cli.ran)
				return
			}
			assert.Equal(t, "Sign in to "+tt.provider.displayName+" now? This runs `"+tt.command+"`.", offered)
			if tt.decline {
				assert.Empty(t, cli.ran)
			} else {
				require.Len(t, cli.ran, 1)
				assert.Equal(t, tt.command, shellquote.Join(cli.ran[0]...))
			}
		})
	}
}

func TestCheckPackageSignIn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		noSignIn bool // as under --yes
		signsIn  bool
		fixed    bool // whether signing in fixes the credentials
		loads    int
		warnings int
		verified bool
		asked    bool
	}{
		{name: "no sign-in under --yes", noSignIn: true, loads: 1, warnings: 1},
		{name: "not signed in", loads: 1, warnings: 1, asked: true},
		{name: "signed in", signsIn: true, fixed: true, loads: 2, warnings: 1, verified: true, asked: true},
		{name: "still failing after sign in", signsIn: true, loads: 2, warnings: 2, asked: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			loads := 0
			fixed := false
			host := &plugin.MockHost{
				ProviderF: func(*plugin.Context, workspace.PluginDescriptor, env.Env) (plugin.Provider, error) {
					loads++
					prov := schemaProvider(`{"name":"aws","displayName":"AWS","validateCredentialsOnNew":true}`)
					prov.CheckConfigF = func(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
						if fixed {
							return plugin.CheckConfigResponse{}, nil
						}
						return plugin.CheckConfigResponse{Failures: []plugin.CheckFailure{{Reason: awsNoCredentials}}}, nil
					}
					prov.ConfigureF = func(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
						return plugin.ConfigureResponse{}, nil
					}
					return prov, nil
				},
			}
			sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
			pctx, err := plugin.NewContextWithHost(t.Context(), sink, sink, host, "", "", nil)
			require.NoError(t, err)
			defer pctx.Close()

			offered := false
			var buf bytes.Buffer
			pf := credentialsPreflight{
				host: host, pctx: pctx, cfg: config.Map{}, stdout: &buf, opts: display.Options{Color: colors.Never},
			}
			if !tt.noSignIn {
				pf.signIn = func(_ context.Context, cp cloudProvider, problem *credentialsProblem) bool {
					offered = true
					assert.Equal(t, "aws", cp.pkg)
					require.Len(t, problem.details, 1)
					assert.Contains(t, problem.details[0], "run `aws login`")
					fixed = tt.fixed
					return tt.signsIn
				}
			}
			pf.checkPackage(t.Context(), resourcePackage("aws", "7.51.1"))

			out := buf.String()
			assert.Equal(t, tt.asked, offered)
			assert.Equal(t, tt.loads, loads)
			assert.Equal(t, tt.warnings, strings.Count(out, "warning:"), out)
			assert.Equal(t, tt.verified, strings.Contains(out, "AWS credentials verified."), out)
		})
	}
}
