// Copyright 2016-2024, Pulumi Corporation.
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
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dustin/go-humanize"
	"github.com/hexops/gotextdiff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func applyEdits(before, deltas json.RawMessage) (json.RawMessage, error) {
	var edits []gotextdiff.TextEdit
	if err := json.Unmarshal(deltas, &edits); err != nil {
		return nil, err
	}
	return json.RawMessage(gotextdiff.ApplyEdits(string(before), edits)), nil
}

// Check that cloudSnapshotPersister can talk the diff-based
// "checkpointverbatim" and "checkpointdelta" protocol when saving
// snapshots.
func TestCloudSnapshotPersisterUseOfDiffProtocol(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	expectationsFile := "testdata/snapshot_test.json"
	expectations := map[string]string{}
	accept := cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))
	if accept {
		t.Cleanup(func() {
			bytes, err := json.MarshalIndent(expectations, "", "  ")
			require.NoError(t, err)
			err = os.WriteFile(expectationsFile, bytes, 0o600)
			require.NoError(t, err)
		})
	} else {
		data, err := os.ReadFile(expectationsFile)
		require.NoError(t, err)
		err = json.Unmarshal(data, &expectations)
		require.NoError(t, err)
	}

	assertEquals := func(expectedKey string, actual string) {
		if accept {
			expectations[expectedKey] = actual
			return
		}
		expected, ok := expectations[expectedKey]
		assert.True(t, ok)
		assert.Equal(t, expected, actual, expectedKey)
	}

	assertEqual := func(expectedKey string, actual json.RawMessage) {
		assertEquals(expectedKey, string(actual))
	}

	stackID := client.StackIdentifier{
		Owner:   "owner",
		Project: "project",
		Stack:   tokens.MustParseStackName("stack"),
	}
	updateID := "update-id"

	var persistedState json.RawMessage

	var lastRequest *http.Request

	lastRequestAsVerbatim := func() (ret apitype.PatchUpdateVerbatimCheckpointRequest) {
		err := json.NewDecoder(lastRequest.Body).Decode(&ret)
		assert.Equal(t, "/api/stacks/owner/project/stack/update/update-id/checkpointverbatim", lastRequest.URL.Path)
		assert.NoError(t, err)
		return
	}

	lastRequestAsDelta := func() (ret apitype.PatchUpdateCheckpointDeltaRequest) {
		err := json.NewDecoder(lastRequest.Body).Decode(&ret)
		assert.Equal(t, "/api/stacks/owner/project/stack/update/update-id/checkpointdelta", lastRequest.URL.Path)
		assert.NoError(t, err)
		return
	}

	handleVerbatim := func(req apitype.PatchUpdateVerbatimCheckpointRequest) {
		persistedState = req.UntypedDeployment
	}

	handleDelta := func(req apitype.PatchUpdateCheckpointDeltaRequest) {
		edits := []gotextdiff.TextEdit{}
		if err := json.Unmarshal(req.DeploymentDelta, &edits); err != nil {
			assert.NoError(t, err)
		}
		persistedState = json.RawMessage([]byte(gotextdiff.ApplyEdits(string(persistedState), edits)))
		assert.Equal(t, req.CheckpointHash, fmt.Sprintf("%x", sha256.Sum256(persistedState)))
	}

	typedPersistedState := func() apitype.DeploymentV3 {
		var ud apitype.UntypedDeployment
		err := json.Unmarshal(persistedState, &ud)
		assert.NoError(t, err)
		var d3 apitype.DeploymentV3
		err = json.Unmarshal(ud.Deployment, &d3)
		assert.NoError(t, err)
		return d3
	}

	newMockServer := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			switch req.URL.Path {
			case "/api/capabilities":
				resp := apitype.CapabilitiesResponse{Capabilities: []apitype.APICapabilityConfig{{
					Capability:    apitype.DeltaCheckpointUploads,
					Configuration: json.RawMessage(`{"checkpointCutoffSizeBytes":1}`),
				}}}
				err := json.NewEncoder(rw).Encode(resp)
				assert.NoError(t, err)
				return
			case "/api/stacks/owner/project/stack/update/update-id/checkpointverbatim",
				"/api/stacks/owner/project/stack/update/update-id/checkpointdelta":
				lastRequest = req
				rw.WriteHeader(200)
				message := `{}`
				reader, err := gzip.NewReader(req.Body)
				assert.NoError(t, err)
				defer reader.Close()
				rbytes, err := io.ReadAll(reader)
				assert.NoError(t, err)
				_, err = rw.Write([]byte(message))
				assert.NoError(t, err)
				req.Body = io.NopCloser(bytes.NewBuffer(rbytes))
			default:
				panic(fmt.Sprintf("Path not supported: %v", req.URL.Path))
			}
		}))
	}

	newMockTokenSource := func() tokenSourceCapability {
		return tokenSourceFn(func() (string, error) {
			return "token", nil
		})
	}

	initPersister := func() *cloudSnapshotPersister {
		server := newMockServer()
		backendGeneric, err := New(ctx, nil, server.URL, nil, false)
		assert.NoError(t, err)
		backend := backendGeneric.(*cloudBackend)
		persister := backend.newSnapshotPersister(ctx, client.UpdateIdentifier{
			StackIdentifier: stackID,
			UpdateKind:      apitype.UpdateUpdate,
			UpdateID:        updateID,
		}, newMockTokenSource())
		return persister
	}

	persister := initPersister()

	// Req 1: the first request sends indented data verbatim to establish a good baseline state for further diffs.

	err := persister.Save(&deploy.Snapshot{
		Resources: []*resource.State{
			{URN: resource.URN("urn-1")},
		},
	})
	assert.NoError(t, err)

	req1 := lastRequestAsVerbatim()
	assert.Equal(t, 1, req1.SequenceNumber)
	assert.Equal(t, 3, req1.Version)
	assertEqual("req1", req1.UntypedDeployment)

	handleVerbatim(req1)
	assert.Equal(t, []apitype.ResourceV3{
		{URN: resource.URN("urn-1")},
	}, typedPersistedState().Resources)

	// Req 2: then it switches to sending deltas as text diffs together with SHA-256 checksum of the expected
	// resulting text representation of state.

	err = persister.Save(&deploy.Snapshot{
		Resources: []*resource.State{
			{URN: resource.URN("urn-1")},
			{URN: resource.URN("urn-2")},
		},
	})
	assert.NoError(t, err)

	req2 := lastRequestAsDelta()
	assert.Equal(t, 2, req2.SequenceNumber)
	assertEqual("req2", req2.DeploymentDelta)
	assertEquals("req2.hash", req2.CheckpointHash)

	handleDelta(req2)
	assert.Equal(t, []apitype.ResourceV3{
		{URN: resource.URN("urn-1")},
		{URN: resource.URN("urn-2")},
	}, typedPersistedState().Resources)

	// Req 3: and continues using the diff protocol.

	err = persister.Save(&deploy.Snapshot{
		Resources: []*resource.State{
			{URN: resource.URN("urn-1")},
		},
	})
	assert.NoError(t, err)

	req3 := lastRequestAsDelta()
	assert.Equal(t, 3, req3.SequenceNumber)
	assertEqual("req3", req3.DeploymentDelta)
	assertEquals("req3.hash", req3.CheckpointHash)

	handleDelta(req3)
	assert.Equal(t, []apitype.ResourceV3{
		{URN: resource.URN("urn-1")},
	}, typedPersistedState().Resources)
}

