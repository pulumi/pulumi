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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/maputil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
)

func SerializeJournalEntry(
	ctx context.Context, je engine.JournalEntry, enc config.Encrypter,
) (apitype.JournalEntry, error) {
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
	var secretsManager *apitype.SecretsProvidersV1
	if je.SecretsManager != nil {
		secretsManager = &apitype.SecretsProvidersV1{
			Type:  je.SecretsManager.Type(),
			State: je.SecretsManager.State(),
		}
	}

	var snapshot *apitype.DeploymentV3
	if je.NewSnapshot != nil {
		var err error
		snapshot, err = stack.SerializeDeployment(ctx, je.NewSnapshot, false)
		if err != nil {
			return apitype.JournalEntry{}, fmt.Errorf("serializing new snapshot: %w", err)
		}
	}

	serializedEntry := apitype.JournalEntry{
		Kind:                  apitype.JournalEntryKind(je.Kind),
		SequenceID:            je.SequenceID,
		OperationID:           je.OperationID,
		RemoveOld:             je.RemoveOld,
		RemoveNew:             je.RemoveNew,
		State:                 state,
		Operation:             operation,
		SecretsProvider:       secretsManager,
		PendingReplacementOld: je.PendingReplacementOld,
		PendingReplacementNew: je.PendingReplacementNew,
		DeleteOld:             je.DeleteOld,
		DeleteNew:             je.DeleteNew,
		IsRefresh:             je.IsRefresh,
		NewSnapshot:           snapshot,
	}

	return serializedEntry, nil
}

type JournalReplayer struct {
	// toRemove tracks operation IDs of resources that are to be removed.
	toRemove map[int64]struct{}
	// toDeleteInSnapshot tracks the indices of resources in the snapshot that are to be deleted.
	toDeleteInSnapshot map[int64]struct{}
	// toReplaceInSnapshot tracks indices of resources in the snapshot that are to be replaced.
	toReplaceInSnapshot map[int64]*apitype.ResourceV3
	// markAsDeletion tracks indices of resources in the snapshot that are to be marked for deletion.
	markAsDeletion map[int64]struct{}
	// markAsPendingReplacement tracks indices of resources in the snapshot that are to be marked for pending replacement.
	markAsPendingReplacement map[int64]struct{}

	// operationIDToResourceIndex maps operation IDs to resource indices in the new resource list.
	// This is used to replace resources that are being replaced, and remove new resources that are being deleted.
	operationIDToResourceIndex map[int64]int64

	// incompleteOps tracks operations that have begun but not yet completed.
	incompleteOps map[int64]apitype.JournalEntry

	// hasRefresh indicates whether any of the journal entries were part of a refresh operation.
	hasRefresh bool

	// index is the current index in the new resource list.
	index int64

	// base is the base snapshot.
	base *apitype.DeploymentV3

	// newResources is the list of new resources created by the current plan.
	newResources []*apitype.ResourceV3
}

func NewJournalReplayer(base *apitype.DeploymentV3) *JournalReplayer {
	replayer := JournalReplayer{
		toRemove:                   make(map[int64]struct{}),
		toDeleteInSnapshot:         make(map[int64]struct{}),
		toReplaceInSnapshot:        make(map[int64]*apitype.ResourceV3),
		markAsDeletion:             make(map[int64]struct{}),
		markAsPendingReplacement:   make(map[int64]struct{}),
		operationIDToResourceIndex: make(map[int64]int64),
		incompleteOps:              make(map[int64]apitype.JournalEntry),
		newResources:               make([]*apitype.ResourceV3, 0),
		base:                       base,
	}
	return &replayer
}

