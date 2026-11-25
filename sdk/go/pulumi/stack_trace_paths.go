// Copyright 2026, Pulumi Corporation.
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

package pulumi

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"golang.org/x/mod/modfile"
)

type stackTraceModule struct {
	path string
	dir  string
}

var stackTraceMainModule = sync.OnceValue(func() stackTraceModule {
	info, _ := debug.ReadBuildInfo()
	if info == nil || info.Main.Path == "" {
		return stackTraceModule{}
	}
	wd, err := os.Getwd()
	if err != nil {
		return stackTraceModule{}
	}
	return findStackTraceModule(wd, info.Main.Path)
})

func findStackTraceModule(programDir, modulePath string) stackTraceModule {
	dir, err := fsutil.WalkUpDirs(programDir, func(dir string) bool {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		return err == nil && modfile.ModulePath(data) == modulePath
	})
	if err != nil {
		return stackTraceModule{}
	}
	return stackTraceModule{path: modulePath, dir: dir}
}

// sourceFile resolves a first-party -trimpath location against the Go module directory.
func (m stackTraceModule) sourceFile(file string) string {
	if m.dir == "" {
		return file
	}
	suffix, ok := strings.CutPrefix(file, m.path+"/")
	relative := filepath.FromSlash(suffix)
	if ok && filepath.IsLocal(relative) {
		return filepath.Join(m.dir, relative)
	}
	return file
}
