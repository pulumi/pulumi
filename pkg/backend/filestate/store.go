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
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"gocloud.dev/blob"
)

// These should be constants
// but we can't make a constant from filepath.Join.
var (
	// StacksDir is a path under the state's root directory
	// where the filestate backend stores stack information.
	StacksDir = filepath.Join(workspace.BookkeepingDir, workspace.StackDir)

	// HistoriesDir is a path under the state's root directory
	// where the filestate backend stores histories for all stacks.
	HistoriesDir = filepath.Join(workspace.BookkeepingDir, workspace.HistoryDir)

	// BackupsDir is a path under the state's root directory
	// where the filestate backend stores backups of stacks.
	BackupsDir = filepath.Join(workspace.BookkeepingDir, workspace.BackupDir)
)

// referenceStore stores and provides access to stack information.
//
// Each implementation of referenceStore is a different version of the stack
// storage format.
type referenceStore interface {
	// StackBasePath returns the base path to for the file
	// where snapshots of this stack are stored.
	//
	// This must be under StacksDir.
	//
	// This is the path to the file without the extension.
	// The real file path is StackBasePath + ".json"
	// or StackBasePath + ".json.gz".
	StackBasePath(*localBackendReference) string

	// HistoryDir returns the path to the directory
	// where history for this stack is stored.
	//
	// This must be under HistoriesDir.
	HistoryDir(*localBackendReference) string

	// BackupDir returns the path to the directory
	// where backups for this stack are stored.
	//
	// This must be under BackupsDir.
	BackupDir(*localBackendReference) string

	// ListReferences lists all stack references in the store.
	ListReferences(context.Context) ([]*localBackendReference, error)

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
	bucket Bucket

	// currentProject is a thread-safe way to get the current project.
	currentProject func() *workspace.Project
}

var _ referenceStore = (*projectReferenceStore)(nil)

func newProjectReferenceStore(bucket Bucket, currentProject func() *workspace.Project) *projectReferenceStore {
	return &projectReferenceStore{
		bucket:         bucket,
		currentProject: currentProject,
	}
}

// newReference builds a new localBackendReference with the provided arguments.
// This DOES NOT modify the underlying storage.
func (p *projectReferenceStore) newReference(project tokens.Name, name tokens.StackName) *localBackendReference {
	return &localBackendReference{
		name:           name,
		project:        project,
		store:          p,
		currentProject: p.currentProject,
	}
}

func (p *projectReferenceStore) StackBasePath(ref *localBackendReference) string {
	contract.Requiref(ref.project != "", "ref.project", "must not be empty")
	// No need for NamePath for the StackName because it's already constrained to characters that are valid for filenames.
	return filepath.Join(StacksDir, fsutil.NamePath(ref.project), ref.name.String())
}

func (p *projectReferenceStore) HistoryDir(stack *localBackendReference) string {
	contract.Requiref(stack.project != "", "ref.project", "must not be empty")
	return filepath.Join(HistoriesDir, fsutil.NamePath(stack.project), stack.name.String())
}

func (p *projectReferenceStore) BackupDir(stack *localBackendReference) string {
	contract.Requiref(stack.project != "", "ref.project", "must not be empty")
	return filepath.Join(BackupsDir, fsutil.NamePath(stack.project), stack.name.String())
}

func (p *projectReferenceStore) ParseReference(stackRef string) (*localBackendReference, error) {
	// We accept the following forms:
	//
	// 1. <stack-name>
	// 2. <org-name>/<stack-name>
	// 3. <org-name>/<project-name>/<stack-name>
	//
	// org-name must always be "organization".
	// This matches the behavior of the Pulumi Service storage backend.
	if stackRef == "" {
		return nil, errors.New("stack name must not be empty")
	}

	var name, project, org string
	split := strings.Split(stackRef, "/") // guaranteed to have at least one element
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
	}

	// If the provided stack name didn't include the org or project,
	// infer them from the local environment.
	if org == "" {
		// Filestate organization MUST always be "organization"
		org = "organization"
	}

	if org != "organization" {
		return nil, errors.New("organization name must be 'organization'")
	}

	if project == "" {
		currentProject := p.currentProject()
		if currentProject == nil {
			return nil, fmt.Errorf("if you're using the --stack flag, " +
				"pass the fully qualified name (organization/project/stack)")
		}

		project = currentProject.Name.String()
	}

	if project != "" {
		if err := tokens.ValidateProjectName(project); err != nil {
			return nil, err
		}
	}

	parsedName, err := tokens.ParseStackName(name)
	if err != nil {
		return nil, err
	}

	return p.newReference(tokens.Name(project), parsedName), nil
}

func (p *projectReferenceStore) ValidateReference(ref *localBackendReference) error {
	if ref.project == "" {
		return fmt.Errorf("bad stack reference, project was not set")
	}
	return nil
}

