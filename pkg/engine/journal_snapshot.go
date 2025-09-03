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
	"fmt"
	"strings"
	"sync/atomic"

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
	// Append appends a new transaction to the journal.
	Append(tx JournalTransaction) error
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
	newResources gsync.Map[*resource.State, int64]

	// A counter used to generate unique operation IDs for journal entries. Note that we use these
	// sequential IDs to track the order of operations. This matters for reconstructing the Snapshot,
	// because we need to know which operations were applied first, so dependencies are resolved correctly.
	//
	// We can still send the operations to the service in parallel, because the engine will only
	// start an operation after all its dependencies have been resolved. However when reconstructing
	// the snapshot we have all journal entries available, so we need to ensure that we apply them
	// in the right order.
	sequenceNumberCounter atomic.Int64
}

var _ SnapshotManager = (*JournalSnapshotManager)(nil)

func (sm *JournalSnapshotManager) Close() error {
	return sm.journal.Close()
}

type JournalEntryKind int

const (
	JournalEntryInvalid                JournalEntryKind = 0
	JournalEntryAddPendingOperation    JournalEntryKind = 1
	JournalEntryRemovePendingOperation JournalEntryKind = 2
	JournalEntryRemoveState            JournalEntryKind = 3
	JournalEntryPatchState             JournalEntryKind = 4
	JournalEntryAppendNewState         JournalEntryKind = 5
	JournalEntryWrite                  JournalEntryKind = 6
)

func (k JournalEntryKind) String() string {
	switch k {
	case JournalEntryInvalid:
		return "Invalid"
	case JournalEntryAddPendingOperation:
		return "AddPendingOperation"
	case JournalEntryRemovePendingOperation:
		return "RemovePendingOperation"
	case JournalEntryRemoveState:
		return "RemoveState"
	case JournalEntryPatchState:
		return "PatchState"
	case JournalEntryAppendNewState:
		return "AppendNewState"
	case JournalEntryWrite:
		return "Write"
	default:
		return "Unknown"
	}
}

func formatSequenceNumber(i int64) string {
	return fmt.Sprintf("%v.%v", (i>>24)&0xffffffffff, i&0xffffff)
}

type JournalState struct {
	// For {Remove,Patch}BaseState
	Base *int64

	// For {Remove,Patch}NewState
	New *int64
}

func (s *JournalState) String() string {
	switch {
	case s == nil:
		return "<nil>"
	case s.Base != nil:
		return fmt.Sprintf("base[%v]", formatSequenceNumber(*s.Base))
	case s.New != nil:
		return fmt.Sprintf("new[%v]", formatSequenceNumber(*s.New))
	default:
		return "<invalid>"
	}
}

type JournalTransaction struct {
	// The 40-bit sequence number of the transaction.
	SequenceNumber int64

	// The journal entries that make up this transaction.
	Entries []JournalEntry
}

func (tx *JournalTransaction) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%v: tx [\n", tx.SequenceNumber)

	for _, entry := range tx.Entries {
		fmt.Fprintf(&sb, "\t%v,\n", entry)
	}

	fmt.Fprintf(&sb, "]\n")
	return sb.String()
}

type JournalEntry struct {
	// The kind of the operation.
	Kind JournalEntryKind

	// For AddPendingOperation
	PendingOperation *resource.Operation

	// For RemovePendingOperation
	RemovePendingOperation *int64

	// For {Remove,Patch}{Base,New}State
	Old *JournalState

	// For PatchBaseState, PatchNewState, and AppendNewState
	State *resource.State

	// For {Remove,Patch}{Base,New}State
	InvalidateDependencies bool

	// For Write
	Snapshot *deploy.Snapshot
}

func (e JournalEntry) String() string {
	var sb strings.Builder
	sb.WriteString(e.Kind.String())

	if e.PendingOperation != nil {
		fmt.Fprintf(&sb, " op:%v", e.PendingOperation.Type)
	}
	if e.RemovePendingOperation != nil {
		fmt.Fprintf(&sb, " rm:%v", formatSequenceNumber(*e.RemovePendingOperation))
	}

	if e.Old != nil {
		fmt.Fprintf(&sb, " old:%v", e.Old)
	}

	if e.State != nil {
		fmt.Fprintf(
			&sb,
			" new:(urn:%v, id:%v, delete:%v, replace:%v)",
			e.State.URN,
			e.State.ID,
			e.State.Delete,
			e.State.PendingReplacement,
		)
	}

	if e.InvalidateDependencies {
		fmt.Fprintf(&sb, " -deps")
	}

	if e.Snapshot != nil {
		fmt.Fprintf(&sb, " +snap")
	}

	return sb.String()
}

