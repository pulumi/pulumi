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

package lifecycletest

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type multiManager []engine.SnapshotManager

type managerError struct {
	err error
}

func newManagerError(err error) error {
	if err == nil {
		return nil
	}
	return managerError{err: err}
}

func (e managerError) Error() string {
	return e.err.Error()
}

func (m multiManager) Close() error {
	var err error
	for _, m := range m {
		if e := m.Close(); e != nil {
			err = errors.Join(err, e)
		}
	}
	return newManagerError(err)
}

func (m multiManager) Rebase(base *deploy.Snapshot) error {
	var err error
	for _, m := range m {
		if e := m.Rebase(base); e != nil {
			err = errors.Join(err, e)
		}
	}
	return newManagerError(err)
}

func (m multiManager) BeginMutation(step deploy.Step) (engine.SnapshotMutation, error) {
	mutations := make(multiMutation, len(m))
	var err error
	for i, m := range m {
		mut, e := m.BeginMutation(step)
		if e != nil {
			err = errors.Join(err, e)
		} else {
			mutations[i] = mut
		}
	}
	return mutations, err
}

func (m multiManager) RegisterResourceOutputs(step deploy.Step) error {
	var err error
	for _, m := range m {
		if e := m.RegisterResourceOutputs(step); e != nil {
			err = errors.Join(err, e)
		}
	}
	return newManagerError(err)
}

type multiMutation []engine.SnapshotMutation

func (m multiMutation) End(step deploy.Step, successful bool) error {
	var err error
	for _, m := range m {
		if e := m.End(step, successful); e != nil {
			err = errors.Join(err, e)
		}
	}
	return newManagerError(err)
}

type apiJournal struct {
	m       sync.Mutex
	base    *apitype.DeploymentV3
	entries []apitype.JournalEntry
}

type deploymentError struct {
	actual *apitype.DeploymentV3
	err    error
}

func (e *deploymentError) Error() string {
	return fmt.Sprintf("deserializing result: %v", e.err)
}

func newAPIJournal(base *deploy.Snapshot, sm secrets.Manager) (*apiJournal, error) {
	baseDeployment, err := stack.SerializeDeployment(base, sm, false)
	if err != nil {
		return nil, fmt.Errorf("serializing base: %w", err)
	}
	return &apiJournal{base: baseDeployment}, nil
}

func (j *apiJournal) Replay() (*apitype.DeploymentV3, error) {
	replayer := backend.NewJournalReplayer()

	sort.Slice(j.entries, func(i, k int) bool { return j.entries[i].SequenceNumber < j.entries[k].SequenceNumber })
	for _, e := range j.entries {
		if err := replayer.Replay(e); err != nil {
			return nil, fmt.Errorf("replaying entry %v: %w", e.SequenceNumber, err)
		}
	}

	new, err := replayer.Finish(j.base)
	if err != nil {
		err = fmt.Errorf("finishing replay: %w", err)
		if new == nil {
			return nil, err
		}
		return nil, &deploymentError{actual: new, err: err}
	}

	return new, nil
}

func (j *apiJournal) Rebase(base *apitype.DeploymentV3) error {
	j.base = base
	return nil
}

func (j *apiJournal) Append(entry apitype.JournalEntry) error {
	j.m.Lock()
	defer j.m.Unlock()

	j.entries = append(j.entries, entry)
	return nil
}
