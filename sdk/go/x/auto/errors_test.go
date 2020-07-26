package auto

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// TODO setup/teardown for npm, python, etc

func TestConcurrentUpdateError(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	ps := ProjectSpec{
		Name:       "conflict_error",
		SourcePath: filepath.Join(".", "test", "errors", "conflict_error"),
	}
	ss := StackSpec{
		Name:    sName,
		Project: ps,
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	c := make(chan error)

	// parallel updates to cause conflict
	for i := 0; i < 5; i++ {
		go func() {
			_, err := s.Up()
			c <- err
		}()
	}

	conflicts := 0

	for i := 0; i < 5; i++ {
		err := <-c
		if IsConcurrentUpdateError(err) {
			conflicts++
		}
	}

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

	// should have at least one conflict
	assert.Greater(t, conflicts, 0)
}

func TestInlineConcurrentUpdateError(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	ps := ProjectSpec{
		Name: "inline_conflict_error",
		InlineSource: func(ctx *pulumi.Context) error {
			ctx.Export("exp_static", pulumi.String("foo"))
			return nil
		},
		Overrides: &ProjectOverrides{
			Project: &workspace.Project{
				Name:    "inline_conflict_error",
				Runtime: workspace.NewProjectRuntimeInfo("go", map[string]interface{}{} /*options*/),
			},
		},
	}
	ss := StackSpec{
		Name:    sName,
		Project: ps,
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	c := make(chan error)

	// parallel updates to cause conflict
	for i := 0; i < 5; i++ {
		go func() {
			_, err := s.Up()
			c <- err
		}()
	}

	conflicts := 0

	for i := 0; i < 5; i++ {
		err := <-c
		if IsConcurrentUpdateError(err) {
			conflicts++
		}
	}

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

	// should have at least one conflict
	assert.Greater(t, conflicts, 0)
}

func TestCompileErrorGo(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	ps := ProjectSpec{
		Name:       "compilation_error",
		SourcePath: filepath.Join(".", "test", "errors", "compilation_error", "go"),
	}
	ss := StackSpec{
		Name:    sName,
		Project: ps,
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	_, err = s.Up()

	assert.NotNil(t, err)
	assert.True(t, IsCompilationError(err))

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

func TestCompileErrorDotnet(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	ps := ProjectSpec{
		Name:       "compilation_error",
		SourcePath: filepath.Join(".", "test", "errors", "compilation_error", "dotnet"),
	}

	ss := StackSpec{
		Name:    sName,
		Project: ps,
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	_, err = s.Up()

	assert.NotNil(t, err)
	assert.True(t, IsCompilationError(err))

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

func TestCompileErrorTypescript(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	ps := ProjectSpec{
		Name:       "compilation_error",
		SourcePath: filepath.Join(".", "test", "errors", "compilation_error", "typescript"),
	}

	ss := StackSpec{
		Name:    sName,
		Project: ps,
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	_, err = s.Up()

	assert.NotNil(t, err)
	assert.True(t, IsCompilationError(err))

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

func TestRuntimeErrorGo(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	ps := ProjectSpec{
		Name:       "runtime_error",
		SourcePath: filepath.Join(".", "test", "errors", "runtime_error", "go"),
	}

	ss := StackSpec{
		Name:    sName,
		Project: ps,
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	_, err = s.Up()

	assert.NotNil(t, err)
	assert.True(t, IsRuntimeError(err))
	assert.False(t, IsCompilationError(err))

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

func TestRuntimeErrorPython(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	ps := ProjectSpec{
		Name:       "runtime_error",
		SourcePath: filepath.Join(".", "test", "errors", "runtime_error", "python"),
	}

	ss := StackSpec{
		Name:    sName,
		Project: ps,
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	_, err = s.Up()

	assert.NotNil(t, err)
	assert.True(t, IsRuntimeError(err))
	assert.False(t, IsCompilationError(err))

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

func TestRuntimeErrorJavascript(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	ps := ProjectSpec{
		Name:       "runtime_error",
		SourcePath: filepath.Join(".", "test", "errors", "runtime_error", "javascript"),
	}

	ss := StackSpec{
		Name:    sName,
		Project: ps,
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	_, err = s.Up()

	assert.NotNil(t, err)
	assert.True(t, IsRuntimeError(err))
	assert.False(t, IsCompilationError(err))

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

func TestRuntimeErrorTypescript(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	ps := ProjectSpec{
		Name:       "runtime_error",
		SourcePath: filepath.Join(".", "test", "errors", "runtime_error", "typescript"),
	}

	ss := StackSpec{
		Name:    sName,
		Project: ps,
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	_, err = s.Up()

	assert.NotNil(t, err)
	assert.True(t, IsRuntimeError(err))
	assert.False(t, IsCompilationError(err))

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

func TestRuntimeErrorDotnet(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	ps := ProjectSpec{
		Name:       "runtime_error",
		SourcePath: filepath.Join(".", "test", "errors", "runtime_error", "dotnet"),
	}

	ss := StackSpec{
		Name:    sName,
		Project: ps,
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	_, err = s.Up()

	assert.NotNil(t, err)
	assert.True(t, IsRuntimeError(err))
	assert.False(t, IsCompilationError(err))

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
