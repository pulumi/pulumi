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

package httpstate

import (
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// getContextAndMain computes the root path of the archive as well as the relative path (from the archive root)
// to the main function. In the case where there is no custom archive root, things are simple, the archive root
// is the root of the project, and main can remain unchanged. When an context is set, however, we need to do some
// work:
//
// 1. We need to ensure the archive root is "above" the project root.
// 2. We need to change "main" which was relative to the project root to be relative to the archive root.
//
// Note that the relative paths in Pulumi.yaml for Context and Main are always unix style paths, but the returned
// context is an absolute path, using file system specific seperators.  We continue use a unix style partial path for
// Main.
func getContextAndMain(proj *workspace.Project, projectRoot string) (string, string, error) {
	context, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", "", err
	}

	main := proj.Main

	if proj.Context != "" {
		context, err = filepath.Abs(filepath.Join(context,
			strings.Replace(proj.Context, "/", string(filepath.Separator), -1)))
		if err != nil {
			return "", "", err
		}

		if !strings.HasPrefix(projectRoot, context) {
			return "", "", errors.Errorf("Context directory '%v' is not a parent of '%v'", context, projectRoot)
		}

		// Walk up to the archive root, starting from the project root, recording the directories we see,
		// we'll combine these with the existing main value to get a main relative to the root of the archive
		// which is what the pulumi-service expects. We use fsutil.WalkUp here, so we have to provide a dummy
		// function which ignores every file we visit.
		ignoreFileVisitFunc := func(string) bool {
			// return false so fsutil.Walk does not stop early
			return false
		}

		prefix := ""
		_, err := fsutil.WalkUp(projectRoot, ignoreFileVisitFunc, func(p string) bool {
			if p != context {
				prefix = filepath.Base(p) + "/" + prefix
				return true
			}

			return false
		})
		if err != nil {
			return "", "", err
		}

		main = prefix + main
	}

	return context, main, nil
}
