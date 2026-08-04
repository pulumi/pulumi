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

// Package autonames derives stable, human-friendly identifiers for the resources in a stack
// snapshot. The same identifier space is used everywhere a CLI command lets the user refer to an
// existing stack resource by a simple name instead of a URN (`pulumi do` input expressions,
// `pulumi state get`, ...), so the assignment rules live here rather than in any one command.
package autonames

import (
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// ResourceNames returns a deterministic identifier→URN map derived from the given snapshot.
// Callers merge this with the user's --resources-file, letting user entries win on collisions.
//
// Naming rules:
//   - Resources produced by a snippet try to use the snippet's plain name.
//   - All other resources, and snippet resources whose plain name is already taken, use the
//     sanitized base name plus a hash of the URN.
//   - Iteration is in URN order, so conflict resolution is deterministic.
//
// Renames can still happen when the resource that previously held a preferred name is deleted —
// the next run may hand that shorter name to a resource that previously fell through to a longer
// candidate. That's an intentional trade for keeping the common-case names short.
func ResourceNames(snap *deploy.Snapshot) map[string]string {
	if snap == nil {
		return nil
	}

	type entry struct {
		urn         resource.URN
		name        string
		snippetName string
	}
	snippets := map[string]string{}
	for _, s := range snap.Snippets {
		snippets[s.UUID] = s.Name
	}
	var entries []entry
	for _, s := range snap.Resources {
		if s == nil || s.Delete {
			continue
		}
		if s.Type == tokens.RootStackType {
			continue
		}
		// Skip provider resources — they're generally not used for their outputs.
		if sdkproviders.IsProviderType(s.Type) {
			continue
		}
		snippetName := ""
		if s.SnippetID != "" {
			snippetName = snippets[s.SnippetID]
		}
		entries = append(entries, entry{
			urn:         s.URN,
			name:        s.URN.Name(),
			snippetName: snippetName,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].urn < entries[j].urn })

	// Count how many entries would claim each sanitized snippet name so that when two entries
	// share one, both fall through to the hashed form rather than one arbitrarily winning the
	// plain name.
	snippetNameCounts := map[string]int{}
	for _, e := range entries {
		if e.snippetName != "" {
			snippetNameCounts[SanitizeIdent(e.snippetName)]++
		}
	}

	assigned := map[string]string{} // ident -> urn
	for _, e := range entries {
		if e.snippetName != "" {
			c := SanitizeIdent(e.snippetName)
			if snippetNameCounts[c] == 1 {
				if _, taken := assigned[c]; !taken {
					assigned[c] = string(e.urn)
					continue
				}
			}
		}

		baseName := e.name
		if e.snippetName != "" {
			baseName = e.snippetName
		}
		c := AvailableHashedIdent(baseName, e.urn, assigned)
		assigned[c] = string(e.urn)
	}
	return assigned
}

func AvailableHashedIdent(name string, urn resource.URN, assigned map[string]string) string {
	base := SanitizeIdent(name)
	hashBytes := sha256.Sum256([]byte(urn))
	hash := hex.EncodeToString(hashBytes[:])
	for chars := 6; chars <= len(hash); chars++ {
		c := base + "_" + hash[:chars]
		if _, taken := assigned[c]; !taken {
			return c
		}
	}
	return base + "_" + hash
}

// Merge overlays user on top of auto, with user entries winning. When a user maps an
// identifier that auto already used for a different URN, the user's mapping replaces it (both the
// identifier and any auto-assigned identifier still pointing at the URN are preserved). User
// entries may map multiple identifiers to the same URN.
func Merge(auto, user map[string]string) map[string]string {
	if len(auto) == 0 && len(user) == 0 {
		return nil
	}
	out := make(map[string]string, len(auto)+len(user))
	maps.Copy(out, auto)
	maps.Copy(out, user)
	return out
}

// SanitizeIdent rewrites s into a valid PCL identifier: it keeps letters, digits and underscores,
// replaces everything else with `_`, and prefixes a leading `_` if s would otherwise start with a
// digit. An empty input returns "".
func SanitizeIdent(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range s {
		switch {
		case r == '_' ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z'):
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			if i == 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
