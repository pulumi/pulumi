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

package main

import (
	"context"
	"errors"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type snapshotText []byte

// snapshotEncoder is an interface that enables roundtripping state to a text format for editing.
type snapshotEncoder interface {
	// Convert snapshot to bytes for use with a backing file.
	SnapshotToText(*deploy.Snapshot) (snapshotText, error)
	// Convert bytes to a snapshot.
	TextToSnapshot(snapshotText) (*deploy.Snapshot, error)
}

type jsonSnapshotEncoder struct {
	ctx context.Context
}

var _ snapshotEncoder = &jsonSnapshotEncoder{}

func (se *jsonSnapshotEncoder) SnapshotToText(snap *deploy.Snapshot) (snapshotText, error) {
	dep, err := stack.SerializeDeployment(snap, snap.SecretsManager, false)
	if err != nil {
		return nil, err
	}

	s, err := makeJSONString(dep, true /* multiline */)
	return snapshotText(s), err
}

func (se *jsonSnapshotEncoder) TextToSnapshot(s snapshotText) (*deploy.Snapshot, error) {
	dep, err := stack.DeserializeUntypedDeployment(se.ctx, &apitype.UntypedDeployment{
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
