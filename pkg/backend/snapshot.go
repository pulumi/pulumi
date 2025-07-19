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

package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/exp/slices"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
)

// DisableIntegrityChecking can be set to true to disable checkpoint state integrity verification.  This is not
// recommended, because it could mean proceeding even in the face of a corrupted checkpoint state file, but can
// be used as a last resort when a command absolutely must be run.
var DisableIntegrityChecking bool

// SnapshotPersister is an interface implemented by our backends that implements snapshot
// persistence. In order to fit into our current model, snapshot persisters have two functions:
// saving snapshots and invalidating already-persisted snapshots.
type SnapshotPersister interface {
	// Persists the given snapshot. Returns an error if the persistence failed.
	Save(snapshot *deploy.Snapshot) error
}

type JournalPersister interface {
	SaveJournalEntry(JournalEntry, secrets.Manager) error
}

// SnapshotManager is an implementation of engine.SnapshotManager that inspects steps and performs
// mutations on the global snapshot object serially. This implementation maintains two bits of state: the "base"
// snapshot, which is completely immutable and represents the state of the world prior to the application
// of the current plan, and a "new" list of resources, which consists of the resources that were operated upon
// by the current plan.
//
// The resources stored in the `resources` slice are pointers to resource objects allocated by the engine.
// This is subtle and a little confusing. The reason for this is that the engine directly mutates resource objects
// that it creates and expects those mutations to be persisted directly to the snapshot.
type SnapshotManager struct {
	journal      Journal          // The journal used to record operations performed by this plan
	baseSnapshot *deploy.Snapshot // The base snapshot for this plan

	// newResources is a map of resources that have been added to the snapshot in this plan, keyed by the resource
	// state.  This is used to track the added resources, in case they are deleted later.
	newResources gsync.Map[*resource.State, uint64]

	operationIDCounter atomic.Uint64 // A counter used to generate unique operation IDs for journal entries.
}

var _ engine.SnapshotManager = (*SnapshotManager)(nil)

func (sm *SnapshotManager) Close() error {
	sm.journal.Close()
	return nil
}

type JournalEntryKind int

const (
	JournalEntryBegin          JournalEntryKind = 0
	JournalEntrySuccess        JournalEntryKind = 1
	JournalEntryFailure        JournalEntryKind = 2
	JournalEntryRefreshSuccess JournalEntryKind = 3
	JournalEntryOutputs        JournalEntryKind = 4
	JournalEntryRebase         JournalEntryKind = 5
)

type OperationID int

type JournalEntry struct {
	Kind               JournalEntryKind
	OperationID        uint64 // The ID of the operation that this journal entry is associated with.
	DeleteOld          int
	DeleteNew          uint64 // ID for the delete Operation
	PendingReplacement int
	State              *resource.State     // The resource state associated with this journal entry.
	Operation          *resource.Operation // The operation associated with this journal entry, if any.
	RefreshDeleteURN   resource.URN        // The URN of the resource that was deleted by a refresh operation.
	// If true, this journal entry can be elided and does not need to be written immediately.
	ElideWrite bool
}

func (je JournalEntry) Serialize(ctx context.Context, enc config.Encrypter) (apitype.JournalEntry, error) {
	var state *apitype.ResourceV3

	if je.State != nil {
		s, err := stack.SerializeResource(ctx, je.State, enc, false)
		if err != nil {
			return apitype.JournalEntry{}, fmt.Errorf("serializing resource state: %w", err)
		}
		state = &s
	}

	var operation *apitype.OperationV2
	if je.Operation != nil {
		op, err := stack.SerializeOperation(ctx, *je.Operation, enc, false)
		if err != nil {
			return apitype.JournalEntry{}, fmt.Errorf("serializing operation: %w", err)
		}
		operation = &op
	}

	serializedEntry := apitype.JournalEntry{
		Kind:             apitype.JournalEntryKind(je.Kind),
		OperationID:      je.OperationID,
		DeleteOld:        je.DeleteOld,
		DeleteNew:        je.DeleteNew,
		State:            state,
		Operation:        operation,
		RefreshDeleteURN: string(je.RefreshDeleteURN),
	}

	return serializedEntry, nil
}

type JournalEntries []JournalEntry

// If you need to understand what's going on in this file, start here!
//
// The snapshot code works on journal entries. Each resource step produces new journal entries
// for beginning and finishing an operation. These journal entries can then be replayed
// in conjunction with the immutable base snapshot, to rebuild the new snapshot.
//
// Currently the backend only supports saving full snapshots, in which case only one journal
// entry is allowed to be processed at a time. In the future journal entries will be processed
// asynchronously in the cloud backend, allowing for better throughput for independent operations..
//
// Serialization is performed by pushing the mutator function onto a channel, where another
// goroutine is polling the channel and executing the mutation functions as they come.
// This function optionally verifies the integrity of the snapshot before and after mutation.
//
// Each journal entry may indicate that its corresponding checkpoint write may be safely elided by
// setting the `ElideWrite` fiield. As of this writing, we only elide writes after same steps with no
// meaningful changes (see sameSnapshotMutation.mustWrite for details). Any elided writes
// are flushed by the next non-elided write or the next call to Close.
//
// You should never observe or mutate the global snapshot without using this function unless
// you have a very good justification.
// func (sm *SnapshotManager) journalMutation(journalEntry JournalEntry) error {
// 	result := make(chan error)
// 	journalEntry.result = result
// 	select {
// 	case sm.journalEvents <- journalEntry:
// 		return <-result
// 	case <-sm.cancel:
// 		return errors.New("snapshot manager closed")
// 	}
// }

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
	return sm.journal.EndOperation(JournalEntry{
		Kind: JournalEntryOutputs,
		// If the outputs have not changed, we do not need to write this journal entry.
		ElideWrite: step.Old() != nil && step.New() != nil && step.Old().Outputs.DeepEquals(step.New().Outputs),
	})
}

