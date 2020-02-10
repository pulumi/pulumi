// Copyright 2016-2020, Pulumi Corporation.
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

package model

type scope map[string]Node

func (s scope) bindReference(name string) (Node, bool) {
	def, ok := s[name]
	return def, ok
}

func (s scope) define(name string, node Node) bool {
	if _, exists := s[name]; exists {
		return false
	}
	s[name] = node
	return true
}

type scopes struct {
	stack []scope
}

func (s *scopes) push() scope {
	next := scope{}
	s.stack = append(s.stack, next)
	return next
}

func (s *scopes) pop() {
	s.stack = s.stack[:len(s.stack)-1]
}

func (s *scopes) bindReference(name string) (Node, bool) {
	for _, s := range s.stack {
		def, ok := s.bindReference(name)
		if ok {
			return def, true
		}
	}
	return nil, false
}
