// Copyright 2025, Pulumi Corporation.
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

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Launches the language host RPC endpoint, which in turn fires
// up an RPC server implementing the LanguageRuntimeServer RPC
// endpoint.
func main() {
	var tracing string
	flag.StringVar(&tracing, "tracing", "",
		"Emit tracing to a Zipkin-compatible tracing endpoint")
	flag.String("root", "", "[obsolete] Project root path to use")
	flag.String("image", "", "[obsolete] Docker image to run")
	flag.Parse()

	args := flag.Args()
	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing("pulumi-language-docker", "pulumi-language-docker", tracing)

	// Optionally pluck out the engine so we can do logging, etc.
	var engineAddress string
	if len(args) > 0 {
		engineAddress = args[0]
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	// map the context Done channel to the rpcutil boolean cancel channel
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		cancel() // deregister the interrupt handler
		close(cancelChannel)
	}()

	if engineAddress != "" {
		err := rpcutil.Healthcheck(ctx, engineAddress, 5*time.Minute, cancel)
		if err != nil {
			cmdutil.Exit(fmt.Errorf("could not start health check host RPC server: %w", err))
		}
	}

	// Fire up a gRPC server, letting the kernel choose a free port.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancelChannel,
		Init: func(srv *grpc.Server) error {
			host := newLanguageHost(engineAddress, tracing)
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		cmdutil.Exit(fmt.Errorf("could not start language host RPC server: %w", err))
	}

	// Otherwise, print out the port so that the spawner knows how to reach us.
	fmt.Printf("%d\n", handle.Port)

	// And finally wait for the server to stop serving.
	if err := <-handle.Done; err != nil {
		cmdutil.Exit(fmt.Errorf("language host RPC stopped serving: %w", err))
	}
}

// dockerLanguageHost implements the LanguageRuntimeServer interface
// for use as an API endpoint.
type dockerLanguageHost struct {
	pulumirpc.UnsafeLanguageRuntimeServer

	engineAddress string
	tracing       string
}

func newLanguageHost(engineAddress, tracing string) pulumirpc.LanguageRuntimeServer {
	return &dockerLanguageHost{
		engineAddress: engineAddress,
		tracing:       tracing,
	}
}

func (host *dockerLanguageHost) GetRequiredPackages(ctx context.Context,
	req *pulumirpc.GetRequiredPackagesRequest,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	return &pulumirpc.GetRequiredPackagesResponse{}, nil
}

func (host *dockerLanguageHost) GetRequiredPlugins(ctx context.Context,
	req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetRequiredPlugins not implemented")
}

func (host *dockerLanguageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	return &pulumirpc.RunResponse{}, nil
}

func (host *dockerLanguageHost) GetPluginInfo(ctx context.Context, req *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: version.Version}, nil
}

func (host *dockerLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest, server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	// closer, stdout, stderr, err := rpcutil.MakeInstallDependenciesStreams(server, req.IsTerminal)
	// if err != nil {
	// 	return err
	// }

	// return closer.Close()
	return nil
}

func (host *dockerLanguageHost) RuntimeOptionsPrompts(ctx context.Context,
	req *pulumirpc.RuntimeOptionsRequest,
) (*pulumirpc.RuntimeOptionsResponse, error) {
	var prompts []*pulumirpc.RuntimeOptionPrompt
	return &pulumirpc.RuntimeOptionsResponse{
		Prompts: prompts,
	}, nil
}

func (host *dockerLanguageHost) About(ctx context.Context,
	req *pulumirpc.AboutRequest,
) (*pulumirpc.AboutResponse, error) {
	getResponse := func(execString string, args ...string) (string, string, error) {
		ex, err := executable.FindExecutable(execString)
		if err != nil {
			return "", "", fmt.Errorf("could not find executable '%s': %w", execString, err)
		}
		cmd := exec.Command(ex, args...)
		var out []byte
		if out, err = cmd.Output(); err != nil {
			cmd := ex
			if len(args) != 0 {
				cmd += " " + strings.Join(args, " ")
			}
			return "", "", fmt.Errorf("failed to execute '%s'", cmd)
		}
		return ex, strings.TrimSpace(string(out)), nil
	}

	docker, version, err := getResponse("docker", "--version")
	if err != nil {
		return nil, err
	}

	return &pulumirpc.AboutResponse{
		Executable: docker,
		Version:    version,
	}, nil
}

