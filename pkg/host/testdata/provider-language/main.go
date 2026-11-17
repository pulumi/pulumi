package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type language struct {
	pulumirpc.UnimplementedLanguageRuntimeServer
}

func (host *language) RunPlugin(
	req *pulumirpc.RunPluginRequest, server pulumirpc.LanguageRuntime_RunPluginServer,
) error {
	if strings.Contains(req.Info.ProgramDirectory, "test-plugin-exit") {
		exitcode, err := strconv.Atoi(os.Getenv("PULUMI_TEST_PLUGIN_EXITCODE"))
		if err != nil {
			return fmt.Errorf("could not convert exit code to int: %w", err)
		}

		return server.Send(&pulumirpc.RunPluginResponse{
			Output: &pulumirpc.RunPluginResponse_Exitcode{
				Exitcode: int32(exitcode),
			},
		})
	}

	return fmt.Errorf("not implemented")
}

func (host *language) GetPluginInfo(ctx context.Context, req *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: "1.0.0",
	}, nil
}

func main() {
	var cancelChannel chan bool
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancelChannel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, &language{})
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