// locateState finds the given state in either the base snapshot or the new states produced during the update.
func (sm *JournalSnapshotManager) tryLocateState(state *resource.State) (*JournalState, bool) {
	contract.Assertf(state != nil, "state must not be nil")

	// If the state is non-nil, either locate it in the base snapshot or in the journal.
	if sequenceNumber, ok := sm.newResources.Load(state); ok {
		return &JournalState{New: &sequenceNumber}, true
	}

	if sm.baseSnapshot != nil {
		for i, res := range sm.baseSnapshot.Resources {
			if res == state {
				index := int64(i)
				return &JournalState{Base: &index}, true
			}
		}
	}

	return nil, false
}

// locateState finds the given state in either the base snapshot or the new states produced during the update.
func (sm *JournalSnapshotManager) locateState(state *resource.State) *JournalState {
	s, ok := sm.tryLocateState(state)
	contract.Assertf(ok, "missing state 0x%p (URN: %v, Delete: %v)", state, state.URN, state.Delete)
	return s
}

func (sm *JournalSnapshotManager) transaction(fn func(tx *journalTransaction)) error {
	tx := &journalTransaction{
		manager:        sm,
		sequenceNumber: sm.sequenceNumberCounter.Add(1),
	}
	fn(tx)

	return sm.journal.Append(JournalTransaction{
		SequenceNumber: tx.sequenceNumber,
		Entries:        tx.entries,
	})
}

type journalTransaction struct {
	manager        *JournalSnapshotManager
	sequenceNumber int64
	entries        []JournalEntry
}

func (tx *journalTransaction) currentSequenceNumber() int64 {
	return ((tx.sequenceNumber & 0xffffffffff) << 24) | int64(len(tx.entries))
}

func (tx *journalTransaction) addPendingOperation(op resource.Operation) int64 {
	sequenceNumber := tx.currentSequenceNumber()
	tx.entries = append(tx.entries, JournalEntry{
		Kind:             JournalEntryAddPendingOperation,
		PendingOperation: &op,
	})
	return sequenceNumber
}

func (tx *journalTransaction) removePendingOperation(sequenceNumber int64) {
	tx.entries = append(tx.entries, JournalEntry{
		Kind:                   JournalEntryRemovePendingOperation,
		RemovePendingOperation: &sequenceNumber,
	})
}

func (tx *journalTransaction) tryRemoveState(old *resource.State) {
	if oldState, ok := tx.manager.tryLocateState(old); ok {
		tx.entries = append(tx.entries, JournalEntry{
			Kind: JournalEntryRemoveState,
			Old:  oldState,
		})
	}
}

func (tx *journalTransaction) removeState(old *resource.State, invalidateDependencies bool) {
	tx.entries = append(tx.entries, JournalEntry{
		Kind:                   JournalEntryRemoveState,
		Old:                    tx.manager.locateState(old),
		InvalidateDependencies: invalidateDependencies,
	})
}

func (tx *journalTransaction) patchState(old, new *resource.State, invalidateDependencies bool) {
	entry := JournalEntry{
		Kind:                   JournalEntryPatchState,
		Old:                    tx.manager.locateState(old),
		State:                  new.Copy(),
		InvalidateDependencies: invalidateDependencies,
	}
	if entry.Old.New != nil {
		tx.manager.newResources.Store(entry.State, tx.currentSequenceNumber())
	}
	tx.entries = append(tx.entries, entry)
}

func (tx *journalTransaction) appendState(old, new *resource.State) {
	entry := JournalEntry{
		Kind:  JournalEntryAppendNewState,
		State: new.Copy(),
	}
	if old != nil {
		entry.Old = tx.manager.locateState(old)
	}
	entry.State = new
	tx.manager.newResources.Store(entry.State, tx.currentSequenceNumber())
	tx.entries = append(tx.entries, entry)
}

