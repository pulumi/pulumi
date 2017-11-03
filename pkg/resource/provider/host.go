// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package provider

import (
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/util/contract"
	lumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

// HostClient is a client interface into the host's engine RPC interface.
type HostClient struct {
	conn   *grpc.ClientConn
	client lumirpc.EngineClient
}

// NewHostClient dials the target address, connects over gRPC, and returns a client interface.
func NewHostClient(addr string) (*HostClient, error) {
	interceptor := grpc.WithUnaryInterceptor(
		otgrpc.OpenTracingClientInterceptor(
			// Use the globally installed tracer
			opentracing.GlobalTracer(),
			// Log full payloads along with trace spans
			otgrpc.LogPayloads(),
			// Customize which gRPC calls are included in trace
			otgrpc.IncludingSpans(func(
				parentSpanCtx opentracing.SpanContext,
				method string,
				req, resp interface{}) bool {
				return true
			}),
		),
	)
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), interceptor)
	if err != nil {
		return nil, err
	}
	return &HostClient{
		conn:   conn,
		client: lumirpc.NewEngineClient(conn),
	}, nil
}

// Close closes and renders the connection and client unusable.
func (host *HostClient) Close() error {
	return host.conn.Close()
}

// Log logs a global message, including errors and warnings.
func (host *HostClient) Log(context context.Context, sev diag.Severity, msg string) error {
	var rpcsev lumirpc.LogSeverity
	switch sev {
	case diag.Debug:
		rpcsev = lumirpc.LogSeverity_DEBUG
	case diag.Info:
		rpcsev = lumirpc.LogSeverity_INFO
	case diag.Warning:
		rpcsev = lumirpc.LogSeverity_WARNING
	case diag.Error:
		rpcsev = lumirpc.LogSeverity_ERROR
	default:
		contract.Failf("Unrecognized log severity type: %v", sev)
	}
	_, err := host.client.Log(context, &lumirpc.LogRequest{
		Severity: rpcsev,
		Message:  msg,
	})
	return err
}
