package auto

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUp(t *testing.T) {
	rt := Go
	p := Project{
		Name:       "testproj",
		SourcePath: filepath.Join(".", "test", "testproj"),
		Overrides: &ProjectOverrides{
			Runtime: &rt,
		},
	}
	s := &Stack{
		Name:    "int_test",
		Project: p,
		Overrides: &StackOverrides{
			Config:  map[string]string{"bar": "abc"},
			Secrets: map[string]string{"buzz": "secret"},
		},
	}
	res, err := s.Up()
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 2, len(res.Outputs), "expected two plain outputs")
	assert.Equal(t, 1, len(res.SecretOutputs), "expected one secret output")
}
