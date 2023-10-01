// Copyright 2016-2022, Pulumi Corporation.
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

package engine

import (
	"errors"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
)

var _ = SnapshotManager((*CombinedManager)(nil))

// CombinedManager combines multiple SnapshotManagers into one, it simply forwards on each call to every manager.
type CombinedManager struct {
	Managers []SnapshotManager
}

func (c *CombinedManager) BeginMutation(step deploy.Step) (SnapshotMutation, error) {
	var errs []error
	mutations := &CombinedMutation{}
	for _, m := range c.Managers {
		mutation, err := m.BeginMutation(step)
		if err != nil {
			errs = append(errs, err)
		} else {
			mutations.Mutations = append(mutations.Mutations, mutation)
		}
	}

	return mutations, errors.Join(errs...)
}

func (c *CombinedManager) RegisterResourceOutputs(step deploy.Step) error {
	errs := make([]error, 0, len(c.Managers))
	for _, m := range c.Managers {
		errs = append(errs, m.RegisterResourceOutputs(step))
	}
	return errors.Join(errs...)
}

func (c *CombinedManager) Close() error {
	errs := make([]error, 0, len(c.Managers))
	for _, m := range c.Managers {
		errs = append(errs, m.Close())
	}
	return errors.Join(errs...)
}

type CombinedMutation struct {
	Mutations []SnapshotMutation
}

func (c *CombinedMutation) End(step deploy.Step, success bool) error {
	errs := make([]error, 0, len(c.Mutations))
	for _, m := range c.Mutations {
		errs = append(errs, m.End(step, success))
	}
	return errors.Join(errs...)
}
