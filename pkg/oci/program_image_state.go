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

package oci

import (
	"os"
	"path/filepath"
	"strings"
)

// The program-image ref bridge between the OCI language host and the container host.
//
// In pod mode the two halves run as SEPARATE processes inside the one engine
// container: the container host (which starts providers) is built from the
// environment when the engine boots, while the OCI language host builds the program
// image later, during Run. When the image is a prebuilt tag the wrapper can set
// PULUMI_POD_PROGRAM_IMAGE up front, and the container host reads it at construction.
// When the image is BUILT on `up`, though, its ref is not known until the language
// host builds it — after the container host already read the (empty) environment.
//
// So the language host records the built ref to a pod-scoped temp file that the
// container host reads lazily, the first time a workspace-coupled (`command`) or dynamic
// provider needs the program image. The two processes share the engine container's
// filesystem and agree on the pod id (PULUMI_POD_ID), so this is a minimal, ephemeral
// IPC: the language host writes the file before it starts the program container, hence
// before the program can register any resource, hence before any provider is started.
//
// The file is deliberately EPHEMERAL (pod-scoped, not persisted across invocations): the
// ref exists only when the program has RUN. That is always true on `up`. It is NOT true
// on `destroy`, which does not run the program by default — so a program that uses a
// workspace-coupled or dynamic provider must be destroyed with `--run-program`, which
// re-runs the program (rebuilding the image and re-writing this ref). The alternative,
// persisting the ref across pods, was rejected: it is invisible cross-stack global state
// (one stack's destroy could read another's last-built ref). Requiring `--run-program` is
// the honest contract, and dynamic providers already need the program on destroy anyway.

// programImageStatePath is the pod-scoped temp file that holds the built program image
// ref. Keyed by pod id so concurrent pods on one daemon do not collide, and ephemeral so
// no ref leaks across runs or stacks.
func programImageStatePath(podID string) string {
	return filepath.Join(os.TempDir(), "pulumi-pod-program-image-"+podID)
}

// WriteProgramImageState records the built program image ref for the pod so the
// container host can run workspace-coupled and dynamic providers from it. The
// language host calls this after it builds (or resolves) the program image.
func WriteProgramImageState(podID, ref string) error {
	return os.WriteFile(programImageStatePath(podID), []byte(ref), 0o600)
}

// readProgramImageState returns the recorded program image ref for the pod, or the
// empty string when none was written — e.g. a program that uses no workspace-coupled
// or dynamic provider, or a prebuilt-image run where PULUMI_POD_PROGRAM_IMAGE already
// carried the ref.
func readProgramImageState(podID string) string {
	b, err := os.ReadFile(programImageStatePath(podID))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
