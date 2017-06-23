// Copyright 2016-2017, Pulumi Corporation
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

package lumidl

import (
	"path/filepath"

	"golang.org/x/tools/go/loader"

	"github.com/pulumi/lumi/pkg/util/contract"
)

// RelFilename gets the target filename for any given position relative to the root.
func RelFilename(root string, prog *loader.Program, p goPos) string {
	pos := p.Pos()
	source := prog.Fset.Position(pos).Filename // the source filename.`
	rel, err := filepath.Rel(root, source)     // make it relative to the root.
	contract.Assert(err == nil)
	return rel
}