func (sm *SnapshotManager) markEntryForDeletion(journalEntry *JournalEntry, toDelete *resource.State) {
	contract.Assertf(journalEntry.DeleteOld == -1, "journalEntry.DeleteOld must be initialized to -1")
	if sm.baseSnapshot != nil {
		for i, res := range sm.baseSnapshot.Resources {
			if res == toDelete {
				journalEntry.DeleteOld = i
				return
			}
		}
	}
	sm.newResources.Range(func(res *resource.State, id uint64) bool {
		if res == toDelete {
			journalEntry.DeleteNew = id
			return false
		}
		return true
	})
}

// BeginMutation signals to the SnapshotManager that the engine intends to mutate the global snapshot
// by performing the given Step. This function gives the SnapshotManager a chance to record the
// intent to mutate before the mutation occurs.
func (sm *SnapshotManager) BeginMutation(step deploy.Step) (engine.SnapshotMutation, error) {
	contract.Requiref(step != nil, "step", "cannot be nil")
	logging.V(9).Infof("SnapshotManager: Beginning mutation for step `%s` on resource `%s`", step.Op(), step.URN())

	operationID := sm.operationIDCounter.Add(1)

	fmt.Println("SnapshotManager: BeginMutation for step", step.Op(), "on resource", step.URN(), "with operation ID", operationID)

	switch step.Op() {
	case deploy.OpSame:
		return sm.doSame(step, operationID)
	case deploy.OpCreate, deploy.OpCreateReplacement:
		return sm.doCreate(step, operationID)
	case deploy.OpUpdate:
		return sm.doUpdate(step, operationID)
	case deploy.OpDelete, deploy.OpDeleteReplaced, deploy.OpReadDiscard, deploy.OpDiscardReplaced:
		return sm.doDelete(step, operationID)
	case deploy.OpReplace:
		return sm.doReplace(step, operationID)
	case deploy.OpRead, deploy.OpReadReplacement:
		return sm.doRead(step, operationID)
	case deploy.OpRefresh:
		return sm.doRefresh(step, operationID)
	case deploy.OpRemovePendingReplace:
		return sm.doRemovePendingReplace(step, operationID)
	case deploy.OpImport, deploy.OpImportReplacement:
		return sm.doImport(step, operationID)
	}

	contract.Failf("unknown StepOp: %s", step.Op())
	return nil, nil
}

func (sm *SnapshotManager) Rebase(base *deploy.Snapshot) error {
	if sm == nil {
		return nil
	}
	sm.baseSnapshot = base
	return sm.journal.Rebase(base)
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
	manager     *SnapshotManager
	operationID uint64
}

