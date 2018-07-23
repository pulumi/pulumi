// Copyright 2016-2018, Pulumi Corporation.
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
	"time"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/version"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// SnapshotPersister is an interface implemented by our backends that implements snapshot
// persistence. In order to fit into our current model, snapshot persisters have two functions:
// saving snapshots and invalidating already-persisted snapshots.
type SnapshotPersister interface {
	// Invalidates the last snapshot that was persisted. This is done as the first step
	// of performing a mutation on the snapshot. Returns an error if the invalidation failed.
	Invalidate() error

	// Persists the given snapshot. Returns an error if the persistence failed.
	Save(snapshot *deploy.Snapshot) error
}

// SnapshotManager is an implementation of engine.SnapshotManager that inspects steps and performs
// mutations on the global snapshot object serially. This implementation maintains two bits of state: the "base"
// snapshot, which is completely immutable and represents the state of the world prior to the application
// of the current plan, and a "new" list of resources, which consists of the resources that were operated upon
// by the current plan.
//
// Important to note is that, although this SnapshotManager is designed to be easily convertible into a thread-safe
// implementation, the code as it is today is *not thread safe*. In particular, it is not legal for there to be
// more than one `SnapshotMutation` active at any point in time. This is because this SnapshotManager invalidates
// the last persisted snapshot in `BeginSnapshot`. This is designed to match existing behavior and will not
// be the state of things going forward.
//
// The resources stored in the `resources` slice are pointers to resource objects allocated by the engine.
// This is subtle and a little confusing. The reason for this is that the engine directly mutates resource objects
// that it creates and expects those mutations to be persisted directly to the snapshot.
type SnapshotManager struct {
	persister        SnapshotPersister        // The persister responsible for invalidating and persisting the snapshot
	baseSnapshot     *deploy.Snapshot         // The base snapshot for this plan
	resources        []*resource.State        // The list of resources operated upon by this plan
	dones            map[*resource.State]bool // The set of resources that have been operated upon already by this plan
	doVerify         bool                     // If true, verify the snapshot before persisting it
	plugins          []workspace.PluginInfo   // The list of plugins loaded by the plan, to be saved in the manifest
	mutationRequests chan func()              // The queue of mutation requests, to be retired serially by the manager
}

var _ engine.SnapshotManager = (*SnapshotManager)(nil)

func (sm *SnapshotManager) Close() error {
	close(sm.mutationRequests)
	return nil
}

// If you need to understand what's going on in this file, start here!
//
// mutate is the serialization point for reads and writes of the global snapshot state.
// The given function will be, at the time of its invocation, the only function allowed to
// mutate state within the SnapshotManager.
//
// Serialization is performed by pushing the mutation function onto a channel, where another
// goroutine is polling the channel and executing the mutation functions as they come.
// This function optionally verifies the integrity of the snapshot before and after mutation.
// Immediately after the mutating function is run, the snapshot's manifest is updated and,
// if there are no verification errors, the snapshot is persisted.
//
// You should never observe or mutate the global snapshot without using this function unless
// you have a very good justification.
func (sm *SnapshotManager) mutate(mutator func()) error {
	responseChan := make(chan error)
	sm.mutationRequests <- func() {
		mutator()

		snap := sm.snap()
		err := sm.persister.Save(snap)
		if err == nil && sm.doVerify {
			if err = snap.VerifyIntegrity(); err != nil {
				err = errors.Wrapf(err, "after mutation of snapshot")
			}
		}

		if err != nil {
			err = errors.Wrap(err, "failed to save snapshot")
		}

		responseChan <- err
	}

	return <-responseChan
}

// RegisterResourceOutputs handles the registering of outputs on a Step that has already
// completed. This is accomplished by doing an in-place mutation of the resources currently
// resident in the snapshot.
//
// Due to the way this is currently implemented, the engine directly mutates output properties
// on the resource State object that it created. Since we are storing pointers to these objects
// in the `resources` slice, we need only to do a no-op mutation in order to flush these new
// mutations to disk.
//
// Note that this is completely not thread-safe and defeats the purpose of having a `mutate` callback
// entirely, but the hope is that this state of things will not be permament.
func (sm *SnapshotManager) RegisterResourceOutputs(step deploy.Step) error {
	return sm.refresh()
}

// RecordPlugin records that the current plan loaded a plugin and saves it in the snapshot.
func (sm *SnapshotManager) RecordPlugin(plugin workspace.PluginInfo) error {
	logging.V(9).Infof("SnapshotManager: RecordPlugin(%v)", plugin)
	return sm.mutate(func() {
		sm.plugins = append(sm.plugins, plugin)
	})
}

