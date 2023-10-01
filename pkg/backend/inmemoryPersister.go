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

package backend

import (
	"fmt"

	"github.com/mitchellh/copystructure"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
)

var _ = SnapshotPersister((*InMemoryPersister)(nil))

type InMemoryPersister struct {
	Snap *deploy.Snapshot
}

func (p *InMemoryPersister) Save(snap *deploy.Snapshot) error {
	s, err := copystructure.Copy(*snap)
	if err != nil {
		return err
	}
	result, ok := s.(deploy.Snapshot)
	if !ok {
		return fmt.Errorf("could not cast snapshot copy to deploy.Snapshot")
	}

	p.Snap = &result
	return nil
}