// mustWrite returns true if any semantically meaningful difference exists between the old and new states of a same
// step that forces us to write the checkpoint. If no such difference exists, the checkpoint write that corresponds to
// this step can be elided.
func (ssm *sameSnapshotMutation) mustWrite(step deploy.Step) bool {
	old := step.Old()
	new := step.New()

	contract.Assertf(old.Delete == new.Delete,
		"either both or neither resource must be pending deletion, got %v (old) != %v (new)",
		old.Delete, new.Delete)
	contract.Assertf(old.External == new.External,
		"either both or neither resource must be external, got %v (old) != %v (new)",
		old.External, new.External)

	if sameStep, isSameStep := step.(*deploy.SameStep); isSameStep {
		contract.Assertf(!sameStep.IsSkippedCreate(), "create cannot be skipped for SameStep")
	}

	// If the URN of this resource has changed, we must write the checkpoint. This should only be possible when a
	// resource is aliased.
	if old.URN != new.URN {
		logging.V(9).Infof("SnapshotManager: mustWrite() true because of URN")
		return true
	}

	// If the type of this resource has changed, we must write the checkpoint. This should only be possible when a
	// resource is aliased.
	if old.Type != new.Type {
		logging.V(9).Infof("SnapshotManager: mustWrite() true because of Type")
		return true
	}

	// If the kind of this resource has changed, we must write the checkpoint.
	if old.Custom != new.Custom {
		logging.V(9).Infof("SnapshotManager: mustWrite() true because of Custom")
		return true
	}

	// We need to persist the changes if CustomTimes have changed
	if old.CustomTimeouts != new.CustomTimeouts {
		logging.V(9).Infof("SnapshotManager: mustWrite() true because of CustomTimeouts")
		return true
	}

	// We need to persist the changes if CustomTimes have changed
	if old.RetainOnDelete != new.RetainOnDelete {
		logging.V(9).Infof("SnapshotManager: mustWrite() true because of RetainOnDelete")
		return true
	}

	contract.Assertf(old.ID == new.ID,
		"old and new resource IDs must be equal, got %v (old) != %v (new)", old.ID, new.ID)

	// If this resource's provider has changed, we must write the checkpoint. This can happen in scenarios involving
	// aliased providers or upgrades to default providers.
	if old.Provider != new.Provider {
		logging.V(9).Infof("SnapshotManager: mustWrite() true because of Provider")
		return true
	}

	// If this resource's parent has changed, we must write the checkpoint.
	if old.Parent != new.Parent {
		logging.V(9).Infof("SnapshotManager: mustWrite() true because of Parent")
		return true
	}

	// If the DeletedWith attribute of this resource has changed, we must write the checkpoint.
	if old.DeletedWith != new.DeletedWith {
		logging.V(9).Infof("SnapshotManager: mustWrite() true because of DeletedWith")
		return true
	}

	// If the protection attribute of this resource has changed, we must write the checkpoint.
	if old.Protect != new.Protect {
		logging.V(9).Infof("SnapshotManager: mustWrite() true because of Protect")
		return true
	}

	// If the inputs or outputs of this resource have changed, we must write the checkpoint. Note that it is possible
	// for the inputs of a "same" resource to have changed even if the contents of the input bags are different if the
	// resource's provider deems the physical change to be semantically irrelevant.
	if !old.Inputs.DeepEquals(new.Inputs) {
		logging.V(9).Infof("SnapshotManager: mustWrite() true because of Inputs")
		return true
	}
	if !old.Outputs.DeepEquals(new.Outputs) {
		logging.V(9).Infof("SnapshotManager: mustWrite() true because of Outputs")
		return true
	}

	// reflect.DeepEqual does not treat `nil` and `[]URN{}` as equal, so we must check for both
	// lists being empty ourselves.
	if len(old.Dependencies) != 0 || len(new.Dependencies) != 0 {
		// Sort dependencies before comparing them. If the dependencies have changed, we must write the checkpoint.
		sortDeps := func(deps []resource.URN) []resource.URN {
			result := make([]resource.URN, len(deps))
			copy(result, deps)
			slices.Sort(result)
			return result
		}
		oldDeps := sortDeps(old.Dependencies)
		newDeps := sortDeps(new.Dependencies)
		if !reflect.DeepEqual(oldDeps, newDeps) {
			logging.V(9).Infof("SnapshotManager: mustWrite() true because of Dependencies")
			return true
		}
	}

	if !reflect.DeepEqual(old.ResourceHooks, new.ResourceHooks) {
		logging.V(9).Infof("SnapshotManager: mustWrite() true because of ResourceHooks")
		return true
	}

	// Init errors are strictly advisory, so we do not consider them when deciding whether or not to write the
	// checkpoint. Likewise source positions are purely metadata and do not affect the system correctness, so
	// for performance we elide those as well. This prevents _every_ resource needing a snapshot write when
	// making large source code changes.

	logging.V(9).Infof("SnapshotManager: mustWrite() false")
	return false
}

func (sm *SnapshotManager) doSame(step deploy.Step, operationID uint64) (engine.SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doSame(%s)", step.URN())
	err := sm.journal.BeginOperation(JournalEntry{
		Kind:        JournalEntryBegin,
		OperationID: operationID,
		ElideWrite:  true,
		DeleteOld:   -1, // Default to -1, which means no deletion.
	})
	if err != nil {
		return nil, err
	}
	return &sameSnapshotMutation{sm, operationID}, nil
}

func (ssm *sameSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	contract.Requiref(step.Op() == deploy.OpSame, "step.Op()", "must be %q, got %q", deploy.OpSame, step.Op())
	logging.V(9).Infof("SnapshotManager: sameSnapshotMutation.End(..., %v)", successful)

	kind := JournalEntrySuccess
	if !successful {
		kind = JournalEntryFailure
	}
	journalEntry := JournalEntry{
		Kind:               kind,
		OperationID:        ssm.operationID,
		DeleteOld:          -1, // Default to -1, which means no deletion.
		PendingReplacement: -1, // Default to -1, which means no pending replacement.
	}
	if old := step.Old(); old != nil {
		ssm.manager.markEntryForDeletion(&journalEntry, step.Old())
	}

	sameStep, isSameStep := step.(*deploy.SameStep)
	if !isSameStep || !sameStep.IsSkippedCreate() {
		journalEntry.State = step.New()
		ssm.manager.newResources.Store(step.New(), ssm.operationID)
	}

	if successful && isSameStep && (sameStep.IsSkippedCreate() || !ssm.mustWrite(sameStep)) {
		journalEntry.ElideWrite = true
	}

	return ssm.manager.journal.EndOperation(journalEntry)
}

