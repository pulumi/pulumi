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
	"reflect"
	"sync/atomic"

	"golang.org/x/exp/slices"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// Journal defines an interface for journal operations. The underlying implementation of this interface
// is responsible for recording and storing the operations, and constructing a snapshot/storing them
// for later replaying.
type Journal interface {
	// BeginOperation begins a new operation in the journal. This should be called before any
	// mutation is performed on the snapshot. The journal entry should contain the operation ID,
	// which is used to correlate the begin and end operations in the journal.
	BeginOperation(entry JournalEntry) error
	// EndOperation ends an operation in the journal. This should be called after the mutation is
	// performed on the snapshot. The journal entry should contain the operation ID, which is used
	// to correlate the begin and end operations in the journal.
	EndOperation(entry JournalEntry) error
	// Write updates the base snapshot for this journal. This is used e.g. when providers have
	// been updated, and we can't simply reuse the base snapshot from the previous plan. This
	// needs to be called before any other mutation requests.
	Write(newBase *deploy.Snapshot) error
	// Close closes the journal, flushing any pending operations.
	Close() error
}

// SnapshotPersister is an interface implemented by our backends that implements snapshot
// persistence. In order to fit into our current model, snapshot persisters have two functions:
// saving snapshots and invalidating already-persisted snapshots.
type SnapshotPersister interface {
	// Persists the given snapshot. Returns an error if the persistence failed.
	Save(snapshot *deploy.Snapshot) error
}

// JournalSnapshotManager is an implementation of engine.JournalSnapshotManager that inspects steps and performs
// mutations on the global snapshot object serially. This implementation maintains two bits of state: the "base"
// snapshot, which is immutable and represents the state of the world prior to the application
// of the current plan, and a journal of operations, which consists of the operations that are being
// applied on top of the immutable snapshot.
type JournalSnapshotManager struct {
	journal      Journal          // The journal used to record operations performed by this plan
	baseSnapshot *deploy.Snapshot // The base snapshot for this plan

	// newResources is a map of resources that have been added to the snapshot in this plan, keyed by the resource
	// state.  This is used to track the added resources and their operation IDs, allowing us too delete
	// them later if necessary.
	newResources gsync.Map[*resource.State, uint64]
	// A counter used to generate unique operation IDs for journal entries. Note that we use these
	// sequential IDs to track the order of operations. This matters for reconstructing the Snapshot,
	// because we need to know which operations were applied first, so dependencies are resolvedd correctly.
	//
	// We can still send the operations to the service in parallel, because the engine will onlyy
	// start an operation after all its dependencies have been resolved. However when reconstructing
	// the snapshot we have all journal entries available, so we need to ensure that we apply them
	// in the right order.
	operationIDCounter atomic.Uint64
}

var _ SnapshotManager = (*JournalSnapshotManager)(nil)

func (sm *JournalSnapshotManager) Close() error {
	return sm.journal.Close()
}

type JournalEntryKind int

const (
	JournalEntryBegin          JournalEntryKind = 0
	JournalEntrySuccess        JournalEntryKind = 1
	JournalEntryFailure        JournalEntryKind = 2
	JournalEntryRefreshSuccess JournalEntryKind = 3
	JournalEntryOutputs        JournalEntryKind = 4
	JournalEntryWrite          JournalEntryKind = 5
)

type JournalEntry struct {
	Kind JournalEntryKind
	// The ID of the operation that this journal entry is associated with.
	OperationID uint64
	// The index of the resource in the base snapshot to delete, or -1 if no deletion is needed.
	DeleteOld int
	// The operation ID of a new resource that should be deleted.
	DeleteNew uint64
	// The index of the resource in the base snapshot that should be marked as pending
	// replacement, or -1 if no pending replacement is needed.
	PendingReplacement int
	// The resource state associated with this journal entry.
	State *resource.State
	// The operation associated with this journal entry, if any.
	Operation *resource.Operation
	// If true, this journal entry can be elided and does not need to be written immediately.
	ElideWrite bool
	// If true, this journal entry is part of a refresh operation.
	IsRefresh bool

	// The new snapshot if this journal entry is part of a rebase operation.
	NewSnapshot *deploy.Snapshot
}

