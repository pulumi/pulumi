package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
)

type analyzer struct {
	pulumirpc.UnimplementedAnalyzerServer
}

func (a *analyzer) Handshake(ctx context.Context, req *pulumirpc.AnalyzerHandshakeRequest) (*pulumirpc.AnalyzerHandshakeResponse, error) {
	if req.Stack != "" {
		return nil, fmt.Errorf("expected stack not to be set, got %s", req.Stack)
	}
	if req.Project != "" {
		return nil, fmt.Errorf("expected project not to be set, got %s", req.Project)
	}
	if req.Organization != "" {
		return nil, fmt.Errorf("expected organization not to be set, got %s", req.Organization)
	}
	if req.DryRun {
		return nil, fmt.Errorf("expected dry run to be false, got true")
	}

	if req.Config != nil {
		return nil, fmt.Errorf("expected config not to be set, got %s", req.Config)
	}

	return &pulumirpc.AnalyzerHandshakeResponse{}, nil
}

func main() {
	// Bootup a policy plugin but first assert that no config was passed

	config := os.Getenv("PULUMI_CONFIG")
	if config != "" {
		fmt.Printf("fatal: expected no config, got %v\n", config)
		os.Exit(1)
	}

	var cancelChannel chan bool
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancelChannel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterAnalyzerServer(srv, &analyzer{})
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		fmt.Printf("fatal: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%d\n", handle.Port)

	if err := <-handle.Done; err != nil {
		fmt.Printf("fatal: %v\n", err)
		os.Exit(1)
	}
}
