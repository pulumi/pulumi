// Copyright 2016-2018, Pulumi Corporation.
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

package plugin

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/rpcutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// MissingError is returned if a plugin is missing.
type MissingError struct {
	// Info contains information about the plugin that was not found.
	Info workspace.PluginInfo
}

// NewMissingError allocates a new error indicating the given plugin info was not found.
func NewMissingError(info workspace.PluginInfo) error {
	return &MissingError{
		Info: info,
	}
}

func (err *MissingError) Error() string {
	if err.Info.Version != nil {
		return fmt.Sprintf("no %[1]s plugin '%[2]s-v%[3]s' found in the workspace or on your $PATH, "+
			"install the plugin using `pulumi plugin install %[1]s %[2]s v%[3]s`",
			err.Info.Kind, err.Info.Name, err.Info.Version)
	}

	return fmt.Sprintf("no %s plugin '%s' found in the workspace or on your $PATH",
		err.Info.Kind, err.Info.String())
}

type plugin struct {
	stdoutDone <-chan bool
	stderrDone <-chan bool

	Bin    string
	Args   []string
	Conn   *grpc.ClientConn
	Proc   *os.Process
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

// pluginRPCConnectionTimeout dictates how long we wait for the plugin's RPC to become available.
var pluginRPCConnectionTimeout = time.Second * 10

// A unique ID provided to the output stream of each plugin.  This allows the output of the plugin
// to be streamed to the display, while still allowing that output to be sent a small piece at a
// time.
var nextStreamID int32

func newPlugin(ctx *Context, bin string, prefix string, args []string) (*plugin, error) {
	if logging.V(9) {
		var argstr string
		for i, arg := range args {
			if i > 0 {
				argstr += ","
			}
			argstr += arg
		}
		logging.V(9).Infof("Launching plugin '%v' from '%v' with args: %v", prefix, bin, argstr)
	}

	// Try to execute the binary.
	plug, err := execPlugin(bin, args, ctx.Pwd)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load plugin %s", bin)
	}
	contract.Assert(plug != nil)

	// If we did not successfully launch the plugin, we still need to wait for stderr and stdout to drain.
	defer func() {
		if plug.Conn == nil {
			contract.IgnoreError(plug.Close())
		}
	}()

	outStreamID := atomic.AddInt32(&nextStreamID, 1)
	errStreamID := atomic.AddInt32(&nextStreamID, 1)

	// For now, we will spawn goroutines that will spew STDOUT/STDERR to the relevant diag streams.
	runtrace := func(t io.Reader, stderr bool, done chan<- bool) {
		reader := bufio.NewReader(t)

		for {
			msg, readerr := reader.ReadString('\n')
			if readerr != nil {
				break
			}

			if strings.TrimSpace(msg) != "" {
				if stderr {
					ctx.Diag.Infoerrf(diag.StreamMessage("" /*urn*/, msg, errStreamID))
				} else {
					ctx.Diag.Infof(diag.StreamMessage("" /*urn*/, msg, outStreamID))
				}
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
	conn, err := grpc.Dial("127.0.0.1:"+port, grpc.WithInsecure(), grpc.WithUnaryInterceptor(
		rpcutil.OpenTracingClientInterceptor(),
	))
	if err != nil {
		return nil, errors.Wrapf(err, "could not dial plugin [%v] over RPC", bin)
	}

	// Now wait for the gRPC connection to the plugin to become ready.
	if err = waitUntilReady(prefix, bin, conn); err != nil {
		return nil, err
	}

	// Done; store the connection and return the plugin info.
	plug.Conn = conn
	return plug, nil
}

// waitUntilReady waits for the gRPC connection to the plugin to become ready.
//
// Historically, we have had issues with gRPC's WaitForReady behavior. It is not clear in 2018 that these issues are
// still present and, if so, the nature of those issues.
func waitUntilReady(prefix, bin string, conn *grpc.ClientConn) error {
	logging.V(9).Infof("Waiting for gRPC client %q to become active", prefix)

	// The gRPC client can be in one of five readiness states:
	//   1. READY, indicating that the server is ready to serve RPCs
	//   2. CONNECTING, the channel is still under construction (e.g. the TCP/TLS handshake is still in progress) and we
	//      have not yet successfully connected to the server
	//   3. TRANSIENT_FAILURE, the connection failed to establish and gRPC is retrying it
	//   4. IDLE, the connection failed to establish and gRPC is NOT retrying it.
	//   5. SHUTDOWN, the connection is shutting down.
	//
	// We should only return successfully from this method once the client is known to be in the READY state, in which
	// case it is ready to receive new RPCs.
	//
	// Since we'll be doing our own retrying, set up a 10 second timeout so we don't retry forever.
	ctx, _ := context.WithTimeout(context.Background(), pluginRPCConnectionTimeout)
	for attempt := 0; ; attempt++ {
		logging.V(9).Infof("Querying gRPC client %q, attempt %d", prefix, attempt)

		// Ping the server by sending it an RPC, with the FailFast option set to false. This instructs gRPC to make the
		// following state transitions:
		//
		//   1. If the connection is READY, do the RPC and exit the state machine loop.
		//   2. If the connection is CONNECTING, WAIT for the connection to transition to its next state
		//   3. If the connection is TRANSIENT_FAILURE, WAIT for the connection to transition back to CONNECTING
		//   4. If the connection is IDLE, transition immediately to CONNECTING,
		//   5. If the connection is SHUTDOWN, fail the RPC immediately.
		//
		// When the RPC that we just issued returns, the connection will either be READY or in an error state. Either
		// way, gRPC should return to us an error because it does not recognize the method that we are attempting to
		// invoke here.
		err := conn.Invoke(ctx, "", nil, nil, grpc.FailFast(false))
		if err == nil {
			// Previously, the code regarded this to be a success. This seems suspect, though, since I would expect the
			// gRPC server to reject the invoke we just made since "" is not a valid method.
			//
			// EIther way, if gRPC thinks things are cool, then we're cool. Just log something so that we can understand
			// what happened if something goes off the rails later.
			logging.V(9).Infof("gRPC client %q returned suspect non-error", prefix)
			return nil
		}

		// gRPC generally responds with errors that can be turned into *Statuses via status.FromError. If this is some
		// other error, bail immediately.
		rpcStatus, ok := status.FromError(err)
		if !ok {
			return errors.Wrapf(err, "%v plugin [%v] did not come alive", prefix, bin)
		}

		// If this is a Status, we'd expect it to respond with Unimplemented or ResourceExhausted.
		switch rpcStatus.Code() {
		case codes.Unavailable:
			// This is apparently the crux of the issue referenced in [pulumi/pulumi#337]. It's unclear if this still
			// happens. If it does, we should just retry.
			logging.V(9).Infof("gRPC client %q claimed it was Unavailable despite returning, waiting to retry", prefix)
			time.Sleep(10 * time.Millisecond)
			logging.V(9).Infof("gRPC client %q retrying", prefix)
			continue
		case codes.Unimplemented, codes.ResourceExhausted:
			// Why codes.ResourceExhausted? According to the gRPC spec, servers should only be sending this if they
			// don't have enough resources to service the invoke that we just performed, which doesn't sound like
			// readiness. The old code accepted codes.ResourceExhausted as evidence of readiness.
			//
			// Since it's not clear if this ever happens, we'll just log it and move on.
			if rpcStatus.Code() == codes.ResourceExhausted {
				logging.V(9).Infof("gRPC client %q responded with ResourceExhausted, proceeding", prefix)
			}

			logging.V(9).Infof("gRPC client %q confirmed online", prefix)
			return nil
		default:
			// This is extremely unlikely, but in the insane event that gRPC does respond with some other error code, we
			// might as well roll with it.
			logging.V(9).Infof("gRPC client %q responded with strange code %d", prefix, rpcStatus.Code())
			return nil
		}
	}
}

func execPlugin(bin string, pluginArgs []string, pwd string) (*plugin, error) {
	var args []string
	// Flow the logging information if set.
	if logging.LogFlow {
		if logging.LogToStderr {
			args = append(args, "-logtostderr")
		}
		if logging.Verbose > 0 {
			args = append(args, "-v="+strconv.Itoa(logging.Verbose))
		}
	}
	// Always flow tracing settings.
	if cmdutil.TracingEndpoint != "" {
		args = append(args, "--tracing", cmdutil.TracingEndpoint)
	}
	args = append(args, pluginArgs...)

	cmd := exec.Command(bin, args...)
	cmdutil.RegisterProcessGroup(cmd)
	cmd.Dir = pwd
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	err, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &plugin{
		Bin:    bin,
		Args:   args,
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

	var result error

	// On each platform, plugins are not loaded directly, instead a shell launches each plugin as a child process, so
	// instead we need to kill all the children of the PID we have recorded, as well. Otherwise we will block waiting
	// for the child processes to close.
	if err := cmdutil.KillChildren(p.Proc.Pid); err != nil {
		result = multierror.Append(result, err)
	}

	// IDEA: consider a more graceful termination than just SIGKILL.
	if err := p.Proc.Kill(); err != nil {
		result = multierror.Append(result, err)
	}

	// Wait for stdout and stderr to drain.
	if p.stdoutDone != nil {
		<-p.stdoutDone
	}
	if p.stderrDone != nil {
		<-p.stderrDone
	}

	return result
}
