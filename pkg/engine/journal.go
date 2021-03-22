package engine

import (
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var _ = SnapshotManager((*Journal)(nil))

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

func (entries JournalEntries) Snap(base *deploy.Snapshot) *deploy.Snapshot {
	// Build up a list of current resources by replaying the journal.
	resources, dones := []*resource.State{}, make(map[*resource.State]bool)
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
			// nolint: lll
			case deploy.OpCreate, deploy.OpCreateReplacement, deploy.OpRead, deploy.OpReadReplacement, deploy.OpUpdate,
				deploy.OpImport, deploy.OpImportReplacement:
				doneOps[e.Step.New()] = true
			case deploy.OpDelete, deploy.OpDeleteReplaced, deploy.OpReadDiscard, deploy.OpDiscardReplaced:
				doneOps[e.Step.Old()] = true
			}
		}

		// Now mark resources done as necessary.
		if e.Kind == JournalEntrySuccess {
			switch e.Step.Op() {
			case deploy.OpSame, deploy.OpUpdate:
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
				dones[e.Step.New()] = true
			}
		}
	}

	// Append any resources from the base snapshot that were not produced by the current snapshot.
	// See backend.SnapshotManager.snap for why this works.
	if base != nil {
		for _, res := range base.Resources {
			if !dones[res] {
				resources = append(resources, res)
			}
		}
	}

	// Append any pending operations.
	var operations []resource.Operation
	for _, op := range ops {
		if !doneOps[op.Resource] {
			operations = append(operations, op)
		}
	}

	// If we have a base snapshot, copy over its secrets manager.
	var secretsManager secrets.Manager
	if base != nil {
		secretsManager = base.SecretsManager
	}

	manifest := deploy.Manifest{}
	manifest.Magic = manifest.NewMagic()
	return deploy.NewSnapshot(manifest, secretsManager, resources, operations)

}

type Journal struct {
	entries JournalEntries
	events  chan JournalEntry
	cancel  chan bool
	done    chan bool
}

func (j *Journal) Entries() []JournalEntry {
	<-j.done

	return j.entries
}

func (j *Journal) Close() error {
	close(j.cancel)
	<-j.done

	return nil
}

func (j *Journal) BeginMutation(step deploy.Step) (SnapshotMutation, error) {
	select {
	case j.events <- JournalEntry{Kind: JournalEntryBegin, Step: step}:
		return j, nil
	case <-j.cancel:
		return nil, errors.New("journal closed")
	}
}

func (j *Journal) End(step deploy.Step, success bool) error {
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

func (j *Journal) RegisterResourceOutputs(step deploy.Step) error {
	select {
	case j.events <- JournalEntry{Kind: JournalEntryOutputs, Step: step}:
		return nil
	case <-j.cancel:
		return errors.New("journal closed")
	}
}

func (j *Journal) RecordPlugin(plugin workspace.PluginInfo) error {
	return nil
}

func (j *Journal) Snap(base *deploy.Snapshot) *deploy.Snapshot {
	return j.entries.Snap(base)
}

func NewJournal() *Journal {
	j := &Journal{
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
