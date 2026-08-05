// Copyright 2016-2026, Pulumi Corporation.
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

package restgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// mockMonitor implements pulumirpc.ResourceMonitorClient for testing.
type mockMonitor struct {
	pulumirpc.ResourceMonitorClient

	registerResourceF        func(ctx context.Context, req *pulumirpc.RegisterResourceRequest) (*pulumirpc.RegisterResourceResponse, error)
	registerResourceOutputsF func(ctx context.Context, req *pulumirpc.RegisterResourceOutputsRequest) (*emptypb.Empty, error)
	invokeF                  func(ctx context.Context, req *pulumirpc.ResourceInvokeRequest) (*pulumirpc.InvokeResponse, error)
	signalAndWaitF           func(ctx context.Context) (*emptypb.Empty, error)
}

func (m *mockMonitor) RegisterResource(
	ctx context.Context, in *pulumirpc.RegisterResourceRequest, opts ...grpc.CallOption,
) (*pulumirpc.RegisterResourceResponse, error) {
	if m.registerResourceF != nil {
		return m.registerResourceF(ctx, in)
	}
	return &pulumirpc.RegisterResourceResponse{}, nil
}

func (m *mockMonitor) RegisterResourceOutputs(
	ctx context.Context, in *pulumirpc.RegisterResourceOutputsRequest, opts ...grpc.CallOption,
) (*emptypb.Empty, error) {
	if m.registerResourceOutputsF != nil {
		return m.registerResourceOutputsF(ctx, in)
	}
	return &emptypb.Empty{}, nil
}

func (m *mockMonitor) Invoke(
	ctx context.Context, in *pulumirpc.ResourceInvokeRequest, opts ...grpc.CallOption,
) (*pulumirpc.InvokeResponse, error) {
	if m.invokeF != nil {
		return m.invokeF(ctx, in)
	}
	return &pulumirpc.InvokeResponse{}, nil
}

func (m *mockMonitor) SignalAndWaitForShutdown(
	ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption,
) (*emptypb.Empty, error) {
	if m.signalAndWaitF != nil {
		return m.signalAndWaitF(ctx)
	}
	return &emptypb.Empty{}, nil
}

// mockEngine implements pulumirpc.EngineClient for testing.
type mockEngine struct {
	pulumirpc.EngineClient

	logF func(ctx context.Context, req *pulumirpc.LogRequest) (*emptypb.Empty, error)
}

func (m *mockEngine) Log(
	ctx context.Context, in *pulumirpc.LogRequest, opts ...grpc.CallOption,
) (*emptypb.Empty, error) {
	if m.logF != nil {
		return m.logF(ctx, in)
	}
	return &emptypb.Empty{}, nil
}

// newTestGateway creates a Gateway with a pre-injected test session.
func newTestGateway(t *testing.T, monitor pulumirpc.ResourceMonitorClient, engine pulumirpc.EngineClient) (*Gateway, string) {
	t.Helper()
	g := NewGateway()
	sess := &Session{
		ID:       "test-session",
		StackURN: "urn:pulumi:dev::test-project::pulumi:pulumi:Stack::test-project-dev",
		Project:  "test-project",
		Stack:    "dev",
		Monitor:  monitor,
		Engine:   engine,
		finished: make(chan struct{}),
	}
	g.AddSession(sess)
	return g, sess.ID
}

func doRequest(t *testing.T, handler http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestRegisterResource(t *testing.T) {
	t.Parallel()

	var captured *pulumirpc.RegisterResourceRequest
	monitor := &mockMonitor{
		registerResourceF: func(ctx context.Context, req *pulumirpc.RegisterResourceRequest) (*pulumirpc.RegisterResourceResponse, error) {
			captured = req
			return &pulumirpc.RegisterResourceResponse{
				Urn: "urn:pulumi:dev::test-project::aws:s3/bucket:Bucket::my-bucket",
				Id:  "my-bucket-abc123",
				Object: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"bucket": structpb.NewStringValue("my-bucket-abc123"),
						"region": structpb.NewStringValue("us-east-1"),
					},
				},
				Stable: true,
				Result: pulumirpc.Result_SUCCESS,
			}, nil
		},
	}

	g, sessID := newTestGateway(t, monitor, &mockEngine{})
	handler := g.Handler()

	w := doRequest(t, handler, "POST", "/sessions/"+sessID+"/resources", RegisterResourceRequest{
		Type:         "aws:s3/bucket:Bucket",
		Name:         "my-bucket",
		Custom:       true,
		Properties:   map[string]interface{}{"acl": "private"},
		Dependencies: []string{"urn:pulumi:dev::test-project::aws:s3/bucket:Bucket::other"},
		Version:      "6.0.0",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp RegisterResourceResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "urn:pulumi:dev::test-project::aws:s3/bucket:Bucket::my-bucket", resp.URN)
	assert.Equal(t, "my-bucket-abc123", resp.ID)
	assert.Equal(t, "my-bucket-abc123", resp.Properties["bucket"])
	assert.True(t, resp.Stable)

	require.NotNil(t, captured)
	assert.Equal(t, "aws:s3/bucket:Bucket", captured.Type)
	assert.Equal(t, "my-bucket", captured.Name)
	assert.True(t, captured.Custom)
	assert.Equal(t, "private", captured.Object.Fields["acl"].GetStringValue())
	assert.True(t, captured.AcceptSecrets)
}

