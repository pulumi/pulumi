// Copyright 2025, Pulumi Corporation.
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

package journal

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

var _ engine.Journal = (*cloudJournaler)(nil)

type cloudJournaler struct {
	context     context.Context         // The context to use for client requests.
	tokenSource tokenSourceCapability   // A token source for interacting with the service.
	client      *client.Client          // A backend for communicating with the service
	update      client.UpdateIdentifier // The UpdateIdentifier for this update sequence.
	sm          secrets.Manager         // Secrets manager for encrypting values when serializing the journal entries.
	wg          sync.WaitGroup          // Wait group to ensure all operations are completed before closing.
}

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
		snapshot, err = stack.SerializeDeployment(ctx, je.NewSnapshot, true)
		if err != nil {
			return apitype.JournalEntry{}, fmt.Errorf("serializing new snapshot: %w", err)
		}
	}

	serializedEntry := apitype.JournalEntry{
		Kind:               apitype.JournalEntryKind(je.Kind),
		OperationID:        je.OperationID,
		DeleteOld:          je.DeleteOld,
		DeleteNew:          je.DeleteNew,
		State:              state,
		Operation:          operation,
		SecretsProvider:    secretsManager,
		PendingReplacement: je.PendingReplacement,
		IsRefresh:          je.IsRefresh,
		NewSnapshot:        snapshot,
	}

	return serializedEntry, nil
}

func (j *cloudJournaler) BeginOperation(entry engine.JournalEntry) error {
	j.wg.Add(1)
	defer j.wg.Done()
	serialized, err := SerializeJournalEntry(j.context, entry, j.sm.Encrypter())
	if err != nil {
		return fmt.Errorf("serializing journal entry: %w", err)
	}
	return j.client.SaveJournalEntry(j.context, j.update, serialized, j.tokenSource)
}

func (j *cloudJournaler) EndOperation(entry engine.JournalEntry) error {
	j.wg.Add(1)
	defer j.wg.Done()
	serialized, err := SerializeJournalEntry(j.context, entry, j.sm.Encrypter())
	if err != nil {
		return fmt.Errorf("serializing journal entry: %w", err)
	}
	return j.client.SaveJournalEntry(j.context, j.update, serialized, j.tokenSource)
}

func (j *cloudJournaler) Write(*deploy.Snapshot) error {
	return errors.New("rebasing not implemented yet")
}

func (j *cloudJournaler) Close() error {
	// No resources to close in the cloud journaler.
	j.wg.Wait() // Wait for all operations to complete before closing.
	// TODO: do we need to wait for all begin/end operations to complete here?
	return nil
}

type tokenSourceCapability interface {
	GetToken(ctx context.Context) (string, error)
}

func NewJournaler(
	ctx context.Context,
	client *client.Client,
	update client.UpdateIdentifier,
	tokenSource tokenSourceCapability,
	sm secrets.Manager,
) (engine.Journal, error) {
	journal := &cloudJournaler{
		context:     ctx,
		tokenSource: tokenSource,
		client:      client,
		update:      update,
		sm:          sm,
	}

	err := journal.BeginOperation(engine.JournalEntry{
		Kind:           engine.JournalEntrySecretsManager,
		SecretsManager: sm,
	})

	return journal, err
}
