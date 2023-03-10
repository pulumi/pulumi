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

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// referenceStore stores and provides access to stack information.
//
// Each implementation of referenceStore is a different version of the stack
// storage format.
type referenceStore interface {
	ListReferences() ([]*localBackendReference, error)

	// ParseReference parses a localBackendReference from a string.
	ParseReference(ref string) (*localBackendReference, error)

	// ConvertReference converts a StackReference to a localBackendReference,
	// ensuring that it's a valid localBackendReference.
	ConvertReference(ref backend.StackReference) (*localBackendReference, error)
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
	return &localBackendReference{name: name, project: project, b: p.b}
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
		if p.b.currentProject == nil {
			return nil, fmt.Errorf("if you're using the --stack flag, " +
				"pass the fully qualified name (organization/project/stack)")
		}

		project = p.b.currentProject.Name.String()
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

func (p *projectReferenceStore) ConvertReference(ref backend.StackReference) (*localBackendReference, error) {
	localStackRef, ok := ref.(*localBackendReference)
	if !ok {
		return nil, fmt.Errorf("bad stack reference type")
	}
	if localStackRef.project == "" {
		return nil, fmt.Errorf("bad stack reference, project was not set")
	}
	return localStackRef, nil
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

func (p *legacyReferenceStore) ParseReference(stackRef string) (*localBackendReference, error) {
	if !tokens.IsName(stackRef) || len(stackRef) > 100 {
		return nil, fmt.Errorf(
			"stack names are limited to 100 characters and may only contain alphanumeric, hyphens, underscores, or periods: %q",
			stackRef)
	}
	return &localBackendReference{name: tokens.Name(stackRef), b: p.b}, nil
}

func (p *legacyReferenceStore) ConvertReference(ref backend.StackReference) (*localBackendReference, error) {
	localStackRef, ok := ref.(*localBackendReference)
	if !ok {
		return nil, fmt.Errorf("bad stack reference type")
	}
	if localStackRef.project != "" {
		return nil, fmt.Errorf("bad stack reference, project was set")
	}
	return localStackRef, nil
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
		stacks = append(stacks, &localBackendReference{
			name: tokens.Name(name),
			b:    p.b,
		})
	}

	return stacks, nil
}
