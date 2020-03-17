// Copyright 2016-2018, Pulumi Corporation.
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
}

// IsEmpty returns true when the settings object is logically empty (no selected stack and nothing in the deprecated
// configuration bag).
func (s *Settings) IsEmpty() bool {
	return s.Stack == ""
}
