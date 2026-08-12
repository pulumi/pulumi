package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type debugPluginProvider struct {
	pulumirpc.UnimplementedResourceProviderServer
}

func (p *debugPluginProvider) GetSchema(ctx context.Context, req *pulumirpc.GetSchemaRequest) (*pulumirpc.GetSchemaResponse, error) {
	return &pulumirpc.GetSchemaResponse{Schema: `{"name":"debugplugin","version":"0.0.1","resources":{"debugplugin:index:MyDebugResource":{}}}`}, nil
}

func (p *debugPluginProvider) CheckConfig(ctx context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	return &pulumirpc.CheckResponse{Inputs: req.News}, nil
}

func (p *debugPluginProvider) DiffConfig(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return &pulumirpc.DiffResponse{}, nil
}

func (p *debugPluginProvider) Configure(ctx context.Context, req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{AcceptSecrets: true, SupportsPreview: true}, nil
}

func (p *debugPluginProvider) Create(ctx context.Context, req *pulumirpc.CreateRequest) (*pulumirpc.CreateResponse, error) {
	return &pulumirpc.CreateResponse{Id: "dummyID"}, nil
}

func (p *debugPluginProvider) GetPluginInfo(ctx context.Context, req *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: "0.0.1"}, nil
}

func (p *debugPluginProvider) Attach(ctx context.Context, req *pulumirpc.PluginAttach) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func main() {
	provider := &debugPluginProvider{}

	stopCh := make(chan bool)
	port, done, err := rpcutil.Serve(0, stopCh, []func(*grpc.Server) error{
		func(s *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(s, provider)
			return nil
		},
	}, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error serving plugin: %v\n", err)
		os.Exit(1)
	}

	os.Stdout.WriteString(strconv.Itoa(port))
	os.Stdout.WriteString("\n")

	err = <-done
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error shutting down plugin: %v\n", err)
		os.Exit(1)
	}
}
