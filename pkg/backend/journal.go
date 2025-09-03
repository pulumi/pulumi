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
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/pgavlin/fx/v2"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
)

// mustWrite returns true if any semantically meaningful difference exists between the old and new states of a same
// step that forces us to write the checkpoint. If no such difference exists, the checkpoint write that corresponds to
// this step can be elided.
func (sj *SnapshotJournal) mustWrite(old, new *resource.State) bool {
	if old.Delete != new.Delete {
		logging.V(9).Infof("SnapshotManager: mustWrite() true because of Delete")
		return true
	}

	if old.External != new.External {
		logging.V(9).Infof("SnapshotManager: mustWrite() true because of External")
		return true
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

	// If IDs have changed, we need to write.
	if old.ID != new.ID {
		logging.V(9).Infof("SnapshotManager: mustWrite() true because of IDs")
		return true
	}

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

type InvalidJournalError struct {
	SequenceNumber int64
	Entry          engine.JournalEntry
	Message        string
}

func (e *InvalidJournalError) Error() string {
	return fmt.Sprintf("entry %v: %v", e.SequenceNumber, e.Message)
}

type journalReplayer struct {
	journal *SnapshotJournal

	// resources contains the new resources appended to the snapshot.
	resources []*resource.State
	// snap contains the base snapshot for replay.
	snap *deploy.Snapshot

	// pendingOperations maps from sequence numbers to in-progress operations.
	pendingOperations map[int64]resource.Operation
	// baseRemoves tracks indices of states to remove from the base snapshot.
	baseRemoves fx.Set[int64]
	// basePatches tracks states to patch in the base snapshot.
	basePatches map[int64]*resource.State
	// news maps sequence numbers to resource indices.
	news map[int64]int

	// rebuildDependencies is true if changes to the snapshot were made that would invalidate dependency lists.
	rebuildDependencies bool
}

func newJournalReplayer(journal *SnapshotJournal) *journalReplayer {
	return &journalReplayer{
		journal:           journal,
		snap:              journal.snapshot,
		pendingOperations: map[int64]resource.Operation{},
		baseRemoves:       fx.Set[int64]{},
		basePatches:       map[int64]*resource.State{},
		news:              map[int64]int{},
	}
}

type journalState struct {
	replayer *journalReplayer

	index  int64
	isBase bool
}

func (s *journalState) remove() *resource.State {
	if s.isBase {
		s.replayer.baseRemoves.Add(s.index)
		return s.replayer.snap.Resources[s.index]
	}

	old := s.replayer.resources[s.index]
	s.replayer.resources[s.index] = nil
	return old
}

func (s *journalState) patch(sequenceNumber int64, new *resource.State) *resource.State {
	if s.isBase {
		s.replayer.basePatches[s.index] = new
		return s.replayer.snap.Resources[s.index]
	}

	old := s.replayer.resources[s.index]
	s.replayer.resources[s.index] = new
	s.replayer.news[sequenceNumber] = int(s.index)
	return old
}

func (r *journalReplayer) invalidf(
	sequenceNumber int64,
	entry engine.JournalEntry,
	message string,
	args ...any,
) *InvalidJournalError {
	return &InvalidJournalError{
		SequenceNumber: sequenceNumber,
		Entry:          entry,
		Message:        fmt.Sprintf(message, args...),
	}
}

func (r *journalReplayer) getOld(sequenceNumber int64, entry engine.JournalEntry) (_ *journalState, err error) {
	s := entry.Old
	if s == nil {
		return nil, r.invalidf(sequenceNumber, entry, "missing old state")
	}

	hasBase, hasNew := s.Base != nil, s.New != nil
	if hasBase && hasNew || !hasBase && !hasNew {
		return nil, r.invalidf(sequenceNumber, entry, "expected exactly one of base or new in old state")
	}
	if hasBase {
		index := *s.Base
		if index >= int64(len(r.snap.Resources)) {
			return nil, r.invalidf(sequenceNumber, entry, "base index out of bounds")
		}
		if r.snap.Resources[index] == nil {
			return nil, r.invalidf(sequenceNumber, entry, "old base resource is nil")
		}
		return &journalState{replayer: r, index: *s.Base, isBase: true}, nil
	}

	index, ok := r.news[*entry.Old.New]
	if !ok {
		return nil, r.invalidf(sequenceNumber, entry, "missing index for old new state")
	}
	if index >= len(r.resources) {
		return nil, r.invalidf(sequenceNumber, entry, "new index out of bounds")
	}
	if r.resources[index] == nil {
		return nil, r.invalidf(sequenceNumber, entry, "old new resource is nil")
	}
	return &journalState{replayer: r, index: int64(index)}, nil
}

func (r *journalReplayer) replayEntry(sequenceNumber int64, entry engine.JournalEntry) (mustWrite bool, err error) {
	mustWrite = ((sequenceNumber >> 24) & 0xffffffffff) > r.journal.lastWrite

	switch entry.Kind {
	case engine.JournalEntryAddPendingOperation:
		r.pendingOperations[sequenceNumber] = *entry.PendingOperation
	case engine.JournalEntryRemovePendingOperation:
		if entry.RemovePendingOperation == nil {
			return false, r.invalidf(sequenceNumber, entry, "missing pending operation sequence number")
		}
		delete(r.pendingOperations, *entry.RemovePendingOperation)
	case engine.JournalEntryRemoveState:
		old, err := r.getOld(sequenceNumber, entry)
		if err != nil {
			return false, err
		}
		old.remove()

		if entry.InvalidateDependencies {
			r.rebuildDependencies = true
		}
	case engine.JournalEntryPatchState:
		if entry.State == nil {
			return false, r.invalidf(sequenceNumber, entry, "missing new state")
		}

		old, err := r.getOld(sequenceNumber, entry)
		if err != nil {
			return false, err
		}
		oldResource := old.patch(sequenceNumber, entry.State)

		if mustWrite && !entry.InvalidateDependencies {
			mustWrite = r.journal.mustWrite(oldResource, entry.State)
		}
		if entry.InvalidateDependencies {
			r.rebuildDependencies = true
		}
	case engine.JournalEntryAppendNewState:
		contract.Assertf(entry.State != nil, "missing new state")

		if entry.Old != nil {
			old, err := r.getOld(sequenceNumber, entry)
			if err != nil {
				return false, err
			}
			oldResource := old.remove()
			if mustWrite {
				mustWrite = r.journal.mustWrite(oldResource, entry.State)
			}
		}
		r.news[sequenceNumber] = len(r.resources)
		r.resources = append(r.resources, entry.State)
	case engine.JournalEntryWrite:
		// Already handled above.
	case engine.JournalEntryInvalid:
		fallthrough
	default:
		contract.Failf("unexpected journal entry kind %v", entry.Kind)
	}

	return mustWrite, nil
}

// rebuildDependencies rebuilds the dependencies of the resources in the snapshot based on the
// resources that are present in the snapshot. This is necessary if a refresh happens, because
// refreshes may delete resources, even if other resources still depend on them.
//
// This function is similar to 'rebuildBaseState' in the engine, but doesn't take care of
// rebuilding the resource list, since that's already done correctly by the journal.
//
// Note that this function assumes that resources are in reverse-dependency order.
func (r *journalReplayer) doRebuildDependencies(resources []*resource.State) {
	referenceable := make(map[resource.URN]bool)
	for i := range resources {
		newDeps := []resource.URN{}
		newPropDeps := map[resource.PropertyKey][]resource.URN{}

		_, allDeps := resources[i].GetAllDependencies()
		for _, dep := range allDeps {
			switch dep.Type {
			case resource.ResourceParent:
				// We handle parents separately later on (see undangleParentResources),
				// so we'll skip over them here.
				continue
			case resource.ResourceDependency:
				if referenceable[dep.URN] {
					newDeps = append(newDeps, dep.URN)
				}
			case resource.ResourcePropertyDependency:
				if referenceable[dep.URN] {
					newPropDeps[dep.Key] = append(newPropDeps[dep.Key], dep.URN)
				}
			case resource.ResourceDeletedWith:
				if !referenceable[dep.URN] {
					resources[i].DeletedWith = ""
				}
			}
		}
		if len(resources[i].Dependencies) > 0 {
			resources[i].Dependencies = newDeps
		}
		if len(resources[i].PropertyDependencies) > 0 {
			resources[i].PropertyDependencies = newPropDeps
		}
		referenceable[resources[i].URN] = true
	}
}

func (r *journalReplayer) finish() *deploy.Snapshot {
	// Filter out any resources that were later removed.
	out := 0
	for _, res := range r.resources {
		if res != nil {
			r.resources[out], out = res, out+1
		}
	}
	r.resources = r.resources[:out]

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
	if r.snap != nil {
		for i, res := range r.snap.Resources {
			if !r.baseRemoves.Has(int64(i)) {
				if patch, ok := r.basePatches[int64(i)]; ok {
					res = patch
				}
				r.resources = append(r.resources, res)
			}
		}
	}

	// Record any pending operations, if there are any outstanding that have not completed yet.
	operations := make([]resource.Operation, 0, len(r.pendingOperations))
	for _, op := range r.pendingOperations {
		operations = append(operations, op)
	}

	// Track pending create operations from the base snapshot
	// and propagate them to the new snapshot: we don't want to clear pending CREATE operations
	// because these must require user intervention to be cleared or resolved.
	if r.snap != nil {
		for _, pendingOperation := range r.snap.PendingOperations {
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

	// Rebuild dependencies if we had a refresh, as refreshes may delete resources,
	// which may cause other resources to have dangling dependencies.
	if r.rebuildDependencies {
		r.doRebuildDependencies(r.resources)
	}

	// The backend.SnapshotManager and backend.SnapshotPersister will keep track of any changes to
	// the Snapshot (checkpoint file) in the HTTP backend. We will reuse the snapshot's secrets manager when possible
	// to ensure that secrets are not re-encrypted on each update.
	secretsManager := r.journal.secretsManager
	if r.snap != nil && secrets.AreCompatible(secretsManager, r.snap.SecretsManager) {
		secretsManager = r.snap.SecretsManager
	}

	var metadata deploy.SnapshotMetadata
	if r.snap != nil {
		metadata = r.snap.Metadata
	}

	manifest.Magic = manifest.NewMagic()
	return deploy.NewSnapshot(manifest, secretsManager, r.resources, operations, metadata)
}

// snap produces a new Snapshot given the base snapshot and a list of resources that the current
// plan has created.
func (sj *SnapshotJournal) snap(mustWrite bool) (*deploy.Snapshot, bool, error) {
	r := newJournalReplayer(sj)
	for _, tx := range sj.transactions {
		contract.Assertf(tx.SequenceNumber != 0, "missing sequence number")

		for i, entry := range tx.Entries {
			sequenceNumber := ((tx.SequenceNumber & 0xffffffffff) << 24) | (int64(i) & 0xffffff)
			mustWriteEntry, err := r.replayEntry(sequenceNumber, entry)
			if err != nil {
				return nil, false, err
			}
			if mustWriteEntry {
				mustWrite = true
			}
		}
	}
	if !mustWrite {
		return nil, false, nil
	}

	return r.finish(), true, nil
}

// saveSnapshot persists the current snapshot. If integrity checking is enabled,
// the snapshot's integrity is also verified. If the snapshot is invalid,
// metadata about this write operation is added to the snapshot before it is
// written, in order to aid debugging should future operations fail with an
// error.
func (sj *SnapshotJournal) saveSnapshot(mustWrite bool) (bool, error) {
	snap, changed, err := sj.snap(mustWrite)
	if !changed || err != nil {
		return false, err
	}
	snap, err = snap.NormalizeURNReferences()
	if err != nil {
		return false, fmt.Errorf("failed to normalize URN references: %w", err)
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
		return true, fmt.Errorf("failed to save snapshot: %w", err)
	}
	if len(sj.transactions) != 0 {
		sj.lastWrite = sj.transactions[len(sj.transactions)-1].SequenceNumber
	}

	if !DisableIntegrityChecking && integrityError != nil {
		return true, fmt.Errorf("failed to verify snapshot: %w", integrityError)
	}
	return true, nil
}

func (sj *SnapshotJournal) append(tx engine.JournalTransaction) {
	for i, e := range tx.Entries {
		if e.Kind == engine.JournalEntryWrite {
			contract.Assertf(len(sj.transactions) == 0 && i == 0, "write must be the first journal entry")
			sj.snapshot = e.Snapshot
		}
	}
	sj.transactions = append(sj.transactions, tx)
}

// defaultServiceLoop saves a Snapshot whenever a mutation occurs
func (sj *SnapshotJournal) defaultServiceLoop(
	journalEvents chan writeJournalEntryRequest, done chan error,
) {
	// True if we have elided writes since the last actual write.
	hasElidedWrites := true

	// Service each mutation request in turn.
serviceLoop:
	for {
		select {
		case request := <-journalEvents:
			sj.append(request.transaction)
			written, err := sj.saveSnapshot(false)
			hasElidedWrites = err == nil && !written
			request.result <- err
		case <-sj.cancel:
			break serviceLoop
		}
	}

	// If we still have elided writes once the channel has closed, flush the snapshot.
	var err error
	if hasElidedWrites {
		logging.V(9).Infof("SnapshotManager: flushing elided writes...")
		_, err = sj.saveSnapshot(true)
	}
	done <- err
}

// unsafeServiceLoop doesn't save Snapshots when mutations occur and instead saves Snapshots when
// SnapshotManager.Close() is invoked. It trades reliability for speed as every mutation does not
// cause a Snapshot to be serialized to the user's state backend.
func (sj *SnapshotJournal) unsafeServiceLoop(
	journalEvents chan writeJournalEntryRequest, done chan error,
) {
	for {
		select {
		case request := <-journalEvents:
			sj.append(request.transaction)
			request.result <- nil
		case <-sj.cancel:
			_, err := sj.saveSnapshot(true)
			done <- err
			return
		}
	}
}

type SnapshotJournal struct {
	persister      SnapshotPersister
	snapshot       *deploy.Snapshot
	journalEvents  chan writeJournalEntryRequest
	transactions   []engine.JournalTransaction
	cancel         chan bool
	done           chan error
	secretsManager secrets.Manager
	lastWrite      int64
}

// NewSnapshotJournaler creates a new Journal that uses a SnapshotPersister to persist the
// snapshot created from the journal entries.
//
// The snapshot code works on journal entries. Each resource step produces new journal entries
// for beginning and finishing an operation. These journal entries can then be replayed
// in conjunction with the immutable base snapshot, to rebuild the new snapshot.
//
// Currently the backend only supports saving full snapshots, in which case only one journal
// entry is allowed to be processed at a time. In the future journal entries will be processed
// asynchronously in the cloud backend, allowing for better throughput for independent operations.
//
// Serialization is performed by pushing the journal entries onto a channel, where another
// goroutine is polling the channel and creating new snapshots using the entries as they come.
// This function optionally verifies the integrity of the snapshot before and after mutation.
//
// Each journal entry may indicate that its corresponding checkpoint write may be safely elided by
// setting the `ElideWrite` field. As of this writing, we only elide writes after same steps with no
// meaningful changes (see sameSnapshotMutation.mustWrite for details). Any elided writes
// are flushed by the next non-elided write or the next call to Close.
func NewSnapshotJournaler(
	persister SnapshotPersister,
	secretsManager secrets.Manager,
	baseSnap *deploy.Snapshot,
) *SnapshotJournal {
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

	journalEvents := make(chan writeJournalEntryRequest)
	done, cancel := make(chan error), make(chan bool)

	journaler := SnapshotJournal{
		persister:      persister,
		snapshot:       snapCopy,
		journalEvents:  journalEvents,
		transactions:   make([]engine.JournalTransaction, 0),
		secretsManager: secretsManager,
		cancel:         cancel,
		done:           done,
	}

	serviceLoop := journaler.defaultServiceLoop

	if env.SkipCheckpoints.Value() {
		serviceLoop = journaler.unsafeServiceLoop
	}

	go serviceLoop(journalEvents, done)

	return &journaler
}

func (sj *SnapshotJournal) Entries() []engine.JournalTransaction {
	return sj.transactions
}

type writeJournalEntryRequest struct {
	transaction engine.JournalTransaction
	result      chan error
}

func (sj *SnapshotJournal) Append(transaction engine.JournalTransaction) error {
	result := make(chan error)
	select {
	case sj.journalEvents <- writeJournalEntryRequest{transaction: transaction, result: result}:
		// We don't need to check for cancellation here, as the service loop guarantees
		// that it will return a result for every journal entry that it processes.
		return <-result
	case <-sj.cancel:
		return errors.New("journal closed")
	}
}

func (sj SnapshotJournal) Close() error {
	sj.cancel <- true
	return <-sj.done
}
