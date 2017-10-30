// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type plugin struct {
	stdoutDone <-chan bool
	stderrDone <-chan bool

	Conn   *grpc.ClientConn
	Proc   *os.Process
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

// pluginRPCConnectionTimeout dictates how long we wait for the plugin's RPC to become available.
var pluginRPCConnectionTimeout = time.Second * 10

func newPlugin(ctx *Context, bin string, prefix string, args []string) (*plugin, error) {
	if glog.V(9) {
		var argstr string
		for i, arg := range args {
			if i > 0 {
				argstr += ","
			}
			argstr += arg
		}
		glog.V(9).Infof("Launching plugin '%v' from '%v' with args: %v", prefix, bin, argstr)
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

	// If we did not successfully launch the plugin, we still need to wait for stderr and stdout to drain.
	defer func() {
		if plug.Conn == nil {
			contract.IgnoreError(plug.Close())
		}
	}()

	// For now, we will spawn goroutines that will spew STDOUT/STDERR to the relevant diag streams.
	runtrace := func(t io.Reader, stderr bool, done chan<- bool) {
		reader := bufio.NewReader(t)
		for {
			line, readerr := reader.ReadString('\n')
			if readerr != nil {
				break
			}
			msg := line[:len(line)-1]
			if stderr {
				ctx.Diag.Infoerrf(diag.Message("%s"), msg)
			} else {
				ctx.Diag.Infof(diag.Message("%s"), msg)
			}
		}
		close(done)
	}

	// Set up a tracer on stderr before going any further, since important errors might get communicated this way.
	stderrDone := make(chan bool)
	plug.stderrDone = stderrDone
	go runtrace(plug.Stderr, true, stderrDone)

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
	stdoutDone := make(chan bool)
	plug.stdoutDone = stdoutDone
	go runtrace(plug.Stdout, false, stdoutDone)

	// Now that we have the port, go ahead and create a gRPC client connection to it.
	conn, err := grpc.Dial(":"+port, grpc.WithInsecure())
	if err != nil {
		return nil, errors.Wrapf(err, "could not dial plugin [%v] over RPC", bin)
	}

	// Now wait for the gRPC connection to the plugin to become ready.
	// TODO[pulumi/pulumi#337]: in theory, this should be unnecessary.  gRPC's default WaitForReady behavior
	//     should auto-retry appropriately.  On Linux, however, we are observing different behavior.  In the meantime
	//     while this bug exists, we'll simply do a bit of waiting of our own up front.
	timeout, _ := context.WithTimeout(context.Background(), pluginRPCConnectionTimeout)
	for {
		s := conn.GetState()
		if s == connectivity.Ready {
			// The connection is supposedly ready; but we will make sure it is *actually* ready by sending a dummy
			// method invocation to the server.  Until it responds successfully, we can't safely proceed.
		outer:
			for {
				err = grpc.Invoke(timeout, "", nil, nil, conn)
				if err == nil {
					break // successful connect
				} else {
					// We have an error; see if it's a known status and, if so, react appropriately.
					status, ok := status.FromError(err)
					if ok {
						switch status.Code() {
						case codes.Unavailable:
							// The server is unavailable.  This is the Linux bug.  Wait a little and retry.
							time.Sleep(time.Millisecond * 10)
							continue // keep retrying
						case codes.Unimplemented, codes.ResourceExhausted:
							// Since we sent "" as the method above, this is the expected response.  Ready to go.
							break outer
						}
					}

					// Unexpected error; get outta dodge.
					return nil, errors.Wrapf(err, "%v plugin [%v] did not come alive", prefix, bin)
				}
			}
			break
		}
		// Not ready yet; ask the gRPC client APIs to block until the state transitions again so we can retry.
		if !conn.WaitForStateChange(timeout, s) {
			return nil, errors.Errorf("%v plugin [%v] did not begin responding to RPC connections", prefix, bin)
		}
	}

	// Done; store the connection and return the plugin info.
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
	if p.Conn != nil {
		closerr := p.Conn.Close()
		contract.IgnoreError(closerr)
	}

	// On windows, plugins are not loaded directly, instead a cmd script launches each plugin as a child process, so instead
	// We need kill all the children of the PID we have recorded, as well. Otherwise we will end up waiting forver.
	if runtime.GOOS == "windows" {
		err := cmdutil.KillChildren(p.Proc.Pid)
		if err != nil {
			return err
		}
	}

	// IDEA: consider a more graceful termination than just SIGKILL.
	err := p.Proc.Kill()

	// Wait for stdout and stderr to drain
	<-p.stdoutDone
	<-p.stderrDone

	return err
}
