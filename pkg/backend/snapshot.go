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
// mutations on the global snapshot object serially.
//
// All public members of SnapshotManager (namely `BeginMutation`, `RegisterResourceOutputs`, `RecordPlugins`,
// and all `End` functions returned by implementations of `SnapshotMutation`) are safe to use
// unsynchronized across goroutines.
//
// In this implementation of SnapshotManager, public members of SnapshotManager and `mutate` are
// the *only* methods that are safe to use without synchronization. `mutate` is the single synchronization
// primitive that should be used within this file; it wraps a mutating function with logic that serializes
// writes to the global snapshot, updates the manifest, serializes it to persistent storage, and optionally
// verifies the invariants of the global snapshot before and after mutation.
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
	snapshot         *deploy.Snapshot  // The snapshot itself, managed by this struct
	doVerify         bool              // If true, call `snapshot.VerifyIntegrity` before and after modifying the snapshot
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
		if sm.doVerify {
			if err := sm.snapshot.VerifyIntegrity(); err != nil {
				responseChan <- errors.Wrapf(err, "before mutation of snapshot")
				return
			}
		}

		mutator()
		sm.updateManifest()
		if sm.doVerify {
			if verifyErr := sm.snapshot.VerifyIntegrity(); verifyErr != nil {
				responseChan <- errors.Wrapf(verifyErr, "after mutation of snapshot")
				return
			}
		}

		responseChan <- sm.persister.SaveSnapshot(sm.snapshot)
	}

	return <-responseChan
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
		pluginsSlice := sm.snapshot.Manifest.Plugins[:0]
		sm.snapshot.Manifest.Plugins = append(pluginsSlice, plugins...)
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
	same *resource.State // The resource that is staying the same.
	// Points to the global snapshot, don't use without `mutate`

	manager *SnapshotManager
}

func (ssm *sameSnapshotMutation) End(step deploy.Step) error {
	err := ssm.manager.mutate(func() {
		step.New().CopyIntoWithoutMetadata(ssm.same)
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
	var sameResource *resource.State
	err := sm.mutate(func() {
		sameResource = sm.findLiveInSnapshot(step.URN())
	})

	if err != nil {
		glog.V(9).Infof("begin same mutation for URN `%s`, persistence failed: %s", step.URN(), err)
		return nil, err
	}

	return &sameSnapshotMutation{same: sameResource, manager: sm}, nil
}

// createSnapshotMutation is a SnapshotMutation for a CreateStep.
type createSnapshotMutation struct {
	new *resource.State // The resource created by this CreateStep.
	// Points into the global snapshot, don't use without `mutate`!

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
			old := csm.manager.findLiveInSnapshot(step.URN())
			contract.Assertf(old != nil, "failed to find step.URN() in snapshot")
			old.Status = resource.ResourceStatusPendingDeletion
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
		sm.snapshot.Resources = append(sm.snapshot.Resources, snapshotResource)
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
	old *resource.State // The resource being updated by this UpdateStep.
	// Points into the global snapshot, don't use without `mutate`!

	manager *SnapshotManager // The snapshot manager that created this mutation
}

func (usm *updateSnapshotMutation) End(step deploy.Step) error {
	err := usm.manager.mutate(func() {
		contract.Assert(usm.old.Status == resource.ResourceStatusUpdating)
		step.New().CopyIntoWithoutMetadata(usm.old)
		usm.old.Status = resource.ResourceStatusUpdated
		usm.old.UpdatedAt = time.Now()
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

	// Instead of creating a new resource for the updated resource, we'll clobber the existing
	// resource with the new resource's inputs. This does mean that the original inputs to the resource
	// are lost, but it's not clear whether or not they are of any value to us.
	//
	// The modified resource's status is set to "updating".
	var snapshotOld *resource.State
	err := sm.mutate(func() {
		snapshotOld = sm.findLiveInSnapshot(step.URN())
		contract.Assertf(snapshotOld != nil, "failed to find live state for resource `%s` in snapshot", step.Old().URN)
		step.New().CopyIntoWithoutMetadata(snapshotOld)
		snapshotOld.Status = resource.ResourceStatusUpdating
	})

	if err != nil {
		glog.V(9).Infof("begin state for URN `%s`, persistence failed: %s", step.URN(), err)
		return nil, err
	}

	return &updateSnapshotMutation{old: snapshotOld, manager: sm}, nil
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
		dsm.manager.removeFromSnapshot(dsm.old)
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
		// our job is significantly harder; we must determine which resource we are going to delete, which depends
		// on whether or not we are deleting-before-creating or creating-before-deleting.
		if step.Op() == deploy.OpDelete {
			snapshotOld = sm.findLiveInSnapshot(step.URN())
		} else {
			// If a CreateReplacement step ran before this, it should have marked a resource as pending-delete
			// in the snapshot. Try and find it.
			snapshotOld = sm.findPendingDeleteInSnapshot(step.URN())
			if snapshotOld == nil {
				// If there's no pending delete snapshot, the condemned resource must still be live.
				snapshotOld = sm.findLiveInSnapshot(step.URN())
			}
		}

		contract.Assert(snapshotOld != nil)
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
	sm.snapshot.Manifest.Time = time.Now()
	sm.snapshot.Manifest.Version = version.Version
	sm.snapshot.Manifest.Magic = sm.snapshot.Manifest.NewMagic()
}

// Finds the resource object in the snapshot corresponding to the given URN, returning
// a pointer to it if found. Can only be called from within a `mutate` callback.
func (sm *SnapshotManager) findLiveInSnapshot(urn resource.URN) *resource.State {
	return sm.findInSnapshot(urn, func(candidate *resource.State) bool {
		return candidate.Status.Live()
	})
}

func (sm *SnapshotManager) findInSnapshot(urn resource.URN, findFunc func(*resource.State) bool) *resource.State {
	contract.Require(urn != "", "urn != \"\"")
	for _, candidate := range sm.snapshot.Resources {
		if candidate.URN == urn && findFunc(candidate) {
			return candidate
		}
	}

	return nil
}

func (sm *SnapshotManager) findPendingDeleteInSnapshot(urn resource.URN) *resource.State {
	return sm.findInSnapshot(urn, func(candidate *resource.State) bool {
		return candidate.Status == resource.ResourceStatusPendingDeletion
	})
}

// Removes a resource from the snapshot. Can only be called from within a `mutate` callback.
func (sm *SnapshotManager) removeFromSnapshot(res *resource.State) {
	contract.Require(res != nil, "res != nil")
	newResources := sm.snapshot.Resources[:0]
	removedSomething := false
	for _, candidate := range sm.snapshot.Resources {
		if candidate.URN == res.URN && candidate.Status == res.Status {
			contract.Assertf(!removedSomething,
				"attempting to remove multiple states from snapshot file: URN `%s`, state `%s`", res.URN, res.Status)
			glog.V(9).Infof("removing URN `%s`, state `%s` from checkpoint file", res.URN, res.Status)
			removedSomething = true
		} else {
			newResources = append(newResources, candidate)
		}
	}

	sm.snapshot.Resources = newResources
	contract.Assertf(removedSomething,
		"failed to locate URN `%s` with state `%s` in checkpoint file for removal", res.URN, res.Status)
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
		snapshot:         snap.Clone(),
		doVerify:         true,
		mutationRequests: make(chan func()),
	}

	go manager.doMutations()
	return manager
}
