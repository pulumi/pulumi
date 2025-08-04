// Copyright 2016-2022, Pulumi Corporation.
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
	"errors"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var _ = SnapshotManager((*TestJournal)(nil))

type JournalEntryKind int

const (
	JournalEntryBegin   JournalEntryKind = 0
	JournalEntrySuccess JournalEntryKind = 1
	JournalEntryFailure JournalEntryKind = 2
	JournalEntryOutputs JournalEntryKind = 4
)

type JournalEntry struct {
	Kind JournalEntryKind
	Step deploy.Step
}

type JournalEntries []JournalEntry

func (entries JournalEntries) Snap(base *deploy.Snapshot) (*deploy.Snapshot, error) {
	// Build up a list of current resources by replaying the journal.
	resources, dones := []*resource.State{}, make(map[*resource.State]bool)
	refreshDeletes := make(map[resource.URN]bool)
	ops, doneOps := []resource.Operation{}, make(map[*resource.State]bool)
	for _, e := range entries {
		logging.V(7).Infof("%v %v (%v)", e.Step.Op(), e.Step.URN(), e.Kind)

		// Begin journal entries add pending operations to the snapshot. As we see success or failure
		// entries, we'll record them in doneOps.
		switch e.Kind {
		case JournalEntryBegin:
			switch e.Step.Op() {
			case deploy.OpCreate, deploy.OpCreateReplacement:
				ops = append(ops, resource.NewOperation(e.Step.New(), resource.OperationTypeCreating))
			case deploy.OpDelete, deploy.OpDeleteReplaced, deploy.OpReadDiscard, deploy.OpDiscardReplaced:
				ops = append(ops, resource.NewOperation(e.Step.Old(), resource.OperationTypeDeleting))
			case deploy.OpRead, deploy.OpReadReplacement:
				ops = append(ops, resource.NewOperation(e.Step.New(), resource.OperationTypeReading))
			case deploy.OpUpdate:
				ops = append(ops, resource.NewOperation(e.Step.New(), resource.OperationTypeUpdating))
			case deploy.OpImport, deploy.OpImportReplacement:
				ops = append(ops, resource.NewOperation(e.Step.New(), resource.OperationTypeImporting))
			}
		case JournalEntryFailure, JournalEntrySuccess:
			switch e.Step.Op() {
			//nolint:lll
			case deploy.OpCreate, deploy.OpCreateReplacement, deploy.OpRead, deploy.OpReadReplacement, deploy.OpUpdate,
				deploy.OpImport, deploy.OpImportReplacement:
				doneOps[e.Step.New()] = true
			case deploy.OpDelete, deploy.OpDeleteReplaced, deploy.OpReadDiscard, deploy.OpDiscardReplaced:
				doneOps[e.Step.Old()] = true
			}
		case JournalEntryOutputs:
			// We do nothing for outputs, since they don't affect the snapshot.
		}

		// Now mark resources done as necessary.
		if e.Kind == JournalEntrySuccess {
			switch e.Step.Op() {
			case deploy.OpSame:
				step, ok := e.Step.(*deploy.SameStep)
				if !ok || !step.IsSkippedCreate() {
					resources = append(resources, e.Step.New())
					dones[e.Step.Old()] = true
				}
			case deploy.OpUpdate:
				resources = append(resources, e.Step.New())
				dones[e.Step.Old()] = true
			case deploy.OpCreate, deploy.OpCreateReplacement:
				resources = append(resources, e.Step.New())
				if old := e.Step.Old(); old != nil && old.PendingReplacement {
					dones[old] = true
				}
			case deploy.OpDelete, deploy.OpDeleteReplaced, deploy.OpReadDiscard, deploy.OpDiscardReplaced:
				if old := e.Step.Old(); !old.PendingReplacement {
					dones[old] = true
				}
			case deploy.OpReplace:
				// do nothing.
			case deploy.OpRead, deploy.OpReadReplacement:
				resources = append(resources, e.Step.New())
				if e.Step.Old() != nil {
					dones[e.Step.Old()] = true
				}
			case deploy.OpRemovePendingReplace:
				dones[e.Step.Old()] = true
			case deploy.OpImport, deploy.OpImportReplacement:
				resources = append(resources, e.Step.New())
			case deploy.OpRefresh:
				refreshStep, isRefreshStep := e.Step.(*deploy.RefreshStep)
				viewStep, isViewStep := e.Step.(*deploy.ViewStep)
				if (isViewStep && viewStep.Persisted()) || (isRefreshStep && refreshStep.Persisted()) {
					if e.Step.New() != nil {
						resources = append(resources, e.Step.New())
					} else {
						refreshDeletes[e.Step.Old().URN] = true
					}
					dones[e.Step.Old()] = true
				}
			}
		}
	}

	// Filter any resources that had an operation (like same or update) but then were deleted by a later
	// operations. This can happen from program based destroy operations were we'll see an event come in to
	// Same/Update/Create a resource and so add it to the `resources` list, but then later see a delete
	// operation for that same resource. In that case, we want to filter out the resource from the list of
	// resources before writing the actual snapshot.
	filteredResources := []*resource.State{}
	for _, res := range resources {
		if !dones[res] {
			filteredResources = append(filteredResources, res)
		}
	}

	// Append any resources from the base snapshot that were not produced by the current snapshot.
	// See backend.SnapshotManager.snap for why this works.
	if base != nil {
		for _, res := range base.Resources {
			if !dones[res] {
				filteredResources = append(filteredResources, res)
			}
		}
	}

	FilterRefreshDeletes(refreshDeletes, filteredResources)

	// Append any pending operations.
	var operations []resource.Operation
	for _, op := range ops {
		if !doneOps[op.Resource] {
			operations = append(operations, op)
		}
	}

	if base != nil {
		// Track pending create operations from the base snapshot
		// and propagate them to the new snapshot: we don't want to clear pending CREATE operations
		// because these must require user intervention to be cleared or resolved.
		for _, pendingOperation := range base.PendingOperations {
			if pendingOperation.Type == resource.OperationTypeCreating {
				operations = append(operations, pendingOperation)
			}
		}
	}

	// If we have a base snapshot, copy over its secrets manager and metadata.
	var secretsManager secrets.Manager
	var metadata deploy.SnapshotMetadata
	if base != nil {
		secretsManager = base.SecretsManager
		metadata = base.Metadata
	}

	manifest := deploy.Manifest{}
	manifest.Magic = manifest.NewMagic()

	snap := deploy.NewSnapshot(manifest, secretsManager, filteredResources, operations, metadata)
	normSnap, err := snap.NormalizeURNReferences()
	if err != nil {
		return snap, err
	}
	return normSnap, normSnap.VerifyIntegrity()
}

