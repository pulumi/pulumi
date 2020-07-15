package auto

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

func TestParseProject(t *testing.T) {
	overrideDesc := "abcdef"
	p := Project{
		Name:       "testproj",
		SourcePath: filepath.Join(".", "test", "testproj"),
		Overrides: &ProjectOverrides{
			Project: &workspace.Project{
				Description: &overrideDesc,
			},
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

	desc := "A minimal Go Pulumi program"
	expected := &workspace.Project{
		Name:        "testproj",
		Runtime:     workspace.NewProjectRuntimeInfo("go", nil),
		Description: &desc,
	}
	wp, err := parsePulumiProject(s.Project.SourcePath)
	assert.NoError(t, err)
	assert.Equal(t, expected, wp)

	// merge
	merged := mergeProjects(wp, p.Overrides.Project)
	expMergeRes := &workspace.Project{
		Name:        "testproj",
		Runtime:     workspace.NewProjectRuntimeInfo("go", nil),
		Description: &overrideDesc,
	}
	assert.Equal(t, expMergeRes, merged)
}

func TestWriteProject(t *testing.T) {
	overrideDesc := "abcdef"
	p := Project{
		Name:       "testproj",
		SourcePath: filepath.Join(".", "test", "testproj"),
		Overrides: &ProjectOverrides{
			Project: &workspace.Project{
				Description: &overrideDesc,
			},
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

	err := s.writeProject()
	assert.Nil(t, err)
}

// TODO tests useful for development but not reproducible

// func TestParseStack(t *testing.T) {
// 	p := Project{
// 		Name:       "testproj",
// 		SourcePath: filepath.Join(".", "test", "testproj"),
// 	}
// 	s := &Stack{
// 		Name:    "int_test",
// 		Project: p,
// 		Overrides: &StackOverrides{
// 			Config:  map[string]string{"bar": "abc"},
// 			Secrets: map[string]string{"buzz": "secret"},
// 			ProjectStack: &workspace.ProjectStack{
// 				EncryptedKey: "abc",
// 			},
// 		},
// 	}

// 	expected := &workspace.ProjectStack{}
// 	ws, err := parsePulumiStack(s.Project.SourcePath, s.Name)
// 	assert.NoError(t, err)
// 	assert.Equal(t, expected, ws)

// 	// merge
// 	merged := mergeStacks(ws, s.Overrides.ProjectStack)
// 	expMergeRes := &workspace.ProjectStack{}
// 	assert.Equal(t, expMergeRes, merged)
// }

// func TestWriteStack(t *testing.T) {
// 	overrideDesc := "abcdef"
// 	p := Project{
// 		Name:       "testproj",
// 		SourcePath: filepath.Join(".", "test", "testproj"),
// 		Overrides: &ProjectOverrides{
// 			Project: &workspace.Project{
// 				Description: &overrideDesc,
// 			},
// 		},
// 	}
// 	s := &Stack{
// 		Name:    "int_test",
// 		Project: p,
// 		Overrides: &StackOverrides{
// 			Config:  map[string]string{"bar": "abc"},
// 			Secrets: map[string]string{"buzz": "secret"},
// 			ProjectStack: &workspace.ProjectStack{
// 				EncryptedKey: "abc",
// 			},
// 		},
// 	}

// 	err := s.writeStack()
// 	assert.Nil(t, err)
// }
