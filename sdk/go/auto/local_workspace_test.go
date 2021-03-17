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
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v2/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v2/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v2/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v2/go/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v2/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi/config"
)

var pulumiOrg = getTestOrg()

const pName = "testproj"

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

	const permalinkSearchStr = "https://app.pulumi.com"
	var startRegex = regexp.MustCompile(permalinkSearchStr)
	permalink, err := GetPermalink(res.StdOut)
	assert.Nil(t, err, "failed to get permalink.")
	assert.True(t, startRegex.MatchString(permalink))

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
	assert.Greater(t, res.Summary.Version, 0)

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
}{
	{
		"higher_major",
		semver.Version{Major: 100, Minor: 0, Patch: 0},
		true,
	},
	{
		"lower_major",
		semver.Version{Major: 1, Minor: 0, Patch: 0},
		true,
	},
	{
		"higher_minor",
		semver.Version{Major: 2, Minor: 22, Patch: 0},
		false,
	},
	{
		"lower_minor",
		semver.Version{Major: 2, Minor: 1, Patch: 0},
		true,
	},
	{
		"equal_minor_higher_patch",
		semver.Version{Major: 2, Minor: 21, Patch: 2},
		false,
	},
	{
		"equal_minor_equal_patch",
		semver.Version{Major: 2, Minor: 21, Patch: 1},
		false,
	},
	{
		"equal_minor_lower_patch",
		semver.Version{Major: 2, Minor: 21, Patch: 0},
		true,
	},
	{
		"equal_minor_equal_patch_prerelease",
		// Note that prerelease < release so this case will error
		semver.Version{Major: 2, Minor: 21, Patch: 1,
			Pre: []semver.PRVersion{{VersionStr: "alpha"}, {VersionNum: 1234, IsNum: true}}},
		true,
	},
}

func TestMinimumVersion(t *testing.T) {
	for _, tt := range minVersionTests {
		t.Run(tt.name, func(t *testing.T) {
			minVersion := semver.Version{Major: 2, Minor: 21, Patch: 1}
			err := validatePulumiVersion(minVersion, tt.currentVersion)
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