type TestJournal struct {
	entries JournalEntries
	events  chan JournalEntry
	cancel  chan bool
	done    chan bool
}

func (j *TestJournal) Entries() JournalEntries {
	<-j.done

	return j.entries
}

func (j *TestJournal) Close() error {
	close(j.cancel)
	<-j.done

	return nil
}

func (j *TestJournal) BeginMutation(step deploy.Step) (SnapshotMutation, error) {
	select {
	case j.events <- JournalEntry{Kind: JournalEntryBegin, Step: step}:
		return j, nil
	case <-j.cancel:
		return nil, errors.New("journal closed")
	}
}

func (j *TestJournal) End(step deploy.Step, success bool) error {
	kind := JournalEntryFailure
	if success {
		kind = JournalEntrySuccess
	}
	select {
	case j.events <- JournalEntry{Kind: kind, Step: step}:
		return nil
	case <-j.cancel:
		return errors.New("journal closed")
	}
}

func (j *TestJournal) RegisterResourceOutputs(step deploy.Step) error {
	select {
	case j.events <- JournalEntry{Kind: JournalEntryOutputs, Step: step}:
		return nil
	case <-j.cancel:
		return errors.New("journal closed")
	}
}

func (j *TestJournal) RecordPlugin(plugin workspace.PluginInfo) error {
	return nil
}

func (j *TestJournal) Snap(base *deploy.Snapshot) (*deploy.Snapshot, error) {
	return j.entries.Snap(base)
}

// NewTestJournal creates a new TestJournal that is used in tests to record journal entries for
// deployment steps. These journal entries are used to reconstruct the snapshot at the end of
// the test. This is used in lifecycletests to check that the snapshot manager and the testjournal
// produce the same snapshot.
func NewTestJournal() *TestJournal {
	j := &TestJournal{
		events: make(chan JournalEntry),
		cancel: make(chan bool),
		done:   make(chan bool),
	}
	go func() {
		for {
			select {
			case <-j.cancel:
				close(j.done)
				return
			case e := <-j.events:
				j.entries = append(j.entries, e)
			}
		}
	}()
	return j
}

// FilterRefreshDeletes filters out any dependencies and parents from 'resources' that refer to a URN that has
// been deleted by a refresh operation. This is pretty much the same as `rebuildBaseState` in the deployment
// executor (see that function for a lot of details about why this is necessary). The main difference is that
// this function does not mutate the state objects in place instead returning a new state object with the
// appropriate fields filtered out, note that the slice containing the states is mutated.
func FilterRefreshDeletes(
	refreshDeletes map[resource.URN]bool,
	resources []*resource.State,
) {
	availableParents := map[resource.URN]resource.URN{}

	for i, res := range resources {
		newDeps := []resource.URN{}
		newPropDeps := map[resource.PropertyKey][]resource.URN{}
		newDeletedWith := resource.URN("")
		newParent := resource.URN("")
		filtered := false

		_, allDeps := res.GetAllDependencies()
		for _, dep := range allDeps {
			switch dep.Type {
			case resource.ResourceParent:
				if !refreshDeletes[dep.URN] {
					availableParents[res.URN] = dep.URN
					newParent = dep.URN
				} else {
					// dep.URN might have be gone so look up _its_ parent
					// Since existing must obey a topological sort, we have already addressed
					// r.Parent. Since we know that it doesn't dangle, and that r.Parent no longer
					// exists, we set r.Parent as r.Parent.Parent.
					newParent = availableParents[res.Parent]
					availableParents[res.URN] = newParent
					newParent = dep.URN
					filtered = true
				}
			case resource.ResourceDependency:
				if !refreshDeletes[dep.URN] {
					newDeps = append(newDeps, dep.URN)
				} else {
					filtered = true
				}
			case resource.ResourcePropertyDependency:
				if !refreshDeletes[dep.URN] {
					newPropDeps[dep.Key] = append(newPropDeps[dep.Key], dep.URN)
				} else {
					filtered = true
				}
			case resource.ResourceDeletedWith:
				if !refreshDeletes[dep.URN] {
					newDeletedWith = dep.URN
				} else {
					filtered = true
				}
			}
		}

		if !filtered {
			continue
		}

		// If we have filtered out any dependencies, we need to create a new state with the filtered dependencies.
		newRes := res.Copy()
		newRes.Dependencies = newDeps
		newRes.PropertyDependencies = newPropDeps
		newRes.DeletedWith = newDeletedWith
		newRes.Parent = newParent
		resources[i] = newRes
	}
}
