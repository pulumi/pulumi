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

import (
	"path"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

// newPulumiIgnorerIgnorer creates an ignorer based on the contents of a .pulumiignore file, which
// has the same semantics as a .gitignore file
func newPulumiIgnorerIgnorer(pathToPulumiIgnore string) (ignorer, error) {
	gitIgnorer, err := ignore.CompileIgnoreFile(pathToPulumiIgnore)
	if err != nil {
		return nil, err
	}

	return &pulumiIgnoreIgnorer{root: path.Dir(pathToPulumiIgnore), ignorer: gitIgnorer}, nil
}

type pulumiIgnoreIgnorer struct {
	root    string
	ignorer *ignore.GitIgnore
}

func (g *pulumiIgnoreIgnorer) IsIgnored(f string) bool {
	return g.ignorer.MatchesPath(strings.TrimPrefix(f, g.root))
}
