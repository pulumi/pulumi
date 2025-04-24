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

package diy

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"gocloud.dev/gcerrors"
	"gopkg.in/yaml.v3"
)

// Path inside the bucket where we store the metadata file.
var pulumiMetaPath = filepath.Join(workspace.BookkeepingDir, "meta.yaml")

// pulumiMeta holds the contents of the .pulumi/meta.yaml file
// in a diy backend.
//
// This file specifies metadata for the backend,
// including a version number that the backend can use
// to maintain compatibility with older versions of the CLI.
//
// The metadata file is not written for legacy layouts.
// However, there was a short period of time where it was written,
// so we should still allow for Version 0 when reading these files.
type pulumiMeta struct {
	// Version is the current version of the state store.
	//
	// Version 0 is the starting version.
	// It does not support project-scoped stacks.
	// Version 1 adds support for project-scoped stacks.
	//
	// Does not use "omitempty" to differentiate
	// between a missing field and a zero value.
	Version int `json:"version" yaml:"version"`
}

// ensurePulumiMeta loads the Pulumi state metadata file from the bucket.
//
// Unlike [readPulumiMeta],
// the result of this function will always be non-nil if the error is nil.
//
// If the bucket is empty, this will create a new metadata file
// with the latest version number.
// This can be overridden by setting the environment variable
// "PULUMI_diy_BACKEND_LEGACY_LAYOUT" to "1".
// ensurePulumiMeta uses the provided 'getenv' function
// to read the environment variable.
func ensurePulumiMeta(ctx context.Context, b Bucket, e env.Env) (*pulumiMeta, error) {
	meta, err := readPulumiMeta(ctx, b)
	if err != nil {
		return nil, err
	}

	if meta != nil {
		return meta, nil
	}

	// If there's no metadata file, we need to create one.
	// The version we pick for the new file decides how we lay out the state.
	//
	// - Version 0 is legacy mode, which is the old layout.
	//   To avoid breaking old stacks, we want to use version 0
	//   if the bucket is not empty.
	//
	// - Version 1 added support for project-scoped stacks.
	//   For entirely new buckets, we'll use version 1
	//   to give new users access to the latest features.
	refs, err := newLegacyReferenceStore(b).ListReferences(ctx)
	if err != nil {
		// If there's an error listing don't fail, just don't print the warnings
		return nil, err
	}

	useLegacy := len(refs) > 0
	if !useLegacy {
		// Allow opting into legacy mode for new states
		// by setting the environment variable.
		useLegacy = e.GetBool(env.DIYBackendLegacyLayout)
	}

	if useLegacy {
		meta = &pulumiMeta{Version: 0}
	} else {
		meta = &pulumiMeta{Version: 1}
	}

	// Implementation detail:
	// For version 0, WriteTo won't write the metadata file.
	// See [pulumiMeta.WriteTo] for details on why.
	if err := meta.WriteTo(ctx, b); err != nil {
		return nil, err
	}

	return meta, nil
}

// readPulumiMeta loads the Pulumi state metadata from the bucket.
// If the file does not exist, it returns nil and no error.
func readPulumiMeta(ctx context.Context, b Bucket) (*pulumiMeta, error) {
	metaBody, err := b.ReadAll(ctx, pulumiMetaPath)
	if err != nil {
		if gcerrors.Code(err) == gcerrors.NotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("read %q: %w", pulumiMetaPath, err)
	}

	// State is a copy of the pulumiMeta shape,
	// but with pointers to fields where we need to differentiate
	// between a missing field and a zero value.
	// Don't use pointers for fields where the zero value is invalid.
	//
	// This is necessary because the YAML unmarshaler
	// will read a zero value for a missing field or an empty file.
	var state struct {
		// Version 0 is valid, so we need to use a pointer.
		Version *int `yaml:"version"`
	}

	if err := yaml.Unmarshal(metaBody, &state); err != nil {
		return nil, fmt.Errorf("corrupt store: unmarshal %q: %w", pulumiMetaPath, err)
	}

	if state.Version == nil {
		return nil, fmt.Errorf("corrupt store: missing version in %q", pulumiMetaPath)
	}

	return &pulumiMeta{
		Version: *state.Version,
	}, nil
}

// WriteTo writes the metadata to the bucket, overwriting any existing metadata.
func (m *pulumiMeta) WriteTo(ctx context.Context, b Bucket) error {
	if m.Version == 0 {
		// We don't want to write a metadata file
		// for legacy layouts.
		//
		// This allows for cases where a user has
		// strict permission controls on their bucket,
		// and doesn't expect a file outside .pulumi/stacks/.
		return nil
	}

	bs, err := yaml.Marshal(m)
	contract.AssertNoErrorf(err, "Could not marshal diy.pulumiMeta to YAML")

	if err := b.WriteAll(ctx, pulumiMetaPath, bs, nil); err != nil {
		return fmt.Errorf("write %q: %w", pulumiMetaPath, err)
	}
	return nil
}
