// Copyright 2016-2020, Pulumi Corporation.
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

package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// LogSeverity is the severity level of a log message.  Errors are fatal; all others are informational.
type LogSeverity int32

const (
	LogSeverity_DEBUG   LogSeverity = 0 // a debug-level message not displayed to end-users (the default).
	LogSeverity_INFO    LogSeverity = 1 // an informational message printed to output during resource operations.
	LogSeverity_WARNING LogSeverity = 2 // a warning to indicate that something went wrong.
	LogSeverity_ERROR   LogSeverity = 3 // a fatal error indicating that the tool should stop processing subsequent resource operations.
)

type LogRequest struct {
	// the logging level of this message.
	Severity LogSeverity
	// the contents of the logged message.
	Message string
	// the (optional) resource urn this log is associated with.
	Urn string
	// the (optional) stream id that a stream of log messages can be associated with. This allows
	// clients to not have to buffer a large set of log messages that they all want to be
	// conceptually connected.  Instead the messages can be sent as chunks (with the same stream id)
	// and the end display can show the messages as they arrive, while still stitching them together
	// into one total log message.
	//
	// 0/not-given means: do not associate with any stream.
	StreamId int32
	// Optional value indicating whether this is a status message.
	Ephemeral bool
}

// Host is a common interface to the engine passed to all plugins.
type Host interface {
	Log(context.Context, LogRequest) error
}

type grpcHost struct {
	engine pulumirpc.EngineClient
}

func (host *grpcHost) Log(ctx context.Context, request LogRequest) error {
	rcpRequest := &pulumirpc.LogRequest{
		Severity:  pulumirpc.LogSeverity(request.Severity),
		Message:   strings.ToValidUTF8(request.Message, "�"),
		Urn:       strings.ToValidUTF8(request.Urn, "�"),
		StreamId:  request.StreamId,
		Ephemeral: request.Ephemeral,
	}
	_, err := host.engine.Log(ctx, rcpRequest)
	return err
}

func NewHost(address string) (Host, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, fmt.Errorf("connecting to engine over RPC: %w", err)
	}
	return &grpcHost{
		engine: pulumirpc.NewEngineClient(conn),
	}, nil
}
