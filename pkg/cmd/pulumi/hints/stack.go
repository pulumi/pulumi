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

package hints

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// minStackResources is how many resources, besides the stack itself and providers, a stack needs before hints with
// Hint.EstablishedStackOnly are shown for it.
const minStackResources = 2

// stackLoadTimeout bounds how long Flush waits for a stack's resources, so that a slow backend doesn't hold up the
// end of a command for a hint.
const stackLoadTimeout = 5 * time.Second

// isEstablished reports whether s has enough resources for hints with Hint.EstablishedStackOnly.
func (m *manager) isEstablished(s backend.Stack) bool {
	if s == nil {
		return false
	}
	n, err := m.stackResources(s)
	if err != nil {
		slog.Info("could not count stack resources for hints", "err", err)
		return false
	}
	return n >= minStackResources
}

// countStackResources counts the resources of s besides the stack itself and providers. It reads the exported
// deployment rather than the snapshot, so that it needs no secrets provider and never prompts for a passphrase.
func countStackResources(s backend.Stack) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), stackLoadTimeout)
	defer cancel()
	deployment, err := s.Backend().ExportDeployment(ctx, s)
	if err != nil {
		return 0, err
	}
	var d struct {
		Resources []struct {
			Type   tokens.Type `json:"type"`
			Delete bool        `json:"delete"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(deployment.Deployment, &d); err != nil {
		return 0, err
	}
	n := 0
	for _, r := range d.Resources {
		if !r.Delete && r.Type != resource.RootStackType && !providers.IsProviderType(r.Type) {
			n++
		}
	}
	return n, nil
}