type tokenSourceFn func() (string, error)

var _ tokenSourceCapability = tokenSourceFn(nil)

func (tsf tokenSourceFn) GetToken(_ context.Context) (string, error) {
	return tsf()
}

func generateSnapshots(t testing.TB, r *rand.Rand, resourceCount, resourcePayloadBytes int) []*apitype.DeploymentV3 {
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
			Project:     info.Project,
			Stack:       info.Stack,
			Parallel:    info.Parallel,
			DryRun:      info.DryRun,
			MonitorAddr: info.MonitorAddress,
		})
		assert.NoError(t, err)

		return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
			type Dummy struct {
				pulumi.ResourceState
			}

			for i := 0; i < resourceCount; i++ {
				var dummy Dummy
				err := ctx.RegisterComponentResource("examples:dummy:Dummy", fmt.Sprintf("dummy-%d", i), &dummy)
				if err != nil {
					return err
				}
				err = ctx.RegisterResourceOutputs(&dummy, pulumi.Map{
					"deadweight": pulumi.String(pseudoRandomString(r, resourcePayloadBytes)),
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF)

	var journalEntries engine.JournalEntries
	p := &lt.TestPlan{
		// This test generates big amounts of data so the event streams that would need to be
		// checked in get too big.  Skip them instead.
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
		Steps: []lt.TestStep{
			{
				Op:          engine.Update,
				SkipPreview: true,
				Validate: func(
					_ workspace.Project,
					_ deploy.Target,
					entries engine.JournalEntries,
					_ []engine.Event,
					_ error,
				) error {
					journalEntries = entries
					return nil
				},
			},
		},
	}
	p.Run(t, nil)

	snaps := make([]*apitype.DeploymentV3, len(journalEntries))
	for i := range journalEntries {
		snap, err := journalEntries[:i].Snap(nil)
		require.NoError(t, err)
		deployment, err := stack.SerializeDeployment(context.Background(), snap, true)
		require.NoError(t, err)
		snaps[i] = deployment
	}
	return snaps
}

func testMarshalDeployment(t *testing.T, snaps []*apitype.DeploymentV3) {
	t.Parallel()

	dds := newDeploymentDiffState(0)
	for _, s := range snaps {
		expected, err := dds.MarshalDeployment(s)
		require.NoError(t, err)

		marshaled, err := json.Marshal(apitype.PatchUpdateVerbatimCheckpointRequest{
			Version:           3,
			UntypedDeployment: expected.raw,
		})
		require.NoError(t, err)

		var req apitype.PatchUpdateVerbatimCheckpointRequest
		err = json.Unmarshal(marshaled, &req)
		require.NoError(t, err)

		assert.Equal(t, expected.raw, req.UntypedDeployment)
	}
}

func testDiffStack(t *testing.T, snaps []*apitype.DeploymentV3) {
	t.Parallel()

	ctx := context.Background()

	dds := newDeploymentDiffState(0)
	for _, s := range snaps {
		json, err := dds.MarshalDeployment(s)
		require.NoError(t, err)
		if dds.ShouldDiff(json) {
			d, err := dds.Diff(ctx, json)
			require.NoError(t, err)
			actual, err := applyEdits(dds.lastSavedDeployment.raw, d.deploymentDelta)
			require.NoError(t, err)
			assert.Equal(t, json.raw, actual)
		}
		err = dds.Saved(ctx, json)
		require.NoError(t, err)
	}
}

func benchmarkDiffStack(b *testing.B, snaps []*apitype.DeploymentV3) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		wireSize, verbatimSize, diffs, verbatims := 0, 0, 0, 0
		dds := newDeploymentDiffState(0)

		for _, s := range snaps {
			json, err := dds.MarshalDeployment(s)
			require.NoError(b, err)
			verbatimSize += len(json.raw)
			if dds.ShouldDiff(json) {
				diffs++
				d, err := dds.Diff(ctx, json)
				require.NoError(b, err)
				wireSize += len(d.deploymentDelta)
			} else {
				verbatims++
				wireSize += len(json.raw)
			}
			err = dds.Saved(ctx, json)
			require.NoError(b, err)
		}
		b.ReportMetric(float64(diffs), "diffs")
		b.ReportMetric(float64(verbatims), "verbatims")
		b.ReportMetric(float64(wireSize), "wire_bytes")
		b.ReportMetric(float64(verbatimSize), "checkpoint_bytes")
		b.ReportMetric(float64(verbatimSize)/float64(wireSize), "ratio")
	}
}

