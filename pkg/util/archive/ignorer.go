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

package archive

type ignorer interface {
	IsIgnored(f string) bool
}

type ignoreState struct {
	ignorer ignorer
	next    *ignoreState
}

func (s *ignoreState) Append(ignorer ignorer) *ignoreState {
	return &ignoreState{ignorer: ignorer, next: s}
}

func (s *ignoreState) IsIgnored(path string) bool {
	if s == nil {
		return false
	}

	if s.ignorer.IsIgnored(path) {
		return true
	}

	return s.next.IsIgnored(path)
}