func (r *JournalReplayer) Add(entry apitype.JournalEntry) {
	switch entry.Kind {
	case apitype.JournalEntryKindBegin:
		r.incompleteOps[entry.OperationID] = entry
	case apitype.JournalEntryKindSuccess:
		delete(r.incompleteOps, entry.OperationID)
		// If this is a success, we need to add the resource to the list of resources.
		if entry.State != nil {
			r.newResources = append(r.newResources, entry.State)
			r.operationIDToResourceIndex[entry.OperationID] = r.index
			r.index++
		}
		if entry.RemoveOld != nil {
			r.toDeleteInSnapshot[*entry.RemoveOld] = struct{}{}
		}
		if entry.RemoveNew != nil {
			r.toRemove[*entry.RemoveNew] = struct{}{}
		}
		if entry.DeleteOld != nil {
			r.markAsDeletion[*entry.DeleteOld] = struct{}{}
		}
		if entry.DeleteNew != nil {
			r.newResources[r.operationIDToResourceIndex[*entry.DeleteNew]].Delete = true
		}
		if entry.PendingReplacementOld != nil {
			r.markAsPendingReplacement[*entry.PendingReplacementOld] = struct{}{}
		}
		if entry.PendingReplacementNew != nil {
			r.newResources[r.operationIDToResourceIndex[*entry.PendingReplacementNew]].PendingReplacement = true
		}

		if entry.IsRefresh {
			r.hasRefresh = true
		}
	case apitype.JournalEntryKindRefreshSuccess:
		delete(r.incompleteOps, entry.OperationID)
		r.hasRefresh = true
		if entry.RemoveOld != nil {
			if entry.State == nil {
				r.toDeleteInSnapshot[*entry.RemoveOld] = struct{}{}
			} else {
				r.toReplaceInSnapshot[*entry.RemoveOld] = entry.State
			}
		}
		if entry.RemoveNew != nil {
			if entry.State == nil {
				r.toRemove[*entry.RemoveNew] = struct{}{}
			} else {
				r.newResources[r.operationIDToResourceIndex[*entry.RemoveNew]] = entry.State
			}
		}
	case apitype.JournalEntryKindFailure:
		delete(r.incompleteOps, entry.OperationID)
	case apitype.JournalEntryKindOutputs:
		if entry.State != nil && entry.RemoveOld != nil {
			r.toReplaceInSnapshot[*entry.RemoveOld] = entry.State
		}
		if entry.State != nil && entry.RemoveNew != nil {
			r.newResources[r.operationIDToResourceIndex[*entry.RemoveNew]] = entry.State
		}
	case apitype.JournalEntryKindWrite:
		// Already handled above.
	case apitype.JournalEntryKindSecretsManager:
		// The backend.SnapshotManager and backend.SnapshotPersister will keep track of any changes to
		// the Snapshot (checkpoint file) in the HTTP backend. We will reuse the snapshot's secrets manager when possible
		// to ensure that secrets are not re-encrypted on each update.
		secretsProvider := entry.SecretsProvider
		if r.base.SecretsProviders != nil &&
			(secretsProvider.Type == r.base.SecretsProviders.Type &&
				bytes.Equal(secretsProvider.State, r.base.SecretsProviders.State)) {
			return
		}

		r.base.SecretsProviders = entry.SecretsProvider
	}
}