func (sm *SnapshotManager) doCreate(step deploy.Step, operationID uint64) (engine.SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doCreate(%s)", step.URN())
	op := resource.NewOperation(step.New(), resource.OperationTypeCreating)
	err := sm.journal.BeginOperation(JournalEntry{
		Kind:        JournalEntryBegin,
		OperationID: operationID,
		Operation:   &op,
		DeleteOld:   -1, // Default to -1, which means no deletion.
	})
	if err != nil {
		return nil, err
	}

	if step.Old() != nil {
		op = resource.NewOperation(step.Old(), resource.OperationTypeDeleting)
		journalEntry := JournalEntry{
			Kind:        JournalEntryBegin,
			OperationID: operationID,
			Operation:   &op,
			DeleteOld:   -1, // Default to -1, which means no deletion.
		}
		sm.markEntryForDeletion(&journalEntry, step.Old())
		err = sm.journal.BeginOperation(journalEntry)
		if err != nil {
			return nil, err
		}
	}

	return &createSnapshotMutation{sm, operationID}, nil
}

type createSnapshotMutation struct {
	manager     *SnapshotManager
	operationID uint64
}

func (csm *createSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	logging.V(9).Infof("SnapshotManager: createSnapshotMutation.End(..., %v)", successful)
	kind := JournalEntrySuccess
	if !successful {
		kind = JournalEntryFailure
	}
	journalEntry := JournalEntry{
		Kind:               kind,
		OperationID:        csm.operationID,
		State:              step.New(),
		DeleteOld:          -1, // Default to -1, which means no deletion.
		PendingReplacement: -1, // Default to -1, which means no pending replacement.
	}
	csm.manager.newResources.Store(step.New(), csm.operationID)
	if old := step.Old(); old != nil && old.PendingReplacement {
		csm.manager.markEntryForDeletion(&journalEntry, step.Old())
	}

	return csm.manager.journal.EndOperation(journalEntry)
}

func (sm *SnapshotManager) doUpdate(step deploy.Step, operationID uint64) (engine.SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doUpdate(%s)", step.URN())
	op := resource.NewOperation(step.New(), resource.OperationTypeUpdating)
	err := sm.journal.BeginOperation(JournalEntry{
		Kind:        JournalEntryBegin,
		OperationID: operationID,
		Operation:   &op,
		DeleteOld:   -1, // Default to -1, which means no deletion.
	})
	if err != nil {
		return nil, err
	}

	return &updateSnapshotMutation{sm, operationID}, nil
}

type updateSnapshotMutation struct {
	manager     *SnapshotManager
	operationID uint64
}

func (usm *updateSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	logging.V(9).Infof("SnapshotManager: updateSnapshotMutation.End(..., %v)", successful)
	kind := JournalEntrySuccess
	if !successful {
		kind = JournalEntryFailure
	}
	journalEntry := JournalEntry{
		Kind:               kind,
		OperationID:        usm.operationID,
		DeleteOld:          -1, // Default to -1, which means no deletion.
		PendingReplacement: -1, // Default to -1, which means no pending replacement.
	}
	if successful {
		if old := step.Old(); old != nil {
			usm.manager.markEntryForDeletion(&journalEntry, step.Old())
		}
		journalEntry.State = step.New()
		usm.manager.newResources.Store(step.New(), usm.operationID)
	}
	return usm.manager.journal.EndOperation(journalEntry)
}

func (sm *SnapshotManager) doDelete(step deploy.Step, operationID uint64) (engine.SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doDelete(%s)", step.URN())
	op := resource.NewOperation(step.Old(), resource.OperationTypeDeleting)
	journalEntry := JournalEntry{
		Kind:        JournalEntryBegin,
		DeleteOld:   -1, // Default to -1, which means no deletion.
		OperationID: operationID,
		Operation:   &op,
	}

	err := sm.journal.BeginOperation(journalEntry)
	if err != nil {
		return nil, err
	}
	return &deleteSnapshotMutation{sm, operationID}, nil
}

type deleteSnapshotMutation struct {
	manager     *SnapshotManager
	operationID uint64
}

func (dsm *deleteSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	logging.V(9).Infof("SnapshotManager: deleteSnapshotMutation.End(..., %v)", successful)
	kind := JournalEntrySuccess
	if !successful {
		kind = JournalEntryFailure
	}
	journalEntry := JournalEntry{
		Kind:               kind,
		OperationID:        dsm.operationID,
		DeleteOld:          -1, // Default to -1, which means no deletion.
		PendingReplacement: -1, // Default to -1, which means no pending replacement.
	}
	if successful {
		contract.Assertf(
			!step.Old().Protect ||
				step.Op() == deploy.OpDiscardReplaced ||
				step.Op() == deploy.OpDeleteReplaced,
			"Old must be unprotected (got %v) or the operation must be a replace (got %q)",
			step.Old().Protect, step.Op())

		if step.Old().PendingReplacement {
			if dsm.manager.baseSnapshot != nil {
				for i, res := range dsm.manager.baseSnapshot.Resources {
					if res == step.Old() {
						journalEntry.PendingReplacement = i
						break
					}
				}
			}
		}

		if !step.Old().PendingReplacement {
			dsm.manager.markEntryForDeletion(&journalEntry, step.Old())
		}
	}
	return dsm.manager.journal.EndOperation(journalEntry)
}

