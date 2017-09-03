// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi-fabric/pkg/diag"
	"github.com/pulumi/pulumi-fabric/pkg/util/cmdutil"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

type plugin struct {
	Conn   *grpc.ClientConn
	Proc   *os.Process
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

func newPlugin(ctx *Context, bin string, prefix string, args []string) (*plugin, error) {
	if glog.V(9) {
		var argstr string
		for _, arg := range args {
			argstr += " " + arg
		}
		glog.V(9).Infof("Launching plugin '%v' from '%v' with args '%v'", prefix, bin, argstr)
	}

	// Try to execute the binary.
	plug, err := execPlugin(bin, args)
	if err != nil {
		// If we failed simply because we couldn't load the binary, return nil rather than an error.
		if execerr, isexecerr := err.(*exec.Error); isexecerr && execerr.Err == exec.ErrNotFound {
			return nil, nil
		}

		return nil, errors.Wrapf(err, "plugin [%v] failed to load", bin)
	}
	contract.Assert(plug != nil)

	// For now, we will spawn goroutines that will spew STDOUT/STDERR to the relevant diag streams.
	// TODO[pulumi/pulumi-fabric#143]: eventually we want real progress reporting, etc., out of band
	//     via RPC.  This will be particularly important when we parallelize the application of the resource graph.
	tracers := map[io.Reader]struct {
		lbl string
		cb  func(string)
	}{
		plug.Stderr: {"stderr", func(line string) { ctx.Diag.Errorf(diag.Message("%s"), line) }},
		plug.Stdout: {"stdout", func(line string) { ctx.Diag.Infof(diag.Message("%s"), line) }},
	}
	runtrace := func(t io.Reader) {
		ts := tracers[t]
		reader := bufio.NewReader(t)
		for {
			line, readerr := reader.ReadString('\n')
			if readerr != nil {
				break
			}
			ts.cb(fmt.Sprintf("%v.%v: %v", prefix, ts.lbl, line[:len(line)-1]))
		}
	}

	// Set up a tracer on stderr before going any further, since important errors might get communicated this way.
	go runtrace(plug.Stderr)

	// Now that we have a process, we expect it to write a single line to STDOUT: the port it's listening on.  We only
	// read a byte at a time so that STDOUT contains everything after the first newline.
	var port string
	b := make([]byte, 1)
	for {
		n, readerr := plug.Stdout.Read(b)
		if readerr != nil {
			killerr := plug.Proc.Kill()
			contract.IgnoreError(killerr) // we are ignoring because the readerr trumps it.
			if port == "" {
				return nil, errors.Wrapf(readerr, "could not read plugin [%v] stdout", bin)
			}
			return nil, errors.Wrapf(readerr, "failure reading plugin [%v] stdout (read '%v')", bin, port)
		}
		if n > 0 && b[0] == '\n' {
			break
		}
		port += string(b[:n])
	}

	// Parse the output line (minus the '\n') to ensure it's a numeric port.
	if _, err = strconv.Atoi(port); err != nil {
		killerr := plug.Proc.Kill()
		contract.IgnoreError(killerr) // ignoring the error because the existing one trumps it.
		return nil, errors.Wrapf(
			err, "%v plugin [%v] wrote a non-numeric port to stdout ('%v')", prefix, bin, port)
	}

	// After reading the port number, set up a tracer on stdout just so other output doesn't disappear.
	go runtrace(plug.Stdout)

	// Now that we have the port, go ahead and create a gRPC client connection to it.
	conn, err := grpc.Dial(":"+port, grpc.WithInsecure())
	if err != nil {
		return nil, errors.Wrapf(err, "could not dial plugin [%v] over RPC", bin)
	}
	plug.Conn = conn
	return plug, nil
}

func execPlugin(bin string, args []string) (*plugin, error) {
	// Flow the logging information if set.
	if cmdutil.LogFlow {
		if cmdutil.LogToStderr {
			args = append(args, "--logtostderr")
		}
		if cmdutil.Verbose > 0 {
			args = append(args, "-v="+strconv.Itoa(cmdutil.Verbose))
		}
	}

	cmd := exec.Command(bin, args...)
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	err, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &plugin{
		Proc:   cmd.Process,
		Stdin:  in,
		Stdout: out,
		Stderr: err,
	}, nil
}

func (p *plugin) Close() error {
	closerr := p.Conn.Close()
	contract.IgnoreError(closerr)
	// IDEA: consider a more graceful termination than just SIGKILL.
	return p.Proc.Kill()
}
