// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

// pulumi-language-dotnet serves as the "language host" for Pulumi programs written in .NET.  It is ultimately
// responsible for spawning the language runtime that executes the program.
//
// The program being executed is executed by a shim exe called `pulumi-language-dotnet-exec`. This script is
// written in the hosted language (in this case, C#) and is responsible for initiating RPC links to the resource
// monitor and engine.
//
// It's therefore the responsibility of this program to implement the LanguageHostServer endpoint by spawning
// instances of `pulumi-language-dotnet-exec` and forwarding the RPC request arguments to the command-line.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/golang/glog"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/rpcutil"
	"github.com/pulumi/pulumi/pkg/version"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
	"google.golang.org/grpc"
)

var (
	// A exit-code we recognize when the nodejs process exits.  If we see this error, there's no
	// need for us to print any additional error messages since the user already got a a good
	// one they can handle.
	dotnetProcessExitedAfterShowingUserActionableMessage = 32
)

// Launches the language host RPC endpoint, which in turn fires up an RPC server implementing the
// LanguageRuntimeServer RPC endpoint.
func main() {
	var tracing string
	flag.StringVar(&tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")

	// You can use the below flag to request that the language host load a specific executor instead of probing the
	// PATH.  This can be used during testing to override the default location.
	var givenExecutor string
	flag.StringVar(&givenExecutor, "use-executor", "",
		"Use the given program as the executor instead of looking for one on PATH")

	flag.Parse()
	args := flag.Args()
	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing("pulumi-language-dotnet", "pulumi-language-dotnet", tracing)
	var dotnetExec string
	if givenExecutor == "" {
		pathExec, err := exec.LookPath("dotnet")
		if err != nil {
			err = errors.Wrap(err, "could not find `dotnet` on the $PATH")
			cmdutil.Exit(err)
		}

		glog.V(3).Infof("language host identified executor from path: `%s`", pathExec)
		dotnetExec = pathExec
	} else {
		glog.V(3).Infof("language host asked to use specific executor: `%s`", givenExecutor)
		dotnetExec = givenExecutor
	}

	// Optionally pluck out the engine so we can do logging, etc.
	var engineAddress string
	if len(args) > 0 {
		engineAddress = args[0]
	}

	// Fire up a gRPC server, letting the kernel choose a free port.
	port, done, err := rpcutil.Serve(0, nil, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			host := newLanguageHost(dotnetExec, engineAddress, tracing)
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
	}, nil)
	if err != nil {
		cmdutil.Exit(errors.Wrapf(err, "could not start language host RPC server"))
	}

	// Otherwise, print out the port so that the spawner knows how to reach us.
	fmt.Printf("%d\n", port)

	// And finally wait for the server to stop serving.
	if err := <-done; err != nil {
		cmdutil.Exit(errors.Wrapf(err, "language host RPC stopped serving"))
	}
}

// dotnetLanguageHost implements the LanguageRuntimeServer interface
// for use as an API endpoint.
type dotnetLanguageHost struct {
	exec          string
	engineAddress string
	tracing       string
}

func newLanguageHost(exec, engineAddress, tracing string) pulumirpc.LanguageRuntimeServer {

	return &dotnetLanguageHost{
		exec:          exec,
		engineAddress: engineAddress,
		tracing:       tracing,
	}
}

// GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
func (host *dotnetLanguageHost) GetRequiredPlugins(ctx context.Context,
	req *pulumirpc.GetRequiredPluginsRequest) (*pulumirpc.GetRequiredPluginsResponse, error) {
	// TODO: implement this.
	return &pulumirpc.GetRequiredPluginsResponse{}, nil
}

// RPC endpoint for LanguageRuntimeServer::Run
func (host *dotnetLanguageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	if err := host.DotnetBuild(ctx, req); err != nil {
		return &pulumirpc.RunResponse{Error: err.Error()}, nil
	}

	return host.DotnetRun(ctx, req)
}

func (host *dotnetLanguageHost) DotnetBuild(ctx context.Context, req *pulumirpc.RunRequest) error {
	args := []string{"build"}

	if req.GetProgram() != "" {
		args = append(args, req.GetProgram())
	}

	if glog.V(5) {
		commandStr := strings.Join(args, " ")
		glog.V(5).Infoln("Language host launching process: ", host.exec, commandStr)
	}

	// Make a connection to the real engine that we will log messages to.
	conn, err := grpc.Dial(host.engineAddress, grpc.WithInsecure())
	if err != nil {
		return errors.Wrapf(err, "language host could not make connection to engine")
	}

	// Make a client around that connection.  We can then make our own server that will act as a
	// monitor for the sdk and forward to the real monitor.
	engineClient := pulumirpc.NewEngineClient(conn)

	// Buffer the writes we see from dotnet from its stdout and stderr streams. We will display
	// these ephemerally as `dotnet build` runs.  If the build does fail though, we will dump
	// messages back to our own stdout/stderr so they get picked up and displayed to the user.
	streamID := rand.Int31()

	infoBuffer := &bytes.Buffer{}
	errorBuffer := &bytes.Buffer{}

	infoWriter := &logWriter{
		ctx:          ctx,
		engineClient: engineClient,
		streamID:     streamID,
		buffer:       infoBuffer,
		severity:     pulumirpc.LogSeverity_INFO,
	}

	errorWriter := &logWriter{
		ctx:          ctx,
		engineClient: engineClient,
		streamID:     streamID,
		buffer:       errorBuffer,
		severity:     pulumirpc.LogSeverity_ERROR,
	}

	// Now simply spawn a process to execute the requested program, wiring up stdout/stderr directly.
	cmd := exec.Command(host.exec, args...) // nolint: gas, intentionally running dynamic program name.

	cmd.Stdout = infoWriter
	cmd.Stderr = errorWriter

	_, err = engineClient.Log(ctx, &pulumirpc.LogRequest{
		Message:   "running 'dotnet build'",
		Urn:       "",
		Ephemeral: true,
		StreamId:  streamID,
		Severity:  pulumirpc.LogSeverity_INFO,
	})
	if err != nil {
		return err
	}

	if err := cmd.Run(); err != nil {
		// The command failed.  Dump any data we collected to the actual stdout/stderr streams so
		// they get displayed to the user.
		os.Stdout.Write(infoBuffer.Bytes())
		os.Stderr.Write(errorBuffer.Bytes())

		if exiterr, ok := err.(*exec.ExitError); ok {
			// If the program ran, but exited with a non-zero error code.  This will happen often, since user
			// errors will trigger this.  So, the error message should look as nice as possible.
			if status, stok := exiterr.Sys().(syscall.WaitStatus); stok {
				return errors.Errorf("'dotnet build' exited with non-zero exit code: %d", status.ExitStatus())
			}

			return errors.Wrapf(exiterr, "'dotnet build' exited unexpectedly")
		}

		// Otherwise, we didn't even get to run the program.  This ought to never happen unless there's
		// a bug or system condition that prevented us from running the language exec.  Issue a scarier error.
		return errors.Wrapf(err, "Problem executing 'dotnet build'")
	}

	_, err = engineClient.Log(ctx, &pulumirpc.LogRequest{
		Message:   "'dotnet build' completed successfully",
		Urn:       "",
		Ephemeral: true,
		StreamId:  streamID,
		Severity:  pulumirpc.LogSeverity_INFO,
	})

	return err
}