func (sm *SnapshotManager) doReplace(step deploy.Step, operationID uint64) (engine.SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doReplace(%s)", step.URN())
	return &replaceSnapshotMutation{sm, operationID}, nil
}

type replaceSnapshotMutation struct {
	manager     *SnapshotManager
	operationID uint64
}

func (rsm *replaceSnapshotMutation) End(step deploy.Step, successful bool) error {
	logging.V(9).Infof("SnapshotManager: replaceSnapshotMutation.End(..., %v)", successful)
	return nil
}

func (sm *SnapshotManager) doRead(step deploy.Step, operationID uint64) (engine.SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doRead(%s)", step.URN())
	op := resource.NewOperation(step.New(), resource.OperationTypeReading)
	journalEntry := JournalEntry{
		Kind:        JournalEntryBegin,
		OperationID: operationID,
		Operation:   &op,
		DeleteOld:   -1, // Default to -1, which means no deletion.
	}
	err := sm.journal.BeginOperation(journalEntry)
	if err != nil {
		return nil, err
	}
	return &readSnapshotMutation{sm, operationID}, nil
}

type readSnapshotMutation struct {
	manager     *SnapshotManager
	operationID uint64
}

func (rsm *readSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	logging.V(9).Infof("SnapshotManager: readSnapshotMutation.End(..., %v)", successful)
	kind := JournalEntrySuccess
	if !successful {
		kind = JournalEntryFailure
	}
	journalEntry := JournalEntry{
		Kind:               kind,
		OperationID:        rsm.operationID,
		State:              step.New(),
		DeleteOld:          -1, // Default to -1, which means no deletion.
		PendingReplacement: -1, // Default to -1, which means no pending replacement.
	}
	rsm.manager.newResources.Store(step.New(), rsm.operationID)
	if old := step.Old(); old != nil && rsm.manager.baseSnapshot != nil {
		rsm.manager.markEntryForDeletion(&journalEntry, step.Old())
	}
	return rsm.manager.journal.EndOperation(journalEntry)
}

func (sm *SnapshotManager) doRefresh(step deploy.Step, operationID uint64) (engine.SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doRefresh(%s)", step.URN())
	journalEntry := JournalEntry{
		Kind:        JournalEntryBegin,
		OperationID: operationID,
		State:       step.New(),
		DeleteOld:   -1, // Default to -1, which means no deletion.
	}

	err := sm.journal.BeginOperation(journalEntry)
	if err != nil {
		return nil, err
	}
	return &refreshSnapshotMutation{sm, operationID}, nil
}

type refreshSnapshotMutation struct {
	manager     *SnapshotManager
	operationID uint64
}

func (rsm *refreshSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	contract.Requiref(step.Op() == deploy.OpRefresh, "step.Op", "must be %q, got %q", deploy.OpRefresh, step.Op())
	logging.V(9).Infof("SnapshotManager: refreshSnapshotMutation.End(..., %v)", successful)
	kind := JournalEntryRefreshSuccess
	if !successful {
		kind = JournalEntryFailure
	}
	journalEntry := JournalEntry{
		Kind:        kind,
		OperationID: rsm.operationID,
		DeleteOld:   -1, // Default to -1, which means no deletion.
	}

	refreshStep, isRefreshStep := step.(*deploy.RefreshStep)
	viewStep, isViewStep := step.(*deploy.ViewStep)
	// Only do this for persistent steps?? I think it needs to be gdone for all steps now though.
	if isRefreshStep || isViewStep {
		if step.New() != nil {
			journalEntry.State = step.New()
			rsm.manager.newResources.Store(step.New(), rsm.operationID)
		}
	} else {
		journalEntry.ElideWrite = true
	}

	if (isRefreshStep && refreshStep.Persisted()) || (isViewStep && viewStep.Persisted()) {
		if old := step.Old(); old != nil {
			journalEntry.RefreshDeleteURN = old.URN
			rsm.manager.markEntryForDeletion(&journalEntry, old)
		}
		journalEntry.Kind = JournalEntrySuccess
	} else {
		if old := step.Old(); old != nil {
			journalEntry.RefreshDeleteURN = old.URN
		}
	}

	return rsm.manager.journal.EndOperation(journalEntry)
}

func (sm *SnapshotManager) doRemovePendingReplace(
	step deploy.Step, operationID uint64,
) (engine.SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doRemovePendingReplace(%s)", step.URN())
	err := sm.journal.BeginOperation(JournalEntry{
		Kind:        JournalEntryBegin,
		OperationID: operationID,
		State:       step.Old(),
		DeleteOld:   -1, // Default to -1, which means no deletion.
	})
	return &removePendingReplaceSnapshotMutation{sm, operationID}, err
}

type removePendingReplaceSnapshotMutation struct {
	manager     *SnapshotManager
	operationID uint64
}

