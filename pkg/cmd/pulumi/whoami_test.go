// Copyright 2023-2024, Pulumi Corporation.
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

package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhoAmICmd_default(t *testing.T) {
	t.Parallel()

	var buff bytes.Buffer
	cmd := whoAmICmd{
		Stdout: &buff,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return &backend.MockBackend{
				CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
					return "user1", []string{"org1", "org2"}, nil, nil
				},
			}, nil
		},
	}

	err := cmd.Run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "user1\n", buff.String())
}

func TestWhoAmICmd_verbose(t *testing.T) {
	t.Parallel()

	var buff bytes.Buffer
	cmd := whoAmICmd{
		verbose: true,
		Stdout:  &buff,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return &backend.MockBackend{
				CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
					return "user2", []string{"org1", "org2"}, nil, nil
				},
				URLF: func() string {
					return "https://pulumi.example.com"
				},
			}, nil
		},
	}

	err := cmd.Run(context.Background())
	require.NoError(t, err)

	stdout := buff.String()
	assert.Contains(t, stdout, "User: user2")
	assert.Contains(t, stdout, "Organizations: org1, org2")
	assert.Contains(t, stdout, "Backend URL: https://pulumi.example.com")
	assert.Contains(t, stdout, "Token type: personal")
}

func TestWhoAmICmd_json(t *testing.T) {
	t.Parallel()

	var buff bytes.Buffer
	cmd := whoAmICmd{
		jsonOut: true,
		Stdout:  &buff,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return &backend.MockBackend{
				CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
					return "user3", []string{"org1", "org2"}, nil, nil
				},
				URLF: func() string {
					return "https://pulumi.example.com"
				},
			}, nil
		},
	}

	err := cmd.Run(context.Background())
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"user": "user3",
		"organizations": ["org1", "org2"],
		"url": "https://pulumi.example.com"
	}`, buff.String())
}

func TestWhoAmICmd_verbose_teamToken(t *testing.T) {
	t.Parallel()

	var buff bytes.Buffer
	cmd := whoAmICmd{
		verbose: true,
		Stdout:  &buff,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return &backend.MockBackend{
				CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
					return "user2", []string{"org1", "org2"}, &workspace.TokenInformation{
						Name: "team-token",
						Team: "myTeam",
					}, nil
				},
				URLF: func() string {
					return "https://pulumi.example.com"
				},
			}, nil
		},
	}

	err := cmd.Run(context.Background())
	require.NoError(t, err)

	stdout := buff.String()
	assert.Contains(t, stdout, "User: user2")
	assert.Contains(t, stdout, "Organizations: org1, org2")
	assert.Contains(t, stdout, "Backend URL: https://pulumi.example.com")
	assert.Contains(t, stdout, "Token type: team: myTeam")
	assert.Contains(t, stdout, "Token name: team-token")
}

func TestWhoAmICmd_json_teamToken(t *testing.T) {
	t.Parallel()

	var buff bytes.Buffer
	cmd := whoAmICmd{
		jsonOut: true,
		Stdout:  &buff,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return &backend.MockBackend{
				CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
					return "user3", []string{"org1", "org2"}, &workspace.TokenInformation{
						Name: "team-token",
						Team: "myTeam",
					}, nil
				},
				URLF: func() string {
					return "https://pulumi.example.com"
				},
			}, nil
		},
	}

	err := cmd.Run(context.Background())
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"user": "user3",
		"organizations": ["org1", "org2"],
		"tokenInformation": {"name": "team-token", "team": "myTeam"},
		"url": "https://pulumi.example.com"
	}`, buff.String())
}

func TestWhoAmICmd_verbose_unknownToken(t *testing.T) {
	t.Parallel()

	var buff bytes.Buffer
	cmd := whoAmICmd{
		verbose: true,
		Stdout:  &buff,
		currentBackend: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
		) (backend.Backend, error) {
			return &backend.MockBackend{
				CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
					return "user2", []string{"org1", "org2"}, &workspace.TokenInformation{
						Name: "bad-token",
					}, nil
				},
				URLF: func() string {
					return "https://pulumi.example.com"
				},
			}, nil
		},
	}

	err := cmd.Run(context.Background())
	require.NoError(t, err)

	stdout := buff.String()
	assert.Contains(t, stdout, "User: user2")
	assert.Contains(t, stdout, "Organizations: org1, org2")
	assert.Contains(t, stdout, "Backend URL: https://pulumi.example.com")
	assert.Contains(t, stdout, "Token type: unknown")
	assert.Contains(t, stdout, "Token name: bad-token")
}