// rebuildDependencies rebuilds the dependencies of the resources in the snapshot based on the
// resources that are present in the snapshot. This is necessary if a refresh happens, because
// refreshes may delete resources, even if other resources still depend on them.
//
// This function is similar to 'rebuildBaseState' in the engine, but doesn't take care of
// rebuilding the resource list, since that's already done correctly by the journal.
//
// Note that this function assumes that resources are in reverse-dependency order.
func rebuildDependencies(resources []apitype.ResourceV3) {
	referenceable := make(map[resource.URN]bool)
	for i := range resources {
		newDeps := []resource.URN{}
		newPropDeps := make(map[resource.PropertyKey][]resource.URN)
		for _, dep := range resources[i].Dependencies {
			if referenceable[dep] {
				newDeps = append(newDeps, dep)
			}
		}
		for k := range resources[i].PropertyDependencies {
			for _, dep := range resources[i].PropertyDependencies[k] {
				if referenceable[dep] {
					newPropDeps[k] = append(newPropDeps[k], dep)
				}
			}
		}
		if !referenceable[resources[i].DeletedWith] {
			resources[i].DeletedWith = ""
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

func (r *JournalReplayer) GenerateDeployment() (*apitype.DeploymentV3, int, []string) {
	features := make(map[string]bool)
	removeIndices := make(map[int64]struct{})
	for k := range r.toRemove {
		removeIndices[r.operationIDToResourceIndex[k]] = struct{}{}
	}

	resources := make([]apitype.ResourceV3, 0)
	for i, res := range r.newResources {
		if _, ok := removeIndices[int64(i)]; !ok {
			resources = append(resources, *res)
			stack.ApplyFeatures(*res, features)
		}
	}

	// Append any resources from the base plan that were not produced by the current plan.
	if r.base != nil {
		for i, res := range r.base.Resources {
			if _, ok := r.toDeleteInSnapshot[int64(i)]; !ok {
				if _, ok := r.markAsPendingReplacement[int64(i)]; ok {
					res.PendingReplacement = true
				}

				if state, ok := r.toReplaceInSnapshot[int64(i)]; ok {
					// If this is a resource that was replaced, we want to
					// replace it in the snapshot.  We only do so if the same
					// resource has not been marked for deletion.  This
					// could happen, e.g. if a refresh happens first (so
					// we're supposed to replace the resource), and then a
					// delete happens (so we're supposed to delete the resource).
					resources = append(resources, *state)
					stack.ApplyFeatures(*state, features)
				} else {
					if _, ok := r.markAsDeletion[int64(i)]; ok {
						res.Delete = true
					}
					resources = append(resources, res)
					stack.ApplyFeatures(res, features)
				}
			}
		}
	}

	// Record any pending operations, if there are any outstanding that have not completed yet.
	var operations []apitype.OperationV2
	for _, op := range r.incompleteOps {
		if op.Operation != nil {
			operations = append(operations, *op.Operation)
			stack.ApplyFeatures(op.Operation.Resource, features)
		}
	}

	// Track pending create operations from the base snapshot
	// and propagate them to the new snapshot: we don't want to clear pending CREATE operations
	// because these must require user intervention to be cleared or resolved.
	if base := r.base; base != nil {
		for _, pendingOperation := range base.PendingOperations {
			if pendingOperation.Type == apitype.OperationTypeCreating {
				operations = append(operations, pendingOperation)
				stack.ApplyFeatures(pendingOperation.Resource, features)
			}
		}
	}

	if r.hasRefresh {
		// Rebuild dependencies if we had a refresh, as refreshes may delete resources,
		// which may cause other resources to have dangling dependencies.
		rebuildDependencies(resources)
	}

	manifest := deploy.Manifest{
		Time:    time.Now(),
		Version: version.Version,
		// Plugins: sm.plugins, - Explicitly dropped, since we don't use the plugin list in the manifest anymore.
	}
	manifest.Magic = manifest.NewMagic()

	deployment := &apitype.DeploymentV3{}
	deployment.SecretsProviders = r.base.SecretsProviders
	deployment.Resources = resources
	deployment.PendingOperations = operations
	deployment.Metadata = r.base.Metadata
	deployment.Manifest = manifest.Serialize()

	version := apitype.DeploymentSchemaVersionCurrent
	if len(features) > 0 {
		version = apitype.DeploymentSchemaVersionLatest
	}

	return deployment, version, maputil.SortedKeys(features)
}

// snap produces a new Snapshot given the base snapshot and a list of resources that the current
// plan has created.
func (sj *snapshotJournaler) snap(ctx context.Context) (*deploy.Snapshot, error) {
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
	snap := sj.snapshot

	// We could have a rebase entry as one of the first two journal entries. If we do use
	// the snapshot from that entry as the base snapshot.
	if len(sj.journalEntries) >= 2 {
		if firstEntry := sj.journalEntries[0]; firstEntry.Kind == apitype.JournalEntryKindWrite {
			snap = firstEntry.NewSnapshot
		} else if secondEntry := sj.journalEntries[1]; secondEntry.Kind == apitype.JournalEntryKindWrite {
			snap = secondEntry.NewSnapshot
		}
	}

	replayer := NewJournalReplayer(snap)

	// Record any pending operations, if there are any outstanding that have not completed yet.
	for _, entry := range sj.journalEntries {
		replayer.Add(entry)
	}

	deploymentV3, _, _ := replayer.GenerateDeployment()

	return stack.DeserializeDeploymentV3(ctx, *deploymentV3, sj.secretsProvider)
}

// saveSnapshot persists the current snapshot. If integrity checking is enabled,
// the snapshot's integrity is also verified. If the snapshot is invalid,
// metadata about this write operation is added to the snapshot before it is
// written, in order to aid debugging should future operations fail with an
// error.
func (sj *snapshotJournaler) saveSnapshot(ctx context.Context) error {
	snap, err := sj.snap(ctx)
	if err != nil {
		return fmt.Errorf("failed to generate snapshot: %w", err)
	}
	if snap == nil {
		return nil
	}
	snap, err = snap.NormalizeURNReferences()
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
	ctx context.Context,
	journalEvents chan writeJournalEntryRequest, done chan error,
) {
	// True if we have elided writes since the last actual write.
	hasElidedWrites := true

	setSecretsManager := false

	// Service each mutation request in turn.
serviceLoop:
	for {
		select {
		case request := <-journalEvents:
			sj.journalEntries = append(sj.journalEntries, request.journalEntry)
			if request.elideWrite {
				hasElidedWrites = true
				if request.result != nil {
					request.result <- nil
				}
				continue
			}
			hasElidedWrites = false
			request.result <- sj.saveSnapshot(ctx)
		case <-sj.cancel:
			break serviceLoop
		}
		if !setSecretsManager {
			setSecretsManager = true
			_ = sj.BeginOperation(engine.JournalEntry{
				Kind:           engine.JournalEntrySecretsManager,
				SecretsManager: sj.secretsManager,
			})
		}
	}

	// If we still have elided writes once the channel has closed, flush the snapshot.
	var err error
	if hasElidedWrites {
		logging.V(9).Infof("SnapshotManager: flushing elided writes...")
		err = sj.saveSnapshot(ctx)
	}
	done <- err
}

// unsafeServiceLoop doesn't save Snapshots when mutations occur and instead saves Snapshots when
// SnapshotManager.Close() is invoked. It trades reliability for speed as every mutation does not
// cause a Snapshot to be serialized to the user's state backend.
func (sj *snapshotJournaler) unsafeServiceLoop(
	ctx context.Context,
	journalEvents chan writeJournalEntryRequest, done chan error,
) {
	setSecretsManager := false
	for {
		select {
		case request := <-journalEvents:
			sj.journalEntries = append(sj.journalEntries, request.journalEntry)
			request.result <- nil
		case <-sj.cancel:
			done <- sj.saveSnapshot(ctx)
			return
		}
		if !setSecretsManager {
			setSecretsManager = true
			_ = sj.BeginOperation(engine.JournalEntry{
				Kind:           engine.JournalEntrySecretsManager,
				SecretsManager: sj.secretsManager,
			})
		}

	}
}

type snapshotJournaler struct {
	ctx             context.Context
	persister       SnapshotPersister
	snapshot        *apitype.DeploymentV3
	journalEvents   chan writeJournalEntryRequest
	journalEntries  []apitype.JournalEntry
	cancel          chan bool
	done            chan error
	secretsManager  secrets.Manager
	secretsProvider secrets.Provider
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
	ctx context.Context,
	persister SnapshotPersister,
	secretsManager secrets.Manager,
	secretsProvider secrets.Provider,
	baseSnap *deploy.Snapshot,
) (engine.Journal, error) {
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

		if snapCopy.SecretsManager == nil {
			snapCopy.SecretsManager = secretsManager
		}
	}

	journalEvents := make(chan writeJournalEntryRequest)
	done, cancel := make(chan error), make(chan bool)

	var deployment *apitype.DeploymentV3
	if baseSnap != nil {
		var err error
		deployment, _, _, err = stack.SerializeDeploymentWithMetadata(
			ctx, snapCopy, false)
		if err != nil {
			return nil, err
		}
	} else {
		deployment = &apitype.DeploymentV3{}
	}

	journaler := snapshotJournaler{
		ctx:             ctx,
		persister:       persister,
		snapshot:        deployment,
		journalEvents:   journalEvents,
		journalEntries:  make([]apitype.JournalEntry, 0),
		secretsManager:  secretsManager,
		secretsProvider: secretsProvider,
		cancel:          cancel,
		done:            done,
	}

	serviceLoop := journaler.defaultServiceLoop

	if env.SkipCheckpoints.Value() {
		serviceLoop = journaler.unsafeServiceLoop
	}

	go serviceLoop(ctx, journalEvents, done)

	return &journaler, nil
}

type writeJournalEntryRequest struct {
	journalEntry apitype.JournalEntry
	elideWrite   bool
	result       chan error
}

func (sj *snapshotJournaler) journalMutation(entry engine.JournalEntry) error {
	serializedEntry, err := SerializeJournalEntry(
		sj.ctx, entry, sj.secretsManager.Encrypter())
	if err != nil {
		return fmt.Errorf("failed to serialize journal entry: %w", err)
	}

	result := make(chan error)
	select {
	case sj.journalEvents <- writeJournalEntryRequest{
		journalEntry: serializedEntry,
		elideWrite:   entry.ElideWrite,
		result:       result,
	}:
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

	deployment, _, _, err := stack.SerializeDeploymentWithMetadata(
		sj.ctx, snapCopy, true)
	if err != nil {
		return fmt.Errorf("serializing base snapshot: %w", err)
	}

	sj.snapshot = deployment
	return sj.journalMutation(engine.JournalEntry{
		Kind:        engine.JournalEntryWrite,
		NewSnapshot: snapCopy,
	})
}

func (sj snapshotJournaler) Close() error {
	sj.cancel <- true
	return <-sj.done
}