func (host *dockerLanguageHost) Handshake(ctx context.Context,
	req *pulumirpc.LanguageHandshakeRequest,
) (*pulumirpc.LanguageHandshakeResponse, error) {
	host.engineAddress = req.EngineAddress

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	// map the context Done channel to the rpcutil boolean cancel channel
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		cancel() // deregister the interrupt handler
		close(cancelChannel)
	}()
	err := rpcutil.Healthcheck(ctx, host.engineAddress, 5*time.Minute, cancel)
	if err != nil {
		cmdutil.Exit(fmt.Errorf("could not start health check host RPC server: %w", err))
	}

	return &pulumirpc.LanguageHandshakeResponse{}, nil
}

func (host *dockerLanguageHost) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	return &pulumirpc.GetProgramDependenciesResponse{}, nil
}

func (host *dockerLanguageHost) RunPlugin(
	req *pulumirpc.RunPluginRequest, server pulumirpc.LanguageRuntime_RunPluginServer,
) error {
	logging.V(5).Infof("RunPlugin: Attempting to run docker plugin with req.Args=%v", req.Args)

	closer, stdout, stderr, err := rpcutil.MakeRunPluginStreams(server, false)
	if err != nil {
		return err
	}
	// best effort close, but we try an explicit close and error check at the end as well
	defer closer.Close()

	options := req.Info.Options.AsMap()
	image, ok := options["image"].(string)
	if !ok {
		return fmt.Errorf("missing 'image' option")
	}
	image = strings.TrimPrefix(image, "docker://")

	args := []string{"run", "--rm", "-p", "4242:4242", "--pull", "missing",
		"--add-host=host.docker.internal:host-gateway", image}
	args = append(args, req.Args...)
	// Hackety hack: When running inside the docker, we need to connect back
	// from docker to the host. On macOs `host.docker.internal` is set to point
	// to the docker host. On Linux we make it work with the `--add-host`
	// option.
	// Args is something like [--logtostderr, -v=6, 127.0.0.1:63047].
	for i, arg := range args {
		args[i] = strings.ReplaceAll(arg, "127.0.0.1", "host.docker.internal")
	}

	cmd := exec.CommandContext(context.Background(), "docker", args...)
	cmd.Stdout, cmd.Stderr = stdout, stderr

	logging.V(6).Infof("RunPlugin docker running %s", cmd.String())
	if err := cmd.Run(); err != nil {
		var exiterr *exec.ExitError
		if errors.As(err, &exiterr) {
			return fmt.Errorf("running docker image %s: %w, stderr=%s", image, err, exiterr.Stderr)
		}
		return fmt.Errorf("running docker image %s: %w", image, err)
	}
	logging.V(6).Infof("RunPlugin: docker finished")

	// TODO: check (language) plugin shutdown, we're leaving the docker container running at the moment.

	return closer.Close()
}

func (host *dockerLanguageHost) GenerateProject(
	ctx context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetRequiredPlugins not implemented")
}

func (host *dockerLanguageHost) GenerateProgram(
	ctx context.Context, req *pulumirpc.GenerateProgramRequest,
) (*pulumirpc.GenerateProgramResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GenerateProgram not implemented")
}

func (host *dockerLanguageHost) GeneratePackage(
	ctx context.Context, req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GeneratePackage not implemented")
}

func (host *dockerLanguageHost) Pack(ctx context.Context, req *pulumirpc.PackRequest) (*pulumirpc.PackResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Pack not implemented")
}
