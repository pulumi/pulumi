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
	"context"
	"reflect"
	"sort"
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
	operations       []resource.Operation     // The set of operations known to be outstanding in this plan
	dones            map[*resource.State]bool // The set of resources that have been operated upon already by this plan
	completeOps      map[*resource.State]bool // The set of resources that have completed their operation
	doVerify         bool                     // If true, verify the snapshot before persisting it
	plugins          []workspace.PluginInfo   // The list of plugins loaded by the plan, to be saved in the manifest
	mutationRequests chan<- mutationRequest   // The queue of mutation requests, to be retired serially by the manager
	cancel           chan bool                // A channel used to request cancellation of any new mutation requests.
	done             <-chan error             // A channel that sends a single result when the manager has shut down.
}

var _ engine.SnapshotManager = (*SnapshotManager)(nil)

type mutationRequest struct {
	mutator func() bool
	result  chan<- error
}

func (sm *SnapshotManager) Close() error {
	close(sm.cancel)
	return <-sm.done
}

// If you need to understand what's going on in this file, start here!
//
// mutate is the serialization point for reads and writes of the global snapshot state.
// The given function will be, at the time of its invocation, the only function allowed to
// mutate state within the SnapshotManager.
//
// Serialization is performed by pushing the mutator function onto a channel, where another
// goroutine is polling the channel and executing the mutation functions as they come.
// This function optionally verifies the integrity of the snapshot before and after mutation.
//
// The mutator may indicate that its corresponding checkpoint write may be safely elided by
// returning `false`. As of this writing, we only elide writes after same steps with no
// meaningful changes (see sameSnapshotMutation.mustWrite for details). Any elided writes
// are flushed by the next non-elided write or the next call to Close.
//
// You should never observe or mutate the global snapshot without using this function unless
// you have a very good justification.
func (sm *SnapshotManager) mutate(mutator func() bool) error {
	result := make(chan error)
	select {
	case sm.mutationRequests <- mutationRequest{mutator: mutator, result: result}:
		return <-result
	case <-sm.cancel:
		return errors.New("snapshot manager closed")
	}
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
	return sm.mutate(func() bool { return true })
}

// RecordPlugin records that the current plan loaded a plugin and saves it in the snapshot.
func (sm *SnapshotManager) RecordPlugin(plugin workspace.PluginInfo) error {
	logging.V(9).Infof("SnapshotManager: RecordPlugin(%v)", plugin)
	return sm.mutate(func() bool {
		sm.plugins = append(sm.plugins, plugin)
		return true
	})
}