func (tx *journalTransaction) write(snap *deploy.Snapshot) {
	tx.entries = append(tx.entries, JournalEntry{
		Kind:     JournalEntryWrite,
		Snapshot: snap,
	})
}

// RegisterResourceOutputs handles the registering of outputs on a Step that has already
// completed.
func (sm *JournalSnapshotManager) RegisterResourceOutputs(step deploy.Step) error {
	elide := step.Old() != nil && step.New() != nil && step.Old().Outputs.DeepEquals(step.New().Outputs)
	if elide {
		return nil
	}

	// The resource we're patching may live in either the base or new resource list.
	old := step.Old()
	if old == nil {
		old = step.New()
	}
	return sm.transaction(func(tx *journalTransaction) {
		tx.patchState(old, step.New(), false)
	})
}

// BeginMutation signals to the SnapshotManager that the engine intends to mutate the global snapshot
// by performing the given Step. This function gives the SnapshotManager a chance to record the
// intent to mutate before the mutation occurs.
func (sm *JournalSnapshotManager) BeginMutation(step deploy.Step) (SnapshotMutation, error) {
	contract.Requiref(step != nil, "step", "cannot be nil")
	logging.V(9).Infof("SnapshotManager: Beginning mutation for step `%s` on resource `%s`", step.Op(), step.URN())

	switch step.Op() {
	case deploy.OpSame:
		return sm.doSame(step)
	case deploy.OpCreate, deploy.OpCreateReplacement:
		return sm.doCreate(step)
	case deploy.OpUpdate:
		return sm.doUpdate(step)
	case deploy.OpDelete, deploy.OpDeleteReplaced, deploy.OpReadDiscard, deploy.OpDiscardReplaced:
		return sm.doDelete(step)
	case deploy.OpReplace:
		return sm.doReplace(step)
	case deploy.OpRead, deploy.OpReadReplacement:
		return sm.doRead(step)
	case deploy.OpRefresh:
		return sm.doRefresh(step)
	case deploy.OpRemovePendingReplace:
		return sm.doRemovePendingReplace(step)
	case deploy.OpImport, deploy.OpImportReplacement:
		return sm.doImport(step)
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
	return sm.transaction(func(tx *journalTransaction) {
		newBase := &deploy.Snapshot{
			Manifest:          base.Manifest,
			SecretsManager:    base.SecretsManager,
			Resources:         make([]*resource.State, 0, len(base.Resources)),
			PendingOperations: make([]resource.Operation, 0, len(base.PendingOperations)),
			Metadata:          base.Metadata,
		}

		// Copy the resources from the base snapshot to the new snapshot.
		for _, res := range base.Resources {
			newBase.Resources = append(newBase.Resources, res.Copy())
		}
		// Copy the pending operations from the base snapshot to the new snapshot.
		for _, op := range base.PendingOperations {
			newBase.PendingOperations = append(newBase.PendingOperations, op.Copy())
		}

		tx.write(newBase)
	})
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
	manager *JournalSnapshotManager
}

func (sm *JournalSnapshotManager) doSame(step deploy.Step) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doSame(%s)", step.URN())
	return &sameSnapshotMutation{sm}, nil
}

func (ssm *sameSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	contract.Requiref(step.Op() == deploy.OpSame, "step.Op()", "must be %q, got %q", deploy.OpSame, step.Op())
	logging.V(9).Infof("SnapshotManager: sameSnapshotMutation.End(..., %v)", successful)

	// Bail out early if the step was not successful.
	if !successful {
		return nil
	}

	return ssm.manager.transaction(func(txn *journalTransaction) {
		sameStep, isSameStep := step.(*deploy.SameStep)
		if !isSameStep || !sameStep.IsSkippedCreate() {
			txn.appendState(step.Old(), step.New())
		} else {
			txn.tryRemoveState(step.Old())
		}
	})
}

func (sm *JournalSnapshotManager) doCreate(step deploy.Step) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doCreate(%s)", step.URN())

	var sequenceNumber int64
	err := sm.transaction(func(tx *journalTransaction) {
		sequenceNumber = tx.addPendingOperation(resource.NewOperation(step.New().Copy(), resource.OperationTypeCreating))
	})
	if err != nil {
		return nil, err
	}
	return &createSnapshotMutation{sm, sequenceNumber}, nil
}

