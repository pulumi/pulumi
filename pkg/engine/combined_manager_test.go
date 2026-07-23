// Copyright 2025, Pulumi Corporation.
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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

var (
	_ SnapshotManager               = (*MockSnapshotManager)(nil)
	_ StateMigrationSnapshotManager = (*MockStateMigrationSnapshotManager)(nil)
)

type MockSnapshotManager struct {
	WriteF                   func(base *deploy.Snapshot) error
	RebuiltBaseStateF        func() error
	SetSnippetsF             func(snippets []resource.Snippet) error
	BeginMutationF           func(step deploy.Step) (SnapshotMutation, error)
	RegisterResourceOutputsF func(step deploy.Step) error
	CloseF                   func() error
}

type MockStateMigrationSnapshotManager struct {
	*MockSnapshotManager
	SupportsStateMigrationsF func() bool
	StateMigrationF          func(plan *deploy.StateMigrationPlan) error
}

type MockSanpshotMutation struct {
	EndF func(step deploy.Step, success bool) error
}

func (m *MockSnapshotManager) Write(base *deploy.Snapshot) error {
	if m.WriteF != nil {
		return m.WriteF(base)
	}
	return nil
}

func (m *MockSnapshotManager) RebuiltBaseState() error {
	if m.RebuiltBaseStateF != nil {
		return m.RebuiltBaseStateF()
	}
	return nil
}

func (m *MockStateMigrationSnapshotManager) SupportsStateMigrations() bool {
	if m.SupportsStateMigrationsF != nil {
		return m.SupportsStateMigrationsF()
	}
	return true
}

func (m *MockStateMigrationSnapshotManager) StateMigration(plan *deploy.StateMigrationPlan) error {
	if m.StateMigrationF != nil {
		return m.StateMigrationF(plan)
	}
	return nil
}

func (m *MockSnapshotManager) SetSnippets(snippets []resource.Snippet) error {
	if m.SetSnippetsF != nil {
		return m.SetSnippetsF(snippets)
	}
	return nil
}

func (m *MockSnapshotManager) BeginMutation(step deploy.Step) (SnapshotMutation, error) {
	if m.BeginMutationF != nil {
		return m.BeginMutationF(step)
	}
	return &MockSanpshotMutation{}, nil
}

func (m *MockSnapshotManager) RegisterResourceOutputs(step deploy.Step) error {
	if m.RegisterResourceOutputsF != nil {
		return m.RegisterResourceOutputsF(step)
	}
	return nil
}

func (m *MockSnapshotManager) Close() error {
	if m.CloseF != nil {
		return m.CloseF()
	}
	return nil
}

func (m *MockSanpshotMutation) End(step deploy.Step, success bool) error {
	if m.EndF != nil {
		return m.EndF(step, success)
	}
	return nil
}

func TestCombinedManagerStateMigration(t *testing.T) {
	t.Parallel()

	plan := &deploy.StateMigrationPlan{}
	calls := 0
	manager := &MockStateMigrationSnapshotManager{
		MockSnapshotManager: &MockSnapshotManager{},
		StateMigrationF: func(got *deploy.StateMigrationPlan) error {
			require.Same(t, plan, got)
			calls++
			return nil
		},
	}
	combined := &CombinedManager{Managers: []SnapshotManager{manager, manager}}

	require.True(t, combined.SupportsStateMigrations())
	require.NoError(t, combined.StateMigration(plan))
	require.Equal(t, 2, calls)
}

func TestCombinedManagerStateMigrationUnsupportedRequiredManager(t *testing.T) {
	t.Parallel()

	combined := &CombinedManager{Managers: []SnapshotManager{&MockSnapshotManager{}}}

	require.False(t, combined.SupportsStateMigrations())
	require.ErrorIs(t, combined.StateMigration(&deploy.StateMigrationPlan{}), deploy.ErrStateMigrationsUnsupported)
}

