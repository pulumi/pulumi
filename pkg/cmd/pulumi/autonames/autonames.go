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
// Stability rules:
//   - Iteration is in URN order, so unrelated changes to the snapshot never perturb a resource that
//     already has a name.
//   - Each resource's candidate list is a pure function of its own URN and parent chain, not of
//     what other resources exist. Conflict resolution is greedy first-writer-wins; the fallback
//     candidate embeds a hash of the URN so termination is guaranteed and the fallback name is
//     also stable across runs.
//
// Renames can still happen when the resource that previously held a preferred name is deleted —
// the next run may hand that shorter name to a resource that previously fell through to a longer
// candidate. That's an intentional trade for keeping the common-case names short.
func ResourceNames(snap *deploy.Snapshot) map[string]string {
	if snap == nil {
		return nil
	}

	type entry struct {
		urn      resource.URN
		parent   resource.URN
		typeName string
		name     string
	}
	var entries []entry
	for _, s := range snap.Resources {
		if s == nil || s.Delete {
			continue
		}
		if s.Type == tokens.RootStackType {
			continue
		}
		// Skip provider resources — they're engine bookkeeping, not something a user would
		// reference by name from an input expression. Users who need a provider URN pass it
		// via --provider instead.
		if sdkproviders.IsProviderType(s.Type) {
			continue
		}
		entries = append(entries, entry{
			urn:      s.URN,
			parent:   s.Parent,
			typeName: typeSuffix(s.Type),
			name:     s.URN.Name(),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].urn < entries[j].urn })

	assigned := map[string]string{} // ident -> urn
	parentNameOf := func(p resource.URN) string {
		if p == "" {
			return ""
		}
		return p.Name()
	}

	for _, e := range entries {
		candidates := []string{
			SanitizeIdent(e.name),
			SanitizeIdent(e.typeName + "_" + e.name),
		}
		if p := parentNameOf(e.parent); p != "" {
			candidates = append(candidates,
				SanitizeIdent(p+"_"+e.name),
				SanitizeIdent(p+"_"+e.typeName+"_"+e.name),
			)
		}
		// Guaranteed-unique fallback: sanitized name + short hash of the URN. The URN is unique
		// per resource so this candidate never collides with another resource's fallback, and it's
		// stable across runs.
		candidates = append(candidates, SanitizeIdent(e.name)+"_"+shortHash(string(e.urn)))

		for _, c := range candidates {
			if c == "" {
				continue
			}
			if _, taken := assigned[c]; taken {
				continue
			}
			assigned[c] = string(e.urn)
			break
		}
	}
	return assigned
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

// typeSuffix returns the trailing type name from a token like "aws:s3/bucket:Bucket" ("Bucket").
func typeSuffix(t tokens.Type) string {
	s := string(t)
	if i := strings.LastIndex(s, ":"); i >= 0 {
		s = s[i+1:]
	}
	return s
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

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:4])
}
