// A minimal analyzer plugin used by TestAnalyzerBinaryVersionFromYaml. It reports a distinctive
// version from GetAnalyzerInfo so the test can verify the yaml-provided version wins.
package main

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type analyzer struct {
	pulumirpc.UnimplementedAnalyzerServer
}

func (a *analyzer) Handshake(
	ctx context.Context, req *pulumirpc.AnalyzerHandshakeRequest,
) (*pulumirpc.AnalyzerHandshakeResponse, error) {
	return &pulumirpc.AnalyzerHandshakeResponse{}, nil
}

func (a *analyzer) GetAnalyzerInfo(ctx context.Context, req *emptypb.Empty) (*pulumirpc.AnalyzerInfo, error) {
	return &pulumirpc.AnalyzerInfo{
		Name:    "binary-analyzer",
		Version: "999.999.999",
	}, nil
}

func (a *analyzer) Cancel(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func main() {
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterAnalyzerServer(srv, &analyzer{})
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		fmt.Printf("fatal: %v\n", err)
		return
	}
	fmt.Printf("%d\n", handle.Port)
	if err := <-handle.Done; err != nil {
		fmt.Printf("fatal: %v\n", err)
	}
}
