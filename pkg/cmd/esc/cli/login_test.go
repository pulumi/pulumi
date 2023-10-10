// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"testing"

	"github.com/pulumi/esc/cmd/esc/cli/workspace"
	pulumi_workspace "github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

func TestNoCreds(t *testing.T) {
	fs := testFS{}
	esc := &escCommand{workspace: workspace.New(fs, &testPulumiWorkspace{})}
	err := esc.getCachedClient(context.Background())
	assert.ErrorContains(t, err, "no credentials")

	esc.command = "pulumi"
	err = esc.getCachedClient(context.Background())
	assert.ErrorContains(t, err, "pulumi login")
}

func TestInvalidSelfHostedBackend(t *testing.T) {
	fs := testFS{}
	esc := &escCommand{workspace: workspace.New(fs, &testPulumiWorkspace{
		credentials: pulumi_workspace.Credentials{
			Current: "http://pulumi.com",
			Accounts: map[string]pulumi_workspace.Account{
				"http://pulumi.com": {},
			},
		},
	})}
	err := esc.getCachedClient(context.Background())
	assert.ErrorContains(t, err, "not a valid self-hosted backend")

	esc.command = "pulumi"
	err = esc.getCachedClient(context.Background())
	assert.ErrorContains(t, err, "pulumi login")
}

func TestFilestateBackend(t *testing.T) {
	fs := testFS{}
	esc := &escCommand{workspace: workspace.New(fs, &testPulumiWorkspace{
		credentials: pulumi_workspace.Credentials{
			Current: "gs://foo",
			Accounts: map[string]pulumi_workspace.Account{
				"gs://foo": {},
			},
		},
	})}
	err := esc.getCachedClient(context.Background())
	assert.ErrorContains(t, err, "does not support Pulumi ESC")
}
