// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
	"github.com/pulumi/pulumi/sdk/python/pkg/version"
)

func main() {
	// Parse args.
	var tracing string
	flag.StringVar(&tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
	flag.Parse()
	args := flag.Args()

	// Initialize loggers.
	cmdutil.InitLogging(false, 0, false)
	cmdutil.InitTracing(os.Args[0], tracing)

	// And now go ahead and execute the program.
	if err := serve(args); err != nil {
		cmdutil.Exit(err)
	}
}

// serve runs the gRPC server for this language host, awaiting demands to run Python programs.
func serve(args []string) error {
	// Ensure python is on the path.
	python, err := exec.LookPath("python")
	if err != nil {
		return errors.Wrapf(err, "could not find `python` on the $PATH")
	}

	// Ensure we can contact the host.
	if len(args) == 0 {
		return errors.New("missing host engine RPC address as first argument")
	}
	engineAddress := args[0]

	// Fire up a gRPC server, letting the kernel choose a free port.
	port, done, err := rpcutil.Serve(0, nil, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			host := newLanguageHost(python, engineAddress)
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

// languageHost is an instance of a Python language host gRPC server.
type languageHost struct {
	python        string
	engineAddress string
}

// newLanguageHost allocates a new Python language host gRPC server, ready for serving.
func newLanguageHost(python string, engineAddress string) pulumirpc.LanguageRuntimeServer {
	return &languageHost{
		python:        python,
		engineAddress: engineAddress,
	}
}

// Run executes the Python program specified in the request, returning its results.
func (hist *languageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	// First, build up the arguments to pass to our Python launcher.
	var args []string
	maybeAppendArg := func(k, v string) {
		if v != "" {
			args = append(args, "--"+k)
			args = append(args, v)
		}
	}
	maybeAppendArg("project", req.GetProject())
	maybeAppendArg("stack", req.GetStack())
	maybeAppendArg("pwd", req.GetPwd())
	maybeAppendArg("program", req.GetProgram())
	maybeAppendArg("dry-run", strconv.FormatBool(req.GetDryRun()))
	maybeAppendArg("parallel", fmt.Sprint(req.GetParallel()))

	// TODO: config and args.

	// Now simply spawn a Python process to execute the requested program, wiring up stdout/stderr directly.
	var errResult string
	cmd := exec.Command(os.Args[0]+"-exec", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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
			// a bug or system condition that prevented us from running Python.  Issue a scarier error.
			err = errors.Wrapf(err, "problem executing program (could not run Python)")
		}
		errResult = err.Error()
	}

	return &pulumirpc.RunResponse{Error: errResult}, nil
}

// GetPluginInfo returns generic information about this plugin, like its version.
func (host *languageHost) GetPluginInfo(ctx context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: version.Version,
	}, nil
}