func TestCombinedManagerStateMigrationSkipsUnsupportedBestEffortManager(t *testing.T) {
	t.Parallel()

	called := false
	supporting := &MockStateMigrationSnapshotManager{
		MockSnapshotManager: &MockSnapshotManager{},
		StateMigrationF: func(*deploy.StateMigrationPlan) error {
			called = true
			return nil
		},
	}
	combined := &CombinedManager{
		Managers:          []SnapshotManager{supporting, &MockSnapshotManager{}},
		CollectErrorsOnly: []bool{false, true},
	}

	require.True(t, combined.SupportsStateMigrations())
	require.NoError(t, combined.StateMigration(&deploy.StateMigrationPlan{}))
	require.True(t, called)
	require.Empty(t, combined.Errors())
}

func TestCombinedManagerStateMigrationCollectsBestEffortError(t *testing.T) {
	t.Parallel()

	expected := errors.New("shadow migration failed")
	required := &MockStateMigrationSnapshotManager{MockSnapshotManager: &MockSnapshotManager{}}
	shadow := &MockStateMigrationSnapshotManager{
		MockSnapshotManager: &MockSnapshotManager{},
		StateMigrationF: func(*deploy.StateMigrationPlan) error {
			return expected
		},
	}
	combined := &CombinedManager{
		Managers:          []SnapshotManager{required, shadow},
		CollectErrorsOnly: []bool{false, true},
	}

	require.True(t, combined.SupportsStateMigrations())
	require.NoError(t, combined.StateMigration(&deploy.StateMigrationPlan{}))
	require.Len(t, combined.Errors(), 1)
	require.ErrorIs(t, combined.Errors()[0], expected)
}

func TestIgnoreSomeErrors(t *testing.T) {
	t.Parallel()

	writeCalled := 0
	rebuiltCalled := 0
	beginCalled := 0
	registerCalled := 0
	closeCalled := 0
	endCalled := 0
	erroringManager := &MockSnapshotManager{
		WriteF: func(base *deploy.Snapshot) error {
			writeCalled++
			return errors.New("write error")
		},
		RebuiltBaseStateF: func() error {
			rebuiltCalled++
			return errors.New("rebuilt error")
		},
		BeginMutationF: func(step deploy.Step) (SnapshotMutation, error) {
			beginCalled++
			return nil, errors.New("begin error")
		},
		RegisterResourceOutputsF: func(step deploy.Step) error {
			registerCalled++
			return errors.New("register error")
		},
		CloseF: func() error {
			closeCalled++
			return errors.New("close error")
		},
	}
	workingManager := &MockSnapshotManager{
		WriteF: func(base *deploy.Snapshot) error {
			writeCalled++
			return nil
		},
		RebuiltBaseStateF: func() error {
			rebuiltCalled++
			return nil
		},
		BeginMutationF: func(step deploy.Step) (SnapshotMutation, error) {
			beginCalled++
			return &MockSanpshotMutation{
				EndF: func(step deploy.Step, success bool) error {
					endCalled++
					return nil
				},
			}, nil
		},
		RegisterResourceOutputsF: func(step deploy.Step) error {
			registerCalled++
			return nil
		},
		CloseF: func() error {
			closeCalled++
			return nil
		},
	}

	cm := &CombinedManager{
		Managers:          []SnapshotManager{workingManager, erroringManager},
		CollectErrorsOnly: []bool{false, true},
	}

	err := cm.Write(&deploy.Snapshot{})
	require.NoError(t, err)

	err = cm.RebuiltBaseState()
	require.NoError(t, err)

	end, err := cm.BeginMutation(nil)
	require.NoError(t, err)

	err = end.End(nil, true)
	require.NoError(t, err)

	err = cm.RegisterResourceOutputs(nil)
	require.NoError(t, err)

	err = cm.Close()
	require.NoError(t, err)

	require.Len(t, cm.errors, 5)
	require.ErrorContains(t, cm.errors[0], "write error")
	require.ErrorContains(t, cm.errors[1], "rebuilt error")
	require.ErrorContains(t, cm.errors[2], "begin error")
	require.ErrorContains(t, cm.errors[3], "register error")
	require.ErrorContains(t, cm.errors[4], "close error")

	require.Equal(t, 2, writeCalled)
	require.Equal(t, 2, rebuiltCalled)
	require.Equal(t, 2, beginCalled)
	require.Equal(t, 2, registerCalled)
	require.Equal(t, 2, closeCalled)
	require.Equal(t, 1, endCalled) // Only the working manager's mutation's End is called
}