// BeginMutation signals to the SnapshotManager that the engine intends to mutate the global snapshot
// by performing the given Step. This function gives the SnapshotManager a chance to record the
// intent to mutate before the mutation occurs.
func (sm *SnapshotManager) BeginMutation(step deploy.Step) (engine.SnapshotMutation, error) {
	contract.Require(step != nil, "step != nil")
	logging.V(9).Infof("SnapshotManager: Beginning mutation for step `%s` on resource `%s`", step.Op(), step.URN())

	switch step.Op() {
	case deploy.OpSame:
		return &sameSnapshotMutation{sm}, nil
	case deploy.OpCreate, deploy.OpCreateReplacement:
		return sm.doCreate(step)
	case deploy.OpUpdate:
		return sm.doUpdate(step)
	case deploy.OpDelete, deploy.OpDeleteReplaced:
		return sm.doDelete(step)
	case deploy.OpReplace:
		return &replaceSnapshotMutation{sm}, nil
	case deploy.OpRead, deploy.OpReadReplacement:
		return sm.doRead(step)
	case deploy.OpRefresh:
		return &refreshSnapshotMutation{sm}, nil
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

// mustWrite returns true if any semantically meaningful difference exists between the old and new states of a same
// step that forces us to write the checkpoint. If no such difference exists, the checkpoint write that corresponds to
// this step can be elided.
func (ssm *sameSnapshotMutation) mustWrite(old, new *resource.State) bool {
	contract.Assert(old.Type == new.Type)
	contract.Assert(old.URN == new.URN)
	contract.Assert(old.Delete == new.Delete)
	contract.Assert(old.External == new.External)

	// If the kind of this resource has changed, we must write the checkpoint.
	if old.Custom != new.Custom {
		return true
	}

	contract.Assert(old.ID == new.ID)
	contract.Assert(old.Provider == new.Provider)

	// If this resource's parent has changed, we must write the checkpoint.
	if old.Parent != new.Parent {
		return true
	}

	// If the protection attribute of this resource has changed, we must write the checkpoint.
	if old.Protect != new.Protect {
		return true
	}

	// If the inputs or outputs of this resource have changed, we must write the checkpoint. Note that it is possible
	// for the inputs of a "same" resource to have changed even if the contents of the input bags are different if the
	// resource's provider deems the physical change to be semantically irrelevant.
	if !reflect.DeepEqual(old.Inputs, new.Inputs) || !reflect.DeepEqual(old.Outputs, new.Outputs) {
		return true
	}

	// Sort dependencies before comparing them. If the dependencies have changed, we must write the checkpoint.
	//
	// Init errors are strictly advisory, so we do not consider them when deciding whether or not to write the
	// checkpoint.
	sortDeps := func(deps []resource.URN) {
		sort.Slice(deps, func(i, j int) bool { return deps[i] < deps[j] })
	}
	sortDeps(old.Dependencies)
	sortDeps(new.Dependencies)
	return !reflect.DeepEqual(old.Dependencies, new.Dependencies)
}

func (ssm *sameSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Require(step != nil, "step != nil")
	contract.Require(step.Op() == deploy.OpSame, "step.Op() == deploy.OpSame")
	contract.Assert(successful)
	logging.V(9).Infof("SnapshotManager: sameSnapshotMutation.End(..., %v)", successful)
	return ssm.manager.mutate(func() bool {
		ssm.manager.markDone(step.Old())
		ssm.manager.markNew(step.New())

		// Note that "Same" steps only consider input and provider diffs, so it is possible to see a same step for a
		// resource with new dependencies, outputs, parent, protection. etc.
		//
		// As such, we diff all of the non-input properties of the resource here and write the snapshot if we find any
		// changes.
		if !ssm.mustWrite(step.Old(), step.New()) {
			logging.V(9).Infof("SnapshotManager: sameSnapshotMutation.End() eliding write")
			return false
		}
		return true
	})
}

func (sm *SnapshotManager) doCreate(step deploy.Step) (engine.SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doCreate(%s)", step.URN())
	err := sm.mutate(func() bool {
		sm.markOperationPending(step.New(), resource.OperationTypeCreating)
		return true
	})
	if err != nil {
		return nil, err
	}

	return &createSnapshotMutation{sm}, nil
}

type createSnapshotMutation struct {
	manager *SnapshotManager
}

func (csm *createSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Require(step != nil, "step != nil")
	logging.V(9).Infof("SnapshotManager: createSnapshotMutation.End(..., %v)", successful)
	return csm.manager.mutate(func() bool {
		csm.manager.markOperationComplete(step.New())
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
		return true
	})
}

func (sm *SnapshotManager) doUpdate(step deploy.Step) (engine.SnapshotMutation, error) {
	logging.V(9).Info("SnapshotManager.doUpdate(%s)", step.URN())
	err := sm.mutate(func() bool {
		sm.markOperationPending(step.New(), resource.OperationTypeUpdating)
		return true
	})
	if err != nil {
		return nil, err
	}

	return &updateSnapshotMutation{sm}, nil
}

type updateSnapshotMutation struct {
	manager *SnapshotManager
}

func (usm *updateSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Require(step != nil, "step != nil")
	logging.V(9).Infof("SnapshotManager: updateSnapshotMutation.End(..., %v)", successful)
	return usm.manager.mutate(func() bool {
		usm.manager.markOperationComplete(step.New())
		if successful {
			usm.manager.markDone(step.Old())
			usm.manager.markNew(step.New())
		}
		return true
	})
}

func (sm *SnapshotManager) doDelete(step deploy.Step) (engine.SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doDelete(%s)", step.URN())
	err := sm.mutate(func() bool {
		sm.markOperationPending(step.Old(), resource.OperationTypeDeleting)
		return true
	})
	if err != nil {
		return nil, err
	}

	return &deleteSnapshotMutation{sm}, nil
}

type deleteSnapshotMutation struct {
	manager *SnapshotManager
}

func (dsm *deleteSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Require(step != nil, "step != nil")
	logging.V(9).Infof("SnapshotManager: deleteSnapshotMutation.End(..., %v)", successful)
	return dsm.manager.mutate(func() bool {
		dsm.manager.markOperationComplete(step.Old())
		if successful {
			contract.Assert(!step.Old().Protect)
			dsm.manager.markDone(step.Old())
		}
		return true
	})
}

type replaceSnapshotMutation struct {
	manager *SnapshotManager
}

func (rsm *replaceSnapshotMutation) End(step deploy.Step, successful bool) error {
	logging.V(9).Infof("SnapshotManager: replaceSnapshotMutation.End(..., %v)", successful)
	return nil
}

func (sm *SnapshotManager) doRead(step deploy.Step) (engine.SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doRead(%s)", step.URN())
	err := sm.mutate(func() bool {
		sm.markOperationPending(step.New(), resource.OperationTypeReading)
		return true
	})
	if err != nil {
		return nil, err
	}

	return &readSnapshotMutation{sm}, nil
}

type readSnapshotMutation struct {
	manager *SnapshotManager
}

func (rsm *readSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Require(step != nil, "step != nil")
	logging.V(9).Infof("SnapshotManager: readSnapshotMutation.End(..., %v)", successful)
	return rsm.manager.mutate(func() bool {
		rsm.manager.markOperationComplete(step.New())
		if successful {
			if step.Old() != nil {
				rsm.manager.markDone(step.Old())
			}

			rsm.manager.markNew(step.New())
		}
		return true
	})
}

type refreshSnapshotMutation struct {
	manager *SnapshotManager
}

func (rsm *refreshSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Require(step != nil, "step != nil")
	contract.Require(step.Op() == deploy.OpRefresh, "step.Op() == deploy.OpRefresh")
	logging.V(9).Infof("SnapshotManager: refreshSnapshotMutation.End(..., %v)", successful)
	return rsm.manager.mutate(func() bool {
		// We always elide refreshes. The expectation is that all of these run before any actual mutations and that
		// some other component will rewrite the base snapshot in-memory, so there's no action the snapshot
		// manager needs to take other than to remember that the base snapshot--and therefore the actual snapshot--may
		// have changed.
		return false
	})
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

// markOperationPending marks a resource as undergoing an operation that will now be considered pending.
func (sm *SnapshotManager) markOperationPending(state *resource.State, op resource.OperationType) {
	contract.Assert(state != nil)
	sm.operations = append(sm.operations, resource.NewOperation(state, op))
	logging.V(9).Infof("SnapshotManager.markPendingOperation(%s, %s)", state.URN, string(op))
}

// markOperationComplete marks a resource as having completed the operation that it previously was performing.
func (sm *SnapshotManager) markOperationComplete(state *resource.State) {
	contract.Assert(state != nil)
	sm.completeOps[state] = true
	logging.V(9).Infof("SnapshotManager.markOperationComplete(%s)", state.URN)
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

	// Record any pending operations, if there are any outstanding that have not completed yet.
	var operations []resource.Operation
	for _, op := range sm.operations {
		if !sm.completeOps[op.Resource] {
			operations = append(operations, op)
		}
	}

	manifest := deploy.Manifest{
		Time:    time.Now(),
		Version: version.Version,
		Plugins: sm.plugins,
	}

	manifest.Magic = manifest.NewMagic()
	return deploy.NewSnapshot(manifest, resources, operations)
}

// saveSnapshot persists the current snapshot and optionally verifies it afterwards.
func (sm *SnapshotManager) saveSnapshot(ctx context.Context) error {
	snap := sm.snap()
	if err := sm.persister.Save(snap); err != nil {
		return errors.Wrap(err, "failed to save snapshot")
	}
	if sm.doVerify {
		if err := snap.VerifyIntegrity(); err != nil {
			return errors.Wrapf(err, "failed to verify snapshot")
		}
	}
	return nil
}

// NewSnapshotManager creates a new SnapshotManager for the given stack name, using the given persister
// and base snapshot.
//
// It is *very important* that the baseSnap pointer refers to the same Snapshot
// given to the engine! The engine will mutate this object and correctness of the
// SnapshotManager depends on being able to observe this mutation. (This is not ideal...)
func NewSnapshotManager(persister SnapshotPersister, baseSnap *deploy.Snapshot) *SnapshotManager {
	mutationRequests, cancel, done := make(chan mutationRequest), make(chan bool), make(chan error)

	ctx := context.Background()

	manager := &SnapshotManager{
		persister:        persister,
		baseSnapshot:     baseSnap,
		dones:            make(map[*resource.State]bool),
		completeOps:      make(map[*resource.State]bool),
		doVerify:         true,
		mutationRequests: mutationRequests,
		cancel:           cancel,
		done:             done,
	}

	go func() {
		// True if we have elided writes since the last actual write.
		hasElidedWrites := false

		// Service each mutation request in turn.
	serviceLoop:
		for {
			select {
			case request := <-mutationRequests:
				var err error
				if request.mutator() {
					err = manager.saveSnapshot(ctx)
					hasElidedWrites = false
				} else {
					hasElidedWrites = true
				}
				request.result <- err
			case <-cancel:
				break serviceLoop
			}
		}

		// If we still have elided writes once the channel has closed, flush the snapshot.
		var err error
		if hasElidedWrites {
			logging.V(9).Infof("SnapshotManager: flushing elided writes...")
			err = manager.saveSnapshot(ctx)
		}
		done <- err
	}()

	return manager
}
