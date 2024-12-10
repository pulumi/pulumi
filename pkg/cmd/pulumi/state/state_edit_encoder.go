// Copyright 2016-2024, Pulumi Corporation.
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

package state

import (
	"context"
	"errors"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type snapshotText []byte

// snapshotEncoder is an interface that enables roundtripping state to a text format for editing.
type snapshotEncoder interface {
	// Convert snapshot to bytes for use with a backing file.
	SnapshotToText(*deploy.Snapshot) (snapshotText, error)
	// Convert bytes to a snapshot.
	TextToSnapshot(context.Context, snapshotText) (*deploy.Snapshot, error)
}

type jsonSnapshotEncoder struct{}

var _ snapshotEncoder = &jsonSnapshotEncoder{}

func (se *jsonSnapshotEncoder) SnapshotToText(snap *deploy.Snapshot) (snapshotText, error) {
	ctx := context.TODO()
	dep, err := stack.SerializeDeployment(ctx, snap, false)
	if err != nil {
		return nil, err
	}

	s, err := ui.MakeJSONString(dep, true /* multiline */)
	return snapshotText(s), err
}

func (se *jsonSnapshotEncoder) TextToSnapshot(ctx context.Context, s snapshotText) (*deploy.Snapshot, error) {
	contract.Requiref(ctx != nil, "ctx", "must not be nil")

	dep, err := stack.DeserializeUntypedDeployment(ctx, &apitype.UntypedDeployment{
		Version:    3,
		Deployment: []byte(s),
	}, stack.DefaultSecretsProvider)
	if err != nil {
		return nil, err
	}

	if dep == nil {
		return nil, errors.New("could not deserialize deployment")
	}
	return dep, nil
}
