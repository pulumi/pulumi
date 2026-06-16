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

package neo

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
)

// debugKind identifies which kind of failed operation `pulumi neo --debug-update`/`--debug-preview`
// targets. The constant values double as the noun used in Neo's seed prompt and the debug context,
// so callers can format a debugKind directly instead of mapping it.
type debugKind string

const (
	debugNone    debugKind = ""
	debugUpdate  debugKind = "update"
	debugPreview debugKind = "preview"
)

// latestID returns the id of the stack's most recent operation of this kind, or "" when none is
// available. Updates and previews are tracked separately — previews never appear in GetHistory — so
// each kind has its own lookup: an update resolves to its history version (an integer), a preview to
// its opaque UpdateID (a UUID). Both lookups are best-effort.
func (k debugKind) latestID(ctx context.Context, be httpstate.Backend, stackRef backend.StackReference) string {
	if stackRef == nil {
		return ""
	}
	switch k {
	case debugUpdate:
		if updates, err := be.GetHistory(ctx, stackRef, 1, 1); err == nil && len(updates) > 0 {
			return strconv.Itoa(updates[0].Version)
		}
	case debugPreview:
		if p, err := be.GetLatestStackPreview(ctx, stackRef); err == nil && p != nil {
			return p.UpdateID
		}
	case debugNone:
		// Not a debug session; nothing to look up.
	}
	return ""
}

// debugSeedPrompt builds the initial Neo prompt for `pulumi neo --debug-update`/`--debug-preview`.
// It is deliberately a short trigger line, not a procedure: Neo's skill evaluator matches
// "debug ... failed update/preview" and loads the pulumi-debug-failed-operation skill, which
// carries the actual debugging steps. With no id the seed targets the user's most recent operation
// of that kind (the skill confirms which one); with an id it targets that specific run. Either way
// the fix should land locally in the working directory.
func debugSeedPrompt(kind debugKind, id string) string {
	if id == "" {
		return fmt.Sprintf(
			"Debug my most recent Pulumi %s on this stack and fix it directly in this working directory.\n",
			kind)
	}
	return fmt.Sprintf(
		"Debug the failed Pulumi %s %s of this stack and fix it directly in this working directory.\n",
		kind, id)
}

// debugStackContext builds a short, human-readable block describing where the debug session is
// running — the organization, user, project, and stack — plus the specific operation being
// debugged. `pulumi neo --debug-update`/`--debug-preview` append this to the seed prompt so Neo
// starts with the failure already in context instead of rediscovering it. kind/id describe the
// resolved target (id is "" when none could be inferred). Every field is best-effort: anything that
// is empty or unavailable is simply omitted so debug still works when, for example, the user isn't
// logged in or no stack is selected.
func debugStackContext(be httpstate.Backend, target taskTarget, kind debugKind, id string) string {
	var b strings.Builder
	b.WriteString("Context for this debug session:\n")
	if target.org != "" {
		fmt.Fprintf(&b, "- Organization: %s\n", target.org)
	}
	if user, _, _, err := be.CurrentUser(); err == nil && user != "" {
		fmt.Fprintf(&b, "- User: %s\n", user)
	}
	if target.project != "" {
		fmt.Fprintf(&b, "- Project: %s\n", target.project)
	}
	if name := target.stackName(); name != "" {
		fmt.Fprintf(&b, "- Stack: %s\n", name)
	}
	// The id was resolved (explicitly or inferred) before this call, so we just report it; the
	// phrasing matches the seed prompt's "<kind> <id>".
	if id != "" {
		fmt.Fprintf(&b, "- Debugging: %s %s\n", kind, id)
	}
	return b.String()
}
