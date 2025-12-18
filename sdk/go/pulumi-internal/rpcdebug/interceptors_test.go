// Copyright 2016-2023, Pulumi Corporation.
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

package rpcdebug

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestClientInterceptorCatchesErrors(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "test.log")

	i, err := NewDebugInterceptor(DebugInterceptorOptions{
		LogFile: logFile,
	})
	require.NoError(t, err)

	uci := i.DebugClientInterceptor(LogOptions{})

	ctx := context.Background()

	giveErr := errors.New("oops")

	var inner grpc.UnaryInvoker = func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		opts ...grpc.CallOption,
	) error {
		return giveErr
	}

	err = uci(ctx, "/pulumirpc.ResourceProvider/Configure",
		&pulumirpc.ConfigureRequest{
			Variables: map[string]string{"x": "y"},
		},
		&pulumirpc.ConfigureResponse{}, nil, inner)

	assert.ErrorIs(t, err, giveErr)

	log, err := os.ReadFile(logFile)
	require.NoError(t, err)

	logContents := string(log)

	entries := strings.Split(logContents, "\n")

	var requestLog debugInterceptorLogEntry
	var requestLogData map[string]map[string]string
	err = json.Unmarshal([]byte(entries[0]), &requestLog)
	require.NoError(t, err)
	err = json.Unmarshal(requestLog.Request, &requestLogData)
	require.NoError(t, err)

	var responseLog debugInterceptorLogEntry
	var responseLogData map[string]map[string]string
	err = json.Unmarshal([]byte(entries[1]), &responseLog)
	require.NoError(t, err)
	err = json.Unmarshal(responseLog.Request, &responseLogData)
	require.NoError(t, err)

	assert.Equal(t, "/pulumirpc.ResourceProvider/Configure", requestLog.Method)
	assert.Equal(t, map[string]string{"x": "y"}, requestLogData["variables"])
	assert.Equal(t, "request_started", requestLog.Progress)
	assert.NotZero(t, requestLog.Timestamp, "request log should have a timestamp")
	assert.Zero(t, requestLog.Duration, "request log should not have a duration")

	assert.Equal(t, "/pulumirpc.ResourceProvider/Configure", responseLog.Method)
	assert.Equal(t, map[string]string{"x": "y"}, responseLogData["variables"])
	assert.Equal(t, "response_completed", responseLog.Progress)
	assert.Equal(t, []string{"oops"}, responseLog.Errors)
	assert.NotZero(t, responseLog.Timestamp, "response log should have a timestamp")
	assert.NotZero(t, responseLog.Duration, "response log should have a duration")

	assert.True(t, responseLog.Timestamp.After(requestLog.Timestamp),
		"response log timestamp should be after request log timestamp")
}