func TestRegisterResourceDefaultsParent(t *testing.T) {
	t.Parallel()

	var captured *pulumirpc.RegisterResourceRequest
	monitor := &mockMonitor{
		registerResourceF: func(ctx context.Context, req *pulumirpc.RegisterResourceRequest) (*pulumirpc.RegisterResourceResponse, error) {
			captured = req
			return &pulumirpc.RegisterResourceResponse{
				Urn: "urn:pulumi:dev::test-project::my:mod:Component::comp",
			}, nil
		},
	}

	g, sessID := newTestGateway(t, monitor, &mockEngine{})
	handler := g.Handler()

	doRequest(t, handler, "POST", "/sessions/"+sessID+"/resources", RegisterResourceRequest{
		Type: "my:mod:Component",
		Name: "comp",
	})

	require.NotNil(t, captured)
	assert.Equal(t, "urn:pulumi:dev::test-project::pulumi:pulumi:Stack::test-project-dev", captured.Parent)
}

func TestInvoke(t *testing.T) {
	t.Parallel()

	var captured *pulumirpc.ResourceInvokeRequest
	monitor := &mockMonitor{
		invokeF: func(ctx context.Context, req *pulumirpc.ResourceInvokeRequest) (*pulumirpc.InvokeResponse, error) {
			captured = req
			return &pulumirpc.InvokeResponse{
				Return: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"arn": structpb.NewStringValue("arn:aws:s3:::my-bucket"),
					},
				},
			}, nil
		},
	}

	g, sessID := newTestGateway(t, monitor, &mockEngine{})
	handler := g.Handler()

	w := doRequest(t, handler, "POST", "/sessions/"+sessID+"/invoke", InvokeRequest{
		Token:   "aws:s3/getBucket:getBucket",
		Args:    map[string]interface{}{"bucket": "my-bucket"},
		Version: "6.0.0",
	})

	assert.Equal(t, http.StatusOK, w.Code)
	var resp InvokeResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "arn:aws:s3:::my-bucket", resp.Return["arn"])

	require.NotNil(t, captured)
	assert.Equal(t, "aws:s3/getBucket:getBucket", captured.Tok)
}

func TestLog(t *testing.T) {
	t.Parallel()

	var captured *pulumirpc.LogRequest
	engine := &mockEngine{
		logF: func(ctx context.Context, req *pulumirpc.LogRequest) (*emptypb.Empty, error) {
			captured = req
			return &emptypb.Empty{}, nil
		},
	}

	g, sessID := newTestGateway(t, &mockMonitor{}, engine)
	handler := g.Handler()

	w := doRequest(t, handler, "POST", "/sessions/"+sessID+"/logs", LogRequest{
		Severity: "warning",
		Message:  "something is wrong",
	})

	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, captured)
	assert.Equal(t, pulumirpc.LogSeverity_WARNING, captured.Severity)
	assert.Equal(t, "something is wrong", captured.Message)
}

func TestSessionNotFound(t *testing.T) {
	t.Parallel()
	g := NewGateway()
	w := doRequest(t, g.Handler(), "POST", "/sessions/nonexistent/resources", RegisterResourceRequest{
		Type: "aws:s3/bucket:Bucket",
		Name: "my-bucket",
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestValidation(t *testing.T) {
	t.Parallel()
	g, sessID := newTestGateway(t, &mockMonitor{}, &mockEngine{})
	handler := g.Handler()

	// Missing type and name.
	w := doRequest(t, handler, "POST", "/sessions/"+sessID+"/resources", RegisterResourceRequest{})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Missing token.
	w = doRequest(t, handler, "POST", "/sessions/"+sessID+"/invoke", InvokeRequest{})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Missing message.
	w = doRequest(t, handler, "POST", "/sessions/"+sessID+"/logs", LogRequest{Severity: "info"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestConvertRoundTrip(t *testing.T) {
	t.Parallel()
	original := map[string]interface{}{
		"name":    "test",
		"count":   float64(42),
		"enabled": true,
	}
	s, err := JSONToStruct(original)
	require.NoError(t, err)
	result := StructToJSON(s)
	assert.Equal(t, original, result)
}

func TestConvertNilHandling(t *testing.T) {
	t.Parallel()
	s, err := JSONToStruct(nil)
	assert.NoError(t, err)
	assert.Nil(t, s)
	assert.Nil(t, StructToJSON(nil))
}

func TestSeverityMapping(t *testing.T) {
	t.Parallel()
	assert.Equal(t, pulumirpc.LogSeverity_DEBUG, SeverityToProto("debug"))
	assert.Equal(t, pulumirpc.LogSeverity_INFO, SeverityToProto("info"))
	assert.Equal(t, pulumirpc.LogSeverity_WARNING, SeverityToProto("warning"))
	assert.Equal(t, pulumirpc.LogSeverity_ERROR, SeverityToProto("error"))
	assert.Equal(t, pulumirpc.LogSeverity_INFO, SeverityToProto("unknown"))
}
