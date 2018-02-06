// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
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
	nodeExecSuffix = "-exec" // the exec shim for Pulumi to run Node programs.
)

func RunLanguageHost() error {
	var tracing string
	flag.StringVar(&tracing, "tracing", "",
		"Emit tracing to a Zipkin-compatible tracing endpoint")

	flag.Parse()
	args := flag.Args()
	cmdutil.InitLogging(false, 0, false)
	cmdutil.InitTracing(os.Args[0], tracing)
	glog.V(3).Info("firing up the language host (for real)!")

	nodeExec, err := exec.LookPath(os.Args[0] + nodeExecSuffix)
	if err != nil {
		err = errors.Wrapf(err, "could not find `node` on the $PATH")
		cmdutil.Exit(err)
	}

	if len(args) == 0 {
		return errors.New("missing host engine RPC address as first argument")
	}

	monitorAddress := args[0]

	// Optionally pluck out the engine so we can do logging, etc.
	var engineAddress string
	if len(args) > 1 {
		engineAddress = args[1]
	}

	// Fire up a gRPC server, letting the kernel choose a free port.
	port, done, err := rpcutil.Serve(0, nil, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			host := newLanguageHost(nodeExec, monitorAddress, engineAddress, tracing)
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
	})
	if err != nil {
		return errors.Wrapf(err, "could not start language host RPC server")
	}

	// Otherwise, print out the port so that the spawner knows how to reach us.
	fmt.Printf("%d\n", port)

	// And finally wait for the server to stop serving.
	if err := <-done; err != nil {
		return errors.Wrapf(err, "language host RPC stopped serving")
	}

	return nil
}

type nodeLanguageHost struct {
	exec           string
	monitorAddress string
	engineAddress  string
	tracing        string
}

func newLanguageHost(exec, monitorAddress, engineAddress, tracing string) pulumirpc.LanguageRuntimeServer {
	return &nodeLanguageHost{
		exec:           exec,
		monitorAddress: monitorAddress,
		engineAddress:  engineAddress,
		tracing:        tracing,
	}
}

func serializeConfig(req *pulumirpc.RunRequest) (string, error) {
	configMap := req.GetConfig()
	if len(configMap) == 0 {
		return "{}", nil
	}

	configJSON, err := json.Marshal(configMap)
	if err != nil {
		return "", err
	}

	return string(configJSON), nil
}

func (host *nodeLanguageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	// First, build up the arguments to pass to our launcher.
	var args []string

	// Append optional settings.
	maybeAppendArg := func(k, v string) {
		if v != "" {
			args = append(args, "--"+k)
			args = append(args, v)
		}
	}

	maybeAppendArg("monitor", host.monitorAddress)
	maybeAppendArg("engine", host.engineAddress)
	maybeAppendArg("project", req.GetProject())
	maybeAppendArg("stack", req.GetStack())
	maybeAppendArg("pwd", req.GetPwd())
	maybeAppendArg("dry-run", strconv.FormatBool(req.GetDryRun()))
	maybeAppendArg("parallel", fmt.Sprint(req.GetParallel()))
	maybeAppendArg("tracing", host.tracing)

	glog.V(3).Info("config from rpc: %v", req.GetConfig())
	configJSON, err := serializeConfig(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to serialize config as json")
	}

	args = append(args, req.GetProgram())
	args = append(args, req.GetArgs()...)

	glog.V(3).Infof("command: %v", host.exec)
	glog.V(3).Infof("arguments: %v", args)
	glog.V(3).Infof("config: %v", string(configJSON))
	// Now simply spawn a Python process to execute the requested program, wiring up stdout/stderr directly.
	var errResult string
	cmd := exec.Command(host.exec, args...) // nolint: gas, intentionally running dynamic program name.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "PULUMI_CONFIG="+string(configJSON))
	if err := cmd.Run(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// If the program ran, but exited with a non-zero error code.  This will happen often, since user
			// errors will trigger this.  So, the error message should look as nice as possible.
			if status, stok := exiterr.Sys().(syscall.WaitStatus); stok {
				err = errors.Errorf("program exited with error code %d", status.ExitStatus())
			} else {
				err = errors.Wrapf(exiterr, "program exited unexpectedly")
			}
		} else {
			// Otherwise, we didn't even get to run the program.  This ought to never happen unless there's
			// a bug or system condition that prevented us from running the language host.  Issue a scarier error.
			err = errors.Wrapf(err, "problem executing program (could not run Node language host)")
		}

		errResult = err.Error()
	}

	return &pulumirpc.RunResponse{Error: errResult}, nil
}

func (host *nodeLanguageHost) GetPluginInfo(ctx context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: version.Version,
	}, nil
}

func main() {
	err := RunLanguageHost()
	if err != nil {
		cmdutil.Exit(err)
	}
}
