// Copyright 2023, Pulumi Corporation.
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

package syntax

import (
	"github.com/hashicorp/hcl/v2"
)

type Syntax interface {
	Range() *hcl.Range
	Path() string
}

var NoSyntax = noSyntax(0)

type noSyntax int

func (noSyntax) Range() *hcl.Range {
	return nil
}

func (noSyntax) Path() string {
	return ""
}

type Trivia interface {
	Syntax

	HeadComment() string
	LineComment() string
	FootComment() string
}

type triviaSyntax struct {
	headComment string
	lineComment string
	footComment string
}

func (s triviaSyntax) Range() *hcl.Range {
	return nil
}

func (s triviaSyntax) Path() string {
	return ""
}

func (s triviaSyntax) HeadComment() string {
	return s.headComment
}

func (s triviaSyntax) LineComment() string {
	return s.lineComment
}

func (s triviaSyntax) FootComment() string {
	return s.footComment
}

func CopyTrivia(s Syntax) Syntax {
	trivia, ok := s.(Trivia)
	if !ok {
		return NoSyntax
	}
	return triviaSyntax{
		headComment: trivia.HeadComment(),
		lineComment: trivia.LineComment(),
		footComment: trivia.FootComment(),
	}
}
