// Copyright 2016-2020, Pulumi Corporation.
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

package auto

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

var pulumiOrg = getTestOrg()

const pName = "testproj"
const agent = "pulumi/pulumi/test"

func TestWorkspaceSecretsProvider(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)

	opts := []LocalWorkspaceOption{
		SecretsProvider("passphrase"),
		EnvVars(map[string]string{
			"PULUMI_CONFIG_PASSPHRASE": "password",
		}),
	}

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, pName, func(ctx *pulumi.Context) error {
		c := config.New(ctx, "")
		ctx.Export("exp_static", pulumi.String("foo"))
		ctx.Export("exp_cfg", pulumi.String(c.Get("bar")))
		ctx.Export("exp_secret", c.GetSecret("buzz"))
		return nil
	}, opts...)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		err := os.Unsetenv("PULUMI_CONFIG_PASSPHRASE")
		assert.Nil(t, err, "failed to unset EnvVar.")

		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	passwordVal := "Password1234!"
	err = s.SetConfig(ctx, "MySecretDatabasePassword", ConfigValue{Value: passwordVal, Secret: true})
	if err != nil {
		t.Errorf("setConfig failed, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	res, err := s.Up(ctx)
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)

	// -- get config --
	conf, err := s.GetConfig(ctx, "MySecretDatabasePassword")
	if err != nil {
		t.Errorf("GetConfig failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, passwordVal, conf.Value)
	assert.Equal(t, true, conf.Secret)

	// -- pulumi destroy --

	dRes, err := s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}

func TestNewStackLocalSource(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	cfg := ConfigMap{
		"bar": ConfigValue{
			Value: "abc",
		},
		"buzz": ConfigValue{
			Value:  "secret",
			Secret: true,
		},
	}

	// initialize
	pDir := filepath.Join(".", "test", "testproj")
	s, err := NewStackLocalSource(ctx, stackName, pDir)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	// Set environment variables scoped to the workspace.
	envvars := map[string]string{
		"foo":    "bar",
		"barfoo": "foobar",
	}
	err = s.Workspace().SetEnvVars(envvars)
	assert.Nil(t, err, "failed to set environment values")
	envvars = s.Workspace().GetEnvVars()
	assert.NotNil(t, envvars, "failed to get environment values after setting many")

	s.Workspace().SetEnvVar("bar", "buzz")
	envvars = s.Workspace().GetEnvVars()
	assert.NotNil(t, envvars, "failed to get environment value after setting")

	s.Workspace().UnsetEnvVar("bar")
	envvars = s.Workspace().GetEnvVars()
	assert.NotNil(t, envvars, "failed to get environment values after unsetting.")

	// -- pulumi up --
	res, err := s.Up(ctx, optup.UserAgent(agent))
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 3, len(res.Outputs), "expected two plain outputs")
	assert.Equal(t, "foo", res.Outputs["exp_static"].Value)
	assert.False(t, res.Outputs["exp_static"].Secret)
	assert.Equal(t, "abc", res.Outputs["exp_cfg"].Value)
	assert.False(t, res.Outputs["exp_cfg"].Secret)
	assert.Equal(t, "secret", res.Outputs["exp_secret"].Value)
	assert.True(t, res.Outputs["exp_secret"].Secret)
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)

	const permalinkSearchStr = "https://app.pulumi.com"
	var startRegex = regexp.MustCompile(permalinkSearchStr)
	permalink, err := GetPermalink(res.StdOut)
	assert.Nil(t, err, "failed to get permalink.")
	assert.True(t, startRegex.MatchString(permalink))

	// -- pulumi preview --

	var previewEvents []events.EngineEvent
	prevCh := make(chan events.EngineEvent)
	go collectEvents(prevCh, &previewEvents)
	prev, err := s.Preview(ctx, optpreview.EventStreams(prevCh), optpreview.UserAgent(agent))
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, 1, prev.ChangeSummary[apitype.OpSame])
	steps := countSteps(previewEvents)
	assert.Equal(t, 1, steps)

	// -- pulumi refresh --

	ref, err := s.Refresh(ctx, optrefresh.UserAgent(agent))

	if err != nil {
		t.Errorf("refresh failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "refresh", ref.Summary.Kind)
	assert.Equal(t, "succeeded", ref.Summary.Result)

	// -- pulumi destroy --

	dRes, err := s.Destroy(ctx, optdestroy.UserAgent(agent))
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}

func TestUpsertStackLocalSource(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	cfg := ConfigMap{
		"bar": ConfigValue{
			Value: "abc",
		},
		"buzz": ConfigValue{
			Value:  "secret",
			Secret: true,
		},
	}

	// initialize
	pDir := filepath.Join(".", "test", "testproj")
	s, err := UpsertStackLocalSource(ctx, stackName, pDir)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	// Set environment variables scoped to the workspace.
	envvars := map[string]string{
		"foo":    "bar",
		"barfoo": "foobar",
	}
	err = s.Workspace().SetEnvVars(envvars)
	assert.Nil(t, err, "failed to set environment values")
	envvars = s.Workspace().GetEnvVars()
	assert.NotNil(t, envvars, "failed to get environment values after setting many")

	s.Workspace().SetEnvVar("bar", "buzz")
	envvars = s.Workspace().GetEnvVars()
	assert.NotNil(t, envvars, "failed to get environment value after setting")

	s.Workspace().UnsetEnvVar("bar")
	envvars = s.Workspace().GetEnvVars()
	assert.NotNil(t, envvars, "failed to get environment values after unsetting.")

	// -- pulumi up --
	res, err := s.Up(ctx)
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 3, len(res.Outputs), "expected two plain outputs")
	assert.Equal(t, "foo", res.Outputs["exp_static"].Value)
	assert.False(t, res.Outputs["exp_static"].Secret)
	assert.Equal(t, "abc", res.Outputs["exp_cfg"].Value)
	assert.False(t, res.Outputs["exp_cfg"].Secret)
	assert.Equal(t, "secret", res.Outputs["exp_secret"].Value)
	assert.True(t, res.Outputs["exp_secret"].Secret)
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)

	// -- pulumi preview --

	var previewEvents []events.EngineEvent
	prevCh := make(chan events.EngineEvent)
	go collectEvents(prevCh, &previewEvents)
	prev, err := s.Preview(ctx, optpreview.EventStreams(prevCh))
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 1, prev.ChangeSummary[apitype.OpSame])
	steps := countSteps(previewEvents)
	assert.Equal(t, 1, steps)

	// -- pulumi refresh --
	ref, err := s.Refresh(ctx)
	if err != nil {
		t.Errorf("refresh failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "refresh", ref.Summary.Kind)
	assert.Equal(t, "succeeded", ref.Summary.Result)

	// -- pulumi destroy --
	dRes, err := s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}

func rangeIn(low, hi int) int {
	rand.Seed(time.Now().UnixNano())
	return low + rand.Intn(hi-low) //nolint:gosec
}

func TestNewStackRemoteSource(t *testing.T) {
	ctx := context.Background()
	pName := "go_remote_proj"
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	cfg := ConfigMap{
		"bar": ConfigValue{
			Value: "abc",
		},
		"buzz": ConfigValue{
			Value:  "secret",
			Secret: true,
		},
	}
	repo := GitRepo{
		URL:         "https://github.com/pulumi/test-repo.git",
		ProjectPath: "goproj",
	}

	// initialize
	s, err := NewStackRemoteSource(ctx, stackName, repo)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	res, err := s.Up(ctx)
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 3, len(res.Outputs), "expected two plain outputs")
	assert.Equal(t, "foo", res.Outputs["exp_static"].Value)
	assert.False(t, res.Outputs["exp_static"].Secret)
	assert.Equal(t, "abc", res.Outputs["exp_cfg"].Value)
	assert.False(t, res.Outputs["exp_cfg"].Secret)
	assert.Equal(t, "secret", res.Outputs["exp_secret"].Value)
	assert.True(t, res.Outputs["exp_secret"].Secret)
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)

	// -- pulumi preview --

	var previewEvents []events.EngineEvent
	prevCh := make(chan events.EngineEvent)
	go collectEvents(prevCh, &previewEvents)
	prev, err := s.Preview(ctx, optpreview.EventStreams(prevCh))
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, 1, prev.ChangeSummary[apitype.OpSame])
	steps := countSteps(previewEvents)
	assert.Equal(t, 1, steps)

	// -- pulumi refresh --

	ref, err := s.Refresh(ctx)

	if err != nil {
		t.Errorf("refresh failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "refresh", ref.Summary.Kind)
	assert.Equal(t, "succeeded", ref.Summary.Result)

	// -- pulumi destroy --

	dRes, err := s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}

func TestUpsertStackRemoteSource(t *testing.T) {
	ctx := context.Background()
	pName := "go_remote_proj"
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	cfg := ConfigMap{
		"bar": ConfigValue{
			Value: "abc",
		},
		"buzz": ConfigValue{
			Value:  "secret",
			Secret: true,
		},
	}
	repo := GitRepo{
		URL:         "https://github.com/pulumi/test-repo.git",
		ProjectPath: "goproj",
	}

	// initialize
	s, err := UpsertStackRemoteSource(ctx, stackName, repo)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	res, err := s.Up(ctx)
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 3, len(res.Outputs), "expected two plain outputs")
	assert.Equal(t, "foo", res.Outputs["exp_static"].Value)
	assert.False(t, res.Outputs["exp_static"].Secret)
	assert.Equal(t, "abc", res.Outputs["exp_cfg"].Value)
	assert.False(t, res.Outputs["exp_cfg"].Secret)
	assert.Equal(t, "secret", res.Outputs["exp_secret"].Value)
	assert.True(t, res.Outputs["exp_secret"].Secret)
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)

	// -- pulumi preview --

	var previewEvents []events.EngineEvent
	prevCh := make(chan events.EngineEvent)
	go collectEvents(prevCh, &previewEvents)
	prev, err := s.Preview(ctx, optpreview.EventStreams(prevCh))
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, 1, prev.ChangeSummary[apitype.OpSame])
	steps := countSteps(previewEvents)
	assert.Equal(t, 1, steps)

	// -- pulumi refresh --

	ref, err := s.Refresh(ctx)

	if err != nil {
		t.Errorf("refresh failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "refresh", ref.Summary.Kind)
	assert.Equal(t, "succeeded", ref.Summary.Result)

	// -- pulumi destroy --

	dRes, err := s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}

func TestNewStackRemoteSourceWithSetup(t *testing.T) {
	ctx := context.Background()
	pName := "go_remote_proj"
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	cfg := ConfigMap{
		"bar": ConfigValue{
			Value: "abc",
		},
		"buzz": ConfigValue{
			Value:  "secret",
			Secret: true,
		},
	}
	binName := "examplesBinary"
	repo := GitRepo{
		URL:         "https://github.com/pulumi/test-repo.git",
		ProjectPath: "goproj",
		Setup: func(ctx context.Context, workspace Workspace) error {
			cmd := exec.Command("go", "build", "-o", binName, "main.go")
			cmd.Dir = workspace.WorkDir()
			return cmd.Run()
		},
	}
	project := workspace.Project{
		Name: tokens.PackageName(pName),
		Runtime: workspace.NewProjectRuntimeInfo("go", map[string]interface{}{
			"binary": binName,
		}),
	}

	// initialize
	s, err := NewStackRemoteSource(ctx, stackName, repo, Project(project))
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	res, err := s.Up(ctx)
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 3, len(res.Outputs), "expected two plain outputs")
	assert.Equal(t, "foo", res.Outputs["exp_static"].Value)
	assert.False(t, res.Outputs["exp_static"].Secret)
	assert.Equal(t, "abc", res.Outputs["exp_cfg"].Value)
	assert.False(t, res.Outputs["exp_cfg"].Secret)
	assert.Equal(t, "secret", res.Outputs["exp_secret"].Value)
	assert.True(t, res.Outputs["exp_secret"].Secret)
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)

	// -- pulumi preview --

	var previewEvents []events.EngineEvent
	prevCh := make(chan events.EngineEvent)
	go collectEvents(prevCh, &previewEvents)
	prev, err := s.Preview(ctx, optpreview.EventStreams(prevCh))
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, 1, prev.ChangeSummary[apitype.OpSame])
	steps := countSteps(previewEvents)
	assert.Equal(t, 1, steps)

	// -- pulumi refresh --

	ref, err := s.Refresh(ctx)

	if err != nil {
		t.Errorf("refresh failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "refresh", ref.Summary.Kind)
	assert.Equal(t, "succeeded", ref.Summary.Result)

	// -- pulumi destroy --

	dRes, err := s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}

func TestUpsertStackRemoteSourceWithSetup(t *testing.T) {
	ctx := context.Background()
	pName := "go_remote_proj"
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	cfg := ConfigMap{
		"bar": ConfigValue{
			Value: "abc",
		},
		"buzz": ConfigValue{
			Value:  "secret",
			Secret: true,
		},
	}
	binName := "examplesBinary"
	repo := GitRepo{
		URL:         "https://github.com/pulumi/test-repo.git",
		ProjectPath: "goproj",
		Setup: func(ctx context.Context, workspace Workspace) error {
			cmd := exec.Command("go", "build", "-o", binName, "main.go")
			cmd.Dir = workspace.WorkDir()
			return cmd.Run()
		},
	}
	project := workspace.Project{
		Name: tokens.PackageName(pName),
		Runtime: workspace.NewProjectRuntimeInfo("go", map[string]interface{}{
			"binary": binName,
		}),
	}

	// initialize or select
	s, err := UpsertStackRemoteSource(ctx, stackName, repo, Project(project))
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	res, err := s.Up(ctx)
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 3, len(res.Outputs), "expected two plain outputs")
	assert.Equal(t, "foo", res.Outputs["exp_static"].Value)
	assert.False(t, res.Outputs["exp_static"].Secret)
	assert.Equal(t, "abc", res.Outputs["exp_cfg"].Value)
	assert.False(t, res.Outputs["exp_cfg"].Secret)
	assert.Equal(t, "secret", res.Outputs["exp_secret"].Value)
	assert.True(t, res.Outputs["exp_secret"].Secret)
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)

	// -- pulumi preview --

	var previewEvents []events.EngineEvent
	prevCh := make(chan events.EngineEvent)
	go collectEvents(prevCh, &previewEvents)
	prev, err := s.Preview(ctx, optpreview.EventStreams(prevCh))
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, 1, prev.ChangeSummary[apitype.OpSame])
	steps := countSteps(previewEvents)
	assert.Equal(t, 1, steps)

	// -- pulumi refresh --

	ref, err := s.Refresh(ctx)

	if err != nil {
		t.Errorf("refresh failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "refresh", ref.Summary.Kind)
	assert.Equal(t, "succeeded", ref.Summary.Result)

	// -- pulumi destroy --

	dRes, err := s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}

func TestNewStackInlineSource(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	cfg := ConfigMap{
		"bar": ConfigValue{
			Value: "abc",
		},
		"buzz": ConfigValue{
			Value:  "secret",
			Secret: true,
		},
	}

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, pName, func(ctx *pulumi.Context) error {
		c := config.New(ctx, "")
		ctx.Export("exp_static", pulumi.String("foo"))
		ctx.Export("exp_cfg", pulumi.String(c.Get("bar")))
		ctx.Export("exp_secret", c.GetSecret("buzz"))
		return nil
	})
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	res, err := s.Up(ctx, optup.UserAgent(agent))
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 3, len(res.Outputs), "expected two plain outputs")
	assert.Equal(t, "foo", res.Outputs["exp_static"].Value)
	assert.False(t, res.Outputs["exp_static"].Secret)
	assert.Equal(t, "abc", res.Outputs["exp_cfg"].Value)
	assert.False(t, res.Outputs["exp_cfg"].Secret)
	assert.Equal(t, "secret", res.Outputs["exp_secret"].Value)
	assert.True(t, res.Outputs["exp_secret"].Secret)
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)
	assert.Greater(t, res.Summary.Version, 0)

	// -- pulumi preview --

	var previewEvents []events.EngineEvent
	prevCh := make(chan events.EngineEvent)
	go collectEvents(prevCh, &previewEvents)
	prev, err := s.Preview(ctx, optpreview.EventStreams(prevCh), optpreview.UserAgent(agent))
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, 1, prev.ChangeSummary[apitype.OpSame])
	steps := countSteps(previewEvents)
	assert.Equal(t, 1, steps)

	// -- pulumi refresh --

	ref, err := s.Refresh(ctx, optrefresh.UserAgent(agent))

	if err != nil {
		t.Errorf("refresh failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "refresh", ref.Summary.Kind)
	assert.Equal(t, "succeeded", ref.Summary.Result)

	// -- pulumi destroy --

	dRes, err := s.Destroy(ctx, optdestroy.UserAgent(agent))
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}

func TestUpsertStackInlineSource(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	cfg := ConfigMap{
		"bar": ConfigValue{
			Value: "abc",
		},
		"buzz": ConfigValue{
			Value:  "secret",
			Secret: true,
		},
	}

	// initialize or select
	s, err := UpsertStackInlineSource(ctx, stackName, pName, func(ctx *pulumi.Context) error {
		c := config.New(ctx, "")
		ctx.Export("exp_static", pulumi.String("foo"))
		ctx.Export("exp_cfg", pulumi.String(c.Get("bar")))
		ctx.Export("exp_secret", c.GetSecret("buzz"))
		return nil
	})
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	res, err := s.Up(ctx)
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 3, len(res.Outputs), "expected two plain outputs")
	assert.Equal(t, "foo", res.Outputs["exp_static"].Value)
	assert.False(t, res.Outputs["exp_static"].Secret)
	assert.Equal(t, "abc", res.Outputs["exp_cfg"].Value)
	assert.False(t, res.Outputs["exp_cfg"].Secret)
	assert.Equal(t, "secret", res.Outputs["exp_secret"].Value)
	assert.True(t, res.Outputs["exp_secret"].Secret)
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)

	// -- pulumi preview --

	var previewEvents []events.EngineEvent
	prevCh := make(chan events.EngineEvent)
	go collectEvents(prevCh, &previewEvents)
	prev, err := s.Preview(ctx, optpreview.EventStreams(prevCh))
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, 1, prev.ChangeSummary[apitype.OpSame])
	steps := countSteps(previewEvents)
	assert.Equal(t, 1, steps)

	// -- pulumi refresh --

	ref, err := s.Refresh(ctx)

	if err != nil {
		t.Errorf("refresh failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "refresh", ref.Summary.Kind)
	assert.Equal(t, "succeeded", ref.Summary.Result)

	// -- pulumi destroy --

	dRes, err := s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}

func TestNestedStackFails(t *testing.T) {
	// FIXME: see https://github.com/pulumi/pulumi/issues/5301
	t.Skip("skipping test, see pulumi/pulumi#5301")
	testCtx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	parentstackName := FullyQualifiedStackName(pulumiOrg, "parent", sName)
	nestedstackName := FullyQualifiedStackName(pulumiOrg, "nested", sName)

	nestedStack, err := NewStackInlineSource(testCtx, nestedstackName, "nested", func(ctx *pulumi.Context) error {
		ctx.Export("exp_static", pulumi.String("foo"))
		return nil
	})
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	// initialize
	s, err := NewStackInlineSource(testCtx, parentstackName, "parent", func(ctx *pulumi.Context) error {
		_, err := nestedStack.Up(testCtx)
		return err
	})
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(testCtx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")

		err = nestedStack.Workspace().RemoveStack(testCtx, nestedStack.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	result, err := s.Up(testCtx)

	t.Log(result)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nested stack operations are not supported")

	// -- pulumi destroy --

	dRes, err := s.Destroy(testCtx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)

	dRes, err = nestedStack.Destroy(testCtx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}

func TestProgressStreams(t *testing.T) {
	ctx := context.Background()
	pName := "inline_progress_streams"
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	cfg := ConfigMap{
		"bar": ConfigValue{
			Value: "abc",
		},
		"buzz": ConfigValue{
			Value:  "secret",
			Secret: true,
		},
	}

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, pName, func(ctx *pulumi.Context) error {
		c := config.New(ctx, "")
		ctx.Export("exp_static", pulumi.String("foo"))
		ctx.Export("exp_cfg", pulumi.String(c.Get("bar")))
		ctx.Export("exp_secret", c.GetSecret("buzz"))
		return nil
	})
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	var upOut bytes.Buffer
	res, err := s.Up(ctx, optup.ProgressStreams(&upOut))
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, upOut.String(), res.StdOut, "expected stdout writers to contain same contents")

	// -- pulumi refresh --
	var refOut bytes.Buffer
	ref, err := s.Refresh(ctx, optrefresh.ProgressStreams(&refOut))

	if err != nil {
		t.Errorf("refresh failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, refOut.String(), ref.StdOut, "expected stdout writers to contain same contents")

	// -- pulumi destroy --
	var desOut bytes.Buffer
	dRes, err := s.Destroy(ctx, optdestroy.ProgressStreams(&desOut))
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, desOut.String(), dRes.StdOut, "expected stdout writers to contain same contents")
}

func TestImportExportStack(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	cfg := ConfigMap{
		"bar": ConfigValue{
			Value: "abc",
		},
		"buzz": ConfigValue{
			Value:  "secret",
			Secret: true,
		},
	}

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, pName, func(ctx *pulumi.Context) error {
		c := config.New(ctx, "")
		ctx.Export("exp_static", pulumi.String("foo"))
		ctx.Export("exp_cfg", pulumi.String(c.Get("bar")))
		ctx.Export("exp_secret", c.GetSecret("buzz"))
		return nil
	})
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	_, err = s.Up(ctx)
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	// -- pulumi stack export --
	state, err := s.Export(ctx)
	if err != nil {
		t.Errorf("export failed, err: %v", err)
		t.FailNow()
	}

	// -- pulumi stack import --
	err = s.Import(ctx, state)
	if err != nil {
		t.Errorf("import failed, err: %v", err)
		t.FailNow()
	}

	// -- pulumi destroy --

	dRes, err := s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}

func TestNestedConfig(t *testing.T) {
	if getTestOrg() != "pulumi-test" {
		return
	}
	ctx := context.Background()
	stackName := FullyQualifiedStackName(pulumiOrg, "nested_config", "dev")

	// initialize
	pDir := filepath.Join(".", "test", "nested_config")
	s, err := UpsertStackLocalSource(ctx, stackName, pDir)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	allConfig, err := s.GetAllConfig(ctx)
	if err != nil {
		t.Errorf("failed to get config, err: %v", err)
		t.FailNow()
	}

	outerVal, ok := allConfig["nested_config:outer"]
	assert.True(t, ok)
	assert.True(t, outerVal.Secret)
	assert.JSONEq(t, "{\"inner\":\"my_secret\", \"other\": \"something_else\"}", outerVal.Value)

	listVal, ok := allConfig["nested_config:myList"]
	assert.True(t, ok)
	assert.False(t, listVal.Secret)
	assert.JSONEq(t, "[\"one\",\"two\",\"three\"]", listVal.Value)

	outer, err := s.GetConfig(ctx, "outer")
	if err != nil {
		t.Errorf("failed to get config, err: %v", err)
		t.FailNow()
	}
	assert.True(t, outer.Secret)
	assert.JSONEq(t, "{\"inner\":\"my_secret\", \"other\": \"something_else\"}", outer.Value)

	list, err := s.GetConfig(ctx, "myList")
	if err != nil {
		t.Errorf("failed to get config, err: %v", err)
		t.FailNow()
	}
	assert.False(t, list.Secret)
	assert.JSONEq(t, "[\"one\",\"two\",\"three\"]", list.Value)
}

func TestStructuredOutput(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	cfg := ConfigMap{
		"bar": ConfigValue{
			Value: "abc",
		},
		"buzz": ConfigValue{
			Value:  "secret",
			Secret: true,
		},
	}

	// initialize
	pDir := filepath.Join(".", "test", "testproj")
	s, err := UpsertStackLocalSource(ctx, stackName, pDir)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	// Set environment variables scoped to the workspace.
	envvars := map[string]string{
		"foo":    "bar",
		"barfoo": "foobar",
	}
	err = s.Workspace().SetEnvVars(envvars)
	assert.Nil(t, err, "failed to set environment values")
	envvars = s.Workspace().GetEnvVars()
	assert.NotNil(t, envvars, "failed to get environment values after setting many")

	s.Workspace().SetEnvVar("bar", "buzz")
	envvars = s.Workspace().GetEnvVars()
	assert.NotNil(t, envvars, "failed to get environment value after setting")

	s.Workspace().UnsetEnvVar("bar")
	envvars = s.Workspace().GetEnvVars()
	assert.NotNil(t, envvars, "failed to get environment values after unsetting.")

	// -- pulumi up --
	var upEvents []events.EngineEvent
	upCh := make(chan events.EngineEvent)
	go collectEvents(upCh, &upEvents)
	res, err := s.Up(ctx, optup.EventStreams(upCh))
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 3, len(res.Outputs), "expected two plain outputs")
	assert.Equal(t, "foo", res.Outputs["exp_static"].Value)
	assert.False(t, res.Outputs["exp_static"].Secret)
	assert.Equal(t, "abc", res.Outputs["exp_cfg"].Value)
	assert.False(t, res.Outputs["exp_cfg"].Secret)
	assert.Equal(t, "secret", res.Outputs["exp_secret"].Value)
	assert.True(t, res.Outputs["exp_secret"].Secret)
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)
	assert.True(t, containsSummary(upEvents))

	// -- pulumi preview --
	var previewEvents []events.EngineEvent
	prevCh := make(chan events.EngineEvent)
	go collectEvents(prevCh, &previewEvents)
	prev, err := s.Preview(ctx, optpreview.EventStreams(prevCh))
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 1, prev.ChangeSummary[apitype.OpSame])
	steps := countSteps(previewEvents)
	assert.Equal(t, 1, steps)
	assert.True(t, containsSummary(previewEvents))

	// -- pulumi refresh --
	var refreshEvents []events.EngineEvent
	refCh := make(chan events.EngineEvent)
	go collectEvents(refCh, &refreshEvents)
	ref, err := s.Refresh(ctx, optrefresh.EventStreams(refCh))
	if err != nil {
		t.Errorf("refresh failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "refresh", ref.Summary.Kind)
	assert.Equal(t, "succeeded", ref.Summary.Result)
	assert.True(t, containsSummary(refreshEvents))

	// -- pulumi destroy --
	var destroyEvents []events.EngineEvent
	desCh := make(chan events.EngineEvent)
	go collectEvents(desCh, &destroyEvents)
	dRes, err := s.Destroy(ctx, optdestroy.EventStreams(desCh))
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
	assert.True(t, containsSummary(destroyEvents))
}

func TestSupportsStackOutputs(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	cfg := ConfigMap{
		"bar": ConfigValue{
			Value: "abc",
		},
		"buzz": ConfigValue{
			Value:  "secret",
			Secret: true,
		},
	}

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, pName, func(ctx *pulumi.Context) error {
		c := config.New(ctx, "")
		ctx.Export("exp_static", pulumi.String("foo"))
		ctx.Export("exp_cfg", pulumi.String(c.Get("bar")))
		ctx.Export("exp_secret", c.GetSecret("buzz"))
		return nil
	})
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	assertOutputs := func(t *testing.T, outputs OutputMap) {
		assert.Equal(t, 3, len(outputs), "expected three outputs")
		assert.Equal(t, "foo", outputs["exp_static"].Value)
		assert.False(t, outputs["exp_static"].Secret)
		assert.Equal(t, "abc", outputs["exp_cfg"].Value)
		assert.False(t, outputs["exp_cfg"].Secret)
		assert.Equal(t, "secret", outputs["exp_secret"].Value)
		assert.True(t, outputs["exp_secret"].Secret)
	}

	initialOutputs, err := s.Outputs(ctx)
	if err != nil {
		t.Errorf("failed to get initial outputs, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 0, len(initialOutputs))

	// -- pulumi up --
	res, err := s.Up(ctx)
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)
	assert.Greater(t, res.Summary.Version, 0)
	assertOutputs(t, res.Outputs)

	outputsAfterUp, err := s.Outputs(ctx)
	if err != nil {
		t.Errorf("failed to get outputs after up, err: %v", err)
		t.FailNow()
	}

	assertOutputs(t, outputsAfterUp)

	// -- pulumi destroy --
	dRes, err := s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)

	outputsAfterDestroy, err := s.Outputs(ctx)
	if err != nil {
		t.Errorf("failed to get outputs after destroy, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 0, len(outputsAfterDestroy))
}

func TestPulumiVersion(t *testing.T) {
	ctx := context.Background()
	ws, err := NewLocalWorkspace(ctx)
	if err != nil {
		t.Errorf("failed to create workspace, err: %v", err)
		t.FailNow()
	}
	version := ws.PulumiVersion()
	assert.NotEqual(t, "v0.0.0", version)
	assert.Regexp(t, `(\d+\.)(\d+\.)(\d+)(-.*)?`, version)
}

var minVersionTests = []struct {
	name           string
	currentVersion semver.Version
	expectError    bool
	optOut         bool
}{
	{
		"higher_major",
		semver.Version{Major: 100, Minor: 0, Patch: 0},
		true,
		false,
	},
	{
		"lower_major",
		semver.Version{Major: 1, Minor: 0, Patch: 0},
		true,
		false,
	},
	{
		"higher_minor",
		semver.Version{Major: 2, Minor: 22, Patch: 0},
		false,
		false,
	},
	{
		"lower_minor",
		semver.Version{Major: 2, Minor: 1, Patch: 0},
		true,
		false,
	},
	{
		"equal_minor_higher_patch",
		semver.Version{Major: 2, Minor: 21, Patch: 2},
		false,
		false,
	},
	{
		"equal_minor_equal_patch",
		semver.Version{Major: 2, Minor: 21, Patch: 1},
		false,
		false,
	},
	{
		"equal_minor_lower_patch",
		semver.Version{Major: 2, Minor: 21, Patch: 0},
		true,
		false,
	},
	{
		"equal_minor_equal_patch_prerelease",
		// Note that prerelease < release so this case will error
		semver.Version{Major: 2, Minor: 21, Patch: 1,
			Pre: []semver.PRVersion{{VersionStr: "alpha"}, {VersionNum: 1234, IsNum: true}}},
		true,
		false,
	},
	{
		"opt_out_of_check_would_fail_otherwise",
		semver.Version{Major: 2, Minor: 20, Patch: 0},
		false,
		true,
	},
	{
		"opt_out_of_check_would_succeed_otherwise",
		semver.Version{Major: 2, Minor: 22, Patch: 0},
		false,
		true,
	},
}

func TestMinimumVersion(t *testing.T) {
	for _, tt := range minVersionTests {
		t.Run(tt.name, func(t *testing.T) {
			minVersion := semver.Version{Major: 2, Minor: 21, Patch: 1}

			err := validatePulumiVersion(minVersion, tt.currentVersion, tt.optOut)

			if tt.expectError {
				assert.Error(t, err)
				if minVersion.Major < tt.currentVersion.Major {
					assert.Regexp(t, `Major version mismatch.`, err.Error())
				} else {
					assert.Regexp(t, `Minimum version requirement failed.`, err.Error())
				}
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestProjectSettingsRespected(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	pName := "correct_project"
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	badProjectName := "project_was_overwritten"
	stack, err := NewStackInlineSource(ctx, stackName, badProjectName, func(ctx *pulumi.Context) error {
		return nil
	}, WorkDir(filepath.Join(".", "test", pName)))

	defer func() {
		// -- pulumi stack rm --
		err = stack.Workspace().RemoveStack(ctx, stack.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	assert.Nil(t, err)
	projectSettings, err := stack.workspace.ProjectSettings(ctx)
	assert.Nil(t, err)
	assert.Equal(t, projectSettings.Name, tokens.PackageName("correct_project"))
	assert.Equal(t, *projectSettings.Description, "This is a description")
}

func TestSaveStackSettings(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)

	opts := []LocalWorkspaceOption{
		SecretsProvider("passphrase"),
		EnvVars(map[string]string{
			"PULUMI_CONFIG_PASSPHRASE": "password",
		}),
	}

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, pName, func(ctx *pulumi.Context) error {
		c := config.New(ctx, "")
		ctx.Export("exp_static", pulumi.String("foo"))
		ctx.Export("exp_cfg", pulumi.String(c.Get("bar")))
		ctx.Export("exp_secret", c.GetSecret("buzz"))
		return nil
	}, opts...)
	require.NoError(t, err, "failed to initialize stack, err: %v", err)

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	// first load settings for created stack
	stackConfig, err := s.Workspace().StackSettings(ctx, stackName)
	require.NoError(t, err)
	stackConfig.SecretsProvider = "passphrase"
	assert.NoError(t, s.Workspace().SaveStackSettings(ctx, stackName, stackConfig))

	// -- pulumi up --

	res, err := s.Up(ctx)
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)

	reloaded, err := s.workspace.StackSettings(ctx, stackName)
	assert.NoError(t, err)
	assert.Equal(t, stackConfig, reloaded)

	// -- pulumi destroy --

	dRes, err := s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}

func TestConfigSecretWarnings(t *testing.T) {
	// TODO[pulumi/pulumi#7127]: Re-enabled the warning.
	t.Skip("Temporarily skipping test until we've re-enabled the warning - pulumi/pulumi#7127")
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	cfg := ConfigMap{
		"plainstr1":    ConfigValue{Value: "1"},
		"plainstr2":    ConfigValue{Value: "2"},
		"plainstr3":    ConfigValue{Value: "3"},
		"plainstr4":    ConfigValue{Value: "4"},
		"plainstr5":    ConfigValue{Value: "5"},
		"plainstr6":    ConfigValue{Value: "6"},
		"plainstr7":    ConfigValue{Value: "7"},
		"plainstr8":    ConfigValue{Value: "8"},
		"plainstr9":    ConfigValue{Value: "9"},
		"plainstr10":   ConfigValue{Value: "10"},
		"plainstr11":   ConfigValue{Value: "11"},
		"plainstr12":   ConfigValue{Value: "12"},
		"plainbool1":   ConfigValue{Value: "true"},
		"plainbool2":   ConfigValue{Value: "true"},
		"plainbool3":   ConfigValue{Value: "true"},
		"plainbool4":   ConfigValue{Value: "true"},
		"plainbool5":   ConfigValue{Value: "true"},
		"plainbool6":   ConfigValue{Value: "true"},
		"plainbool7":   ConfigValue{Value: "true"},
		"plainbool8":   ConfigValue{Value: "true"},
		"plainbool9":   ConfigValue{Value: "true"},
		"plainbool10":  ConfigValue{Value: "true"},
		"plainbool11":  ConfigValue{Value: "true"},
		"plainbool12":  ConfigValue{Value: "true"},
		"plainint1":    ConfigValue{Value: "1"},
		"plainint2":    ConfigValue{Value: "2"},
		"plainint3":    ConfigValue{Value: "3"},
		"plainint4":    ConfigValue{Value: "4"},
		"plainint5":    ConfigValue{Value: "5"},
		"plainint6":    ConfigValue{Value: "6"},
		"plainint7":    ConfigValue{Value: "7"},
		"plainint8":    ConfigValue{Value: "8"},
		"plainint9":    ConfigValue{Value: "9"},
		"plainint10":   ConfigValue{Value: "10"},
		"plainint11":   ConfigValue{Value: "11"},
		"plainint12":   ConfigValue{Value: "12"},
		"plainfloat1":  ConfigValue{Value: "1.1"},
		"plainfloat2":  ConfigValue{Value: "2.2"},
		"plainfloat3":  ConfigValue{Value: "3.3"},
		"plainfloat4":  ConfigValue{Value: "4.4"},
		"plainfloat5":  ConfigValue{Value: "5.5"},
		"plainfloat6":  ConfigValue{Value: "6.6"},
		"plainfloat7":  ConfigValue{Value: "7.7"},
		"plainfloat8":  ConfigValue{Value: "8.8"},
		"plainfloat9":  ConfigValue{Value: "9.9"},
		"plainfloat10": ConfigValue{Value: "10.1"},
		"plainfloat11": ConfigValue{Value: "11.11"},
		"plainfloat12": ConfigValue{Value: "12.12"},
		"plainobj1":    ConfigValue{Value: "{}"},
		"plainobj2":    ConfigValue{Value: "{}"},
		"plainobj3":    ConfigValue{Value: "{}"},
		"plainobj4":    ConfigValue{Value: "{}"},
		"plainobj5":    ConfigValue{Value: "{}"},
		"plainobj6":    ConfigValue{Value: "{}"},
		"plainobj7":    ConfigValue{Value: "{}"},
		"plainobj8":    ConfigValue{Value: "{}"},
		"plainobj9":    ConfigValue{Value: "{}"},
		"plainobj10":   ConfigValue{Value: "{}"},
		"plainobj11":   ConfigValue{Value: "{}"},
		"plainobj12":   ConfigValue{Value: "{}"},
		"str1":         ConfigValue{Value: "1", Secret: true},
		"str2":         ConfigValue{Value: "2", Secret: true},
		"str3":         ConfigValue{Value: "3", Secret: true},
		"str4":         ConfigValue{Value: "4", Secret: true},
		"str5":         ConfigValue{Value: "5", Secret: true},
		"str6":         ConfigValue{Value: "6", Secret: true},
		"str7":         ConfigValue{Value: "7", Secret: true},
		"str8":         ConfigValue{Value: "8", Secret: true},
		"str9":         ConfigValue{Value: "9", Secret: true},
		"str10":        ConfigValue{Value: "10", Secret: true},
		"str11":        ConfigValue{Value: "11", Secret: true},
		"str12":        ConfigValue{Value: "12", Secret: true},
		"bool1":        ConfigValue{Value: "true", Secret: true},
		"bool2":        ConfigValue{Value: "true", Secret: true},
		"bool3":        ConfigValue{Value: "true", Secret: true},
		"bool4":        ConfigValue{Value: "true", Secret: true},
		"bool5":        ConfigValue{Value: "true", Secret: true},
		"bool6":        ConfigValue{Value: "true", Secret: true},
		"bool7":        ConfigValue{Value: "true", Secret: true},
		"bool8":        ConfigValue{Value: "true", Secret: true},
		"bool9":        ConfigValue{Value: "true", Secret: true},
		"bool10":       ConfigValue{Value: "true", Secret: true},
		"bool11":       ConfigValue{Value: "true", Secret: true},
		"bool12":       ConfigValue{Value: "true", Secret: true},
		"int1":         ConfigValue{Value: "1", Secret: true},
		"int2":         ConfigValue{Value: "2", Secret: true},
		"int3":         ConfigValue{Value: "3", Secret: true},
		"int4":         ConfigValue{Value: "4", Secret: true},
		"int5":         ConfigValue{Value: "5", Secret: true},
		"int6":         ConfigValue{Value: "6", Secret: true},
		"int7":         ConfigValue{Value: "7", Secret: true},
		"int8":         ConfigValue{Value: "8", Secret: true},
		"int9":         ConfigValue{Value: "9", Secret: true},
		"int10":        ConfigValue{Value: "10", Secret: true},
		"int11":        ConfigValue{Value: "11", Secret: true},
		"int12":        ConfigValue{Value: "12", Secret: true},
		"float1":       ConfigValue{Value: "1.1", Secret: true},
		"float2":       ConfigValue{Value: "2.2", Secret: true},
		"float3":       ConfigValue{Value: "3.3", Secret: true},
		"float4":       ConfigValue{Value: "4.4", Secret: true},
		"float5":       ConfigValue{Value: "5.5", Secret: true},
		"float6":       ConfigValue{Value: "6.6", Secret: true},
		"float7":       ConfigValue{Value: "7.7", Secret: true},
		"float8":       ConfigValue{Value: "8.8", Secret: true},
		"float9":       ConfigValue{Value: "9.9", Secret: true},
		"float10":      ConfigValue{Value: "10.1", Secret: true},
		"float11":      ConfigValue{Value: "11.11", Secret: true},
		"float12":      ConfigValue{Value: "12.12", Secret: true},
		"obj1":         ConfigValue{Value: "{}", Secret: true},
		"obj2":         ConfigValue{Value: "{}", Secret: true},
		"obj3":         ConfigValue{Value: "{}", Secret: true},
		"obj4":         ConfigValue{Value: "{}", Secret: true},
		"obj5":         ConfigValue{Value: "{}", Secret: true},
		"obj6":         ConfigValue{Value: "{}", Secret: true},
		"obj7":         ConfigValue{Value: "{}", Secret: true},
		"obj8":         ConfigValue{Value: "{}", Secret: true},
		"obj9":         ConfigValue{Value: "{}", Secret: true},
		"obj10":        ConfigValue{Value: "{}", Secret: true},
		"obj11":        ConfigValue{Value: "{}", Secret: true},
		"obj12":        ConfigValue{Value: "{}", Secret: true},
	}

	// initialize
	//nolint:errcheck
	s, err := NewStackInlineSource(ctx, stackName, pName, func(ctx *pulumi.Context) error {
		c := config.New(ctx, "")

		config.Get(ctx, "plainstr1")
		config.Require(ctx, "plainstr2")
		config.Try(ctx, "plainstr3")
		config.GetSecret(ctx, "plainstr4")
		config.RequireSecret(ctx, "plainstr5")
		config.TrySecret(ctx, "plainstr6")
		c.Get("plainstr7")
		c.Require("plainstr8")
		c.Try("plainstr9")
		c.GetSecret("plainstr10")
		c.RequireSecret("plainstr11")
		c.TrySecret("plainstr12")

		config.GetBool(ctx, "plainbool1")
		config.RequireBool(ctx, "plainbool2")
		config.TryBool(ctx, "plainbool3")
		config.GetSecretBool(ctx, "plainbool4")
		config.RequireSecretBool(ctx, "plainbool5")
		config.TrySecretBool(ctx, "plainbool6")
		c.GetBool("plainbool7")
		c.RequireBool("plainbool8")
		c.TryBool("plainbool9")
		c.GetSecretBool("plainbool10")
		c.RequireSecretBool("plainbool11")
		c.TrySecretBool("plainbool12")

		config.GetInt(ctx, "plainint1")
		config.RequireInt(ctx, "plainint2")
		config.TryInt(ctx, "plainint3")
		config.GetSecretInt(ctx, "plainint4")
		config.RequireSecretInt(ctx, "plainint5")
		config.TrySecretInt(ctx, "plainint6")
		c.GetInt("plainint7")
		c.RequireInt("plainint8")
		c.TryInt("plainint9")
		c.GetSecretInt("plainint10")
		c.RequireSecretInt("plainint11")
		c.TrySecretInt("plainint12")

		config.GetFloat64(ctx, "plainfloat1")
		config.RequireFloat64(ctx, "plainfloat2")
		config.TryFloat64(ctx, "plainfloat3")
		config.GetSecretFloat64(ctx, "plainfloat4")
		config.RequireSecretFloat64(ctx, "plainfloat5")
		config.TrySecretFloat64(ctx, "plainfloat6")
		c.GetFloat64("plainfloat7")
		c.RequireFloat64("plainfloat8")
		c.TryFloat64("plainfloat9")
		c.GetSecretFloat64("plainfloat10")
		c.RequireSecretFloat64("plainfloat11")
		c.TrySecretFloat64("plainfloat12")

		var obj interface{}
		config.GetObject(ctx, "plainobjj1", &obj)
		config.RequireObject(ctx, "plainobj2", &obj)
		config.TryObject(ctx, "plainobj3", &obj)
		config.GetSecretObject(ctx, "plainobj4", &obj)
		config.RequireSecretObject(ctx, "plainobj5", &obj)
		config.TrySecretObject(ctx, "plainobj6", &obj)
		c.GetObject("plainobjj7", &obj)
		c.RequireObject("plainobj8", &obj)
		c.TryObject("plainobj9", &obj)
		c.GetSecretObject("plainobj10", &obj)
		c.RequireSecretObject("plainobj11", &obj)
		c.TrySecretObject("plainobj12", &obj)

		config.Get(ctx, "str1")
		config.Require(ctx, "str2")
		config.Try(ctx, "str3")
		config.GetSecret(ctx, "str4")
		config.RequireSecret(ctx, "str5")
		config.TrySecret(ctx, "str6")
		c.Get("str7")
		c.Require("str8")
		c.Try("str9")
		c.GetSecret("str10")
		c.RequireSecret("str11")
		c.TrySecret("str12")

		config.GetBool(ctx, "bool1")
		config.RequireBool(ctx, "bool2")
		config.TryBool(ctx, "bool3")
		config.GetSecretBool(ctx, "bool4")
		config.RequireSecretBool(ctx, "bool5")
		config.TrySecretBool(ctx, "bool6")
		c.GetBool("bool7")
		c.RequireBool("bool8")
		c.TryBool("bool9")
		c.GetSecretBool("bool10")
		c.RequireSecretBool("bool11")
		c.TrySecretBool("bool12")

		config.GetInt(ctx, "int1")
		config.RequireInt(ctx, "int2")
		config.TryInt(ctx, "int3")
		config.GetSecretInt(ctx, "int4")
		config.RequireSecretInt(ctx, "int5")
		config.TrySecretInt(ctx, "int6")
		c.GetInt("int7")
		c.RequireInt("int8")
		c.TryInt("int9")
		c.GetSecretInt("int10")
		c.RequireSecretInt("int11")
		c.TrySecretInt("int12")

		config.GetFloat64(ctx, "float1")
		config.RequireFloat64(ctx, "float2")
		config.TryFloat64(ctx, "float3")
		config.GetSecretFloat64(ctx, "float4")
		config.RequireSecretFloat64(ctx, "float5")
		config.TrySecretFloat64(ctx, "float6")
		c.GetFloat64("float7")
		c.RequireFloat64("float8")
		c.TryFloat64("float9")
		c.GetSecretFloat64("float10")
		c.RequireSecretFloat64("float11")
		c.TrySecretFloat64("float12")

		config.GetObject(ctx, "obj1", &obj)
		config.RequireObject(ctx, "obj2", &obj)
		config.TryObject(ctx, "obj3", &obj)
		config.GetSecretObject(ctx, "obj4", &obj)
		config.RequireSecretObject(ctx, "obj5", &obj)
		config.TrySecretObject(ctx, "obj6", &obj)
		c.GetObject("obj7", &obj)
		c.RequireObject("obj8", &obj)
		c.TryObject("obj9", &obj)
		c.GetSecretObject("obj10", &obj)
		c.RequireSecretObject("obj11", &obj)
		c.TrySecretObject("obj12", &obj)

		return nil
	})
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	validate := func(engineEvents []events.EngineEvent) {
		expectedWarnings := []string{
			"Configuration 'testproj:str1' value is a secret; use `GetSecret` instead of `Get`",
			"Configuration 'testproj:str2' value is a secret; use `RequireSecret` instead of `Require`",
			"Configuration 'testproj:str3' value is a secret; use `TrySecret` instead of `Try`",
			"Configuration 'testproj:str7' value is a secret; use `GetSecret` instead of `Get`",
			"Configuration 'testproj:str8' value is a secret; use `RequireSecret` instead of `Require`",
			"Configuration 'testproj:str9' value is a secret; use `TrySecret` instead of `Try`",

			"Configuration 'testproj:bool1' value is a secret; use `GetSecretBool` instead of `GetBool`",
			"Configuration 'testproj:bool2' value is a secret; use `RequireSecretBool` instead of `RequireBool`",
			"Configuration 'testproj:bool3' value is a secret; use `TrySecretBool` instead of `TryBool`",
			"Configuration 'testproj:bool7' value is a secret; use `GetSecretBool` instead of `GetBool`",
			"Configuration 'testproj:bool8' value is a secret; use `RequireSecretBool` instead of `RequireBool`",
			"Configuration 'testproj:bool9' value is a secret; use `TrySecretBool` instead of `TryBool`",

			"Configuration 'testproj:int1' value is a secret; use `GetSecretInt` instead of `GetInt`",
			"Configuration 'testproj:int2' value is a secret; use `RequireSecretInt` instead of `RequireInt`",
			"Configuration 'testproj:int3' value is a secret; use `TrySecretInt` instead of `TryInt`",
			"Configuration 'testproj:int7' value is a secret; use `GetSecretInt` instead of `GetInt`",
			"Configuration 'testproj:int8' value is a secret; use `RequireSecretInt` instead of `RequireInt`",
			"Configuration 'testproj:int9' value is a secret; use `TrySecretInt` instead of `TryInt`",

			"Configuration 'testproj:float1' value is a secret; use `GetSecretFloat64` instead of `GetFloat64`",
			"Configuration 'testproj:float2' value is a secret; use `RequireSecretFloat64` instead of `RequireFloat64`",
			"Configuration 'testproj:float3' value is a secret; use `TrySecretFloat64` instead of `TryFloat64`",
			"Configuration 'testproj:float7' value is a secret; use `GetSecretFloat64` instead of `GetFloat64`",
			"Configuration 'testproj:float8' value is a secret; use `RequireSecretFloat64` instead of `RequireFloat64`",
			"Configuration 'testproj:float9' value is a secret; use `TrySecretFloat64` instead of `TryFloat64`",

			"Configuration 'testproj:obj1' value is a secret; use `GetSecretObject` instead of `GetObject`",
			"Configuration 'testproj:obj2' value is a secret; use `RequireSecretObject` instead of `RequireObject`",
			"Configuration 'testproj:obj3' value is a secret; use `TrySecretObject` instead of `TryObject`",
			"Configuration 'testproj:obj7' value is a secret; use `GetSecretObject` instead of `GetObject`",
			"Configuration 'testproj:obj8' value is a secret; use `RequireSecretObject` instead of `RequireObject`",
			"Configuration 'testproj:obj9' value is a secret; use `TrySecretObject` instead of `TryObject`",
		}
		for _, warning := range expectedWarnings {
			var found bool
			for _, event := range engineEvents {
				if event.DiagnosticEvent != nil && event.DiagnosticEvent.Severity == "warning" &&
					strings.Contains(event.DiagnosticEvent.Message, warning) {
					found = true
					break
				}
			}
			assert.True(t, found, "expected warning %q", warning)
		}

		// These keys should not be in any warning messages.
		unexpectedWarnings := []string{
			"plainstr1",
			"plainstr2",
			"plainstr3",
			"plainstr4",
			"plainstr5",
			"plainstr6",
			"plainstr7",
			"plainstr8",
			"plainstr9",
			"plainstr10",
			"plainstr11",
			"plainstr12",
			"plainbool1",
			"plainbool2",
			"plainbool3",
			"plainbool4",
			"plainbool5",
			"plainbool6",
			"plainbool7",
			"plainbool8",
			"plainbool9",
			"plainbool10",
			"plainbool11",
			"plainbool12",
			"plainint1",
			"plainint2",
			"plainint3",
			"plainint4",
			"plainint5",
			"plainint6",
			"plainint7",
			"plainint8",
			"plainint9",
			"plainint10",
			"plainint11",
			"plainint12",
			"plainfloat1",
			"plainfloat2",
			"plainfloat3",
			"plainfloat4",
			"plainfloat5",
			"plainfloat6",
			"plainfloat7",
			"plainfloat8",
			"plainfloat9",
			"plainfloat10",
			"plainfloat11",
			"plainfloat12",
			"plainobj1",
			"plainobj2",
			"plainobj3",
			"plainobj4",
			"plainobj5",
			"plainobj6",
			"plainobj7",
			"plainobj8",
			"plainobj9",
			"plainobj10",
			"plainobj11",
			"plainobj12",
		}
		for _, warning := range unexpectedWarnings {
			for _, event := range engineEvents {
				if event.DiagnosticEvent != nil {
					assert.NotContains(t, event.DiagnosticEvent.Message, warning)
				}
			}
		}
	}

	// -- pulumi up --
	var upEvents []events.EngineEvent
	upCh := make(chan events.EngineEvent)
	go collectEvents(upCh, &upEvents)
	_, err = s.Up(ctx, optup.EventStreams(upCh))
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}
	validate(upEvents)

	// -- pulumi preview --
	var previewEvents []events.EngineEvent
	prevCh := make(chan events.EngineEvent)
	go collectEvents(prevCh, &previewEvents)
	_, err = s.Preview(ctx, optpreview.EventStreams(prevCh))
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	validate(previewEvents)
}

func BenchmarkBulkSetConfigMixed(b *testing.B) {
	ctx := context.Background()
	stackName := FullyQualifiedStackName(pulumiOrg, "set_config_mixed", "dev")

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, "set_config_mixed", func(ctx *pulumi.Context) error { return nil })
	if err != nil {
		b.Errorf("failed to initialize stack, err: %v", err)
		b.FailNow()
	}

	cfg := ConfigMap{
		"one":        ConfigValue{Value: "one", Secret: true},
		"two":        ConfigValue{Value: "two"},
		"three":      ConfigValue{Value: "three", Secret: true},
		"four":       ConfigValue{Value: "four"},
		"five":       ConfigValue{Value: "five", Secret: true},
		"six":        ConfigValue{Value: "six"},
		"seven":      ConfigValue{Value: "seven", Secret: true},
		"eight":      ConfigValue{Value: "eight"},
		"nine":       ConfigValue{Value: "nine", Secret: true},
		"ten":        ConfigValue{Value: "ten"},
		"eleven":     ConfigValue{Value: "one", Secret: true},
		"twelve":     ConfigValue{Value: "two"},
		"thirteen":   ConfigValue{Value: "three", Secret: true},
		"fourteen":   ConfigValue{Value: "four"},
		"fifteen":    ConfigValue{Value: "five", Secret: true},
		"sixteen":    ConfigValue{Value: "six"},
		"seventeen":  ConfigValue{Value: "seven", Secret: true},
		"eighteen":   ConfigValue{Value: "eight"},
		"nineteen":   ConfigValue{Value: "nine", Secret: true},
		"twenty":     ConfigValue{Value: "ten"},
		"one1":       ConfigValue{Value: "one", Secret: true},
		"two1":       ConfigValue{Value: "two"},
		"three1":     ConfigValue{Value: "three", Secret: true},
		"four1":      ConfigValue{Value: "four"},
		"five1":      ConfigValue{Value: "five", Secret: true},
		"six1":       ConfigValue{Value: "six"},
		"seven1":     ConfigValue{Value: "seven", Secret: true},
		"eight1":     ConfigValue{Value: "eight"},
		"nine1":      ConfigValue{Value: "nine", Secret: true},
		"ten1":       ConfigValue{Value: "ten"},
		"eleven1":    ConfigValue{Value: "one", Secret: true},
		"twelve1":    ConfigValue{Value: "two"},
		"thirteen1":  ConfigValue{Value: "three", Secret: true},
		"fourteen1":  ConfigValue{Value: "four"},
		"fifteen1":   ConfigValue{Value: "five", Secret: true},
		"sixteen1":   ConfigValue{Value: "six"},
		"seventeen1": ConfigValue{Value: "seven", Secret: true},
		"eighteen1":  ConfigValue{Value: "eight"},
		"nineteen1":  ConfigValue{Value: "nine", Secret: true},
		"twenty1":    ConfigValue{Value: "ten"},
	}

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		b.Errorf("failed to set config, err: %v", err)
		b.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(b, err, "failed to remove stack. Resources have leaked.")
	}()
}

func BenchmarkBulkSetConfigPlain(b *testing.B) {
	ctx := context.Background()
	stackName := FullyQualifiedStackName(pulumiOrg, "set_config_plain", "dev")

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, "set_config_plain", func(ctx *pulumi.Context) error { return nil })
	if err != nil {
		b.Errorf("failed to initialize stack, err: %v", err)
		b.FailNow()
	}

	cfg := ConfigMap{
		"one":        ConfigValue{Value: "one"},
		"two":        ConfigValue{Value: "two"},
		"three":      ConfigValue{Value: "three"},
		"four":       ConfigValue{Value: "four"},
		"five":       ConfigValue{Value: "five"},
		"six":        ConfigValue{Value: "six"},
		"seven":      ConfigValue{Value: "seven"},
		"eight":      ConfigValue{Value: "eight"},
		"nine":       ConfigValue{Value: "nine"},
		"ten":        ConfigValue{Value: "ten"},
		"eleven":     ConfigValue{Value: "one"},
		"twelve":     ConfigValue{Value: "two"},
		"thirteen":   ConfigValue{Value: "three"},
		"fourteen":   ConfigValue{Value: "four"},
		"fifteen":    ConfigValue{Value: "five"},
		"sixteen":    ConfigValue{Value: "six"},
		"seventeen":  ConfigValue{Value: "seven"},
		"eighteen":   ConfigValue{Value: "eight"},
		"nineteen":   ConfigValue{Value: "nine"},
		"twenty":     ConfigValue{Value: "ten"},
		"one1":       ConfigValue{Value: "one"},
		"two1":       ConfigValue{Value: "two"},
		"three1":     ConfigValue{Value: "three"},
		"four1":      ConfigValue{Value: "four"},
		"five1":      ConfigValue{Value: "five"},
		"six1":       ConfigValue{Value: "six"},
		"seven1":     ConfigValue{Value: "seven"},
		"eight1":     ConfigValue{Value: "eight"},
		"nine1":      ConfigValue{Value: "nine"},
		"ten1":       ConfigValue{Value: "ten"},
		"eleven1":    ConfigValue{Value: "one"},
		"twelve1":    ConfigValue{Value: "two"},
		"thirteen1":  ConfigValue{Value: "three"},
		"fourteen1":  ConfigValue{Value: "four"},
		"fifteen1":   ConfigValue{Value: "five"},
		"sixteen1":   ConfigValue{Value: "six"},
		"seventeen1": ConfigValue{Value: "seven"},
		"eighteen1":  ConfigValue{Value: "eight"},
		"nineteen1":  ConfigValue{Value: "nine"},
		"twenty1":    ConfigValue{Value: "ten"},
	}

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		b.Errorf("failed to set config, err: %v", err)
		b.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(b, err, "failed to remove stack. Resources have leaked.")
	}()
}

func BenchmarkBulkSetConfigSecret(b *testing.B) {
	ctx := context.Background()
	stackName := FullyQualifiedStackName(pulumiOrg, "set_config_plain", "dev")

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, "set_config_plain", func(ctx *pulumi.Context) error { return nil })
	if err != nil {
		b.Errorf("failed to initialize stack, err: %v", err)
		b.FailNow()
	}

	cfg := ConfigMap{
		"one":        ConfigValue{Value: "one", Secret: true},
		"two":        ConfigValue{Value: "two", Secret: true},
		"three":      ConfigValue{Value: "three", Secret: true},
		"four":       ConfigValue{Value: "four", Secret: true},
		"five":       ConfigValue{Value: "five", Secret: true},
		"six":        ConfigValue{Value: "six", Secret: true},
		"seven":      ConfigValue{Value: "seven", Secret: true},
		"eight":      ConfigValue{Value: "eight", Secret: true},
		"nine":       ConfigValue{Value: "nine", Secret: true},
		"ten":        ConfigValue{Value: "ten", Secret: true},
		"eleven":     ConfigValue{Value: "one", Secret: true},
		"twelve":     ConfigValue{Value: "two", Secret: true},
		"thirteen":   ConfigValue{Value: "three", Secret: true},
		"fourteen":   ConfigValue{Value: "four", Secret: true},
		"fifteen":    ConfigValue{Value: "five", Secret: true},
		"sixteen":    ConfigValue{Value: "six", Secret: true},
		"seventeen":  ConfigValue{Value: "seven", Secret: true},
		"eighteen":   ConfigValue{Value: "eight", Secret: true},
		"nineteen":   ConfigValue{Value: "nine", Secret: true},
		"1twenty":    ConfigValue{Value: "ten", Secret: true},
		"one1":       ConfigValue{Value: "one", Secret: true},
		"two1":       ConfigValue{Value: "two", Secret: true},
		"three1":     ConfigValue{Value: "three", Secret: true},
		"four1":      ConfigValue{Value: "four", Secret: true},
		"five1":      ConfigValue{Value: "five", Secret: true},
		"six1":       ConfigValue{Value: "six", Secret: true},
		"seven1":     ConfigValue{Value: "seven", Secret: true},
		"eight1":     ConfigValue{Value: "eight", Secret: true},
		"nine1":      ConfigValue{Value: "nine", Secret: true},
		"ten1":       ConfigValue{Value: "ten", Secret: true},
		"eleven1":    ConfigValue{Value: "one", Secret: true},
		"twelve1":    ConfigValue{Value: "two", Secret: true},
		"thirteen1":  ConfigValue{Value: "three", Secret: true},
		"fourteen1":  ConfigValue{Value: "four", Secret: true},
		"fifteen1":   ConfigValue{Value: "five", Secret: true},
		"sixteen1":   ConfigValue{Value: "six", Secret: true},
		"seventeen1": ConfigValue{Value: "seven", Secret: true},
		"eighteen1":  ConfigValue{Value: "eight", Secret: true},
		"nineteen1":  ConfigValue{Value: "nine", Secret: true},
		"twenty1":    ConfigValue{Value: "ten", Secret: true},
	}

	err = s.SetAllConfig(ctx, cfg)
	if err != nil {
		b.Errorf("failed to set config, err: %v", err)
		b.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(b, err, "failed to remove stack. Resources have leaked.")
	}()
}

func getTestOrg() string {
	testOrg := "pulumi-test"
	if _, set := os.LookupEnv("PULUMI_TEST_ORG"); set {
		testOrg = os.Getenv("PULUMI_TEST_ORG")
	}
	return testOrg
}

func countSteps(log []events.EngineEvent) int {
	steps := 0
	for _, e := range log {
		if e.ResourcePreEvent != nil {
			steps++
		}
	}
	return steps
}

func containsSummary(log []events.EngineEvent) bool {
	hasSummary := false
	for _, e := range log {
		if e.SummaryEvent != nil {
			hasSummary = true
		}
	}
	return hasSummary
}

func collectEvents(eventChannel <-chan events.EngineEvent, events *[]events.EngineEvent) {
	for {
		event, ok := <-eventChannel
		if !ok {
			return
		}
		*events = append(*events, event)
	}
}
