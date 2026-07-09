// Copyright 2026, Pulumi Corporation.
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

// This is the language plugin for the `executable` runtime, used by policy packs that ship as pre-built
// per-platform binaries serving the analyzer gRPC protocol. There is no language toolchain involved: the
// only real work is picking the binary matching the host platform out of the pack manifest and running it.
//
// `executable` is not a language, so the program-facing halves of the LanguageRuntime protocol (Run, codegen,
// Pack, Link) are left unimplemented.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	goruntime "runtime"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type executableLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	cancel context.CancelFunc
}

// The engine address is required by the protocol but unused here: the pack binary receives it
// through RunPlugin's args, not from this host.
func (host *executableLanguageHost) Handshake(
	ctx context.Context, req *pulumirpc.LanguageHandshakeRequest,
) (*pulumirpc.LanguageHandshakeResponse, error) {
	if req.GetEngineAddress() == "" {
		return nil, errors.New("handshake request must contain an engine address")
	}
	return &pulumirpc.LanguageHandshakeResponse{}, nil
}

func (host *executableLanguageHost) GetPluginInfo(context.Context, *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: version.Version}, nil
}

func (host *executableLanguageHost) Cancel(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	if host.cancel != nil {
		host.cancel()
	}
	return &emptypb.Empty{}, nil
}

// There is no language runtime underpinning this host, so there is no executable or version to
// report. AboutResponse.Executable is an absolute path to a runtime binary, not a platform.
func (host *executableLanguageHost) About(
	context.Context, *pulumirpc.AboutRequest,
) (*pulumirpc.AboutResponse, error) {
	return &pulumirpc.AboutResponse{}, nil
}

// Run is what distinguishes a program runtime from this one. An executable pack is an analyzer plugin, not a
// Pulumi program, so there is nothing to run here.
func (host *executableLanguageHost) Run(context.Context, *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	return nil, errors.New(
		"the 'executable' runtime cannot run Pulumi programs; it is only valid for policy packs " +
			"that ship pre-built binaries")
}

func (host *executableLanguageHost) GetRequiredPackages(
	context.Context, *pulumirpc.GetRequiredPackagesRequest,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	return &pulumirpc.GetRequiredPackagesResponse{}, nil
}

//nolint:staticcheck // GetRequiredPlugins is deprecated but still part of the interface.
func (host *executableLanguageHost) GetRequiredPlugins(
	context.Context, *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return &pulumirpc.GetRequiredPluginsResponse{}, nil
}

func (host *executableLanguageHost) GetProgramDependencies(
	context.Context, *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	return &pulumirpc.GetProgramDependenciesResponse{}, nil
}

// InstallDependencies has nothing to install — the pack is a binary. It does mark that binary executable,
// since the artifact may have been unpacked by something that dropped the mode bits.
func (host *executableLanguageHost) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest, server grpc.ServerStreamingServer[pulumirpc.InstallDependenciesResponse],
) error {
	return ensureExecutable(req.Info)
}

func ensureExecutable(programInfo *pulumirpc.ProgramInfo) error {
	if goruntime.GOOS == "windows" {
		return nil
	}
	binary, err := hostBinary(programInfo)
	if err != nil {
		return err
	}
	path := filepath.Join(programInfo.ProgramDirectory, binary)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("policy pack binary not found at %s: %w", path, err)
	}
	if err := os.Chmod(path, info.Mode()|0o111); err != nil {
		return fmt.Errorf("marking policy pack binary %s executable: %w", path, err)
	}
	return nil
}

func (host *executableLanguageHost) RunPlugin(
	req *pulumirpc.RunPluginRequest, server grpc.ServerStreamingServer[pulumirpc.RunPluginResponse],
) error {
	binary, err := hostBinary(req.Info)
	if err != nil {
		return err
	}
	program := filepath.Join(req.Info.ProgramDirectory, binary)

	closer, stdout, stderr, err := rpcutil.MakeRunPluginStreams(server, false)
	if err != nil {
		return err
	}
	// Best effort close; we also close explicitly and check the error at the end.
	defer closer.Close()

	cmd := exec.CommandContext(server.Context(), program, req.Args...)
	cmd.Dir = req.Pwd
	cmd.Env = req.Env
	cmd.Stdout, cmd.Stderr = stdout, stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return fmt.Errorf("could not execute policy pack binary %s: %w", program, err)
		}
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if !ok {
			return fmt.Errorf("policy pack binary %s exited unexpectedly: %w", program, exitErr)
		}
		return server.Send(&pulumirpc.RunPluginResponse{
			//nolint:gosec // WaitStatus always uses the lower 8 bits for the exit code.
			Output: &pulumirpc.RunPluginResponse_Exitcode{Exitcode: int32(status.ExitStatus())},
		})
	}

	return closer.Close()
}

// hostBinary resolves the pack's binary for the platform this CLI is running on.
func hostBinary(info *pulumirpc.ProgramInfo) (string, error) {
	if info == nil {
		return "", errors.New("the engine did not supply program info for the policy pack")
	}
	binaries, err := workspace.ParseExecutableBinaries(info.Options.AsMap())
	if err != nil {
		return "", err
	}
	return workspace.SelectPlatformBinary(binaries)
}

func main() {
	var tracing string
	flag.StringVar(&tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
	showVersion := flag.Bool("version", false, "Print the current plugin version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing("pulumi-language-executable", "pulumi-language-executable", tracing)

	var engineAddress string
	if args := flag.Args(); len(args) > 0 {
		engineAddress = args[0]
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		cancel()
		close(cancelChannel)
	}()

	if engineAddress != "" {
		if err := rpcutil.Healthcheck(ctx, engineAddress, 5*time.Minute, cancel); err != nil {
			cmdutil.Exit(fmt.Errorf("could not start health check host RPC server: %w", err))
		}
	}

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancelChannel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, &executableLanguageHost{cancel: cancel})
			return nil
		},
		Options: rpcutil.TracingServerInterceptorOptions(nil),
	})
	if err != nil {
		cmdutil.Exit(fmt.Errorf("could not start language host RPC server: %w", err))
	}

	fmt.Printf("%d\n", handle.Port)

	if err := <-handle.Done; err != nil {
		cmdutil.Exit(fmt.Errorf("language host RPC stopped serving: %w", err))
	}
}
