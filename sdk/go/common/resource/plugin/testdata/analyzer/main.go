package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
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

func main() {
	// Bootup a policy plugin but first assert that the config is what we expect

	config := os.Getenv("PULUMI_CONFIG")
	var actual map[string]interface{}
	if err := json.Unmarshal([]byte(config), &actual); err != nil {
		fmt.Printf("fatal: %v\n", err)
		os.Exit(1)
	}
	expect := map[string]interface{}{
		"test-project:bool":   "true",
		"test-project:float":  "1.5",
		"test-project:string": "hello",
		"test-project:obj":    "{\"key\":\"value\"}",
	}
	if !reflect.DeepEqual(actual, expect) {
		fmt.Printf("fatal: expected config to be %v, got %v\n", expect, actual)
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
