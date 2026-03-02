package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

type conformanceComponentProvider struct {
	plugin.UnimplementedProvider
}

func (p *conformanceComponentProvider) Close() error {
	return nil
}

func (p *conformanceComponentProvider) Pkg() tokens.Package {
	return "conformance-component"
}

func (p *conformanceComponentProvider) Configure(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *conformanceComponentProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{Version: &semver.Version{Major: 22}}, nil
}

func (p *conformanceComponentProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	schemaJSON := `{
		"name":"conformance-component",
		"version":"22.0.0",
		"resources":{
			"conformance-component:index:Simple":{
				"isComponent":true,
				"inputProperties":{"value":{"type":"boolean"}},
				"requiredInputs":["value"],
				"properties":{"value":{"type":"boolean"}},
				"required":["value"]
			}
		}
	}`
	return plugin.GetSchemaResponse{Schema: []byte(schemaJSON)}, nil
}

func (p *conformanceComponentProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *conformanceComponentProvider) Diff(
	_ context.Context, req plugin.DiffRequest,
) (plugin.DiffResult, error) {
	objectDiff := req.OldInputs.Diff(req.NewInputs)
	if objectDiff == nil || !objectDiff.AnyChanges() {
		return plugin.DiffResult{Changes: plugin.DiffNone}, nil
	}
	return plugin.DiffResult{
		Changes:      plugin.DiffSome,
		ChangedKeys:  objectDiff.ChangedKeys(),
		DetailedDiff: plugin.NewDetailedDiffFromObjectDiff(objectDiff, true /* inputDiff */),
	}, nil
}

func (p *conformanceComponentProvider) Construct(
	ctx context.Context,
	req plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
	conn, err := grpc.NewClient(
		req.Info.MonitorAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("connect to resource monitor: %w", err)
	}
	defer conn.Close()

	monitor := pulumirpc.NewResourceMonitorClient(conn)

	valueProp, ok := req.Inputs[resource.PropertyKey("value")]
	if !ok || !valueProp.IsBool() {
		return plugin.ConstructResponse{}, fmt.Errorf("missing required boolean input 'value'")
	}
	value := valueProp.BoolValue()

	parent, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:     "conformance-component:index:Simple",
		Name:     req.Name,
		Parent:   string(req.Parent),
		Provider: req.Options.Providers["conformance-component"],
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": structpb.NewBoolValue(value),
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register parent component: %w", err)
	}

	_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:     "simple:index:Resource",
		Custom:   true,
		Name:     req.Name + "-child",
		Parent:   parent.Urn,
		Provider: req.Options.Providers["simple"],
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": structpb.NewBoolValue(!value),
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register child resource: %w", err)
	}

	_, err = monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn: parent.Urn,
		Outputs: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": structpb.NewBoolValue(value),
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register resource outputs: %w", err)
	}

	return plugin.ConstructResponse{
		URN: resource.URN(parent.Urn),
		Outputs: resource.NewPropertyMapFromMap(map[string]any{
			"value": value,
		}),
	}, nil
}

func main() {
	var tracing string
	flag.StringVar(&tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
	flag.Parse()

	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing("conformance-component", "conformance-component", tracing)

	ctx := context.Background()
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		close(cancelChannel)
	}()

	args := flag.Args()
	if len(args) == 1 {
		ctx, cancel := context.WithCancel(ctx)
		_ = rpcutil.Healthcheck(ctx, args[0], 5*time.Minute, cancel)
	}

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancelChannel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, plugin.NewProviderServer(&conformanceComponentProvider{}))
			return nil
		},
		Options: rpcutil.TracingServerInterceptorOptions(nil),
	})
	if err != nil {
		cmdutil.ExitError(fmt.Sprintf("fatal: %v", err))
	}

	fmt.Printf("%d\n", handle.Port)
	if err := <-handle.Done; err != nil {
		cmdutil.ExitError(fmt.Sprintf("fatal: %v", err))
	}
}
