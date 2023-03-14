// Copyright 2016-2023, Pulumi Corporation.
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

package filestate

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// referenceStore stores and provides access to stack information.
//
// Each implementation of referenceStore is a different version of the stack
// storage format.
type referenceStore interface {
	// StackDir returns the path to the directory
	// where stack snapshots are stored.
	StackDir() string

	// StackBasePath returns the base path to for the file
	// where snapshots of this stack are stored.
	//
	// This must be under StackDir().
	//
	// This is the path to the file without the extension.
	// The real file path is StackBasePath + ".json"
	// or StackBasePath + ".json.gz".
	StackBasePath(*localBackendReference) string

	// HistoryDir returns the path to the directory
	// where history for this stack is stored.
	HistoryDir(*localBackendReference) string

	// BackupDir returns the path to the directory
	// where backups for this stack are stored.
	BackupDir(*localBackendReference) string

	// ListReferences lists all stack references in the store.
	ListReferences() ([]*localBackendReference, error)

	// ParseReference parses a localBackendReference from a string.
	ParseReference(ref string) (*localBackendReference, error)

	// ValidateReference verifies that the provided reference is valid
	// returning an error if it is not.
	ValidateReference(*localBackendReference) error
}

// projectReferenceStore is a referenceStore that stores stack
// information with the new project-based layout.
//
// This is version 1 of the stack storage format.
type projectReferenceStore struct {
	b *localBackend
}

var _ referenceStore = (*projectReferenceStore)(nil)

// newReference builds a new localBackendReference with the provided arguments.
// This DOES NOT modify the underlying storage.
func (p *projectReferenceStore) newReference(project, name tokens.Name) *localBackendReference {
	return &localBackendReference{
		name:           name,
		project:        project,
		store:          p,
		currentProject: p.b.currentProjectName(),
	}
}

func (p *projectReferenceStore) StackDir() string {
	return filepath.Join(workspace.BookkeepingDir, workspace.StackDir)
}

func (p *projectReferenceStore) StackBasePath(ref *localBackendReference) string {
	contract.Requiref(ref.project != "", "ref.project", "must not be empty")
	return filepath.Join(p.StackDir(), fsutil.NamePath(ref.project), fsutil.NamePath(ref.name))
}

func (p *projectReferenceStore) HistoryDir(stack *localBackendReference) string {
	contract.Requiref(stack.project != "", "ref.project", "must not be empty")
	return filepath.Join(workspace.BookkeepingDir, workspace.HistoryDir, fsutil.NamePath(stack.project), fsutil.NamePath(stack.name))
}

func (p *projectReferenceStore) BackupDir(stack *localBackendReference) string {
	contract.Requiref(stack.project != "", "ref.project", "must not be empty")
	return filepath.Join(workspace.BookkeepingDir, workspace.BackupDir, fsutil.NamePath(stack.project), fsutil.NamePath(stack.name))
}

func (p *projectReferenceStore) ParseReference(stackRef string) (*localBackendReference, error) {
	var name, project, org string
	split := strings.Split(stackRef, "/")
	switch len(split) {
	case 1:
		name = split[0]
	case 2:
		org = split[0]
		name = split[1]
	case 3:
		org = split[0]
		project = split[1]
		name = split[2]
	default:
		return nil, fmt.Errorf("could not parse stack reference '%s'", stackRef)
	}

	// If the provided stack name didn't include the org or project, infer them from the local
	// environment.
	if org == "" {
		// Filestate organization MUST always be "organization"
		org = "organization"
	}

	if org != "organization" {
		return nil, errors.New("organization name must be 'organization'")
	}

	if project == "" {
		project = p.b.currentProjectName()
		if project == "" {
			return nil, fmt.Errorf("if you're using the --stack flag, " +
				"pass the fully qualified name (organization/project/stack)")
		}
	}

	if len(project) > 100 {
		return nil, errors.New("project names must be less than 100 characters")
	}

	if project != "" && !tokens.IsName(project) {
		return nil, fmt.Errorf(
			"project names may only contain alphanumerics, hyphens, underscores, and periods: %s",
			project)
	}

	if !tokens.IsName(name) || len(name) > 100 {
		return nil, fmt.Errorf(
			"stack names are limited to 100 characters and may only contain alphanumeric, hyphens, underscores, or periods: %s",
			name)
	}

	return p.newReference(tokens.Name(project), tokens.Name(name)), nil
}

func (p *projectReferenceStore) ValidateReference(ref *localBackendReference) error {
	if ref.project == "" {
		return fmt.Errorf("bad stack reference, project was not set")
	}
	return nil
}

