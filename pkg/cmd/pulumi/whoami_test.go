package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhoAmICmd_none(t *testing.T) {
	t.Parallel()

	var buff bytes.Buffer
	cmd := whoAmICmd{
		Stdout: &buff,
		currentBackend: func(context.Context, *workspace.Project, display.Options) (backend.Backend, error) {
			return &backend.MockBackend{
				CurrentUserF: func() (*workspace.Account, error) {
					return nil, nil
				},
			}, nil
		},
	}

	err := cmd.Run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "organization", buff.String())
}

func TestWhoAmICmd_default(t *testing.T) {
	t.Parallel()

	var buff bytes.Buffer
	cmd := whoAmICmd{
		Stdout: &buff,
		currentBackend: func(context.Context, *workspace.Project, display.Options) (backend.Backend, error) {
			return &backend.MockBackend{
				CurrentUserF: func() (*workspace.Account, error) {
					return &workspace.Account{Username: "user1", Organizations: []string{"org1", "org2"}}, nil
				},
			}, nil
		},
	}

	err := cmd.Run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "user1", buff.String())
}

func TestWhoAmICmd_verbose(t *testing.T) {
	t.Parallel()

	var buff bytes.Buffer
	cmd := whoAmICmd{
		verbose: true,
		Stdout:  &buff,
		currentBackend: func(context.Context, *workspace.Project, display.Options) (backend.Backend, error) {
			return &backend.MockBackend{
				CurrentUserF: func() (*workspace.Account, error) {
					return &workspace.Account{Username: "user2", Organizations: []string{"org1", "org2"}}, nil
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
}

func TestWhoAmICmd_json(t *testing.T) {
	t.Parallel()

	var buff bytes.Buffer
	cmd := whoAmICmd{
		jsonOut: true,
		Stdout:  &buff,
		currentBackend: func(context.Context, *workspace.Project, display.Options) (backend.Backend, error) {
			return &backend.MockBackend{
				CurrentUserF: func() (*workspace.Account, error) {
					return &workspace.Account{Username: "user3", Organizations: []string{"org1", "org2"}}, nil
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