func newJournalEntry(kind JournalEntryKind, operationID uint64) JournalEntry {
	return JournalEntry{
		Kind:               kind,
		OperationID:        operationID,
		DeleteOld:          -1, // Default to -1, which means no deletion.
		PendingReplacement: -1, // Default to -1, which means no pending replacement.
	}
}

// RegisterResourceOutputs handles the registering of outputs on a Step that has already
// completed.
func (sm *JournalSnapshotManager) RegisterResourceOutputs(step deploy.Step) error {
	operationID := sm.operationIDCounter.Add(1)

	journalEntry := newJournalEntry(JournalEntryOutputs, operationID)
	journalEntry.ElideWrite = step.Old() != nil && step.New() != nil && step.Old().Outputs.DeepEquals(step.New().Outputs)
	// If the outputs have changed, we create a journal entry.  This will cause the resource
	// to be replaced in the snapshot, and thus the new outputs written.
	if step.Old() != nil && step.New() != nil && !step.Old().Outputs.DeepEquals(step.New().Outputs) {
		journalEntry.State = step.New()
		sm.newResources.Store(step.New(), operationID)
		sm.markEntryForDeletion(&journalEntry, step.Old())
	}
	return sm.journal.EndOperation(journalEntry)
}

// markEntryForDeletion marks the given resource state for deletion in the journal entry. We compare the
// pointer to the resource state in the base snapshot, to find the position in the baseSnapshot here,
// in case the resource is already in the base snapshot.
//
// If we have a new resource that was created in this plan, but then gets deleted by a subsequent step,
// we record the operation ID of the new resource, so the snapshot generation can skip the earlier operation,
// and thus the new resource won't be written to the snapshot..
func (sm *JournalSnapshotManager) markEntryForDeletion(journalEntry *JournalEntry, toDelete *resource.State) {
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
func (sm *JournalSnapshotManager) BeginMutation(step deploy.Step) (SnapshotMutation, error) {
	contract.Requiref(step != nil, "step", "cannot be nil")
	logging.V(9).Infof("SnapshotManager: Beginning mutation for step `%s` on resource `%s`", step.Op(), step.URN())

	operationID := sm.operationIDCounter.Add(1)

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

// Write sets the base snapshot for this SnapshotManager. This is used to rebase the journal
// on a new base snapshot, in particular when providers have been updated. We always expect
// this to be before any other mutation requests, so we can safely record the index for deletions
// without the base snapshot changing under us.
func (sm *JournalSnapshotManager) Write(base *deploy.Snapshot) error {
	if sm == nil {
		return nil
	}
	sm.baseSnapshot = base
	return sm.journal.Write(base)
}

// All SnapshotMutation implementations in this file follow the same basic formula:
//
// 1. Begin the operation in the journal, recording the ID, and storing the intent to do an
//    operation on the snapshot. If the operation fails after this point, we'll have the
//    operation recorded in the snapshot as pending, and can ask the user to finish it in
//    whatever way is appropriate.
//
// 2. When the operation completes, call `End` on the mutation, which will record the
//    operation's success or failure in the journal. The journal entry indicates whether
//    a new resource was created, and/or deleted. Using these journal entries the snapshot
//    can then be rebuilt.
//
// Each mutation has a unique operation ID, which is used to correlate the begin and end
// operations in the journal. This ID is also used to track the newly created resources.

type sameSnapshotMutation struct {
	manager     *JournalSnapshotManager
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

func (sm *JournalSnapshotManager) doSame(step deploy.Step, operationID uint64) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doSame(%s)", step.URN())
	journalEntry := newJournalEntry(JournalEntryBegin, operationID)
	journalEntry.ElideWrite = true
	err := sm.journal.BeginOperation(journalEntry)
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
	journalEntry := newJournalEntry(kind, ssm.operationID)
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

func (sm *JournalSnapshotManager) doCreate(step deploy.Step, operationID uint64) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doCreate(%s)", step.URN())
	op := resource.NewOperation(step.New(), resource.OperationTypeCreating)

	journalEntry := newJournalEntry(JournalEntryBegin, operationID)
	journalEntry.Operation = &op
	// If this step is a create replacement, we need to mark the old resource for deletion.
	// The engine marks this in its in-memory representation, but since the snapshot manager
	// is operating on a copy of the snapshot, we need to explicitly mark the resource.
	if step.Old() != nil {
		sm.markEntryForDeletion(&journalEntry, step.Old())
	}
	err := sm.journal.BeginOperation(journalEntry)
	if err != nil {
		return nil, err
	}

	return &createSnapshotMutation{sm, operationID}, nil
}

type createSnapshotMutation struct {
	manager     *JournalSnapshotManager
	operationID uint64
}

func (csm *createSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	logging.V(9).Infof("SnapshotManager: createSnapshotMutation.End(..., %v)", successful)
	kind := JournalEntrySuccess
	if !successful {
		kind = JournalEntryFailure
	}
	journalEntry := newJournalEntry(kind, csm.operationID)
	journalEntry.State = step.New()
	csm.manager.newResources.Store(step.New(), csm.operationID)
	if old := step.Old(); old != nil && old.PendingReplacement {
		csm.manager.markEntryForDeletion(&journalEntry, step.Old())
	}

	return csm.manager.journal.EndOperation(journalEntry)
}

func (sm *JournalSnapshotManager) doUpdate(step deploy.Step, operationID uint64) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doUpdate(%s)", step.URN())
	op := resource.NewOperation(step.New(), resource.OperationTypeUpdating)
	journalEntry := newJournalEntry(JournalEntryBegin, operationID)
	journalEntry.Operation = &op
	err := sm.journal.BeginOperation(journalEntry)
	if err != nil {
		return nil, err
	}

	return &updateSnapshotMutation{sm, operationID}, nil
}

type updateSnapshotMutation struct {
	manager     *JournalSnapshotManager
	operationID uint64
}

func (usm *updateSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	logging.V(9).Infof("SnapshotManager: updateSnapshotMutation.End(..., %v)", successful)
	kind := JournalEntrySuccess
	if !successful {
		kind = JournalEntryFailure
	}
	journalEntry := newJournalEntry(kind, usm.operationID)
	if old := step.Old(); old != nil {
		usm.manager.markEntryForDeletion(&journalEntry, step.Old())
	}
	journalEntry.State = step.New()
	usm.manager.newResources.Store(step.New(), usm.operationID)
	return usm.manager.journal.EndOperation(journalEntry)
}

func (sm *JournalSnapshotManager) doDelete(step deploy.Step, operationID uint64) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doDelete(%s)", step.URN())
	op := resource.NewOperation(step.Old(), resource.OperationTypeDeleting)
	journalEntry := newJournalEntry(JournalEntryBegin, operationID)
	journalEntry.Operation = &op

	err := sm.journal.BeginOperation(journalEntry)
	if err != nil {
		return nil, err
	}
	return &deleteSnapshotMutation{sm, operationID}, nil
}