func pseudoRandomString(r *rand.Rand, desiredLength int) string {
	buf := make([]byte, desiredLength)
	r.Read(buf)
	text := base64.StdEncoding.EncodeToString(buf)
	return text[0:desiredLength]
}

type testingTB[TB any] interface {
	testing.TB

	Run(name string, inner func(tb TB)) bool
}

type diffStackTestFunc[TB testingTB[TB]] func(tb TB, snaps []*apitype.DeploymentV3)

type diffStackCase interface {
	getName() string
	getSnaps(t testing.TB) []*apitype.DeploymentV3
}

func testOrBenchmarkDiffStack[TB testingTB[TB]](
	tb TB,
	inner diffStackTestFunc[TB],
	cases []diffStackCase,
) {
	for _, c := range cases {
		name, snaps := c.getName(), c.getSnaps(tb)
		tb.Run(name, func(tb TB) {
			inner(tb, snaps)
		})
	}
}

type dynamicStackCase struct {
	seed                 int
	resourceCount        int
	resourcePayloadBytes int
}

func (c dynamicStackCase) getName() string {
	//nolint:gosec // resourcePayloadBytes is always positive
	return fmt.Sprintf("%v_x_%v", c.resourceCount, humanize.Bytes(uint64(c.resourcePayloadBytes)))
}

