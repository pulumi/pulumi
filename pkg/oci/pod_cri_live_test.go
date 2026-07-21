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
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// criLiveEnvVar gates the live CRI test below. It is the counterpart to the nerdctl manager's
// PULUMI_OCI_NERDCTL_ENV: unset (a plain `go test ./oci`), the test skips, so the package stays
// green off a CRI host. The transport also differs from nerdctl's — where that test shells
// `docker exec nerdenv …`, this one drives the *in-process gRPC client directly*, so it must run
// where the CRI socket is reachable: inside the crienv container. The orchestration
// (cross-compile → docker cp → docker exec crienv) is scratch (see the CRI findings kit); this
// file is the committed proof.
const criLiveEnvVar = "PULUMI_OCI_CRIENV"

// TestCriLiveRunWaitLogs drives the real criPodManager against a live CRI (containerd via the CRI
// plugin) to prove the run path works as *code*, not just as constructed requests — the gap the
// unit tests deliberately leave open. It exercises the verbs the spike proved with crictl
// stand-ins, now through the gRPC client:
//
//   - PullImage lands busybox in the k8s.io namespace the CRI image service uses (the Gate-3
//     namespace flip, exercised for free), and ImageExists then sees it there;
//   - RunContainer creates + starts a container in the adopted sandbox and WaitContainer reads
//     its real exit code (7 — a marker a broken status poll cannot fake);
//   - ContainerLogs tails the container's on-disk log file and strips the K8s framing, returning
//     the line the container printed. This last assertion is the whole point: it is the file-based
//     handshake (§6's asterisk) proven against actual containerd log output, framing and all.
//
// The sandbox is created out-of-band by the orchestration (crictl runp with the spike's proven
// pod config) and its id/log-dir handed in via the environment, exactly as the wrapper will do in
// production — the manager only ever adopts, never creates, a sandbox.
func TestCriLiveRunWaitLogs(t *testing.T) {
	t.Parallel()
	if os.Getenv(criLiveEnvVar) == "" {
		t.Skipf("set %s (and run inside crienv) to exercise the live CRI path; see the CRI findings kit", criLiveEnvVar)
	}
	// The orchestration creates the sandbox and forwards these; fail loudly if it did not, since
	// a missing sandbox is a setup error, not a skip.
	require.NotEmpty(t, os.Getenv(CRISandboxIDEnvVar),
		"orchestration must create a sandbox (crictl runp) and set %s", CRISandboxIDEnvVar)
	require.NotEmpty(t, os.Getenv(CRILogDirEnvVar),
		"orchestration must set %s to the sandbox's log_directory so ContainerLogs reads where containerd writes",
		CRILogDirEnvVar)

	ctx := t.Context()
	// Read sandbox id, log dir, and endpoint from the environment — the same path the language
	// host and build sink take inside the engine container.
	m := NewCriPodManager("crilive")
	t.Cleanup(func() { _ = m.Cleanup(context.WithoutCancel(ctx)) })

	const image = "docker.io/library/busybox:latest"
	// Pull before run: the pull lands the image in k8s.io, which is where a CRI-run container
	// reads it from, and it keeps lazy-pull progress out of the log we scrape.
	require.NoError(t, m.PullImage(ctx, image), "PullImage should fetch busybox into the CRI image store")
	exists, err := m.ImageExists(ctx, image)
	require.NoError(t, err)
	assert.True(t, exists, "the pulled image should be present in the CRI (k8s.io) image store")

	const marker = "HELLO_FROM_CRI"
	c, err := m.RunContainer(ctx, ContainerConfig{
		Image:      image,
		Name:       "crilive-probe",
		Entrypoint: []string{"/bin/sh", "-c"},
		Cmd:        []string{"echo " + marker + "; exit 7"},
	})
	require.NoError(t, err, "RunContainer should create+start a container in the adopted sandbox")
	assert.NotEmpty(t, c.ID, "the container id from CreateContainer must be non-empty")

	code, err := m.WaitContainer(ctx, c)
	require.NoError(t, err)
	assert.Equal(t, 7, code, "WaitContainer must read the container's real exit code from CRI status")

	// The decisive assertion: ContainerLogs reads the container's output from its log file and
	// removes the K8s framing (`<ts> stdout F <content>`). If the log directory the sandbox was
	// created with and the one the manager reads diverged, this would capture nothing.
	rc, err := m.ContainerLogs(ctx, c, false /*follow*/)
	require.NoError(t, err)
	defer rc.Close()
	out, err := io.ReadAll(rc)
	require.NoError(t, err)
	logs := string(out)
	assert.Contains(t, logs, marker, "ContainerLogs must return the line the container printed")
	// The framing prefix must be gone — proof the de-framer ran against real containerd output.
	assert.NotContains(t, logs, " stdout ", "the K8s stream tag must be stripped")
	assert.False(t, strings.Contains(logs, " F ") || strings.Contains(logs, " P "),
		"the K8s full/partial tag must be stripped, got %q", logs)
}

