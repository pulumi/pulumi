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
	"os"
	"strings"
)

// newPathIgnorer creates an ignorer based that ignores either a single file (when dir is false) or
// and entire directory tree (when dir is true).
func newPathIgnorer(path string, isDir bool) ignorer {
	if !isDir {
		return &fileIgnorer{path: path}
	}

	return &directoryIgnorer{path: path + string(os.PathSeparator)}
}

type fileIgnorer struct {
	path string
}

func (fi *fileIgnorer) IsIgnored(f string) bool {
	return f == fi.path
}

type directoryIgnorer struct {
	path string
}

func (di *directoryIgnorer) IsIgnored(f string) bool {
	return strings.HasPrefix(f, di.path)
}
