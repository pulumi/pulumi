// Copyright 2016-2022, Pulumi Corporation.
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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"net/http"
	"net/http/httptest"

	"github.com/hexops/gotextdiff"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
)

// Check that cloudSnapshotPersister can talk the diff-based
// "checkpointverbatim" and "checkpointdelta" protocol when saving
// snapshots.
func TestCloudSnapshotPersisterUseOfDiffProtocol(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	stackID := client.StackIdentifier{
		Owner:   "owner",
		Project: "project",
		Stack:   "stack",
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
				rbytes, err := ioutil.ReadAll(reader)
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
		backendGeneric, err := New(nil, server.URL)
		assert.NoError(t, err)
		backend := backendGeneric.(*cloudBackend)
		persister := backend.newSnapshotPersister(ctx, client.UpdateIdentifier{
			StackIdentifier: stackID,
			UpdateKind:      apitype.UpdateUpdate,
			UpdateID:        updateID,
		}, newMockTokenSource(), nil)
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
	assert.Equal(t, "{\"version\":3,\"deployment\":{\n\"manifest\": {\n\"time\": \"0001-01-01T00:00:00Z\""+
		",\n\"magic\": \"\",\n\"version\": \"\"\n},\n\"resources\": [\n{\n\"urn\": \"urn-1\",\n\"custom\":"+
		" false,\n\"type\": \"\"\n}\n]\n}}", string(req1.UntypedDeployment))

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
	assert.Equal(t, "[{\"Span\":{\"uri\":\"\",\"start\":{\"line\":12,\"column\":1,\"offset\":-1},\"end\""+
		":{\"line\":12,\"column\":1,\"offset\":-1}},\"NewText\":\"},\\n\"},{\"Span\":{\"uri\":\"\","+
		"\"start\":{\"line\":12,\"column\":1,\"offset\":-1},\"end\":{\"line\":12,"+
		"\"column\":1,\"offset\":-1}},\"NewText\":\"{\\n\"},{\"Span\":{\"uri\":\"\",\"start\":"+
		"{\"line\":12,\"column\":1,\"offset\":-1},\"end\":{\"line\":12,\"column\":1,\"offset\":-1}}"+
		",\"NewText\":\"\\\"urn\\\": \\\"urn-2\\\",\\n\"},{\"Span\":{\"uri\":\"\",\"start\":"+
		"{\"line\":12,\"column\":1,\"offset\":-1},\"end\":{\"line\":12,\"column\":1,\"offset\":-1}}"+
		",\"NewText\":\"\\\"custom\\\": false,\\n\"},{\"Span\":{\"uri\":\"\",\"start\":{\"line\":12,"+
		"\"column\":1,\"offset\":-1},\"end\":{\"line\":12,\"column\":1,\"offset\":-1}},\"NewText\":\""+
		"\\\"type\\\": \\\"\\\"\\n\"}]", string(req2.DeploymentDelta))
	assert.Equal(t, "75e2f82ca2735650366fba27b53ec97f310abd457aca27266fb29c4377ee00e7", req2.CheckpointHash)

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
	assert.Equal(t, "[{\"Span\":{\"uri\":\"\",\"start\":{\"line\":12,\"column\":1,\"offset\":-1"+
		"},\"end\":{\"line\":13,\"column\":1,\"offset\":-1}},\"NewText\":\"\"},{\"Span\":{"+
		"\"uri\":\"\",\"start\":{\"line\":13,\"column\":1,\"offset\":-1},\"end\":{\"line\":14,"+
		"\"column\":1,\"offset\":-1}},\"NewText\":\"\"},{\"Span\":{\"uri\":\"\",\"start\":"+
		"{\"line\":14,\"column\":1,\"offset\":-1},\"end\":{\"line\":15,\"column\":1,"+
		"\"offset\":-1}},\"NewText\":\"\"},{\"Span\":{\"uri\":\"\",\"start\":{\"line\":15,"+
		"\"column\":1,\"offset\":-1},\"end\":{\"line\":16,\"column\":1,\"offset\":-1}},"+
		"\"NewText\":\"\"},{\"Span\":{\"uri\":\"\",\"start\":{\"line\":16,\"column\":1,"+
		"\"offset\":-1},\"end\":{\"line\":17,\"column\":1,\"offset\":-1}},\"NewText\":\"\"}]",
		string(req3.DeploymentDelta))
	assert.Equal(t, "5f2fd84a225e7c1895528b4b9394607d6b39aefa4ee74f57e018dadcb16bf2e2", req3.CheckpointHash)

	handleDelta(req3)
	assert.Equal(t, []apitype.ResourceV3{
		{URN: resource.URN("urn-1")},
	}, typedPersistedState().Resources)
}

type tokenSourceFn func() (string, error)

var _ tokenSourceCapability = tokenSourceFn(nil)

func (tsf tokenSourceFn) GetToken() (string, error) {
	return tsf()
}
