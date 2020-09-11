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
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi/config"
	"github.com/pulumi/pulumi/sdk/v2/go/x/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v2/go/x/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v2/go/x/auto/optup"
	"github.com/stretchr/testify/assert"
)

const pulumiOrg = "pulumi"

func TestNewStackLocalSource(t *testing.T) {
	ctx := context.Background()
	pName := "testproj"
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	fqsn := FullyQualifiedStackName(pulumiOrg, pName, sName)
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
	s, err := NewStackLocalSource(ctx, fqsn, pDir)
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

	prev, err := s.Preview(ctx)
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, 1, prev.ChangeSummary["same"])
	assert.Equal(t, 1, len(prev.Steps))

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
	return low + rand.Intn(hi-low)
}

func TestNewStackRemoteSource(t *testing.T) {
	ctx := context.Background()
	pName := "go_remote_proj"
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	fqsn := FullyQualifiedStackName(pulumiOrg, pName, sName)
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
	s, err := NewStackRemoteSource(ctx, fqsn, repo)
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

	prev, err := s.Preview(ctx)
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, 1, prev.ChangeSummary["same"])
	assert.Equal(t, 1, len(prev.Steps))

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
	fqsn := FullyQualifiedStackName(pulumiOrg, pName, sName)
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
	s, err := NewStackRemoteSource(ctx, fqsn, repo, Project(project))
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

	prev, err := s.Preview(ctx)
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, 1, prev.ChangeSummary["same"])
	assert.Equal(t, 1, len(prev.Steps))

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
	pName := "testproj"
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	fqsn := FullyQualifiedStackName(pulumiOrg, pName, sName)
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
	s, err := NewStackInlineSource(ctx, fqsn, func(ctx *pulumi.Context) error {
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

	prev, err := s.Preview(ctx)
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, 1, prev.ChangeSummary["same"])
	assert.Equal(t, 1, len(prev.Steps))

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
	testCtx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	parentFQSN := FullyQualifiedStackName(pulumiOrg, "parent", sName)
	nestedFQSN := FullyQualifiedStackName(pulumiOrg, "nested", sName)

	nestedStack, err := NewStackInlineSource(testCtx, nestedFQSN, func(ctx *pulumi.Context) error {
		ctx.Export("exp_static", pulumi.String("foo"))
		return nil
	})
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	// initialize
	s, err := NewStackInlineSource(testCtx, parentFQSN, func(ctx *pulumi.Context) error {
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

	_, err = s.Up(testCtx)
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
	fqsn := FullyQualifiedStackName(pulumiOrg, pName, sName)
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
	s, err := NewStackInlineSource(ctx, fqsn, func(ctx *pulumi.Context) error {
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
