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

package engine

import (
	"errors"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
)

var _ = SnapshotManager((*CombinedManager)(nil))

// CombinedManager combines multiple SnapshotManagers into one, it simply forwards on each call to every manager.
type CombinedManager struct {
	Managers          []SnapshotManager
	CollectErrorsOnly []bool
	errors            []error
	errorMutex        sync.Mutex
}

func (c *CombinedManager) appendError(err error) {
	c.errorMutex.Lock()
	defer c.errorMutex.Unlock()

	c.errors = append(c.errors, err)
}

func (c *CombinedManager) Write(base *deploy.Snapshot) error {
	var errs []error
	for i, m := range c.Managers {
		if err := m.Write(base); err != nil {
			if len(c.CollectErrorsOnly) > i && c.CollectErrorsOnly[i] {
				c.appendError(err)
			} else {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (c *CombinedManager) RebuiltBaseState() error {
	var errs []error
	for i, m := range c.Managers {
		if err := m.RebuiltBaseState(); err != nil {
			if len(c.CollectErrorsOnly) > i && c.CollectErrorsOnly[i] {
				c.appendError(err)
			} else {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (c *CombinedManager) BeginMutation(step deploy.Step) (SnapshotMutation, error) {
	var errs []error
	mutations := &CombinedMutation{
		manager: c,
	}
	for i, m := range c.Managers {
		mutation, err := m.BeginMutation(step)
		if err != nil {
			if len(c.CollectErrorsOnly) > i && c.CollectErrorsOnly[i] {
				c.appendError(err)
			} else {
				errs = append(errs, err)
			}
		} else {
			mutations.Mutations = append(mutations.Mutations, mutation)
			if len(c.CollectErrorsOnly) > i {
				mutations.CollectErrorsOnly = append(mutations.CollectErrorsOnly, c.CollectErrorsOnly[i])
			}
		}
	}

	return mutations, errors.Join(errs...)
}

func (c *CombinedManager) RegisterResourceOutputs(step deploy.Step) error {
	errs := make([]error, 0, len(c.Managers))
	for i, m := range c.Managers {
		err := m.RegisterResourceOutputs(step)
		if err != nil {
			if len(c.CollectErrorsOnly) > i && c.CollectErrorsOnly[i] {
				c.appendError(err)
			} else {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (c *CombinedManager) Close() error {
	errs := make([]error, 0, len(c.Managers))
	for i, m := range c.Managers {
		err := m.Close()
		if err != nil {
			if len(c.CollectErrorsOnly) > i && c.CollectErrorsOnly[i] {
				c.appendError(err)
			} else {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (c *CombinedManager) Errors() []error {
	c.errorMutex.Lock()
	defer c.errorMutex.Unlock()
	return c.errors
}

type CombinedMutation struct {
	Mutations         []SnapshotMutation
	CollectErrorsOnly []bool
	manager           *CombinedManager
}

func (c *CombinedMutation) End(step deploy.Step, success bool) error {
	errs := make([]error, 0, len(c.Mutations))
	for i, m := range c.Mutations {
		err := m.End(step, success)
		if len(c.CollectErrorsOnly) > i && c.CollectErrorsOnly[i] {
			c.manager.appendError(err)
		} else {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
