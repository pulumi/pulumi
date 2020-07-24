package auto

import (
	"fmt"
	"math/rand"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi/config"
	"github.com/stretchr/testify/assert"
)

func TestUpBasic(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	ps := ProjectSpec{
		Name:       "testproj",
		SourcePath: filepath.Join(".", "test", "testproj"),
	}
	ss := StackSpec{
		Name:    sName,
		Project: ps,
		Overrides: &StackOverrides{
			Config:  map[string]string{"bar": "abc"},
			Secrets: map[string]string{"buzz": "secret"},
		},
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	res, err := s.Up()
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 2, len(res.Outputs), "expected two plain outputs")
	assert.Equal(t, 1, len(res.SecretOutputs), "expected one secret output")
	assert.Equal(t, "foo", res.Outputs["exp_static"])
	assert.Equal(t, "abc", res.Outputs["exp_cfg"])
	assert.Equal(t, "secret", res.SecretOutputs["exp_secret"])
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)

	// -- pulumi preview --

	prev, err := s.Preview()
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, 1, prev.ChangeSummary["same"])
	assert.Equal(t, 1, len(prev.Steps))

	// -- pulumi refresh --

	ref, err := s.Refresh()

	if err != nil {
		t.Errorf("refresh failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "refresh", ref.Summary.Kind)
	assert.Equal(t, "succeeded", ref.Summary.Result)

	// -- pulumi destroy --

	dRes, err := s.Destroy()
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)

	// -- pulumi stack rm --

	err = s.Remove()
	assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
}

func TestUpRemote(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	projPath := "aws-go-s3-folder"
	ps := ProjectSpec{
		Name: "aws-go-s3-folder",
		Remote: &RemoteArgs{
			RepoURL:     "https://github.com/pulumi/examples.git",
			ProjectPath: &projPath,
		},
	}
	ss := StackSpec{
		Name:    sName,
		Project: ps,
		Overrides: &StackOverrides{
			Config: map[string]string{"aws:region": "us-west-2"},
		},
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	res, err := s.Up()
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 2, len(res.Outputs), "expected two plain outputs")
	assert.NotNil(t, res.Outputs["bucketName"])
	assert.NotNil(t, res.Outputs["websiteUrl"])
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)

	// -- pulumi preview --

	prev, err := s.Preview()
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, 5, prev.ChangeSummary["same"])
	assert.Equal(t, 1, len(prev.Steps))

	// -- pulumi refresh --

	ref, err := s.Refresh()

	if err != nil {
		t.Errorf("refresh failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "refresh", ref.Summary.Kind)
	assert.Equal(t, "succeeded", ref.Summary.Result)

	// -- pulumi destroy --

	dRes, err := s.Destroy()
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)

	// -- pulumi stack rm --

	err = s.Remove()
	assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
}

func TestUpRemoteWithSetup(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))

	binName := "examplesBinary"
	setupFn := func(path string) error {
		cmd := exec.Command("go", "build", "-o", binName, "main.go")
		cmd.Dir = path
		return cmd.Run()
	}

	projPath := "aws-go-s3-folder"
	ps := ProjectSpec{
		Name: "aws-go-s3-folder",
		Remote: &RemoteArgs{
			RepoURL:     "https://github.com/pulumi/examples.git",
			ProjectPath: &projPath,
			Setup:       setupFn,
		},
		Overrides: &ProjectOverrides{
			Project: &workspace.Project{
				Runtime: workspace.NewProjectRuntimeInfo("go", map[string]interface{}{
					"binary": "examplesBinary",
				}),
			},
		},
	}
	ss := StackSpec{
		Name:    sName,
		Project: ps,
		Overrides: &StackOverrides{
			Config: map[string]string{"aws:region": "us-west-2"},
		},
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	res, err := s.Up()
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 2, len(res.Outputs), "expected two plain outputs")
	assert.NotNil(t, res.Outputs["bucketName"])
	assert.NotNil(t, res.Outputs["websiteUrl"])
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)

	// -- pulumi preview --

	prev, err := s.Preview()
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, 5, prev.ChangeSummary["same"])
	assert.Equal(t, 1, len(prev.Steps))

	// -- pulumi refresh --

	ref, err := s.Refresh()

	if err != nil {
		t.Errorf("refresh failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "refresh", ref.Summary.Kind)
	assert.Equal(t, "succeeded", ref.Summary.Result)

	// -- pulumi destroy --

	dRes, err := s.Destroy()
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)

	// -- pulumi stack rm --

	err = s.Remove()
	assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
}

func TestUpInlineSource(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	ps := ProjectSpec{
		Name: "inlineproj1",
		InlineSource: func(ctx *pulumi.Context) error {
			c := config.New(ctx, "")
			ctx.Export("exp_static", pulumi.String("foo"))
			ctx.Export("exp_cfg", pulumi.String(c.Get("bar")))
			ctx.Export("exp_secret", c.GetSecret("buzz"))
			return nil
		},
		Overrides: &ProjectOverrides{
			Project: &workspace.Project{
				Name:    "inlineproj1",
				Runtime: workspace.NewProjectRuntimeInfo("go", map[string]interface{}{} /*options*/),
			},
		},
	}
	ss := StackSpec{
		Name:    sName,
		Project: ps,
		Overrides: &StackOverrides{
			Config:  map[string]string{"bar": "abc"},
			Secrets: map[string]string{"buzz": "secret"},
		},
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	res, err := s.Up()
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 2, len(res.Outputs), "expected two plain outputs")
	assert.Equal(t, 1, len(res.SecretOutputs), "expected one secret output")
	assert.Equal(t, "foo", res.Outputs["exp_static"])
	assert.Equal(t, "abc", res.Outputs["exp_cfg"])
	assert.Equal(t, "secret", res.SecretOutputs["exp_secret"])
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)

	// -- pulumi preview --

	prev, err := s.Preview()
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, 1, prev.ChangeSummary["same"])
	assert.Equal(t, 1, len(prev.Steps))

	// // -- pulumi refresh --

	// ref, err := s.Refresh()

	// if err != nil {
	// 	t.Errorf("refresh failed, err: %v", err)
	// 	t.FailNow()
	// }
	// assert.Equal(t, "refresh", ref.Summary.Kind)
	// assert.Equal(t, "succeeded", ref.Summary.Result)

	// // -- pulumi destroy --

	// dRes, err := s.Destroy()
	// if err != nil {
	// 	t.Errorf("destroy failed, err: %v", err)
	// 	t.FailNow()
	// }

	// assert.Equal(t, "destroy", dRes.Summary.Kind)
	// assert.Equal(t, "succeeded", dRes.Summary.Result)

	// // -- pulumi stack rm --

	// err = s.Remove()
	// assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
}

func rangeIn(low, hi int) int {
	return low + rand.Intn(hi-low)
}
