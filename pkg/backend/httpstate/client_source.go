// Copyright 2026, Pulumi Corporation.
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

package httpstate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type clientKey struct {
	url      string
	insecure bool
}

type resolvedClient struct {
	client    *client.Client
	account   workspace.Account
	fromAgent bool
}

// ClientSource shares authenticated backend clients within a command.
type ClientSource struct {
	mu      sync.Mutex
	diag    diag.Sink
	clients map[clientKey]resolvedClient
}

// NewClientSource creates a lazy source of authenticated backend clients.
func NewClientSource(d diag.Sink) *ClientSource {
	return &ClientSource{diag: d, clients: map[clientKey]resolvedClient{}}
}

// ClientForURL resolves credentials without changing the selected backend or starting an interactive login.
func (s *ClientSource) ClientForURL(ctx context.Context, apiURL string, insecure bool) (*client.Client, error) {
	r, err := s.resolve(ctx, apiURL, insecure)
	if err != nil {
		return nil, err
	}
	if !r.account.HasCredential() {
		return nil, fmt.Errorf("could not find credentials for %s, have you logged in?", apiURL)
	}
	return r.client, nil
}

func (s *ClientSource) resolve(ctx context.Context, apiURL string, insecure bool) (resolvedClient, error) {
	key := clientKey{url: strings.TrimSuffix(apiURL, "/"), insecure: insecure}
	if key.url == "" {
		return resolvedClient{}, errors.New("credentials lookup requires a backend URL")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.clients[key]; ok {
		if c.fromAgent {
			MarkAgentCredentialsUsed(ctx, key.url)
		}
		return c, nil
	}
	account, fromAgent, err := getBackendAccount(ctx, key.url)
	if err != nil {
		return resolvedClient{}, fmt.Errorf("getting stored credentials: %w", err)
	}
	c := client.NewClient(key.url, account.AccessToken, insecure, s.diag)
	c.WithRefresh(account.RefreshToken, func(at string, expiresAt time.Time, rt string) error {
		account.SetCredentials(at, expiresAt, rt)
		s.mu.Lock()
		cached := s.clients[key]
		cached.account = account
		s.clients[key] = cached
		s.mu.Unlock()
		return account.Save(key.url, false)
	})
	r := resolvedClient{client: c, account: account, fromAgent: fromAgent}
	if account.HasCredential() {
		s.clients[key] = r
	}
	return r, nil
}