func (rsm *removePendingReplaceSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	contract.Requiref(step.Op() == deploy.OpRemovePendingReplace, "step.Op",
		"must be %q, got %q", deploy.OpRemovePendingReplace, step.Op())
	journalEntry := JournalEntry{
		Kind:               JournalEntrySuccess,
		OperationID:        rsm.operationID,
		DeleteOld:          -1, // Default to -1, which means no deletion.
		PendingReplacement: -1, // Default to -1, which means no pending replacement.
	}
	if step.Old() != nil {
		rsm.manager.markEntryForDeletion(&journalEntry, step.Old())
	}
	return rsm.manager.journal.EndOperation(journalEntry)
}

func (sm *SnapshotManager) doImport(step deploy.Step, operationID uint64) (engine.SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doImport(%s)", step.URN())
	op := resource.NewOperation(step.New(), resource.OperationTypeImporting)
	journalEntry := JournalEntry{
		Kind:        JournalEntryBegin,
		OperationID: operationID,
		Operation:   &op,
		DeleteOld:   -1, // Default to -1, which means no deletion.
	}
	importStep, isImportStep := step.(*deploy.ImportStep)
	contract.Assertf(isImportStep, "step must be an ImportStep, got %T", step)
	if importStep.Original() != nil {
		// This is a import replacement, so we need to mark the old resource for deletion.
		sm.markEntryForDeletion(&journalEntry, importStep.Original())
	}
	err := sm.journal.BeginOperation(journalEntry)
	if err != nil {
		return nil, err
	}

	return &importSnapshotMutation{sm, operationID}, nil
}

type importSnapshotMutation struct {
	manager     *SnapshotManager
	operationID uint64
}

func (ism *importSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	contract.Requiref(step.Op() == deploy.OpImport || step.Op() == deploy.OpImportReplacement, "step.Op",
		"must be %q or %q, got %q", deploy.OpImport, deploy.OpImportReplacement, step.Op())
	kind := JournalEntrySuccess
	if !successful {
		kind = JournalEntryFailure
	}
	journalEntry := JournalEntry{
		Kind:               kind,
		OperationID:        ism.operationID,
		State:              step.New(),
		DeleteOld:          -1, // Default to -1, which means no deletion.
		PendingReplacement: -1, // Default to -1, which means no pending replacement.
	}
	ism.manager.newResources.Store(step.New(), ism.operationID)
	return ism.manager.journal.EndOperation(journalEntry)
}

// snap produces a new Snapshot given the base snapshot and a list of resources that the current
// plan has created.
func (sj *snapshotJournaler) snap() *deploy.Snapshot {
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
	resources := make([]*resource.State, 0)

	refreshDeletes := make(map[resource.URN]bool)
	toDelete := make(map[uint64]struct{})
	toDeleteInSnapshot := make(map[int]struct{})
	toReplace := make(map[uint64]*resource.State)
	toReplaceInSnapshot := make(map[int]*resource.State)
	markAsDeletion := make(map[int]struct{})
	markAsPendingReplacement := make(map[int]struct{})
	for _, entry := range sj.journalEntries {
		if entry.Kind == JournalEntrySuccess && entry.DeleteNew != 0 {
			toDelete[entry.DeleteNew] = struct{}{}
		}

		if entry.Kind == JournalEntryRefreshSuccess && entry.State != nil {
			// If we have a refresh, and the resource is not being deleted,
			// we want to substitute the old resource, instead of appending
			// it to the end.
			if entry.DeleteNew != 0 {
				toReplace[entry.DeleteNew] = entry.State
			} else if entry.DeleteOld != -1 {
				toReplaceInSnapshot[entry.DeleteOld] = entry.State
			}
		}

		if entry.Kind == JournalEntryBegin {
			if entry.DeleteOld != -1 {
				markAsDeletion[entry.DeleteOld] = struct{}{}
			}
		}
	}

	incompleteOps := make(map[uint64]JournalEntry)

	// Record any pending operations, if there are any outstanding that have not completed yet.
	for _, entry := range sj.journalEntries {
		switch entry.Kind {
		case JournalEntryBegin:
			incompleteOps[entry.OperationID] = entry
		case JournalEntrySuccess:
			delete(incompleteOps, entry.OperationID)
			// If this is a success, we need to add the resource to the list of resources.
			_, del := toDelete[entry.OperationID]
			state, replace := toReplace[entry.OperationID]
			if replace {
				resources = append(resources, state)
			} else if entry.State != nil && !del {
				resources = append(resources, entry.State)
			}
			if entry.DeleteOld >= 0 {
				toDeleteInSnapshot[entry.DeleteOld] = struct{}{}
			}
			if entry.RefreshDeleteURN != "" {
				refreshDeletes[entry.RefreshDeleteURN] = true
			}
			if entry.PendingReplacement != -1 {
				markAsPendingReplacement[entry.PendingReplacement] = struct{}{}
			}

		case JournalEntryRefreshSuccess:
			delete(incompleteOps, entry.OperationID)

			_, del := toDelete[entry.OperationID]
			if del {
				delete(toDeleteInSnapshot, entry.DeleteOld)
				toDeleteInSnapshot[entry.DeleteOld] = struct{}{}
			}
			// if entry.DeleteOld >= 0 && entry.State != nil {
			// 	toDeleteInSnapshot[entry.DeleteOld] = struct{}{}
			// }
		case JournalEntryFailure:
			delete(incompleteOps, entry.OperationID)
		case JournalEntryOutputs:
			// Nothing to do here, this is just a marker that outputs were registered.
		}
	}

	// Append any resources from the base plan that were not produced by the current plan.
	if sj.snapshot != nil {
		for i, res := range sj.snapshot.Resources {
			if _, ok := toDeleteInSnapshot[i]; !ok {
				if _, ok := markAsPendingReplacement[i]; ok {
					res.PendingReplacement = true
				}

				if state, ok := toReplaceInSnapshot[i]; ok {
					// If this is a resource that was replaced, we want to
					// replace it in the snapshot.  We only do so if the same
					// resource has not been marked for deletion.  This
					// could happen, e.g. if a refresh happens first (so
					// we're supposed to replace the resource), and then a
					// delete happens (so we're supposed to delete the resource).
					resources = append(resources, state)
				} else {
					if _, ok := markAsDeletion[i]; ok {
						res.Delete = true
					}
					resources = append(resources, res)
				}
			}
		}
	}

	engine.FilterRefreshDeletes(refreshDeletes, resources)

	// Record any pending operations, if there are any outstanding that have not completed yet.
	var operations []resource.Operation
	for _, op := range incompleteOps {
		if op.Operation != nil {
			operations = append(operations, *op.Operation)
		}
	}

	// Track pending create operations from the base snapshot
	// and propagate them to the new snapshot: we don't want to clear pending CREATE operations
	// because these must require user intervention to be cleared or resolved.
	if base := sj.snapshot; base != nil {
		for _, pendingOperation := range base.PendingOperations {
			if pendingOperation.Type == resource.OperationTypeCreating {
				operations = append(operations, pendingOperation)
			}
		}
	}

	manifest := deploy.Manifest{
		Time:    time.Now(),
		Version: version.Version,
		// Plugins: sm.plugins, - Explicitly dropped, since we don't use the plugin list in the manifest anymore.
	}

	// The backend.SnapshotManager and backend.SnapshotPersister will keep track of any changes to
	// the Snapshot (checkpoint file) in the HTTP backend. We will reuse the snapshot's secrets manager when possible
	// to ensure that secrets are not re-encrypted on each update.
	secretsManager := sj.secretsManager
	if sj.snapshot != nil && secrets.AreCompatible(secretsManager, sj.snapshot.SecretsManager) {
		secretsManager = sj.snapshot.SecretsManager
	}

	var metadata deploy.SnapshotMetadata
	if sj.snapshot != nil {
		metadata = sj.snapshot.Metadata
	}

	manifest.Magic = manifest.NewMagic()
	return deploy.NewSnapshot(manifest, secretsManager, resources, operations, metadata)
}

