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
	"fmt"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
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

func (j *cloudJournaler) AddJournalEntry(entry engine.JournalEntry) error {
	j.wg.Add(1)
	defer j.wg.Done()
	serialized, err := backend.SerializeJournalEntry(j.context, entry, j.sm.Encrypter())
	if err != nil {
		return fmt.Errorf("serializing journal entry: %w", err)
	}
	return j.client.SaveJournalEntry(j.context, j.update, serialized, j.tokenSource)
}

func (j *cloudJournaler) Close() error {
	j.wg.Wait() // Wait for all operations to complete before closing.
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
) engine.Journal {
	journal := &cloudJournaler{
		context:     ctx,
		tokenSource: tokenSource,
		client:      client,
		update:      update,
		sm:          sm,
	}
	return journal
}
