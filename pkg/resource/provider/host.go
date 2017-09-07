// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package provider

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi-fabric/pkg/diag"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
	lumirpc "github.com/pulumi/pulumi-fabric/sdk/proto/go"
)

// HostClient is a client interface into the host's engine RPC interface.
type HostClient struct {
	conn   *grpc.ClientConn
	client lumirpc.EngineClient
}

// NewHostClient dials the target address, connects over gRPC, and returns a client interface.
func NewHostClient(addr string) (*HostClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
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
func (host *HostClient) Log(sev diag.Severity, msg string) error {
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
	_, err := host.client.Log(context.TODO(), &lumirpc.LogRequest{
		Severity: rpcsev,
		Message:  msg,
	})
	return err
}
