// Copyright 2016, Pulumi Corporation.
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

package workspace

// Settings defines workspace settings shared amongst many related projects.
type Settings struct {
	// Stack is an optional default stack to use.
	Stack string `json:"stack,omitempty" yaml:"env,omitempty"`
	// Checkouts records the active service-backed config checkouts, keyed by fully-qualified stack name.
	Checkouts map[string]Checkout `json:"checkouts,omitempty" yaml:"checkouts,omitempty"`
}

// Checkout records a service-backed config working copy that materialized a stack's linked ESC environment
// into a local file. Its presence marks the stack as checked out: config reads and writes route to FilePath
// until the working copy is committed or discarded. Etag is the conflict token captured at checkout;
// Revision is display-only; ContentHash is over canonical config content (not file bytes) so the no-op
// detection survives a YAML re-marshal; Imports is the baseline import list for the commit reconcile.
type Checkout struct {
	EnvRef      string   `json:"envRef"`
	Etag        string   `json:"etag"`
	Revision    int      `json:"revision"`
	FilePath    string   `json:"filePath"`
	ContentHash string   `json:"contentHash"`
	Imports     []string `json:"imports,omitempty"`
}

// IsEmpty returns true when the settings object is logically empty (no selected stack and nothing in the deprecated
// configuration bag).
func (s *Settings) IsEmpty() bool {
	return s.Stack == "" && len(s.Checkouts) == 0
}
