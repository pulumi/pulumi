// Copyright 2016-2018, Pulumi Corporation.
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

package provider

import (
	"io"
	"os"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
)

// HostClient is a client interface into the host's engine RPC interface.
type HostClient struct {
	conn   *grpc.ClientConn
	client pulumirpc.EngineClient
}

// Provider client is sensitive to GRPC info logging to stdout, so ensure they are dropped.
// See https://github.com/pulumi/pulumi/issues/7156
//
// grpclog.SetLoggerV2 sets global state, and thus must be called before any gRPC
// connection is initiated. To avoid race conditions, we call it when the package loads.
func init() { grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, os.Stderr)) }

// NewHostClient dials the target address, connects over gRPC, and returns a client interface.
func NewHostClient(addr string) (*HostClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(rpcutil.OpenTracingClientInterceptor()),
		grpc.WithStreamInterceptor(rpcutil.OpenTracingStreamClientInterceptor()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, err
	}
	return &HostClient{
		conn:   conn,
		client: pulumirpc.NewEngineClient(conn),
	}, nil
}

// Close closes and renders the connection and client unusable.
func (host *HostClient) Close() error {
	return host.conn.Close()
}

// EngineConn provides the engine gRPC client connection.
func (host *HostClient) EngineConn() *grpc.ClientConn {
	return host.conn
}

func (host *HostClient) log(
	context context.Context, sev diag.Severity, urn resource.URN, msg string, ephemeral bool,
) error {
	var rpcsev pulumirpc.LogSeverity
	switch sev {
	case diag.Debug:
		rpcsev = pulumirpc.LogSeverity_DEBUG
	case diag.Info, diag.Infoerr:
		rpcsev = pulumirpc.LogSeverity_INFO
	case diag.Warning:
		rpcsev = pulumirpc.LogSeverity_WARNING
	case diag.Error:
		rpcsev = pulumirpc.LogSeverity_ERROR
	default:
		contract.Failf("Unrecognized log severity type: %v", sev)
	}
	_, err := host.client.Log(context, &pulumirpc.LogRequest{
		Severity:  rpcsev,
		Message:   strings.ToValidUTF8(msg, "�"),
		Urn:       string(urn),
		Ephemeral: ephemeral,
	})
	return err
}

// Log logs a global message, including errors and warnings.
func (host *HostClient) Log(
	context context.Context, sev diag.Severity, urn resource.URN, msg string,
) error {
	return host.log(context, sev, urn, msg, false)
}

// LogStatus logs a global status message, including errors and warnings. Status messages will
// appear in the `Info` column of the progress display, but not in the final output.
func (host *HostClient) LogStatus(
	context context.Context, sev diag.Severity, urn resource.URN, msg string,
) error {
	return host.log(context, sev, urn, msg, true)
}