// BeginMutation signals to the SnapshotManager that the engine intends to mutate the global snapshot
// by performing the given Step. This function gives the SnapshotManager a chance to record the
// intent to mutate before the mutation occurs.
func (sm *SnapshotManager) BeginMutation(step deploy.Step) (engine.SnapshotMutation, error) {
	contract.Require(step != nil, "step != nil")
	logging.V(9).Infof("SnapshotManager: Beginning mutation for step `%s` on resource `%s`", step.Op(), step.URN())

	// This is for compat with the existing update model with the service. Invalidating a
	// stack sets a bit in a database indicating that the stored snapshot is not valid.
	if err := sm.persister.Invalidate(); err != nil {
		logging.V(9).Infof("SnapshotManager: Failed to invalidate snapshot: %s", err.Error())
		return nil, err
	}

	switch step.Op() {
	case deploy.OpSame:
		return &sameSnapshotMutation{sm}, nil
	case deploy.OpCreate, deploy.OpCreateReplacement:
		return &createSnapshotMutation{sm}, nil
	case deploy.OpUpdate:
		return &updateSnapshotMutation{sm}, nil
	case deploy.OpDelete, deploy.OpDeleteReplaced:
		return &deleteSnapshotMutation{sm}, nil
	case deploy.OpReplace:
		return &replaceSnapshotMutation{}, nil
	case deploy.OpRead, deploy.OpReadReplacement:
		return &readSnapshotMutation{sm}, nil
	}

	contract.Failf("unknown StepOp: %s", step.Op())
	return nil, nil
}

// All SnapshotMutation implementations in this file follow the same basic formula:
// mark the "old" state as done and mark the "new" state as new. The two special
// cases are Create (where the "old" state does not exist) and Delete (where the "new" state
// does not exist).
//
// Marking a resource state as old prevents it from being persisted to the snapshot in
// the `snap` function. Marking a resource state as new /enables/ it to be persisted to
// the snapshot in `snap`. See the comments in `snap` for more details.

type sameSnapshotMutation struct {
	manager *SnapshotManager
}

func (ssm *sameSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Require(step != nil, "step != nil")
	logging.V(9).Infof("SnapshotManager: sameSnapshotMutation.End(..., %v)", successful)
	return ssm.manager.mutate(func() {
		if successful {
			ssm.manager.markDone(step.Old())
			ssm.manager.markNew(step.New())
		}
	})
}

type createSnapshotMutation struct {
	manager *SnapshotManager
}

func (csm *createSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Require(step != nil, "step != nil")
	logging.V(9).Infof("SnapshotManager: createSnapshotMutation.End(..., %v)", successful)
	return csm.manager.mutate(func() {
		if successful {
			// There is some very subtle behind-the-scenes magic here that
			// comes into play whenever this create is a CreateReplacement.
			//
			// Despite intending for the base snapshot to be immutable, the engine
			// does in fact mutate it by setting a `Delete` flag on resources
			// being replaced as part of a Create-Before-Delete replacement sequence.
			// Since we are storing the base snapshot and all resources by reference
			// (we have pointers to engine-allocated objects), this transparently
			// "just works" for the SnapshotManager.
			csm.manager.markNew(step.New())
		}
	})
}

type updateSnapshotMutation struct {
	manager *SnapshotManager
}

func (usm *updateSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Require(step != nil, "step != nil")
	logging.V(9).Infof("SnapshotManager: updateSnapshotMutation.End(..., %v)", successful)
	return usm.manager.mutate(func() {
		if successful {
			usm.manager.markDone(step.Old())
			usm.manager.markNew(step.New())
		}
	})
}

type deleteSnapshotMutation struct {
	manager *SnapshotManager
}

func (dsm *deleteSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Require(step != nil, "step != nil")
	logging.V(9).Infof("SnapshotManager: deleteSnapshotMutation.End(..., %v)", successful)
	return dsm.manager.mutate(func() {
		if successful {
			contract.Assert(!step.Old().Protect)
			dsm.manager.markDone(step.Old())
		}
	})
}

type replaceSnapshotMutation struct{}

func (rsm *replaceSnapshotMutation) End(step deploy.Step, successful bool) error { return nil }

type readSnapshotMutation struct {
	manager *SnapshotManager
}

func (rsm *readSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Require(step != nil, "step != nil")
	logging.V(9).Infof("SnapshotManager: readSnapshotMutation.End(..., %v)", successful)
	return rsm.manager.mutate(func() {
		if successful {
			if step.Old() != nil {
				rsm.manager.markDone(step.Old())
			}

			rsm.manager.markNew(step.New())
		}
	})
}