// TestCriLiveRunToCompletion drives the build primitive against live CRI, proving RunToCompletion
// as code — not just as constructed requests. A source-less builder (busybox) prints a ref to
// stdout and progress to stderr, then exits; the test proves the two design departures documented
// on RunToCompletion hold against real containerd log framing:
//
//   - the COMBINED stream reaches the writer (both the stderr progress and the stdout ref, de-framed
//     of the K8s tags), and
//   - the return is "" — the ref on stdout is streamed, NOT captured (no docker-style split).
//
// A second, failing builder proves a non-zero exit surfaces as an error (a build has no bail). The
// `sleep` keeps the container running across at least one WaitContainer poll, so the follow-tailer
// streams concurrently rather than racing a sub-poll-interval exit.
func TestCriLiveRunToCompletion(t *testing.T) {
	t.Parallel()
	if os.Getenv(criLiveEnvVar) == "" {
		t.Skipf("set %s (and run inside crienv) to exercise the live CRI build primitive; see the CRI findings kit", criLiveEnvVar)
	}
	require.NotEmpty(t, os.Getenv(CRISandboxIDEnvVar),
		"orchestration must create a sandbox (crictl runp) and set %s", CRISandboxIDEnvVar)
	require.NotEmpty(t, os.Getenv(CRILogDirEnvVar),
		"orchestration must set %s to the sandbox's log_directory", CRILogDirEnvVar)

	ctx := t.Context()
	m := NewCriPodManager("crilive-build")
	t.Cleanup(func() { _ = m.Cleanup(context.WithoutCancel(ctx)) })

	const image = "docker.io/library/busybox:latest"
	require.NoError(t, m.PullImage(ctx, image), "PullImage should fetch busybox into the CRI image store")

	var out strings.Builder
	ref, err := m.RunToCompletion(ctx, ContainerConfig{
		Image:      image,
		Name:       "build-ok",
		Entrypoint: []string{"/bin/sh", "-c"},
		Cmd:        []string{"echo building-progress 1>&2; sleep 1; echo BUILT_REF_sha256"},
	}, &out)
	require.NoError(t, err, "RunToCompletion should run the builder to a clean exit")
	assert.Empty(t, ref, "CRI RunToCompletion captures no stdout — the ref on stdout is streamed, not returned")

	logs := out.String()
	assert.Contains(t, logs, "building-progress", "stderr progress must reach the writer")
	assert.Contains(t, logs, "BUILT_REF_sha256", "stdout (the ref) must also reach the writer — combined, not captured")
	assert.NotContains(t, logs, " stdout ", "the K8s stream tag must be stripped from real containerd output")
	assert.NotContains(t, logs, " stderr ", "the K8s stream tag must be stripped from real containerd output")

	_, err = m.RunToCompletion(ctx, ContainerConfig{
		Image:      image,
		Name:       "build-fail",
		Entrypoint: []string{"/bin/sh", "-c"},
		Cmd:        []string{"echo oops 1>&2; exit 3"},
	}, io.Discard)
	require.Error(t, err, "a non-zero build exit must surface as an error (a build has no bail)")
	assert.Contains(t, err.Error(), "exited 3")
}
