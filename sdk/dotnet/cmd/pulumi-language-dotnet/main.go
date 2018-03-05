// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

// pulumi-language-python serves as the "language host" for Pulumi programs written in Python.  It is ultimately
// responsible for spawning the language runtime that executes the program.
//
// The program being executed is executed by a shim script called `pulumi-language-python-exec`. This script is
// written in the hosted language (in this case, Python) and is responsible for initiating RPC links to the resource
// monitor and engine.
//
// It's therefore the responsibility of this program to implement the LanguageHostServer endpoint by spawning
// instances of `pulumi-language-python-exec` and forwarding the RPC request arguments to the command-line.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/golang/glog"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/rpcutil"
	"github.com/pulumi/pulumi/pkg/version"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
	"google.golang.org/grpc"
)

const (
	// By convention, the executor is the name of the current program (pulumi-language-python) plus this suffix.
	pythonExecSuffix = "-exec" // the exec shim for Pulumi to run Python programs.

	// The runtime expects the config object to be saved to this environment variable.
	pulumiConfigVar = "PULUMI_CONFIG"
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
	cmdutil.InitLogging(false, 0, false)
	cmdutil.InitTracing(os.Args[0], tracing)
	var pythonExec string
	if givenExecutor == "" {
		// The -exec binary is the same name as the current language host, except that we must trim off
		// the file extension (if any) and then append -exec to it.
		bin := os.Args[0]
		if ext := filepath.Ext(bin); ext != "" {
			bin = bin[:len(bin)-len(ext)]
		}
		bin += pythonExecSuffix
		pathExec, err := exec.LookPath(bin)
		if err != nil {
			err = errors.Wrapf(err, "could not find `%s` on the $PATH", bin)
			cmdutil.Exit(err)
		}

		glog.V(3).Infof("language host identified executor from path: `%s`", pathExec)
		pythonExec = pathExec
	} else {
		glog.V(3).Infof("language host asked to use specific executor: `%s`", givenExecutor)
		pythonExec = givenExecutor
	}

	// Optionally pluck out the engine so we can do logging, etc.
	var engineAddress string
	if len(args) > 0 {
		engineAddress = args[0]
	}

	// Fire up a gRPC server, letting the kernel choose a free port.
	port, done, err := rpcutil.Serve(0, nil, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			host := newLanguageHost(pythonExec, engineAddress, tracing)
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
	})
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

// pythonLanguageHost implements the LanguageRuntimeServer interface
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
	args := host.constructArguments(req)
	config, err := host.constructConfig(req)
	if err != nil {
		err = errors.Wrap(err, "failed to serialize configuration")
		return nil, err
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
	if config != "" {
		cmd.Env = append(os.Environ(), pulumiConfigVar+"="+config)
	}
	if err := cmd.Run(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// If the program ran, but exited with a non-zero error code.  This will happen often, since user
			// errors will trigger this.  So, the error message should look as nice as possible.
			if status, stok := exiterr.Sys().(syscall.WaitStatus); stok {
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

// constructArguments constructs a command-line for `pulumi-language-python`
// by enumerating all of the optional and non-optional arguments present
// in a RunRequest.
func (host *dotnetLanguageHost) constructArguments(req *pulumirpc.RunRequest) []string {
	var args []string
	maybeAppendArg := func(k, v string) {
		if v != "" {
			args = append(args, "--"+k, v)
		}
	}

	maybeAppendArg("monitor", req.GetMonitorAddress())
	maybeAppendArg("engine", host.engineAddress)
	maybeAppendArg("project", req.GetProject())
	maybeAppendArg("stack", req.GetStack())
	maybeAppendArg("pwd", req.GetPwd())
	maybeAppendArg("dry_run", fmt.Sprintf("%v", req.GetDryRun()))
	maybeAppendArg("parallel", fmt.Sprint(req.GetParallel()))
	maybeAppendArg("tracing", host.tracing)

	// If no program is specified, just default to the current directory (which will invoke "__main__.py").
	if req.GetProgram() == "" {
		args = append(args, ".")
	} else {
		args = append(args, req.GetProgram())
	}

	args = append(args, req.GetArgs()...)
	return args

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
