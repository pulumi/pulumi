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

package eval

import (
	"gopkg.in/yaml.v3"

	"github.com/pulumi/esc/syntax"
)

// The TagDecoder is responsible for decoding YAML tags that represent calls to builtin functions.
//
// No tags are presently supported, but the machinery to support tags is useful to preserve until
// we are confident that we won't re-introduce.
var TagDecoder = tagDecoder(0)

type tagDecoder int

func (d tagDecoder) DecodeTag(filename string, n *yaml.Node) (syntax.Node, syntax.Diagnostics, bool) {
	return nil, nil, false
}
