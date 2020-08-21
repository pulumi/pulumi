package auto

import (
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
	"github.com/stretchr/testify/assert"
)

const pulumiOrg = "pulumi"

func TestNewStackLocalSource(t *testing.T) {
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
	s, err := NewStackLocalSource(fqsn, pDir)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	res, err := s.Up()
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
}

func rangeIn(low, hi int) int {
	rand.Seed(time.Now().UnixNano())
	return low + rand.Intn(hi-low)
}

func TestNewStackRemoteSource(t *testing.T) {
	// TODO this example has a policy violation if created in the pulumi org.
	// Should use a non-cloud remote example for this test.
	oName := "evanboyle"
	pName := "aws-go-s3-folder"
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	fqsn := FullyQualifiedStackName(oName, pName, sName)
	cfg := ConfigMap{
		"aws:region": ConfigValue{
			Value: "us-west-2",
		},
	}
	repo := GitRepo{
		URL:         "https://github.com/pulumi/examples.git",
		ProjectPath: pName,
	}

	// initialize
	s, err := NewStackRemoteSource(fqsn, repo)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	res, err := s.Up()
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 2, len(res.Outputs), "expected two outputs")
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
}

func TestNewStackRemoteSourceWithSetup(t *testing.T) {
	// TODO this example has a policy violation if created in the pulumi org.
	// Should use a non-cloud remote example for this test.
	oName := "evanboyle"
	pName := "aws-go-s3-folder"
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	fqsn := FullyQualifiedStackName(oName, pName, sName)
	cfg := ConfigMap{
		"aws:region": ConfigValue{
			Value: "us-west-2",
		},
	}
	binName := "examplesBinary"
	repo := GitRepo{
		URL:         "https://github.com/pulumi/examples.git",
		ProjectPath: pName,
		Setup: func(path string) error {
			cmd := exec.Command("go", "build", "-o", binName, "main.go")
			cmd.Dir = path
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
	s, err := NewStackRemoteSource(fqsn, repo, Project(project))
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	res, err := s.Up()
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 2, len(res.Outputs), "expected two outputs")
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
}

func TestNewStackInlineSource(t *testing.T) {
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
	s, err := NewStackInlineSource(fqsn, func(ctx *pulumi.Context) error {
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
		err = s.Workspace().RemoveStack(s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	err = s.SetAllConfig(cfg)
	if err != nil {
		t.Errorf("failed to set config, err: %v", err)
		t.FailNow()
	}

	// -- pulumi up --
	res, err := s.Up()
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
}

func TestNestedStackFails(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	parentFQSN := FullyQualifiedStackName(pulumiOrg, "parent", sName)
	nestedFQSN := FullyQualifiedStackName(pulumiOrg, "nested", sName)

	nestedStack, err := NewStackInlineSource(nestedFQSN, func(ctx *pulumi.Context) error {
		ctx.Export("exp_static", pulumi.String("foo"))
		return nil
	})
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	// initialize
	s, err := NewStackInlineSource(parentFQSN, func(ctx *pulumi.Context) error {
		_, err := nestedStack.Up()
		return err
	})
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")

		err = nestedStack.Workspace().RemoveStack(nestedStack.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nested stack operations are not supported")

	// -- pulumi destroy --

	dRes, err := s.Destroy()
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)

	dRes, err = nestedStack.Destroy()
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}
