package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type analyzer struct {
	pulumirpc.UnimplementedAnalyzerServer
}

func (a *analyzer) Handshake(ctx context.Context, req *pulumirpc.AnalyzerHandshakeRequest) (*pulumirpc.AnalyzerHandshakeResponse, error) {
	if req.Stack != "test-stack" {
		return nil, fmt.Errorf("expected stack to be test-stack, got %s", req.Stack)
	}
	if req.Project != "test-project" {
		return nil, fmt.Errorf("expected project to be test-project, got %s", req.Project)
	}
	if req.Organization != "test-org" {
		return nil, fmt.Errorf("expected organization to be test-org, got %s", req.Organization)
	}
	if !req.DryRun {
		return nil, fmt.Errorf("expected dry run to be true, got false")
	}

	actualConfig, err := plugin.UnmarshalProperties(req.Config, plugin.MarshalOptions{
		KeepSecrets: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	expectedConfig := resource.PropertyMap{
		"test-project:bool":   resource.NewBoolProperty(true),
		"test-project:float":  resource.NewNumberProperty(1.5),
		"test-project:string": resource.NewStringProperty("hello"),
		"test-project:obj": resource.NewObjectProperty(resource.PropertyMap{
			"key": resource.NewStringProperty("value"),
		}),
	}

	if !actualConfig.DeepEquals(expectedConfig) {
		return nil, fmt.Errorf("expected config to be %v, got %v", expectedConfig, actualConfig)
	}

	return &pulumirpc.AnalyzerHandshakeResponse{}, nil
}

type language struct {
	pulumirpc.UnimplementedLanguageRuntimeServer
}

func (l *language) GetPluginInfo(context.Context, *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: "1.0.0",
	}, nil
}

func (l *language) RunPlugin(req *pulumirpc.RunPluginRequest, srv pulumirpc.LanguageRuntime_RunPluginServer) error {
	// This should be trying to run the analyzer plugin

	if req.Kind != string(apitype.AnalyzerPlugin) {
		return fmt.Errorf("expected kind to be ANALYZER, got %s", req.Kind)
	}

	// See if PULUMI_CONFIG= is in the environment
	found := -1
	for i, v := range req.Env {
		if strings.HasPrefix(v, "PULUMI_CONFIG=") {
			found = i
			break
		}
	}
	if found != -1 {
		return fmt.Errorf("expected PULUMI_CONFIG not to be set, got %s", req.Env[found])
	}
	if req.Info.RootDirectory != req.Info.ProgramDirectory {
		return fmt.Errorf("expected root directory to be the same as program directory, got %s and %s", req.Info.RootDirectory, req.Info.ProgramDirectory)
	}
	// Expect root directory to point to the policy pack
	_, err := os.Stat(filepath.Join(req.Info.RootDirectory, "PulumiPolicy.yaml"))
	if err != nil {
		return fmt.Errorf("expected root directory to point to the policy pack, got %s", req.Info.RootDirectory)
	}

	// Run the analyzer plugin
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
		return err
	}

	err = srv.Send(&pulumirpc.RunPluginResponse{
		Output: &pulumirpc.RunPluginResponse_Stdout{
			Stdout: []byte(fmt.Sprintf("%d\n", handle.Port)),
		},
	})
	if err != nil {
		return err
	}

	if err := <-handle.Done; err != nil {
		return err
	}

	return nil
}

func main() {
	// Bootup a language plugin but first assert that the config is what we expect, i.e. empty
	config := os.Getenv("PULUMI_CONFIG")
	if config != "" {
		fmt.Printf("fatal: expected PULUMI_CONFIG to be empty, got %s\n", config)
		os.Exit(1)
	}

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