type createSnapshotMutation struct {
	manager        *JournalSnapshotManager
	sequenceNumber int64
}

func (csm *createSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	logging.V(9).Infof("SnapshotManager: createSnapshotMutation.End(..., %v)", successful)

	return csm.manager.transaction(func(tx *journalTransaction) {
		tx.removePendingOperation(csm.sequenceNumber)

		// If this step is a create replacement, we need to mark the old resource for deletion.
		// The engine marks this in its in-memory representation, but since the snapshot manager
		// is operating on a copy of the snapshot, we need to explicitly mark the resource.
		if successful {
			if old := step.Old(); old != nil {
				if old.PendingReplacement {
					tx.removeState(old, false)
				} else {
					tx.patchState(old, old, false)
				}
			}

			tx.appendState(nil, step.New())
		}
	})
}

func (sm *JournalSnapshotManager) doUpdate(step deploy.Step) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doUpdate(%s)", step.URN())

	var sequenceNumber int64
	err := sm.transaction(func(tx *journalTransaction) {
		sequenceNumber = tx.addPendingOperation(resource.NewOperation(step.New().Copy(), resource.OperationTypeUpdating))
	})
	if err != nil {
		return nil, err
	}
	return &updateSnapshotMutation{sm, sequenceNumber}, nil
}

type updateSnapshotMutation struct {
	manager        *JournalSnapshotManager
	sequenceNumber int64
}

func (usm *updateSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	logging.V(9).Infof("SnapshotManager: updateSnapshotMutation.End(..., %v)", successful)

	return usm.manager.transaction(func(tx *journalTransaction) {
		tx.removePendingOperation(usm.sequenceNumber)
		if successful {
			tx.appendState(step.Old(), step.New())
		}
	})
}

func (sm *JournalSnapshotManager) doDelete(step deploy.Step) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doDelete(%s)", step.URN())

	var sequenceNumber int64
	err := sm.transaction(func(tx *journalTransaction) {
		sequenceNumber = tx.addPendingOperation(resource.NewOperation(step.Old().Copy(), resource.OperationTypeDeleting))
	})
	if err != nil {
		return nil, err
	}
	return &deleteSnapshotMutation{sm, sequenceNumber}, nil
}

type deleteSnapshotMutation struct {
	manager        *JournalSnapshotManager
	sequenceNumber int64
}

func (dsm *deleteSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	logging.V(9).Infof("SnapshotManager: deleteSnapshotMutation.End(..., %v)", successful)

	return dsm.manager.transaction(func(tx *journalTransaction) {
		tx.removePendingOperation(dsm.sequenceNumber)
		if successful {
			contract.Assertf(
				!step.Old().Protect ||
					step.Op() == deploy.OpDiscardReplaced ||
					step.Op() == deploy.OpDeleteReplaced,
				"Old must be unprotected (got %v) or the operation must be a replace (got %q)",
				step.Old().Protect, step.Op())

			old := step.Old()
			if !old.PendingReplacement {
				tx.removeState(old, false)
			} else {
				tx.patchState(old, old, false)
			}
		}
	})
}

func (sm *JournalSnapshotManager) doReplace(step deploy.Step) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doReplace(%s)", step.URN())
	return &replaceSnapshotMutation{sm, 0}, nil
}

type replaceSnapshotMutation struct {
	manager        *JournalSnapshotManager
	sequenceNumber int64
}

func (rsm *replaceSnapshotMutation) End(step deploy.Step, successful bool) error {
	logging.V(9).Infof("SnapshotManager: replaceSnapshotMutation.End(..., %v)", successful)
	return nil
}

func (sm *JournalSnapshotManager) doRead(step deploy.Step) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doRead(%s)", step.URN())

	var sequenceNumber int64
	err := sm.transaction(func(tx *journalTransaction) {
		sequenceNumber = tx.addPendingOperation(resource.NewOperation(step.New().Copy(), resource.OperationTypeReading))
	})
	if err != nil {
		return nil, err
	}
	return &readSnapshotMutation{sm, sequenceNumber}, nil
}

type readSnapshotMutation struct {
	manager        *JournalSnapshotManager
	sequenceNumber int64
}