// refresh does a no-op mutation that forces the SnapshotManager to persist the
// snapshot exactly as it is currently to disk. This is useful when a mutation
// has failed and we do not intend to persist the failed mutation.
func (sm *SnapshotManager) refresh() error {
	logging.V(9).Infof("SnapshotManager: refresh()")
	return sm.mutate(func() {})
}

// markDone marks a resource as having been processed. Resources that have been marked
// in this manner won't be persisted in the snapshot.
func (sm *SnapshotManager) markDone(state *resource.State) {
	contract.Assert(state != nil)
	sm.dones[state] = true
	logging.V(9).Infof("Marked old state snapshot as done: %v", state.URN)
}

// markNew marks a resource as existing in the new snapshot. This occurs on
// successful non-deletion operations where the given state is the new state
// of a resource that will be persisted to the snapshot.
func (sm *SnapshotManager) markNew(state *resource.State) {
	contract.Assert(state != nil)
	sm.resources = append(sm.resources, state)
	logging.V(9).Infof("Appended new state snapshot to be written: %v", state.URN)
}

// snap produces a new Snapshot given the base snapshot and a list of resources that the current
// plan has created.
func (sm *SnapshotManager) snap() *deploy.Snapshot {
	// At this point we have two resource DAGs. One of these is the base DAG for this plan; the other is the current DAG
	// for this plan. Any resource r may be present in both DAGs. In order to produce a snapshot, we need to merge these
	// DAGs such that all resource dependencies are correctly preserved. Conceptually, the merge proceeds as follows:
	//
	// - Begin with an empty merged DAG.
	// - For each resource r in the current DAG, insert r and its outgoing edges into the merged DAG.
	// - For each resource r in the base DAG:
	//     - If r is in the merged DAG, we are done: if the resource is in the merged DAG, it must have been in the
	//       current DAG, which accurately captures its current dependencies.
	//     - If r is not in the merged DAG, insert it and its outgoing edges into the merged DAG.
	//
	// Physically, however, each DAG is represented as list of resources without explicit dependency edges. In place of
	// edges, it is assumed that the list represents a valid topological sort of its source DAG. Thus, any resource r at
	// index i in a list L must be assumed to be dependent on all resources in L with index j s.t. j < i. Due to this
	// representation, we implement the algorithm above as follows to produce a merged list that represents a valid
	// topological sort of the merged DAG:
	//
	// - Begin with an empty merged list.
	// - For each resource r in the current list, append r to the merged list. r must be in a correct location in the
	//   merged list, as its position relative to its assumed dependencies has not changed.
	// - For each resource r in the base list:
	//     - If r is in the merged list, we are done by the logic given in the original algorithm.
	//     - If r is not in the merged list, append r to the merged list. r must be in a correct location in the merged
	//       list:
	//         - If any of r's dependencies were in the current list, they must already be in the merged list and their
	//           relative order w.r.t. r has not changed.
	//         - If any of r's dependencies were not in the current list, they must already be in the merged list, as
	//           they would have been appended to the list before r.

	// Start with a copy of the resources produced during the evaluation of the current plan.
	resources := make([]*resource.State, len(sm.resources))
	copy(resources, sm.resources)

	// Append any resources from the base plan that were not produced by the current plan.
	if base := sm.baseSnapshot; base != nil {
		for _, res := range base.Resources {
			if !sm.dones[res] {
				resources = append(resources, res)
			}
		}
	}

	manifest := deploy.Manifest{
		Time:    time.Now(),
		Version: version.Version,
		Plugins: sm.plugins,
	}

	manifest.Magic = manifest.NewMagic()
	return deploy.NewSnapshot(manifest, resources)
}

// NewSnapshotManager creates a new SnapshotManager for the given stack name, using the given persister
// and base snapshot.
//
// It is *very important* that the baseSnap pointer refers to the same Snapshot
// given to the engine! The engine will mutate this object and correctness of the
// SnapshotManager depends on being able to observe this mutation. (This is not ideal...)
func NewSnapshotManager(persister SnapshotPersister, baseSnap *deploy.Snapshot) *SnapshotManager {
	manager := &SnapshotManager{
		persister:        persister,
		baseSnapshot:     baseSnap,
		dones:            make(map[*resource.State]bool),
		doVerify:         true,
		mutationRequests: make(chan func()),
	}

	go func() {
		for request := range manager.mutationRequests {
			request()
		}
	}()

	return manager
}
