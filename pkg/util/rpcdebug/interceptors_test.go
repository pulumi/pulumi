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
	"errors"
	"os"
	"path/filepath"
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
		req, reply interface{},
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

	assert.JSONEq(t, `{
		"method": "/pulumirpc.ResourceProvider/Configure",
		"request": {"variables": {"x": "y"}},
		"errors": ["oops"]
	}`, string(log))
}
