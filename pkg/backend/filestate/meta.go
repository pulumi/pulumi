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
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"gocloud.dev/blob"
	"gocloud.dev/gcerrors"
	"gopkg.in/yaml.v3"
)

// pulumiMeta holds the contents of the .pulumi/Pulumi.yaml file
// in a filestate backend.
//
// This file holds metadata for the backend,
// including a version number that the backend can use
// to maintain compatibility with older versions of the CLI.
type pulumiMeta struct {
	// Version is the current version of the state store
	Version int `json:"version,omitempty" yaml:"version,omitempty"`
}

// ensurePulumiMeta loads the .pulumi/Pulumi.yaml file from the bucket,
// creating it if the bucket is new.
//
// If the bucket is not new, and the file does not exist,
// it returns a Version of 0 to indicate that the bucket is in legacy mode (no project).
func ensurePulumiMeta(ctx context.Context, b Bucket) (*pulumiMeta, error) {
	statePath := filepath.Join(workspace.BookkeepingDir, "Pulumi.yaml")
	stateBody, err := b.ReadAll(ctx, statePath)
	if err != nil {
		if gcerrors.Code(err) != gcerrors.NotFound {
			return nil, fmt.Errorf("could not read 'Pulumi.yaml': %w", err)
		}
	}

	if err == nil {
		// File exists. Load and validate it.
		var state pulumiMeta
		if err := yaml.Unmarshal(stateBody, &state); err != nil {
			return nil, fmt.Errorf("state store corrupted, could not unmarshal 'Pulumi.yaml': %w", err)
		}
		if state.Version < 1 {
			return nil, fmt.Errorf("state store corrupted, 'Pulumi.yaml' reports an invalid version of %d", state.Version)
		}
		if state.Version > 1 {
			return nil, fmt.Errorf(
				"state store unsupported, 'Pulumi.yaml' reports an version of %d unsupported by this version of pulumi",
				state.Version)
		}
		return &state, nil
	}

	// We'll only get here if err is NotFound, at this point we want to see if this is a fresh new store,
	// in which case we'll write the new Pulumi.yaml, or if there's existing data here we'll fallback to
	// non-project mode.
	bucketIter := b.List(&blob.ListOptions{
		Delimiter: "/",
		Prefix:    workspace.BookkeepingDir,
	})
	if _, err := bucketIter.Next(ctx); err == nil {
		// Already exists. We're in legacy mode.
		return &pulumiMeta{Version: 0}, nil
	} else if !errors.Is(err, io.EOF) {
		// io.EOF is expected, but any other error is not.
		return nil, fmt.Errorf("could not examine bucket: %w", err)
	}

	// Empty bucket. Turn on project mode.
	state := pulumiMeta{Version: 1}
	stateBody, err = yaml.Marshal(state)
	contract.AssertNoErrorf(err, "Could not marshal filestate.pulumiMeta to yaml")
	if err := b.WriteAll(ctx, statePath, stateBody, nil); err != nil {
		return nil, fmt.Errorf("could not write 'Pulumi.yaml': %w", err)
	}

	return &state, nil
}