type deleteSnapshotMutation struct {
	manager     *JournalSnapshotManager
	operationID uint64
}

func (dsm *deleteSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	logging.V(9).Infof("SnapshotManager: deleteSnapshotMutation.End(..., %v)", successful)
	kind := JournalEntrySuccess
	if !successful {
		kind = JournalEntryFailure
	}
	journalEntry := newJournalEntry(kind, dsm.operationID)
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

func (sm *JournalSnapshotManager) doReplace(step deploy.Step, operationID uint64) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doReplace(%s)", step.URN())
	return &replaceSnapshotMutation{sm, operationID}, nil
}

type replaceSnapshotMutation struct {
	manager     *JournalSnapshotManager
	operationID uint64
}

func (rsm *replaceSnapshotMutation) End(step deploy.Step, successful bool) error {
	logging.V(9).Infof("SnapshotManager: replaceSnapshotMutation.End(..., %v)", successful)
	return nil
}

func (sm *JournalSnapshotManager) doRead(step deploy.Step, operationID uint64) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doRead(%s)", step.URN())
	op := resource.NewOperation(step.New(), resource.OperationTypeReading)
	journalEntry := newJournalEntry(JournalEntryBegin, operationID)
	journalEntry.Operation = &op
	err := sm.journal.BeginOperation(journalEntry)
	if err != nil {
		return nil, err
	}
	return &readSnapshotMutation{sm, operationID}, nil
}

type readSnapshotMutation struct {
	manager     *JournalSnapshotManager
	operationID uint64
}

