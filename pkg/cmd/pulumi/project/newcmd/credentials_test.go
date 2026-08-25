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
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func awsTemplate() cmdTemplates.ProjectTemplate {
	return cmdTemplates.ProjectTemplate{
		Config: map[string]workspace.ProjectTemplateConfigValue{
			"aws:region": {Description: "The AWS region to deploy into", Default: "us-east-1"},
		},
	}
}

func TestShouldCheckAWSCredentials(t *testing.T) {
	tests := []struct {
		name     string
		args     newArgs
		template cmdTemplates.ProjectTemplate
		env      string
		expected bool
	}{
		{
			name:     "aws template interactive",
			args:     newArgs{interactive: true},
			template: awsTemplate(),
			expected: true,
		},
		{
			name:     "no template config",
			args:     newArgs{interactive: true},
			template: cmdTemplates.ProjectTemplate{},
			expected: false,
		},
		{
			name: "aws-native namespace does not match",
			args: newArgs{interactive: true},
			template: cmdTemplates.ProjectTemplate{
				Config: map[string]workspace.ProjectTemplateConfigValue{
					"aws-native:region": {},
				},
			},
			expected: false,
		},
		{
			name: "gcp template",
			args: newArgs{interactive: true},
			template: cmdTemplates.ProjectTemplate{
				Config: map[string]workspace.ProjectTemplateConfigValue{
					"gcp:project": {},
				},
			},
			expected: false,
		},
		{
			name:     "non-interactive",
			args:     newArgs{interactive: false},
			template: awsTemplate(),
			expected: false,
		},
		{
			name:     "generate-only",
			args:     newArgs{interactive: true, generateOnly: true},
			template: awsTemplate(),
			expected: false,
		},
		{
			name:     "offline",
			args:     newArgs{interactive: true, offline: true},
			template: awsTemplate(),
			expected: false,
		},
		{
			name:     "kill switch",
			args:     newArgs{interactive: true},
			template: awsTemplate(),
			env:      "true",
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv("PULUMI_SKIP_NEW_CREDENTIALS_CHECK", tt.env)
			}
			assert.Equal(t, tt.expected, shouldCheckAWSCredentials(tt.args, tt.template))
		})
	}
}

func TestAWSConfigProperties(t *testing.T) {
	t.Parallel()

	cfg := config.Map{
		config.MustMakeKey("aws", "region"):  config.NewValue("us-east-1"),
		config.MustMakeKey("aws", "secret"):  config.NewSecureValue("ciphertext"),
		config.MustMakeKey("proj", "etcetc"): config.NewValue("unrelated"),
	}
	props := awsConfigProperties(cfg)
	require.Equal(t, 1, props.Len())
	assert.Equal(t, "us-east-1", props.Get("region").AsString())
}

func TestCheckAWSCredentialsRPCError(t *testing.T) {
	t.Parallel()

	closed := false
	mock := &plugin.MockProvider{
		CheckConfigF: func(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			assert.Equal(t, "pulumi:providers:aws", string(req.URN.Type()))
			assert.Empty(t, req.Name)
			assert.Empty(t, req.Type)
			assert.Equal(t, "us-east-1", req.News.Get("region").AsString())
			return plugin.CheckConfigResponse{},
				status.Error(codes.Unknown, "unable to validate AWS credentials.\nDetails: no valid credential sources found")
		},
		CloseF: func() error {
			closed = true
			return nil
		},
	}

	var buf bytes.Buffer
	news := property.NewMap(map[string]property.Value{"region": property.New("us-east-1")})
	warned := checkAWSCredentials(t.Context(),
		func() (plugin.Provider, error) { return mock, nil },
		news, &buf, display.Options{Color: colors.Never}, time.Second)
	assert.True(t, warned)

	out := buf.String()
	assert.Contains(t, out, "warning:")
	assert.Contains(t, out, "Could not validate your AWS credentials")
	assert.Contains(t, out, "unable to validate AWS credentials.")
	assert.Contains(t, out, "    Details: no valid credential sources found")
	assert.Contains(t, out, awsCredentialsDocURL)
	assert.True(t, closed)
}

func TestCheckAWSCredentialsFailures(t *testing.T) {
	t.Parallel()

	mock := &plugin.MockProvider{
		CheckConfigF: func(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			return plugin.CheckConfigResponse{Failures: []plugin.CheckFailure{
				{Property: "region", Reason: "expected a valid region"},
				{Reason: "unable to validate AWS credentials.\nDetails: no valid credential sources found"},
			}}, nil
		},
	}

	var buf bytes.Buffer
	warned := checkAWSCredentials(t.Context(),
		func() (plugin.Provider, error) { return mock, nil },
		property.Map{}, &buf, display.Options{Color: colors.Never}, time.Second)
	assert.True(t, warned)

	out := buf.String()
	assert.Contains(t, out, "The AWS provider reported problems with this stack's configuration")
	assert.Contains(t, out, "    region: expected a valid region")
	assert.Contains(t, out, "    unable to validate AWS credentials.\n    Details: no valid credential sources found")
	assert.Contains(t, out, awsCredentialsDocURL)
}

func TestCheckAWSCredentialsSuccess(t *testing.T) {
	t.Parallel()

	mock := &plugin.MockProvider{
		CheckConfigF: func(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			return plugin.CheckConfigResponse{Properties: req.News}, nil
		},
	}

	var buf bytes.Buffer
	warned := checkAWSCredentials(t.Context(),
		func() (plugin.Provider, error) { return mock, nil },
		property.Map{}, &buf, display.Options{Color: colors.Never}, time.Second)

	assert.False(t, warned)
	assert.Empty(t, buf.String())
}

func TestCheckAWSCredentialsTimeout(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	mock := &plugin.MockProvider{
		CheckConfigF: func(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			<-release
			return plugin.CheckConfigResponse{}, nil
		},
	}

	var buf bytes.Buffer
	var warned bool
	done := make(chan struct{})
	go func() {
		warned = checkAWSCredentials(t.Context(),
			func() (plugin.Provider, error) { return mock, nil },
			property.Map{}, &buf, display.Options{Color: colors.Never}, 10*time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("checkAWSCredentials did not return after its timeout")
	}
	assert.False(t, warned)
	assert.Empty(t, buf.String())
}

func TestCheckAWSCredentialsLoaderError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	warned := checkAWSCredentials(t.Context(),
		func() (plugin.Provider, error) { return nil, errors.New("no such plugin") },
		property.Map{}, &buf, display.Options{Color: colors.Never}, time.Second)

	assert.False(t, warned)
	assert.Empty(t, buf.String())
}
