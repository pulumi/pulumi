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
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
)

// rebuildDependencies rebuilds the dependencies of the resources in the snapshot based on the
// resources that are present in the snapshot. This is necessary if a refresh happens, because
// refreshes may delete resources, even if other resources still depend on them.
//
// This function is similar to 'rebuildBaseState' in the engine, but doesn't take care of
// rebuilding the resource list, since that's already done correctly by the journal.
//
// Note that this function assumes that resources are in reverse-dependency order.
func (sj *snapshotJournaler) rebuildDependencies(resources []*resource.State) {
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

	snap := sj.snapshot

	for _, entry := range sj.journalEntries {
		if entry.Kind == engine.JournalEntryWrite {
			snap = entry.NewSnapshot
			contract.Assertf(entry.OperationID == 0, "rebase journal entry must not have an operation ID")
		}
	}

	// toDelete tracks operation IDs of resources that are to be deleted.
	toDelete := make(map[int64]struct{})
	// toDeleteInSnapshot tracks the indices of resources in the snapshot that are to be deleted.
	toDeleteInSnapshot := make(map[int64]struct{})
	// toReplace tracks operation IDs of resources that are to be replaced.
	toReplace := make(map[int64]*resource.State)
	// toReplaceInSnapshot tracks indices of resources in the snapshot that are to be replaced.
	toReplaceInSnapshot := make(map[int64]*resource.State)
	// markAsDeletion tracks indices of resources in the snapshot that are to be marked for deletion.
	markAsDeletion := make(map[int64]struct{})
	// markAsPendingReplacement tracks indices of resources in the snapshot that are to be marked for pending replacement.
	markAsPendingReplacement := make(map[int64]struct{})
	for _, entry := range sj.journalEntries {
		if entry.Kind == engine.JournalEntrySuccess && entry.DeleteNew != 0 {
			toDelete[entry.DeleteNew] = struct{}{}
		}

		if entry.Kind == engine.JournalEntryRefreshSuccess && entry.State != nil {
			// If we have a refresh, and the resource is not being deleted,
			// we want to substitute the old resource, instead of appending
			// it to the end.
			if entry.DeleteNew != 0 {
				toReplace[entry.DeleteNew] = entry.State
			}
		}

		if entry.Kind == engine.JournalEntryOutputs && entry.State != nil && !entry.ElideWrite {
			// Similar to refreshes, if we have new outputs, we need to *replace* the
			// old resource at the same place in the resource list as the new one.
			if entry.DeleteNew != 0 {
				toReplace[entry.DeleteNew] = entry.State
			}
		}
	}

	incompleteOps := make(map[int64]engine.JournalEntry)
	hasRefresh := false
	// Record any pending operations, if there are any outstanding that have not completed yet.
	for _, entry := range sj.journalEntries {
		switch entry.Kind {
		case engine.JournalEntryBegin:
			incompleteOps[entry.OperationID] = entry
			if entry.DeleteOld >= 0 {
				markAsDeletion[entry.DeleteOld] = struct{}{}
			}
		case engine.JournalEntrySuccess:
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
			if entry.PendingReplacement >= 0 {
				markAsPendingReplacement[entry.PendingReplacement] = struct{}{}
			}
			if entry.IsRefresh {
				hasRefresh = true
			}
		case engine.JournalEntryRefreshSuccess:
			delete(incompleteOps, entry.OperationID)
			hasRefresh = true
			if entry.DeleteOld >= 0 {
				if entry.State == nil {
					toDeleteInSnapshot[entry.DeleteOld] = struct{}{}
				} else {
					toReplaceInSnapshot[entry.DeleteOld] = entry.State
				}
			}
		case engine.JournalEntryFailure:
			op := incompleteOps[entry.OperationID]
			if op.Kind == engine.JournalEntryBegin {
				// If we marked this resource for deletion earlier, we need to
				// undo that if the operation failed.
				if _, ok := markAsDeletion[op.DeleteOld]; ok {
					delete(markAsDeletion, op.DeleteOld)
					sj.snapshot.Resources[op.DeleteOld].Delete = false
				}
			}
			delete(incompleteOps, entry.OperationID)
		case engine.JournalEntryOutputs:
			if entry.State != nil && !entry.ElideWrite && entry.DeleteOld >= 0 {
				toReplaceInSnapshot[entry.DeleteOld] = entry.State
			}
		case engine.JournalEntryWrite:
			// Already handled above.
		}
	}

	// Append any resources from the base plan that were not produced by the current plan.
	if snap != nil {
		for i, res := range snap.Resources {
			if _, ok := toDeleteInSnapshot[int64(i)]; !ok {
				if _, ok := markAsPendingReplacement[int64(i)]; ok {
					res.PendingReplacement = true
				}

				if state, ok := toReplaceInSnapshot[int64(i)]; ok {
					// If this is a resource that was replaced, we want to
					// replace it in the snapshot.  We only do so if the same
					// resource has not been marked for deletion.  This
					// could happen, e.g. if a refresh happens first (so
					// we're supposed to replace the resource), and then a
					// delete happens (so we're supposed to delete the resource).
					resources = append(resources, state)
				} else {
					if _, ok := markAsDeletion[int64(i)]; ok {
						res.Delete = true
					}
					resources = append(resources, res)
				}
			}
		}
	}

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
	if base := snap; base != nil {
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

	if hasRefresh {
		// Rebuild dependencies if we had a refresh, as refreshes may delete resources,
		// which may cause other resources to have dangling dependencies.
		sj.rebuildDependencies(resources)
	}

	// The backend.SnapshotManager and backend.SnapshotPersister will keep track of any changes to
	// the Snapshot (checkpoint file) in the HTTP backend. We will reuse the snapshot's secrets manager when possible
	// to ensure that secrets are not re-encrypted on each update.
	secretsManager := sj.secretsManager
	if snap != nil && secrets.AreCompatible(secretsManager, snap.SecretsManager) {
		secretsManager = snap.SecretsManager
	}

	var metadata deploy.SnapshotMetadata
	if snap != nil {
		metadata = snap.Metadata
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
	journalEvents chan writeJournalEntryRequest, done chan error,
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
	journalEvents chan writeJournalEntryRequest, done chan error,
) {
	for {
		select {
		case request := <-journalEvents:
			if request.JournalEntry.Kind == engine.JournalEntryWrite {
				contract.Assertf(len(sj.journalEntries) == 0, "should not have seen an jornalentry before a rebase")
			}
			sj.journalEntries = append(sj.journalEntries, request.JournalEntry)
			request.result <- nil
		case <-sj.cancel:
			done <- sj.saveSnapshot()
			return
		}
	}
}

type snapshotJournaler struct {
	persister      SnapshotPersister
	snapshot       *deploy.Snapshot
	journalEvents  chan writeJournalEntryRequest
	journalEntries []engine.JournalEntry
	cancel         chan bool
	done           chan error
	secretsManager secrets.Manager
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
// asynchronously in the cloud backend, allowing for better throughput for independent operations..
//
// Serialization is performed by pushing the journal entries onto a channel, where another
// goroutine is polling the channel and creating new snapshots using the entries as they come.
// This function optionally verifies the integrity of the snapshot before and after mutation.
//
// Each journal entry may indicate that its corresponding checkpoint write may be safely elided by
// setting the `ElideWrite` fiield. As of this writing, we only elide writes after same steps with no
// meaningful changes (see sameSnapshotMutation.mustWrite for details). Any elided writes
// are flushed by the next non-elided write or the next call to Close.
func NewSnapshotJournaler(
	persister SnapshotPersister,
	secretsManager secrets.Manager,
	baseSnap *deploy.Snapshot,
) engine.Journal {
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

	journaler := snapshotJournaler{
		persister:      persister,
		snapshot:       snapCopy,
		journalEvents:  journalEvents,
		journalEntries: make([]engine.JournalEntry, 0),
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

type writeJournalEntryRequest struct {
	JournalEntry engine.JournalEntry
	result       chan error
}

func (sj *snapshotJournaler) journalMutation(entry engine.JournalEntry) error {
	result := make(chan error)
	select {
	case sj.journalEvents <- writeJournalEntryRequest{JournalEntry: entry, result: result}:
		// We don't need to check for cancellation here, as the service loop guarantees
		// that it will return a result for every journal entry that it processes.
		return <-result
	case <-sj.cancel:
		return errors.New("snapshot manager closed")
	}
}

func (sj *snapshotJournaler) BeginOperation(entry engine.JournalEntry) error {
	return sj.journalMutation(entry)
}

func (sj *snapshotJournaler) EndOperation(entry engine.JournalEntry) error {
	return sj.journalMutation(entry)
}

func (sj *snapshotJournaler) Write(newBase *deploy.Snapshot) error {
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
	return sj.journalMutation(engine.JournalEntry{
		Kind:        engine.JournalEntryWrite,
		NewSnapshot: snapCopy,
	})
}

func (sj snapshotJournaler) Close() error {
	sj.cancel <- true
	return <-sj.done
}
