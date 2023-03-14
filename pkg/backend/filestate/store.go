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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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
	ListReferences() ([]*localBackendReference, error)

	// ParseReference parses a localBackendReference from a string.
	ParseReference(ref string) (*localBackendReference, error)
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
func (p *legacyReferenceStore) newReference(name tokens.Name) *localBackendReference {
	return &localBackendReference{
		name:  name,
		store: p,
	}
}

func (p *legacyReferenceStore) StackBasePath(ref *localBackendReference) string {
	return filepath.Join(StacksDir, fsutil.NamePath(ref.name))
}

func (p *legacyReferenceStore) HistoryDir(stack *localBackendReference) string {
	return filepath.Join(HistoriesDir, fsutil.NamePath(stack.name))
}

func (p *legacyReferenceStore) BackupDir(stack *localBackendReference) string {
	return filepath.Join(BackupsDir, fsutil.NamePath(stack.name))
}

func (p *legacyReferenceStore) ParseReference(stackRef string) (*localBackendReference, error) {
	if !tokens.IsName(stackRef) || len(stackRef) > 100 {
		return nil, fmt.Errorf(
			"stack names are limited to 100 characters and may only contain alphanumeric, hyphens, underscores, or periods: %q",
			stackRef)
	}
	return p.newReference(tokens.Name(stackRef)), nil
}

func (p *legacyReferenceStore) ListReferences() ([]*localBackendReference, error) {
	files, err := listBucket(p.bucket, StacksDir)
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
