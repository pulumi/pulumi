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

// debugKind identifies which failed operation a debug session targets. The values double as the
// noun used in the seed prompt, so callers can format a debugKind directly.
type debugKind string

const (
	debugNone    debugKind = ""
	debugUpdate  debugKind = "update"
	debugPreview debugKind = "preview"
)

// latestID returns the id of the stack's most recent operation of this kind, or "" when none is
// available. Updates and previews are looked up separately (previews never appear in GetHistory):
// an update resolves to its history version, a preview to its opaque UpdateID. Best-effort.
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

// buildDebugPrompt assembles the full initial prompt for a debug session: the seed trigger line
// (so Neo's skill evaluator loads the debugging skill) followed by the stack context. An empty id
// is resolved to the latest operation of kind; any userPrompt sits between the two as extra
// guidance. Every lookup is best-effort, so the prompt is always well-formed.
func buildDebugPrompt(
	ctx context.Context, be httpstate.Backend,
	target taskTarget, kind debugKind, id, userPrompt string,
) string {
	if id == "" {
		id = kind.latestID(ctx, be, target.ref)
	}
	seed := debugSeedPrompt(kind, id)
	if userPrompt != "" {
		seed += "\n\n" + userPrompt
	}
	return seed + "\n\n" + debugStackContext(be, target, kind, id)
}

// debugSeedPrompt builds Neo's initial prompt for a debug session. It is a short trigger line, not
// a procedure: Neo's skill evaluator matches it and loads the debugging skill. With no id it targets
// the most recent operation of that kind; with an id, that specific run.
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

// debugStackContext builds a block describing the debug session's org, user, project, stack, and
// resolved operation, appended to the seed prompt so Neo starts with the failure in context. Every
// field is best-effort: anything empty or unavailable is omitted.
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
	if id != "" {
		fmt.Fprintf(&b, "- Debugging: %s %s\n", kind, id)
	}
	return b.String()
}
