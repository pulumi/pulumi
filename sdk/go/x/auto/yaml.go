package auto

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

var exts = []string{".yaml", ".yml", ".json"}

func (s *StackSpec) writeProject() error {
	var proj *workspace.Project = &workspace.Project{}
	wp, err := parsePulumiProject(s.Project.SourcePath)
	if err == nil {
		proj = wp
	}

	if s.Project.Overrides != nil && s.Project.Overrides.Project != nil {
		proj = mergeProjects(proj, s.Project.Overrides.Project)
	}

	err = proj.Save(filepath.Join(s.Project.SourcePath, "Pulumi.yaml"))
	if err != nil {
		return errors.Wrap(err, "unable to write project file Pulumi.yaml.")
	}
	return nil
}

func parsePulumiProject(path string) (*workspace.Project, error) {

	var projPath string
	for _, e := range exts {
		f := fmt.Sprintf("%s%s", "Pulumi", e)
		fp := filepath.Join(path, f)
		_, err := os.Stat(fp)
		if err == nil {
			projPath = fp
		}
	}

	if projPath != "" {
		proj, err := workspace.LoadProject(projPath)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to unmarshal project at path: %s", projPath)
		}

		return proj, nil
	}

	return nil, errors.New("unable to find existing project file")
}

func mergeProjects(src *workspace.Project, overrides *workspace.Project) *workspace.Project {
	// TODO deepcopy
	res := *src
	if overrides.Name != "" {
		res.Name = overrides.Name
	}
	if overrides.Runtime.Name() != "" {
		res.Runtime = overrides.Runtime
	}
	if overrides.Main != "" {
		res.Main = overrides.Main
	}
	if overrides.Description != nil {
		res.Description = overrides.Description
	}
	if overrides.Author != nil {
		res.Author = overrides.Author
	}
	if overrides.Website != nil {
		res.Website = overrides.Website
	}
	if overrides.License != nil {
		res.License = overrides.License
	}
	if overrides.Config != "" {
		res.Config = overrides.Config
	}
	if overrides.Template != nil {
		res.Template = overrides.Template
	}
	if overrides.Backend != nil {
		res.Backend = overrides.Backend
	}

	return &res
}

func (s *StackSpec) writeStack() error {
	var stack *workspace.ProjectStack = &workspace.ProjectStack{}
	ws, err := parsePulumiStack(s.Project.SourcePath, s.Name)
	if err == nil {
		stack = ws
	}

	if s.Overrides != nil && s.Overrides.ProjectStack != nil {
		stack = mergeStacks(stack, s.Overrides.ProjectStack)
	}

	fName := fmt.Sprintf("Pulumi.%s.yaml", s.Name)
	err = stack.Save(filepath.Join(s.Project.SourcePath, fName))
	if err != nil {
		return errors.Wrap(err, "unable to write project file Pulumi.yaml.")
	}
	return nil
}

func parsePulumiStack(path string, stackName string) (*workspace.ProjectStack, error) {
	var projPath string
	for _, e := range exts {
		// Pulumi.<stack>.ext
		f := fmt.Sprintf("%s.%s%s", "Pulumi", stackName, e)
		fp := filepath.Join(path, f)
		_, err := os.Stat(fp)
		if err == nil {
			projPath = fp
		}
	}

	if projPath != "" {
		proj, err := workspace.LoadProjectStack(projPath)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to unmarshal project at path: %s", projPath)
		}

		return proj, nil
	}

	return nil, errors.New("unable to find existing project file")
}

func mergeStacks(src *workspace.ProjectStack, overrides *workspace.ProjectStack) *workspace.ProjectStack {
	// TODO deepcopy
	res := *src
	if overrides.SecretsProvider != "" {
		res.SecretsProvider = overrides.SecretsProvider
	}
	if overrides.EncryptedKey != "" {
		res.EncryptedKey = overrides.EncryptedKey
	}
	if overrides.EncryptionSalt != "" {
		res.EncryptionSalt = overrides.EncryptionSalt
	}
	// Config is intentionally skipped. Interface should be limited to reflect

	return &res
}