type logWriter struct {
	ctx          context.Context
	engineClient pulumirpc.EngineClient
	streamID     int32
	severity     pulumirpc.LogSeverity
	buffer       *bytes.Buffer
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	n, err = w.buffer.Write(p)
	if err != nil {
		return
	}

	_, err = w.engineClient.Log(w.ctx, &pulumirpc.LogRequest{
		Message:   string(p),
		Urn:       "",
		Ephemeral: true,
		StreamId:  w.streamID,
		Severity:  w.severity,
	})

	if err != nil {
		return 0, err
	}

	return len(p), nil
}

func (host *dotnetLanguageHost) DotnetRun(
	ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {

	config, err := host.constructConfig(req)
	if err != nil {
		err = errors.Wrap(err, "failed to serialize configuration")
		return nil, err
	}

	args := []string{"run"}

	if req.GetProgram() != "" {
		args = append(args, req.GetProgram())
	}

	if glog.V(5) {
		commandStr := strings.Join(args, " ")
		glog.V(5).Infoln("Language host launching process: ", host.exec, commandStr)
	}

	// Now simply spawn a process to execute the requested program, wiring up stdout/stderr directly.
	var errResult string
	cmd := exec.Command(host.exec, args...) // nolint: gas, intentionally running dynamic program name.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = host.constructEnv(req, config)
	if err := cmd.Run(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// If the program ran, but exited with a non-zero error code.  This will happen often, since user
			// errors will trigger this.  So, the error message should look as nice as possible.
			if status, stok := exiterr.Sys().(syscall.WaitStatus); stok {
				// Check if we got special exit code that means "we already gave the user an
				// actionable message". In that case, we can simply bail out and terminate `pulumi`
				// without showing any more messages.
				if status.ExitStatus() == dotnetProcessExitedAfterShowingUserActionableMessage {
					return &pulumirpc.RunResponse{Error: "", Bail: true}, nil
				}

				err = errors.Errorf("Program exited with non-zero exit code: %d", status.ExitStatus())
			} else {
				err = errors.Wrapf(exiterr, "Program exited unexpectedly")
			}
		} else {
			// Otherwise, we didn't even get to run the program.  This ought to never happen unless there's
			// a bug or system condition that prevented us from running the language exec.  Issue a scarier error.
			err = errors.Wrapf(err, "Problem executing program (could not run language executor)")
		}

		errResult = err.Error()
	}

	return &pulumirpc.RunResponse{Error: errResult}, nil
}

func (host *dotnetLanguageHost) constructEnv(req *pulumirpc.RunRequest, config string) []string {
	env := os.Environ()

	maybeAppendEnv := func(k, v string) {
		if v != "" {
			env = append(env, strings.ToUpper("PULUMI_"+k)+"="+v)
		}
	}

	maybeAppendEnv("monitor", req.GetMonitorAddress())
	maybeAppendEnv("engine", host.engineAddress)
	maybeAppendEnv("project", req.GetProject())
	maybeAppendEnv("stack", req.GetStack())
	maybeAppendEnv("pwd", req.GetPwd())
	maybeAppendEnv("dry_run", fmt.Sprintf("%v", req.GetDryRun()))
	maybeAppendEnv("query_mode", fmt.Sprint(req.GetQueryMode()))
	maybeAppendEnv("parallel", fmt.Sprint(req.GetParallel()))
	maybeAppendEnv("tracing", host.tracing)
	maybeAppendEnv("config", config)

	return env
}

// constructConfig json-serializes the configuration data given as part of a RunRequest.
func (host *dotnetLanguageHost) constructConfig(req *pulumirpc.RunRequest) (string, error) {
	configMap := req.GetConfig()
	if configMap == nil {
		return "", nil
	}

	configJSON, err := json.Marshal(configMap)
	if err != nil {
		return "", err
	}

	return string(configJSON), nil
}

func (host *dotnetLanguageHost) GetPluginInfo(ctx context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: version.Version,
	}, nil
}
