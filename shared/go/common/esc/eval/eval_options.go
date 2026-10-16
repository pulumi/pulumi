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

package eval

// TraceMode controls how much of each value's Trace the eval entry points
// retain. The Trace is only useful to a consumer that walks the Base
// merge-history chain end-to-end (e.g. `esc env get`'s provenance view, on the
// Check path); for callers that never read it, that chain grows the payload
// with import-merge depth for no benefit.
type TraceMode int

const (
	// TraceModeFull builds the entire Trace. The zero value, so EvalOptions{}
	// preserves historical behavior and no Trace consumer regresses.
	TraceModeFull TraceMode = iota

	// TraceModeNone omits the Trace entirely (no Def, no Base chain). Safe only
	// when no consumer reads it.
	TraceModeNone
)

// EvalOptions configures an evaluation. Fields must default to historical
// behavior when zero so EvalOptions{} is a safe default.
type EvalOptions struct {
	// TraceMode selects how much of each value's Trace to retain. Applied during
	// export, so TraceModeNone never allocates the dropped Trace.
	TraceMode TraceMode
}
