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
	// Stack is the legacy unscoped selected stack. Prefer Stacks[backendURL].
	// Kept so older workspace.json files continue to load; new writes use Stacks.
	Stack string `json:"stack,omitempty" yaml:"env,omitempty"`

	// Stacks maps backend URL -> selected stack name for that backend.
	Stacks map[string]string `json:"stacks,omitempty"`
}

// IsEmpty returns true when the settings object is logically empty (no selected stack).
func (s *Settings) IsEmpty() bool {
	return s.Stack == "" && len(s.Stacks) == 0
}

// StackForBackend returns the selected stack for backendURL.
// If there is no per-backend entry, it falls back to the legacy Stack field.
func (s *Settings) StackForBackend(backendURL string) (name string, fromLegacy bool) {
	if s.Stacks != nil {
		if name, ok := s.Stacks[backendURL]; ok {
			return name, false
		}
	}
	return s.Stack, s.Stack != ""
}

// SetStackForBackend sets or clears the selected stack for backendURL.
func (s *Settings) SetStackForBackend(backendURL, name string) {
	if name == "" {
		if s.Stacks != nil {
			delete(s.Stacks, backendURL)
			if len(s.Stacks) == 0 {
				s.Stacks = nil
			}
		}
		// Also clear the legacy field so unselect / remove still clear
		// pre-migration workspace.json files that only have "stack".
		s.Stack = ""
		return
	}
	if s.Stacks == nil {
		s.Stacks = map[string]string{}
	}
	s.Stacks[backendURL] = name
}
