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
	"github.com/stretchr/testify/require"
)

var _ = SnapshotManager(&MockSnapshotManager{})

type MockSnapshotManager struct {
	WriteF                   func(base *deploy.Snapshot) error
	RebuiltBaseStateF        func() error
	BeginMutationF           func(step deploy.Step) (SnapshotMutation, error)
	RegisterResourceOutputsF func(step deploy.Step) error
	CloseF                   func() error
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
