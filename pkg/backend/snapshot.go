// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package backend

import (
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/version"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// SnapshotPersister is an interface for persisting the in-memory snapshot
// to some sort of persistent storage. The error returned is nil if the
// persisting was successful, false otherwise.
type SnapshotPersister interface {
	// Persists the in-memory snapshot to storage in some manner.
	SaveSnapshot(snapshot *deploy.Snapshot) error
}

// SnapshotManager is an implementation of engine.SnapshotManager that inspects steps and performs
// mutations on the global snapshot object serially. This implementation maintains two snapshots: the "base"
// snapshot, which is completely immutable and represents the state of the world prior to the application
// of the current plan, and the "new" snapshot, which consists of the changes that have occurred as a result
// of executing the current plan.
//
// All public members of SnapshotManager (namely `BeginMutation`, `RegisterResourceOutputs`, `RecordPlugins`,
// and all `End` functions returned by implementations of `SnapshotMutation`) are safe to use
// unsynchronized across goroutines.
//
// A more principled language might allow us to express that `mutate` yields a mutable reference to the snapshot,
// but this is Go, and we can't do that.
//
// It is worth noting that some `*resource.State` pointers point into the global Snapshot. `mutate`
// must be used to read *OR* write such pointers. These pointers are generally stored within the implementations
// of `SnapshotMutation` and will be called-out explicitly in documentation.
//
// ## Debugging Tips
// The environment variable `PULUMI_RETAIN_CHECKPOINTS`, when set, causes local backups of the checkpoint file
// to be suffixed with a timestamp instead repeatedly overwriting the same file. You can use this to reconstruct
// an exact sequence of events that resulted in an invalid checkpoint. The scripts `diff_checkpoint.sh` and
// `find_checkpoint.sh` are useful for diffing and viewing checkpoint files in this format, respectively. In particular,
// `diff_checkpoint.sh n` shows a diff of the n'th mutation to the snapshot.
type SnapshotManager struct {
	persister        SnapshotPersister // The persister responsible for saving the snapshot
	baseSnapshot     *deploy.Snapshot  // The base snapshot of the current plan.
	newSnapshot      *deploy.Snapshot  // The new snapshot representing changes made by the current plan.
	doVerify         bool              // If true, call `snapshot.VerifyIntegrity` after modifying the snapshot
	mutationRequests chan func()       // channel for serializing mutations to the snapshot
}

var _ engine.SnapshotManager = (*SnapshotManager)(nil)

// If you need to understand what's going on in this file, start here!
//
// mutate is the serialization point for reads and writes of the global snapshot state.
// The given function will be, at the time of its invocation, the only function allowed to
// mutate the global snapshot and associated data.
//
// Serialization is performed by pushing the mutation function onto a channel, where another
// goroutine is polling the channel and executing the mutation functions as they come.
// This function optionally verifies the integrity of the snapshot before and after mutation.
// Immediately after the mutating function is run, the snapshot's manifest is updated and,
// if there are no verification errors, the snapshot is persisted to disk.
//
// You should never observe or mutate the global snapshot without using this function unless
// you have a very good justification.
func (sm *SnapshotManager) mutate(mutator func()) error {
	responseChan := make(chan error)
	sm.mutationRequests <- func() {
		mutator()
		sm.updateManifest()
		snap := sm.merge()
		err := sm.persister.SaveSnapshot(snap)
		if err != nil && sm.doVerify {
			if verifyErr := snap.VerifyIntegrity(); verifyErr != nil {
				responseChan <- errors.Wrapf(verifyErr, "after mutation of snapshot")
				return
			}
		}

		responseChan <- err
	}

	return <-responseChan
}

// merge is responsible for "merging" the base snapshot, `baseSnapshot`, and the current list
// of resources, `newSnapshot` into a snapshot that ultimately will be persisted.
func (sm *SnapshotManager) merge() *deploy.Snapshot {
	// This algorithm is loosely based on one originally devised by Pat for `PlanIterator.Snap()`, which
	// conceptually used to do something quite similar to what is done here. Below is a sketch of
	// the algorithm used here.
	//
	// This is the algorithm for producing a "merged" snapshot, given a valid base and new snapshot:
	//   1. Begin with an empty merged snapshot, called "merge".
	//   2. For each resource R in the new snapshot:
	//       a. If R is not pending deletion, deleting, or deleted,
	//         i. Insert R into "merge".
	//   2. For each live resource R in the base snapshot:
	//       a. If R does not exist in the new snapshot
	//         i. Insert R into "merge".
	//       b. If R does exist in the new snapshot AND at least one of its occurrences is pending deletion:
	//         i. Insert the node that is pending deletion from the new snapshot into "merge".
	//       c. If R does exist in the new snapshot AND at least one of its occurrences is deleting:
	//         i. Insert the node that is deleting from the new snapshot into "merge".
	//       d. Otherwise, do nothing.
	//   3. "merge" now contains a correctly merged DAG of resources.
	//
	// The difference between the `PlanIterator.Snap()` algorithm and this one is that deletions are not
	// guaranteed to come in to the snapshot manager in a dependency-ordered fashion; in fact, they almost
	// always will appear to us in the *opposite* of a dependency-ordered fashion. In order to ensure that
	// in-progress deletes, pending deletions, and fully-deleted resources appear correctly in the merged DAG,
	// we will insert pending-delete and deleting nodes from the new snapshot into the merged snapshot if such
	// nodes exist in the new snapshot and the algorithm would otherwise dictate that we copy the live
	// (i.e. resource that this plan deleted) node from the base snapshot into the merged snapshot.
	snap := deploy.NewSnapshot(sm.newSnapshot.Stack, sm.newSnapshot.Manifest, nil)
	var resources []*resource.State

	addNews := func() {
		for _, newResource := range sm.newSnapshot.Resources {
			switch newResource.Status {
			case resource.ResourceStatusPendingDeletion:
			case resource.ResourceStatusDeleting:
			case resource.ResourceStatusDeleted:
				continue
			default:
				resources = append(resources, newResource.Clone())
			}
		}
	}

	copyBase := func(oldResource *resource.State) {
		var pendingDelete *resource.State
		var deleting *resource.State
		var seenResource bool

		for _, newResource := range sm.newSnapshot.Resources {
			if newResource.URN != oldResource.URN {
				continue
			}

			switch newResource.Status {
			case resource.ResourceStatusPendingDeletion:
				seenResource = true
				pendingDelete = newResource
			case resource.ResourceStatusDeleting:
				seenResource = true
				deleting = newResource
			default:
				seenResource = true
			}
		}

		if !seenResource {
			resources = append(resources, oldResource.Clone())
			return
		}

		// If the resource is condemned in the old snapshot and we saw it in the new snapshot,
		// consider it to be handled already.
		if oldResource.Status.Condemned() {
			return
		}

		if pendingDelete != nil {
			contract.Assertf(deleting == nil, "should not have seen resource that is both deleting and pending delete")
			resources = append(resources, pendingDelete.Clone())
			return
		}

		if deleting != nil {
			resources = append(resources, deleting.Clone())
		}
	}

	copyBases := func() {
		for _, oldResource := range sm.baseSnapshot.Resources {
			// All live and condemned resources need to be copied over into the merged snapshot.
			if oldResource.Status.Live() || oldResource.Status.Condemned() {
				copyBase(oldResource)
			}
		}
	}

	addNews()
	copyBases()
	snap.Resources = resources
	return snap
}

func (sm *SnapshotManager) doMutations() {
	for mutatingFunc := range sm.mutationRequests {
		mutatingFunc()
	}
}

// Close closes this SnapshotManager when no further mutations will be made.
func (sm *SnapshotManager) Close() error {
	close(sm.mutationRequests)
	return nil
}

// RecordPlugins records the given list of plugins in the manifest, so that Destroy operations
// operating on this snapshot know which plugins need to be loaded without having to inspect
// the program.
func (sm *SnapshotManager) RecordPlugins(plugins []workspace.PluginInfo) error {
	err := sm.mutate(func() {
		pluginsSlice := sm.newSnapshot.Manifest.Plugins[:0]
		sm.newSnapshot.Manifest.Plugins = append(pluginsSlice, plugins...)
	})

	if err != nil {
		glog.V(9).Infof("failed to persist plugin information: %v", err)
	}

	return err
}

// RegisterResourceOutputs registers the outputs of a step with the snapshot, once a step has completed. The
// engine has populated the `New` resource of the given step with the Outputs that it would like to save in
// the snapshot.
func (sm *SnapshotManager) RegisterResourceOutputs(step deploy.Step) error {
	err := sm.mutate(func() {
		res := sm.findLiveInSnapshot(step.URN())
		contract.Assertf(res != nil, "registerResourceOutputs target (%s) not found in snapshot", step.URN())
		step.New().CopyIntoWithoutMetadata(res)
	})

	if err != nil {
		glog.V(9).Infof("failed to persist registerResourceOutputs info: %v", err)
	}

	return err
}

// noOpSnapshotMutation is a snapshot mutation that doesn't actually mutate anything. It is invoked
// whenever the planner creates logical steps that do not mutate anything when applied. In practice, it is
// only the `ReplaceStep` that truly requires no mutations, since it is a purely logical operation.
type noOpSnapshotMutation struct{}

func (same noOpSnapshotMutation) End(step deploy.Step) error {
	return nil
}

// BeginMutation signals to the SnapshotManager that the planner intends to mutate the global
// snapshot. It provides the step that it intends to execute. Based on that step, BeginMutation
// will record this intent in the global snapshot and return a `SnapshotMutation` that, when ended,
// will complete the transaction.
func (sm *SnapshotManager) BeginMutation(step deploy.Step) (engine.SnapshotMutation, error) {
	contract.Require(step != nil, "step != nil")
	glog.V(9).Infof("Beginning mutation for step `%s` on resource `%s`", step.Op(), step.URN())
	switch step.Op() {
	case deploy.OpSame:
		return sm.doSameMutation(step)
	case deploy.OpCreate, deploy.OpCreateReplacement:
		return sm.doCreationMutation(step)
	case deploy.OpUpdate:
		return sm.doUpdateMutation(step)
	case deploy.OpDelete, deploy.OpDeleteReplaced:
		return sm.doDeleteMutation(step)
	case deploy.OpReplace:
		return noOpSnapshotMutation{}, nil
	}

	contract.Failf("unknown StepOp: %s", step.Op())
	return nil, nil
}

// It is counterintuitive, but Same steps still need to be recorded. It is possible for the dependency
// graph of a resource to change without changing input properties, which will result in a Same step
// generated by the planner but with new state that must be saved in the snapshot.
type sameSnapshotMutation struct {
	old *resource.State // The node in the base snapshot that is being "same"-d. Points into
	// the global base snapshot, which is legal to use outside of `mutate`.

	manager *SnapshotManager
}

func (ssm *sameSnapshotMutation) End(step deploy.Step) error {
	err := ssm.manager.mutate(func() {
		new := step.New().Clone()
		new.Status = ssm.old.Status
		new.CreatedAt = ssm.old.CreatedAt
		new.UpdatedAt = ssm.old.UpdatedAt
		ssm.manager.addResource(new)
	})

	if err != nil {
		glog.V(9).Infof("failed to persist same snapshot mutation: %v", err)
	}

	return err
}

// Same steps do not require a mutation before step application, since they do not modify the
// liveness of existing resources. They do require a mutation after step application.
func (sm *SnapshotManager) doSameMutation(step deploy.Step) (engine.SnapshotMutation, error) {
	contract.Require(step.Op() == deploy.OpSame, "step.Op() == deploy.OpSame")
	old := sm.findLiveInBaseSnapshot(step.URN())
	contract.Assertf(old != nil, "failed to find same URN in base snapshot")
	return &sameSnapshotMutation{old: old, manager: sm}, nil
}

// createSnapshotMutation is a SnapshotMutation for a CreateStep.
type createSnapshotMutation struct {
	new *resource.State // The resource created by this CreateStep.
	// Points into the global new snapshot, don't use without `mutate`!

	manager *SnapshotManager // The snapshot manager that created this mutation
}

func (csm *createSnapshotMutation) End(step deploy.Step) error {
	err := csm.manager.mutate(func() {
		contract.Assert(csm.new.Status == resource.ResourceStatusCreating)
		step.New().CopyIntoWithoutMetadata(csm.new)
		if step.Op() == deploy.OpCreateReplacement && step.Old().Delete {
			// This create is potentially the first of a sequence of steps that intends to replace
			// a resource. If this step is creating the resource before the one it intends
			// to replace is deleted (i.e. Create-Before-Delete), this resource will exist "live"
			// in the snapshot at the same time as the resource it intends to replace, which violates
			// our liveness invariant.
			//
			// To deal with this, we'll set the old resource to be "pending-deletion" to mark it
			// as condemned. The engine will delete it later.
			//
			// Not that findLiveInSnapshot should never find the resource we just created because it
			// has status "creating", which is not live, and the fact that we are replacing something
			// means that there is exactly one resource that is live for this URN.
			old := csm.manager.findLiveInBaseSnapshot(step.URN())
			contract.Assertf(old != nil, "failed to find step.URN() in snapshot")

			clonedOld := old.Clone()
			clonedOld.Status = resource.ResourceStatusPendingDeletion
			csm.manager.addResource(clonedOld)
		}

		csm.new.Status = resource.ResourceStatusCreated
		csm.new.CreatedAt = time.Now()
	})

	if err != nil {
		glog.V(9).Infof("failed to persist create snapshot mutation: %v", err)
	}

	return err
}

func (sm *SnapshotManager) doCreationMutation(step deploy.Step) (engine.SnapshotMutation, error) {
	contract.Require(step.Op() == deploy.OpCreate || step.Op() == deploy.OpCreateReplacement,
		"step.Op() == deploy.OpCreate || step.Op() == deploy.OpCreateReplacement")

	// Create steps conceptually create a new resource, so we will take ownership of the resource
	// provided to us by the Step and insert it into the snapshot with the "creating" status.
	snapshotResource := step.New().Clone()
	snapshotResource.Status = resource.ResourceStatusCreating
	err := sm.mutate(func() {
		sm.addResource(snapshotResource)
	})

	if err != nil {
		glog.V(9).Infof("begin state for URN `%s`, persistence failed: %s", step.URN(), err)
		return nil, err
	}

	return &createSnapshotMutation{new: snapshotResource, manager: sm}, nil
}

// updateSnapshotMutation is a SnapshotMutation for an Update, pointing to a resource
// that will be updated without replacement.
type updateSnapshotMutation struct {
	new *resource.State // The resource being updated by this UpdateStep.
	// Points into the global snapshot, don't use without `mutate`!

	manager *SnapshotManager // The snapshot manager that created this mutation
}

func (usm *updateSnapshotMutation) End(step deploy.Step) error {
	err := usm.manager.mutate(func() {
		replacedNode := usm.manager.findLiveInBaseSnapshot(step.URN())
		contract.Assertf(replacedNode != nil, "failed to find updated node in base snapshot")
		contract.Assert(usm.new.Status == resource.ResourceStatusUpdating)
		step.New().CopyIntoWithoutMetadata(usm.new)
		usm.new.Status = resource.ResourceStatusUpdated
		usm.new.CreatedAt = replacedNode.CreatedAt
		usm.new.UpdatedAt = time.Now()
	})

	if err != nil {
		glog.V(9).Infof("failed to persist update snapshot mutation: %v", err)
	}

	return err
}

// doUpdateMutation handles UpdateSteps, which overwrite a resource without replacement.
func (sm *SnapshotManager) doUpdateMutation(step deploy.Step) (engine.SnapshotMutation, error) {
	contract.Require(step.Op() == deploy.OpUpdate, "step.Op() == deploy.OpUpdate")
	contract.Require(step.New() != nil, "step.New() != nil")
	contract.Require(step.Old() != nil, "step.Old() != nil")

	// add a comment here
	var updateNode *resource.State
	err := sm.mutate(func() {
		updateNode = step.New().Clone()
		updateNode.Status = resource.ResourceStatusUpdating
		sm.addResource(updateNode)
	})

	if err != nil {
		glog.V(9).Infof("begin state for URN `%s`, persistence failed: %s", step.URN(), err)
		return nil, err
	}

	return &updateSnapshotMutation{new: updateNode, manager: sm}, nil
}

// deleteSnapshotMutation is generated by resource deletions. Deleted resources get removed
// from the snapshot once their deletion is confirmed.
type deleteSnapshotMutation struct {
	old *resource.State // The resource being deleted
	// Points into the global snapshot, don't use without `mutate`

	manager *SnapshotManager // The snapshot manager that created this mutation
}

func (dsm *deleteSnapshotMutation) End(step deploy.Step) error {
	err := dsm.manager.mutate(func() {
		// Nothing to do here except unlink this resource from the snapshot.
		contract.Assert(dsm.old.Status == resource.ResourceStatusDeleting)
		dsm.old.Status = resource.ResourceStatusDeleted
	})

	if err != nil {
		glog.V(9).Infof("failed to persist delete snapshot mutation: %v", err)
	}

	return err
}

// doDeleteMutation handles the pre-application state management for resource deletion. The only
// modification that needs to be done before the step is applied is marking the condemned resource
// as "deleting".
func (sm *SnapshotManager) doDeleteMutation(step deploy.Step) (engine.SnapshotMutation, error) {
	contract.Require(step.Op() == deploy.OpDelete || step.Op() == deploy.OpDeleteReplaced,
		"step.Op() == deploy.OpDelete || step.Op() == deploy.OpDeleteReplaced")
	contract.Require(step.Old() != nil, "step.Old() != nil")

	var snapshotOld *resource.State
	err := sm.mutate(func() {
		// What resource are we deleting? Whether or not the resource is live depends on the deletion
		// operation that we are doing.
		//
		// If our operation is a simple OpDelete, then we don't have much to do here; the resource being
		// deleted is most definitely the only live resource with the given URN. If we are doing an OpDeleteReplaced,
		// we must determine which resource we are going to delete, which depends on whether or not we
		// are deleting-before-creating or creating-before-deleting.
		//
		// If we are creating-before-deleting, then there must be a resource in the new snapshot with the
		// "pending-delete" status.
		snapshotOld = sm.findCondemnedInSnapshot(step.URN())
		if snapshotOld == nil {
			// Is there one in the base snapshot?
			snapshotOld = sm.findCondemnedInBaseSnapshot(step.URN())
			if snapshotOld != nil {
				snapshotOld = snapshotOld.Clone()
				sm.addResource(snapshotOld)
			}
		}

		if snapshotOld == nil {
			// If there is not such a resource, we are either doing an OpDelete (in which case we are not
			// replacing anything) or a delete-before-create (in which case the thing that we are deleting is
			// still live).
			old := sm.findLiveInBaseSnapshot(step.URN())
			contract.Assertf(old != nil, "failed to find condemned node in base snapshot")

			// Either way, we need to insert a node into the new snapshot indicating that we are beginning
			// a delete operation on a URN that is not present yet in the new snapshot.
			snapshotOld = old.Clone()
			sm.addResource(snapshotOld)
		}

		snapshotOld.Status = resource.ResourceStatusDeleting
	})

	if err != nil {
		glog.V(9).Infof("begin state for URN `%s`, persistence failed: %s", step.URN(), err)
		return nil, err
	}

	glog.V(9).Infof("begin state persistence for step `Delete` on URN `%s` successful", step.URN())
	return &deleteSnapshotMutation{old: snapshotOld, manager: sm}, nil
}

// Updates the snapshot manifest. Can only be called from within a `mutate` callback.
func (sm *SnapshotManager) updateManifest() {
	sm.newSnapshot.Manifest.Time = time.Now()
	sm.newSnapshot.Manifest.Version = version.Version
	sm.newSnapshot.Manifest.Magic = sm.newSnapshot.Manifest.NewMagic()
}

// Adds a resource to the new snapshot. Can only be called from within a `mutate` callback.
func (sm *SnapshotManager) addResource(resource *resource.State) {
	sm.newSnapshot.Resources = append(sm.newSnapshot.Resources, resource)
}

// findLiveInBaseSnapshot finds the live resource with the given URN in the *base* snapshot,
// returning nil if one could not be found. The base snapshot is not mutable and it is safe
// to use this method from outside `mutate` callbacks.
func (sm *SnapshotManager) findLiveInBaseSnapshot(urn resource.URN) *resource.State {
	return findInSnapshot(sm.baseSnapshot, urn, func(candidate *resource.State) bool {
		return candidate.Status.Live()
	})
}

// Finds the resource object in the snapshot corresponding to the given URN, returning
// a pointer to it if found. Can only be called from within a `mutate` callback.
func (sm *SnapshotManager) findLiveInSnapshot(urn resource.URN) *resource.State {
	return findInSnapshot(sm.newSnapshot, urn, func(candidate *resource.State) bool {
		return candidate.Status.Live()
	})
}

// Finds a resource object that matches the given predicate function in a given snapshot.
func findInSnapshot(snap *deploy.Snapshot, urn resource.URN, findFunc func(*resource.State) bool) *resource.State {
	contract.Require(urn != "", "urn != \"\"")
	for _, candidate := range snap.Resources {
		if candidate.URN == urn && findFunc(candidate) {
			return candidate
		}
	}

	return nil
}

// Finds a resource with the given URN that is pending deletion in the new snapshot, returning nil
// if no such resource exists. Can only be called from within a `mutate` callback.
func (sm *SnapshotManager) findCondemnedInSnapshot(urn resource.URN) *resource.State {
	return findInSnapshot(sm.newSnapshot, urn, func(candidate *resource.State) bool {
		return candidate.Status.Condemned()
	})
}

// Finds a resource with the given URN that is pending deletion in the base snapshot, returning
// nil if no such resource exists. Safe to call outside a `mutate` callback.
func (sm *SnapshotManager) findCondemnedInBaseSnapshot(urn resource.URN) *resource.State {
	return findInSnapshot(sm.baseSnapshot, urn, func(candidate *resource.State) bool {
		return candidate.Status.Condemned()
	})
}

// NewSnapshotManager creates a new SnapshotManager given a persister and an original snapshot to use as
// the baseline for further snapshot mutations. The new SnapshotManager makes a copy of the passed-in
// snapshot and does not mutate it.
func NewSnapshotManager(persister SnapshotPersister, update engine.UpdateInfo) *SnapshotManager {
	contract.Require(persister != nil, "persister != nil")
	contract.Require(update != nil, "update != nil")

	target := update.GetTarget()
	snap := target.Snapshot
	if snap == nil {
		manifest := deploy.Manifest{
			Time:    time.Now(),
			Version: version.Version,
			Plugins: nil,
		}

		snap = deploy.NewSnapshot(target.Name, manifest, nil)
	}

	manager := &SnapshotManager{
		persister:        persister,
		baseSnapshot:     snap.Clone(),
		newSnapshot:      deploy.NewSnapshot(snap.Stack, snap.Manifest, nil),
		doVerify:         true,
		mutationRequests: make(chan func()),
	}

	go manager.doMutations()
	return manager
}