func (p *projectReferenceStore) ListProjects(ctx context.Context) ([]tokens.Name, error) {
	path := StacksDir

	files, err := listBucket(ctx, p.bucket, path)
	if err != nil {
		return nil, fmt.Errorf("error listing stacks: %w", err)
	}

	projects := slice.Prealloc[tokens.Name](len(files))
	for _, file := range files {
		if !file.IsDir {
			continue // ignore files
		}

		projName := objectName(file)
		if !tokens.IsName(projName) {
			// If this isn't a valid Name
			// it won't be a project directory,
			// so skip it.
			continue
		}

		projects = append(projects, tokens.Name(projName))
	}

	return projects, nil
}

func (p *projectReferenceStore) ProjectExists(ctx context.Context, projectName string) (bool, error) {
	contract.Requiref(projectName != "", "projectName", "must not be empty")

	path := path.Join(StacksDir, projectName)

	files, err := listBucket(ctx, p.bucket, path)
	if err != nil {
		return false, fmt.Errorf("list stacks at %q: %w", path, err)
	}

	// If files is empty, it means that project is not found in bucket
	return len(files) > 0, nil
}

func (p *projectReferenceStore) ListReferences(ctx context.Context) ([]*localBackendReference, error) {
	// The first level of the bucket is the project name.
	// The second level of the bucket is the stack name.
	prefix := filepath.ToSlash(StacksDir) + "/"
	iter := p.bucket.List(&blob.ListOptions{
		Prefix: prefix,
		// Don't set the Delimiter.
		// This will treat the entire bucket as a flat list,
		// returning only files under the prefix.
	})

	var stacks []*localBackendReference
	for {
		file, err := iter.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("list bucket: %w", err)
		}

		if file.IsDir {
			continue
		}

		// Key is in the form,
		//   $StacksDir/$projName/$stackName.json[.gz]
		// We want to extract projName and stackName from it.

		parts := strings.Split(strings.TrimPrefix(file.Key, prefix), "/")
		if len(parts) != 2 {
			continue // skip paths too shallow or too deep
		}
		projName := parts[0]
		objName := parts[1]

		if !tokens.IsName(projName) {
			// If this isn't a valid Name
			// it won't be a project directory,
			// so skip it.
			continue
		}

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
		parsedName, err := tokens.ParseStackName(name)
		if err != nil {
			// This looked like a stack file, but it wasn't a valid stack name so skip it.
			continue
		}

		stacks = append(stacks, p.newReference(tokens.Name(projName), parsedName))
	}
	return stacks, nil
}

// legacyReferenceStore is a referenceStore that stores stack
// information with the legacy layout that did not support projects.
//
// This is the format we used before we introduced versioning.
type legacyReferenceStore struct {
	bucket Bucket
}

var _ referenceStore = (*legacyReferenceStore)(nil)

// newLegacyReferenceStore builds a referenceStore in the legacy format
// (no project support) backed by the provided bucket.
func newLegacyReferenceStore(b Bucket) *legacyReferenceStore {
	return &legacyReferenceStore{
		bucket: b,
	}
}

// newReference builds a new localBackendReference with the provided arguments.
// This DOES NOT modify the underlying storage.
func (p *legacyReferenceStore) newReference(name tokens.StackName) *localBackendReference {
	return &localBackendReference{
		name:  name,
		store: p,
	}
}

func (p *legacyReferenceStore) StackBasePath(ref *localBackendReference) string {
	contract.Requiref(ref.project == "", "ref.project", "must be empty")
	return filepath.Join(StacksDir, ref.name.String())
}

func (p *legacyReferenceStore) HistoryDir(stack *localBackendReference) string {
	contract.Requiref(stack.project == "", "ref.project", "must be empty")
	return filepath.Join(HistoriesDir, stack.name.String())
}

func (p *legacyReferenceStore) BackupDir(stack *localBackendReference) string {
	contract.Requiref(stack.project == "", "ref.project", "must be empty")
	return filepath.Join(BackupsDir, stack.name.String())
}

func (p *legacyReferenceStore) ParseReference(stackRef string) (*localBackendReference, error) {
	parsedName, err := tokens.ParseStackName(stackRef)
	if err != nil {
		return nil, err
	}
	return p.newReference(parsedName), nil
}

func (p *legacyReferenceStore) ValidateReference(ref *localBackendReference) error {
	if ref.project != "" {
		return fmt.Errorf("bad stack reference, project was set")
	}
	return nil
}

func (p *legacyReferenceStore) ListReferences(ctx context.Context) ([]*localBackendReference, error) {
	files, err := listBucket(ctx, p.bucket, StacksDir)
	if err != nil {
		return nil, fmt.Errorf("error listing stacks: %w", err)
	}
	stacks := slice.Prealloc[*localBackendReference](len(files))

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
		parsedName, err := tokens.ParseStackName(name)
		if err != nil {
			// This looked like a stack file, but it wasn't a valid stack name so skip it.
			continue
		}

		stacks = append(stacks, p.newReference(parsedName))
	}

	return stacks, nil
}