//nolint:gosec
func (c dynamicStackCase) getSnaps(tb testing.TB) []*apitype.DeploymentV3 {
	r := rand.New(rand.NewSource(int64(c.seed)))
	return generateSnapshots(tb, r, c.resourceCount, c.resourcePayloadBytes)
}

var dynamicCases = []diffStackCase{
	dynamicStackCase{seed: 0, resourceCount: 1, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 2, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 4, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 8, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 16, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 32, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 48, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 64, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 1, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 2, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 4, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 8, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 16, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 32, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 48, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 64, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 1, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 2, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 4, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 8, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 16, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 32, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 48, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 64, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 2, resourcePayloadBytes: 131072},
	dynamicStackCase{seed: 0, resourceCount: 4, resourcePayloadBytes: 131072},
	dynamicStackCase{seed: 0, resourceCount: 8, resourcePayloadBytes: 131072},
	dynamicStackCase{seed: 0, resourceCount: 16, resourcePayloadBytes: 131072},
	dynamicStackCase{seed: 0, resourceCount: 32, resourcePayloadBytes: 131072},
	dynamicStackCase{seed: 0, resourceCount: 48, resourcePayloadBytes: 131072},
	dynamicStackCase{seed: 0, resourceCount: 64, resourcePayloadBytes: 131072},
	dynamicStackCase{seed: 0, resourceCount: 1, resourcePayloadBytes: 524288},
	dynamicStackCase{seed: 0, resourceCount: 2, resourcePayloadBytes: 524288},
	dynamicStackCase{seed: 0, resourceCount: 4, resourcePayloadBytes: 524288},
	dynamicStackCase{seed: 0, resourceCount: 8, resourcePayloadBytes: 524288},
	dynamicStackCase{seed: 0, resourceCount: 16, resourcePayloadBytes: 524288},
}

func BenchmarkDiffStack(b *testing.B) {
	testOrBenchmarkDiffStack(b, benchmarkDiffStack, dynamicCases)
}

func TestDiffStack(t *testing.T) {
	t.Parallel()

	testOrBenchmarkDiffStack(t, testDiffStack, dynamicCases)
}

type recordedStackCase string

func (c recordedStackCase) getName() string {
	return string(c)
}

func (c recordedStackCase) getSnaps(tb testing.TB) []*apitype.DeploymentV3 {
	f, err := os.Open(filepath.Join("testdata", string(c)))
	require.NoError(tb, err)
	defer contract.IgnoreClose(f)

	var deployments []*apitype.DeploymentV3
	dec := json.NewDecoder(f)
	for {
		var d struct {
			Version    int
			Deployment *apitype.DeploymentV3
		}
		err := dec.Decode(&d)
		if err == io.EOF {
			break
		}
		require.NoError(tb, err)
		deployments = append(deployments, d.Deployment)
	}
	return deployments
}

var recordedCases = []diffStackCase{
	recordedStackCase("two-large-checkpoints.json"),
}

func init() {
	for _, c := range strings.Split(os.Getenv("PULUMI_TEST_CHECKPOINT_DIFFS"), ",") {
		if c != "" {
			recordedCases = append(recordedCases, recordedStackCase(c))
		}
	}
}

func BenchmarkDiffStackRecorded(b *testing.B) {
	testOrBenchmarkDiffStack(b, benchmarkDiffStack, recordedCases)
}

func TestDiffStackRecorded(t *testing.T) {
	t.Parallel()

	testOrBenchmarkDiffStack(t, testDiffStack, recordedCases)
}

func TestMarshalDeployment(t *testing.T) {
	t.Parallel()

	testOrBenchmarkDiffStack(t, testMarshalDeployment, dynamicCases)
	testOrBenchmarkDiffStack(t, testMarshalDeployment, recordedCases)
}