func (rsm *readSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	logging.V(9).Infof("SnapshotManager: readSnapshotMutation.End(..., %v)", successful)

	return rsm.manager.transaction(func(tx *journalTransaction) {
		tx.removePendingOperation(rsm.sequenceNumber)
		if successful {
			if old := step.Old(); old != nil {
				tx.removeState(step.Old(), false)
			}
			tx.appendState(nil, step.New())
		}
	})
}

func (sm *JournalSnapshotManager) doRefresh(step deploy.Step) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doRefresh(%s)", step.URN())
	return &refreshSnapshotMutation{sm, 0}, nil
}

type refreshSnapshotMutation struct {
	manager        *JournalSnapshotManager
	sequenceNumber int64
}

func (rsm *refreshSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	contract.Requiref(step.Op() == deploy.OpRefresh, "step.Op", "must be %q, got %q", deploy.OpRefresh, step.Op())
	logging.V(9).Infof("SnapshotManager: refreshSnapshotMutation.End(..., %v)", successful)

	return rsm.manager.transaction(func(tx *journalTransaction) {
		if successful {
			// We're treating persisted refreshes and slightly different than non-persisted ones.
			// Persisted refreshes are just a delete and create of the resource, and the new resource
			// can be appended at the end of the base snapshot.  Meanwhile for "non-persisted" refreshes
			// the resource needs to be updated in place, to make sure all ordering constraints
			// are satisfied.

			refreshStep, isRefreshStep := step.(*deploy.RefreshStep)
			viewStep, isViewStep := step.(*deploy.ViewStep)
			if (isRefreshStep && refreshStep.Persisted()) || (isViewStep && viewStep.Persisted()) {
				hasNew := step.New() != nil
				tx.removeState(step.Old(), !hasNew)
				if hasNew {
					tx.appendState(nil, step.New())
				}
			} else {
				if new := step.New(); new != nil {
					tx.patchState(step.Old(), new, true)
				} else {
					tx.removeState(step.Old(), true)
				}
			}
		}
	})
}

func (sm *JournalSnapshotManager) doRemovePendingReplace(
	step deploy.Step,
) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doRemovePendingReplace(%s)", step.URN())
	return &removePendingReplaceSnapshotMutation{sm}, nil
}

type removePendingReplaceSnapshotMutation struct {
	manager *JournalSnapshotManager
}

func (rsm *removePendingReplaceSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	contract.Requiref(step.Op() == deploy.OpRemovePendingReplace, "step.Op",
		"must be %q, got %q", deploy.OpRemovePendingReplace, step.Op())

	return rsm.manager.transaction(func(tx *journalTransaction) {
		tx.removeState(step.Old(), false)
	})
}

func (sm *JournalSnapshotManager) doImport(step deploy.Step) (SnapshotMutation, error) {
	logging.V(9).Infof("SnapshotManager.doImport(%s)", step.URN())

	var sequenceNumber int64
	err := sm.transaction(func(tx *journalTransaction) {
		sequenceNumber = tx.addPendingOperation(resource.NewOperation(step.New().Copy(), resource.OperationTypeImporting))
	})
	if err != nil {
		return nil, err
	}
	return &importSnapshotMutation{sm, sequenceNumber}, nil
}

type importSnapshotMutation struct {
	manager        *JournalSnapshotManager
	sequenceNumber int64
}

func (ism *importSnapshotMutation) End(step deploy.Step, successful bool) error {
	contract.Requiref(step != nil, "step", "must not be nil")
	contract.Requiref(step.Op() == deploy.OpImport || step.Op() == deploy.OpImportReplacement, "step.Op",
		"must be %q or %q, got %q", deploy.OpImport, deploy.OpImportReplacement, step.Op())

	return ism.manager.transaction(func(tx *journalTransaction) {
		tx.removePendingOperation(ism.sequenceNumber)

		// If this step is a create replacement, we need to mark the old resource for deletion.
		// The engine marks this in its in-memory representation, but since the snapshot manager
		// is operating on a copy of the snapshot, we need to explicitly mark the resource.
		if successful {
			if original := step.(*deploy.ImportStep).Original(); original != nil {
				tx.patchState(original, original, false)
			}

			if old := step.Old(); old != nil && (old.Delete || old.PendingReplacement) {
				tx.patchState(old, old, false)
			}

			tx.appendState(nil, step.New())
		}
	})
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
