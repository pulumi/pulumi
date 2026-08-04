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

package do

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	hclv2syntax "github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	backendSecrets "github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// autoResourceNames returns a deterministic identifier→URN map derived from the given snapshot.
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
func autoResourceNames(snap *deploy.Snapshot) map[string]string {
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
		// Extract the trailing segment of the type token (e.g. "Bucket" from "aws:s3/bucket:Bucket").
		typeName := string(s.Type)
		if i := strings.LastIndex(typeName, ":"); i >= 0 {
			typeName = typeName[i+1:]
		}
		entries = append(entries, entry{
			urn:      s.URN,
			parent:   s.Parent,
			typeName: typeName,
			name:     s.URN.Name(),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].urn < entries[j].urn })

	assigned := map[string]string{} // ident -> urn
	for _, e := range entries {
		candidates := []string{
			sanitizeIdent(e.name),
			sanitizeIdent(e.typeName + "_" + e.name),
		}
		if e.parent != "" {
			p := e.parent.Name()
			candidates = append(candidates,
				sanitizeIdent(p+"_"+e.name),
				sanitizeIdent(p+"_"+e.typeName+"_"+e.name),
			)
		}
		// Guaranteed-unique fallback: sanitized name + short hash of the URN. The URN is unique
		// per resource so this candidate never collides with another resource's fallback, and it's
		// stable across runs.
		hash := sha256.Sum256([]byte(e.urn))
		candidates = append(candidates, sanitizeIdent(e.name)+"_"+hex.EncodeToString(hash[:4]))

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

// mergeResourceNames overlays user on top of auto, with user entries winning. When a user maps an
// identifier that auto already used for a different URN, the user's mapping replaces it (both the
// identifier and any auto-assigned identifier still pointing at the URN are preserved). User
// entries may map multiple identifiers to the same URN.
func mergeResourceNames(auto, user map[string]string) map[string]string {
	if len(auto) == 0 && len(user) == 0 {
		return nil
	}
	out := make(map[string]string, len(auto)+len(user))
	maps.Copy(out, auto)
	maps.Copy(out, user)
	return out
}

// sanitizeIdent rewrites s into a valid PCL identifier: it keeps letters, digits and underscores,
// replaces everything else with `_`, and prefixes a leading `_` if s would otherwise start with a
// digit. An empty input returns "".
func sanitizeIdent(s string) string {
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

// referencedIdentsInPCL returns the set of top-level identifier roots (the leading name in a
// scope traversal, e.g. `myBucket` in `myBucket.arn`) that appear in the given PCL source. It's
// used to trim the persisted snippet.References map down to only the identifiers the snippet's
// code actually consumes — auto-derived entries that aren't referenced would otherwise freeze
// stale URNs into the snapshot.
//
// The result is best-effort: parse errors are swallowed and yield an empty set (safer to persist
// nothing than to persist a wrong subset). Callers that need the full map for the converter
// upstream must not use this to gate the converter's inputs — only the write-side reference set.
func referencedIdentsInPCL(src []byte, filename string) map[string]struct{} {
	if len(src) == 0 {
		return nil
	}
	file, diags := hclv2syntax.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() || file == nil {
		return nil
	}
	body, ok := file.Body.(*hclv2syntax.Body)
	if !ok {
		return nil
	}
	out := map[string]struct{}{}
	visit := func(node hclv2syntax.Node) hcl.Diagnostics {
		if trav, ok := node.(*hclv2syntax.ScopeTraversalExpr); ok {
			if len(trav.Traversal) > 0 {
				if root, ok := trav.Traversal[0].(hcl.TraverseRoot); ok {
					out[root.Name] = struct{}{}
				}
			}
		}
		return nil
	}
	walkBody(body, visit)
	return out
}

func walkBody(body *hclv2syntax.Body, visit hclv2syntax.VisitFunc) {
	for _, attr := range body.Attributes {
		_ = hclv2syntax.VisitAll(attr.Expr, visit)
	}
	for _, block := range body.Blocks {
		walkBody(block.Body, visit)
	}
}

// filterReferencesByUsage returns the subset of refs whose keys appear in used. When used is nil
// (e.g. the PCL failed to parse), refs is returned unchanged rather than blanking the snippet.
func filterReferencesByUsage(refs map[string]string, used map[string]struct{}) map[string]string {
	if refs == nil || used == nil {
		return refs
	}
	out := make(map[string]string, len(used))
	for k, v := range refs {
		if _, ok := used[k]; ok {
			out[k] = v
		}
	}
	return out
}

// hasResourcesFlag reports whether the raw argv contains the top-level `--resources` flag.
// The top-level `do` command runs with DisableFlagParsing (provider schemas contribute unknown
// flags), so we scan by hand rather than relying on the bound cobra flag's value — that flag is
// wired for --help discoverability but never actually parsed at the top-level.
func hasResourcesFlag(args []string) bool {
	for _, a := range args {
		if a == "--resources" || a == "--resources=true" {
			return true
		}
		if a == "--" {
			return false
		}
	}
	return false
}

// runResourcesFlag is the handler for `pulumi do --resources`: it opens the currently-selected
// stack, computes the auto-name map and prints it (identifier -> URN). This is the discoverability
// surface for the auto-map that upsert/create silently merge into the user's --resources-file.
func runResourcesFlag(cmd *cobra.Command, ws pkgWorkspace.Context, lm cmdBackend.LoginManager) error {
	ctx := cmd.Context()
	base := diag.DefaultSink(cmd.OutOrStdout(), cmd.ErrOrStderr(), diag.FormatOptions{
		Color: cmdutil.GetGlobalColorization(),
	})
	sink := &forwardingSink{base: base}
	displayOpts := display.Options{Color: cmdutil.GetGlobalColorization()}
	stack, err := cmdStack.RequireStack(
		ctx, sink, ws, lm,
		"", cmdStack.LoadOnly, displayOpts, "",
	)
	if err != nil {
		return fmt.Errorf("load stack: %w", err)
	}
	snap, err := stack.Snapshot(ctx, backendSecrets.DefaultProvider)
	if err != nil {
		return fmt.Errorf("load stack snapshot: %w", err)
	}
	names := autoResourceNames(snap)
	if len(names) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no resources)")
		return nil
	}
	idents := make([]string, 0, len(names))
	width := 0
	for k := range names {
		idents = append(idents, k)
		if len(k) > width {
			width = len(k)
		}
	}
	sort.Strings(idents)
	for _, k := range idents {
		fmt.Fprintf(cmd.OutOrStdout(), "%-*s  %s\n", width, k, names[k])
	}
	return nil
}