// saveSnapshot persists the current snapshot. If integrity checking is enabled,
// the snapshot's integrity is also verified. If the snapshot is invalid,
// metadata about this write operation is added to the snapshot before it is
// written, in order to aid debugging should future operations fail with an
// error.
func (sj *snapshotJournaler) saveSnapshot() error {
	snap, err := sj.snap().NormalizeURNReferences()
	if err != nil {
		return fmt.Errorf("failed to normalize URN references: %w", err)
	}

	// In order to persist metadata about snapshot integrity issues, we check the
	// snapshot's validity *before* we write it. However, should an error occur,
	// we will only raise this *after* the write has completed. In the event that
	// integrity checking is disabled, we still actually perform the check (and
	// write metadata appropriately), but we will not raise the error following a
	// successful write.
	//
	// If the actual write fails for any reason, this error will supersede any
	// integrity error. This matches behaviour prior to when integrity metadata
	// writing was introduced.
	//
	// Metadata will be cleared out by a successful operation (even if integrity
	// checking is being enforced).
	integrityError := snap.VerifyIntegrity()
	if integrityError == nil {
		snap.Metadata.IntegrityErrorMetadata = nil
	} else {
		snap.Metadata.IntegrityErrorMetadata = &deploy.SnapshotIntegrityErrorMetadata{
			Version: version.Version,
			Command: strings.Join(os.Args, " "),
			Error:   integrityError.Error(),
		}
	}
	persister := sj.persister
	if err := persister.Save(snap); err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}
	if !DisableIntegrityChecking && integrityError != nil {
		return fmt.Errorf("failed to verify snapshot: %w", integrityError)
	}
	return nil
}

// defaultServiceLoop saves a Snapshot whenever a mutation occurs
func (sj *snapshotJournaler) defaultServiceLoop(
	journalEvents chan ResultJournalEntry, done chan error,
) {
	// True if we have elided writes since the last actual write.
	hasElidedWrites := true

	// Service each mutation request in turn.
serviceLoop:
	for {
		select {
		case request := <-journalEvents:
			sj.journalEntries = append(sj.journalEntries, request.JournalEntry)
			if request.JournalEntry.ElideWrite {
				hasElidedWrites = true
				if request.result != nil {
					request.result <- nil
				}
				continue
			}
			hasElidedWrites = false
			request.result <- sj.saveSnapshot()
		case <-sj.cancel:
			break serviceLoop
		}
	}

	// If we still have elided writes once the channel has closed, flush the snapshot.
	var err error
	if hasElidedWrites {
		logging.V(9).Infof("SnapshotManager: flushing elided writes...")
		err = sj.saveSnapshot()
	}
	done <- err
}

