package auto

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPreviewBasic(t *testing.T) {
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
	res, err := s.Preview()
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 1, res.ChangeSummary["same"])
	assert.Equal(t, 1, len(res.Steps))
}