func (rsm *readSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	logging.V(9).Infof("SnapshotManager: readSnapshotMutation.End(..., %v)", successful)
	kind := JournalEntrySuccess
	if !successful {
		kind = JournalEntryFailure
	}
	journalEntry := newJournalEntry(kind, rsm.operationID)
	journalEntry.State = step.New()
	rsm.manager.newResources.Store(step.New(), rsm.operationID)
	if old := step.Old(); old != nil && rsm.manager.baseSnapshot != nil {
		rsm.manager.markEntryForDeletion(&journalEntry, step.Old())
	}
	return rsm.manager.journal.EndOperation(journalEntry)
}

func (sm *JournalSnapshotManager) doRefresh(step deploy.Step, operationID uint64) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doRefresh(%s)", step.URN())
	journalEntry := newJournalEntry(JournalEntryBegin, operationID)

	err := sm.journal.BeginOperation(journalEntry)
	if err != nil {
		return nil, err
	}
	return &refreshSnapshotMutation{sm, operationID}, nil
}

type refreshSnapshotMutation struct {
	manager     *JournalSnapshotManager
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
	journalEntry := newJournalEntry(kind, rsm.operationID)

	if step.New() != nil {
		journalEntry.State = step.New()
		rsm.manager.newResources.Store(step.New(), rsm.operationID)
	}

	refreshStep, isRefreshStep := step.(*deploy.RefreshStep)
	viewStep, isViewStep := step.(*deploy.ViewStep)
	if (isRefreshStep && refreshStep.Persisted()) || (isViewStep && viewStep.Persisted()) && successful {
		// We're treating persisted refreshes and slightly different than non-persisted ones.
		// Persisted refreshes are just a delete and create of the resource, and the new resource
		// can be appended at the end of the base snapshot.  Meanwhile for "non-persisted" refreshes
		// the resource needs to be updated in place, to make sure all ordering constraints
		// are satisfied.
		journalEntry.Kind = JournalEntrySuccess
		// We still need to know it is a refresh, so we can update dependencies correctly.
		journalEntry.IsRefresh = true
	}
	if old := step.Old(); old != nil {
		rsm.manager.markEntryForDeletion(&journalEntry, old)
	}

	return rsm.manager.journal.EndOperation(journalEntry)
}

func (sm *JournalSnapshotManager) doRemovePendingReplace(
	step deploy.Step, operationID uint64,
) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doRemovePendingReplace(%s)", step.URN())
	journalEntry := newJournalEntry(JournalEntryBegin, operationID)
	err := sm.journal.BeginOperation(journalEntry)
	if err != nil {
		return nil, err
	}
	return &removePendingReplaceSnapshotMutation{sm, operationID}, err
}

type removePendingReplaceSnapshotMutation struct {
	manager     *JournalSnapshotManager
	operationID uint64
}

func (rsm *removePendingReplaceSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	contract.Requiref(step.Op() == deploy.OpRemovePendingReplace, "step.Op",
		"must be %q, got %q", deploy.OpRemovePendingReplace, step.Op())
	journalEntry := newJournalEntry(JournalEntrySuccess, rsm.operationID)
	if step.Old() != nil {
		rsm.manager.markEntryForDeletion(&journalEntry, step.Old())
	}
	return rsm.manager.journal.EndOperation(journalEntry)
}

func (sm *JournalSnapshotManager) doImport(step deploy.Step, operationID uint64) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doImport(%s)", step.URN())
	op := resource.NewOperation(step.New(), resource.OperationTypeImporting)
	journalEntry := newJournalEntry(JournalEntryBegin, operationID)
	journalEntry.Operation = &op
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
	manager     *JournalSnapshotManager
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
	journalEntry := newJournalEntry(kind, ism.operationID)
	journalEntry.State = step.New()
	ism.manager.newResources.Store(step.New(), ism.operationID)
	return ism.manager.journal.EndOperation(journalEntry)
}

// NewJournalSnapshotManager creates a new SnapshotManager for the given stack name, using the
// given persister, default secrets manager and base snapshot.
//
// It is *very important* that the baseSnap pointer refers to the same Snapshot given to the engine! The engine will
// mutate this object, and the snapshot manager will do pointer comparisons to determine indices
// for journal entries.
func NewJournalSnapshotManager(
	journal Journal,
	baseSnap *deploy.Snapshot,
) *JournalSnapshotManager {
	manager := &JournalSnapshotManager{
		journal:      journal,
		baseSnapshot: baseSnap,
	}

	return manager
}
