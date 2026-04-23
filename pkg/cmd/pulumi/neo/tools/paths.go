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

package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// canonicalRoot returns root as an absolute, symlink-resolved path. Callers pass the
// result as the containment anchor for resolveUnderRoot so that sandbox checks cannot be
// bypassed by symlinks further down the tree.
func canonicalRoot(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// resolveUnderRoot resolves p to an absolute, symlink-free path and returns an error if
// it escapes root. root must already be the output of canonicalRoot.
//
// When allowMissing is true, a missing leaf (or chain of missing intermediate directories)
// is permitted: the closest existing ancestor is resolved and the missing tail is
// re-joined. This supports write targets where the destination directory does not yet
// exist.
func resolveUnderRoot(root, p string, allowMissing bool) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("resolving %q: %w", p, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if !allowMissing || !os.IsNotExist(err) {
			return "", fmt.Errorf("resolving %q: %w", p, err)
		}
		resolved, err = evalClosestAncestor(abs)
		if err != nil {
			return "", fmt.Errorf("resolving %q: %w", p, err)
		}
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", fmt.Errorf("resolving %q: %w", p, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside the working directory %q", p, root)
	}
	return resolved, nil
}

// evalClosestAncestor walks up from abs until it finds an existing directory, resolves
// symlinks on that ancestor, and re-joins the remaining path tail.
func evalClosestAncestor(abs string) (string, error) {
	cur := abs
	var tail []string
	for {
		parent := filepath.Dir(cur)
		tail = append(tail, filepath.Base(cur))
		resolved, err := filepath.EvalSymlinks(parent)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err == nil {
			slices.Reverse(tail)
			return filepath.Join(append([]string{resolved}, tail...)...), nil
		}
		if parent == cur {
			return "", fmt.Errorf("no existing ancestor for %q", abs)
		}
		cur = parent
	}
}
