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

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// resolveRuntimeBin locates the runtime executable, using the managed Node.js
// fallback when the nodejs runtime is missing from the PATH.
func (host *nodeLanguageHost) resolveRuntimeBin(ctx context.Context, out io.Writer) (string, error) {
	runtimeExec := host.runtime
	if runtimeExec == "nodejs" {
		runtimeExec = "node"
	}
	if host.runtime != "nodejs" || host.resolveNode == nil {
		return exec.LookPath(runtimeExec)
	}
	res, err := host.resolveNode(ctx, out)
	if err != nil {
		return "", fmt.Errorf("locating node: %w", err)
	}
	if res.Managed {
		prependProcessPath(res.BinDir)
	}
	return res.Node, nil
}

// prependProcessPath puts the managed bin dir on the process PATH by design,
// for every consumer in this host process: npm re-invokes node/npm by bare
// name, and program scripts do too. It is only reachable on a managed
// resolve, i.e. when LookPath found nothing, so it can never shadow an
// ambient toolchain.
func prependProcessPath(dir string) {
	path := os.Getenv("PATH")
	for _, p := range filepath.SplitList(path) {
		if p == dir {
			return
		}
	}
	os.Setenv("PATH", dir+string(os.PathListSeparator)+path)
}
