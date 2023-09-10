package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

// Test for https://github.com/pulumi/pulumi/issues/11703, check we give an error when trying to install a
// bundled plugin
func TestBundledError(t *testing.T) {
	t.Parallel()

	cmd := &pluginInstallCmd{
		diag: diagtest.LogSink(t),
		env: env.NewEnv(
			env.MapStore{"PULUMI_DEV": "false"},
		),
	}

	err := cmd.Run(context.Background(), []string{"language", "nodejs"})
	assert.EqualError(t, err,
		"the nodejs language plugin is bundled with Pulumi, "+
			"and cannot be directly installed with this command. "+
			"If you need to reinstall this plugin, reinstall Pulumi via your package manager or install script.")
}

// Test for https://github.com/pulumi/pulumi/issues/11703, check we still try to install bundled plugins if
// PULUMI_DEV is set.
func TestBundledDev(t *testing.T) {
	t.Parallel()

	var getLatestVersionCalled bool
	defer func() {
		assert.True(t, getLatestVersionCalled, "GetLatestVersion should have been called")
	}()

	cmd := &pluginInstallCmd{
		diag: diagtest.LogSink(t),
		env: env.NewEnv(
			env.MapStore{"PULUMI_DEV": "true"},
		),
		pluginGetLatestVersion: func(ps workspace.PluginSpec) (*semver.Version, error) {
			getLatestVersionCalled = true
			assert.Equal(t, "nodejs", ps.Name)
			assert.Equal(t, workspace.LanguagePlugin, ps.Kind)
			return nil, fmt.Errorf("404 HTTP error fetching plugin")
		},
	}

	err := cmd.Run(context.Background(), []string{"language", "nodejs"})
	assert.ErrorContains(t, err, "404 HTTP error fetching plugin")
}