// unsafeServiceLoop doesn't save Snapshots when mutations occur and instead saves Snapshots when
// SnapshotManager.Close() is invoked. It trades reliability for speed as every mutation does not
// cause a Snapshot to be serialized to the user's state backend.
func (sj *snapshotJournaler) unsafeServiceLoop(
	journalEvents chan ResultJournalEntry, done chan error,
) {
	for {
		select {
		case request := <-journalEvents:
			sj.journalEntries = append(sj.journalEntries, request.JournalEntry)
			request.result <- nil
		case <-sj.cancel:
			done <- sj.saveSnapshot()
			return
		}
	}
}

type Journal interface {
	BeginOperation(entry JournalEntry) error
	EndOperation(entry JournalEntry) error
	Rebase(newBase *deploy.Snapshot) error
	Close()
}

type snapshotJournaler struct {
	persister      SnapshotPersister
	snapshot       *deploy.Snapshot
	journalEvents  chan ResultJournalEntry
	journalEntries []JournalEntry
	cancel         chan bool
	secretsManager secrets.Manager
}

func NewSnapshotJournaler(
	persister SnapshotPersister,
	secretsManager secrets.Manager,
	baseSnap *deploy.Snapshot,
) Journal {
	snapCopy := &deploy.Snapshot{}
	if baseSnap != nil {
		snapCopy = &deploy.Snapshot{
			Manifest:          baseSnap.Manifest,
			SecretsManager:    baseSnap.SecretsManager,
			Resources:         make([]*resource.State, 0),
			PendingOperations: make([]resource.Operation, 0),
			Metadata:          baseSnap.Metadata,
		}
		// Copy the resources from the base snapshot to the new snapshot.
		for _, res := range baseSnap.Resources {
			snapCopy.Resources = append(snapCopy.Resources, res.Copy())
		}
		// Copy the pending operations from the base snapshot to the new snapshot.
		for _, op := range baseSnap.PendingOperations {
			snapCopy.PendingOperations = append(snapCopy.PendingOperations, op.Copy())
		}
	}

	journalEvents := make(chan ResultJournalEntry)
	done, cancel := make(chan error), make(chan bool)

	journaler := snapshotJournaler{
		persister:      persister,
		snapshot:       snapCopy,
		journalEvents:  journalEvents,
		journalEntries: make([]JournalEntry, 0),
		secretsManager: secretsManager,
		cancel:         cancel,
	}

	serviceLoop := journaler.defaultServiceLoop

	if env.SkipCheckpoints.Value() {
		serviceLoop = journaler.unsafeServiceLoop
	}

	go serviceLoop(journalEvents, done)

	return &journaler
}

type ResultJournalEntry struct {
	JournalEntry JournalEntry
	result       chan error
}

func (sj *snapshotJournaler) journalMutation(entry JournalEntry) error {
	result := make(chan error)
	select {
	case sj.journalEvents <- ResultJournalEntry{JournalEntry: entry, result: result}:
		return <-result
	case <-sj.cancel:
		return errors.New("snapshot manager closed")
	}
}

func (sj *snapshotJournaler) BeginOperation(entry JournalEntry) error {
	return sj.journalMutation(entry)
}

func (sj *snapshotJournaler) EndOperation(entry JournalEntry) error {
	return sj.journalMutation(entry)
}

func (sj *snapshotJournaler) Rebase(newBase *deploy.Snapshot) error {
	snapCopy := &deploy.Snapshot{
		Manifest:          newBase.Manifest,
		SecretsManager:    newBase.SecretsManager,
		Resources:         make([]*resource.State, 0, len(newBase.Resources)),
		PendingOperations: make([]resource.Operation, 0, len(newBase.PendingOperations)),
		Metadata:          newBase.Metadata,
	}
	// Copy the resources from the base snapshot to the new snapshot.
	for _, res := range newBase.Resources {
		snapCopy.Resources = append(snapCopy.Resources, res.Copy())
	}
	// Copy the pending operations from the base snapshot to the new snapshot.
	for _, op := range newBase.PendingOperations {
		snapCopy.PendingOperations = append(snapCopy.PendingOperations, op.Copy())
	}
	sj.snapshot = snapCopy
	return sj.journalMutation(JournalEntry{Kind: JournalEntryRebase})
}

func (sj snapshotJournaler) Close() {
	sj.cancel <- true
}

// NewSnapshotManager creates a new SnapshotManager for the given stack name, using the given persister, default secrets
// manager and base snapshot.
//
// It is *very important* that the baseSnap pointer refers to the same Snapshot given to the engine! The engine will
// mutate this object and correctness of the SnapshotManager depends on being able to observe this mutation. (This is
// not ideal...)
func NewSnapshotManager(
	journal Journal,
	baseSnap *deploy.Snapshot,
) *SnapshotManager {
	manager := &SnapshotManager{
		journal:      journal,
		baseSnapshot: baseSnap,
	}

	return manager
}