func (p *projectReferenceStore) ListReferences() ([]*localBackendReference, error) {
	// The first level of the bucket is the project name.
	// The second level of the bucket is the stack name.
	path := p.b.stackPath(nil)

	files, err := listBucket(p.b.bucket, path)
	if err != nil {
		return nil, fmt.Errorf("error listing stacks: %w", err)
	}

	var stacks []*localBackendReference
	for _, file := range files {
		if file.IsDir {
			projName := objectName(file)
			// If this isn't a valid Name it won't be a project directory, so skip it
			if !tokens.IsName(projName) {
				continue
			}

			// TODO: Could we improve the efficiency here by firstly making listBucket return an enumerator not
			// eagerly collecting all keys into a slice, and secondly by getting listBucket to return all
			// descendent items not just the immediate children. We could then do the necessary splitting by
			// file paths here to work out project names.
			projectFiles, err := listBucket(p.b.bucket, filepath.Join(path, projName))
			if err != nil {
				return nil, fmt.Errorf("error listing stacks: %w", err)
			}

			for _, projectFile := range projectFiles {
				// Can ignore directories at this level
				if projectFile.IsDir {
					continue
				}

				objName := objectName(projectFile)
				// Skip files without valid extensions (e.g., *.bak files).
				ext := filepath.Ext(objName)
				// But accept gzip compression
				if ext == encoding.GZIPExt {
					objName = strings.TrimSuffix(objName, encoding.GZIPExt)
					ext = filepath.Ext(objName)
				}

				if _, has := encoding.Marshalers[ext]; !has {
					continue
				}

				// Read in this stack's information.
				name := objName[:len(objName)-len(ext)]
				stacks = append(stacks, p.newReference(tokens.Name(projName), tokens.Name(name)))
			}
		}
	}

	return stacks, nil
}

// legacyReferenceStore is a referenceStore that stores stack
// information with the legacy layout that did not support projects.
//
// This is the format we used before we introduced versioning.
type legacyReferenceStore struct {
	b *localBackend
}

var _ referenceStore = (*legacyReferenceStore)(nil)

// newReference builds a new localBackendReference with the provided arguments.
// This DOES NOT modify the underlying storage.
func (p *legacyReferenceStore) newReference(name tokens.Name) *localBackendReference {
	return &localBackendReference{
		name:  name,
		store: p,
		// currentProject is not relevant for legacy stacks
	}
}

func (p *legacyReferenceStore) StackDir() string {
	return filepath.Join(workspace.BookkeepingDir, workspace.StackDir)
}

func (p *legacyReferenceStore) StackBasePath(ref *localBackendReference) string {
	contract.Requiref(ref.project == "", "ref.project", "must be empty")
	return filepath.Join(p.StackDir(), fsutil.NamePath(ref.name))
}

func (p *legacyReferenceStore) HistoryDir(stack *localBackendReference) string {
	contract.Requiref(stack.project == "", "ref.project", "must be empty")
	return filepath.Join(workspace.BookkeepingDir, workspace.HistoryDir, fsutil.NamePath(stack.name))
}

func (p *legacyReferenceStore) BackupDir(stack *localBackendReference) string {
	contract.Requiref(stack.project == "", "ref.project", "must be empty")
	return filepath.Join(workspace.BookkeepingDir, workspace.BackupDir, fsutil.NamePath(stack.name))
}

func (p *legacyReferenceStore) ParseReference(stackRef string) (*localBackendReference, error) {
	if !tokens.IsName(stackRef) || len(stackRef) > 100 {
		return nil, fmt.Errorf(
			"stack names are limited to 100 characters and may only contain alphanumeric, hyphens, underscores, or periods: %q",
			stackRef)
	}
	return p.newReference(tokens.Name(stackRef)), nil
}

func (p *legacyReferenceStore) ValidateReference(ref *localBackendReference) error {
	if ref.project != "" {
		return fmt.Errorf("bad stack reference, project was set")
	}
	return nil
}

func (p *legacyReferenceStore) ListReferences() ([]*localBackendReference, error) {
	// Read the stack directory.
	path := p.b.stackPath(nil)

	files, err := listBucket(p.b.bucket, path)
	if err != nil {
		return nil, fmt.Errorf("error listing stacks: %w", err)
	}
	stacks := make([]*localBackendReference, 0, len(files))

	for _, file := range files {
		if file.IsDir {
			continue
		}

		objName := objectName(file)
		// Skip files without valid extensions (e.g., *.bak files).
		ext := filepath.Ext(objName)
		// But accept gzip compression
		if ext == encoding.GZIPExt {
			objName = strings.TrimSuffix(objName, encoding.GZIPExt)
			ext = filepath.Ext(objName)
		}

		if _, has := encoding.Marshalers[ext]; !has {
			continue
		}

		// Read in this stack's information.
		name := objName[:len(objName)-len(ext)]
		stacks = append(stacks, p.newReference(tokens.Name(name)))
	}

	return stacks, nil
}
