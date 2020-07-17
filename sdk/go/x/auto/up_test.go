package auto

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpBasic(t *testing.T) {
	p := Project{
		Name:       "testproj",
		SourcePath: filepath.Join(".", "test", "testproj"),
	}
	s := &Stack{
		Name:    "int_test",
		Project: p,
		Overrides: &StackOverrides{
			Config:  map[string]string{"bar": "abc"},
			Secrets: map[string]string{"buzz": "secret"},
		},
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
