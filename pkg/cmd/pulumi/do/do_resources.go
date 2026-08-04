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
	"fmt"
	"sort"

	"github.com/hashicorp/hcl/v2"
	hclv2syntax "github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	backendSecrets "github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/autonames"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

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
	names := autonames.ResourceNames(snap)
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
