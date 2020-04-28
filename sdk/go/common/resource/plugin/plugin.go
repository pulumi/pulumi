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
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/rpcutil"
)

type plugin struct {
	stdoutDone <-chan bool
	stderrDone <-chan bool

	Bin  string
	Args []string
	// Env specifies the environment of the plugin in the same format as go's os/exec.Cmd.Env
	// https://golang.org/pkg/os/exec/#Cmd (each entry is of the form "key=value").
	Env    []string
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

// errRunPolicyModuleNotFound is returned when we determine that the plugin failed to load because
// the stack's Pulumi SDK did not have the required modules. i.e. is too old.
var errRunPolicyModuleNotFound = errors.New("pulumi SDK does not support policy as code")

// errPluginNotFound is returned when we try to execute a plugin but it is not found on disk.
var errPluginNotFound = errors.New("plugin not found")

func newPlugin(ctx *Context, pwd, bin, prefix string, args, env []string) (*plugin, error) {
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
	plug, err := execPlugin(bin, args, pwd, env)
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
	var sawPolicyModuleNotFoundErr bool
	runtrace := func(t io.Reader, stderr bool, done chan<- bool) {
		reader := bufio.NewReader(t)

		for {
			msg, readerr := reader.ReadString('\n')
			if readerr != nil {
				break
			}

			// We may be trying to run a plugin that isn't present in the SDK installed with the Policy Pack.
			// e.g. the stack's package.json does not contain a recent enough @pulumi/pulumi.
			//
			// Rather than fail with an opaque error because we didn't get the gRPC port, inspect if it
			// is a well-known problem and return a better error as appropriate.
			if strings.Contains(msg, "Cannot find module '@pulumi/pulumi/cmd/run-policy-pack'") {
				sawPolicyModuleNotFoundErr = true
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
			contract.IgnoreError(killerr) // We are ignoring because the readerr trumps it.

			// If from the output we have seen, return a specific error if possible.
			if sawPolicyModuleNotFoundErr {
				return nil, errRunPolicyModuleNotFound
			}

			// Fall back to a generic, opaque error.
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
	conn, err := grpc.Dial(
		"127.0.0.1:"+port,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(rpcutil.OpenTracingClientInterceptor()),
		rpcutil.GrpcChannelOptions(),
	)
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
						default:
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

// execPlugin starts the plugin executable.
func execPlugin(bin string, pluginArgs []string, pwd string, env []string) (*plugin, error) {
	var args []string
	// Flow the logging information if set.
	if logging.LogFlow {
		if logging.LogToStderr {
			args = append(args, "--logtostderr")
		}
		if logging.Verbose > 0 {
			args = append(args, "-v="+strconv.Itoa(logging.Verbose))
		}
	}
	// Flow tracing settings if we are using a remote collector.
	if cmdutil.TracingEndpoint != "" && !cmdutil.TracingToFile {
		args = append(args, "--tracing", cmdutil.TracingEndpoint)
	}
	args = append(args, pluginArgs...)

	cmd := exec.Command(bin, args...)
	cmdutil.RegisterProcessGroup(cmd)
	cmd.Dir = pwd
	if len(env) > 0 {
		cmd.Env = env
	}
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	err, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		// If we try to run a plugin that isn't found, intercept the error
		// and instead return a custom one so we can more easily check for
		// it upstream
		//
		// In the case of PAC, note that the plugin usually _does_ exist.
		// It is a shell script like "pulumi-analyzer-policy". But during
		// the execution of that script, it fails with the ENOENT error.
		if pathErr, ok := err.(*os.PathError); ok {
			syscallErr, ok := pathErr.Err.(syscall.Errno)
			if ok && syscallErr == syscall.ENOENT {
				return nil, errPluginNotFound
			}

		}
		return nil, err
	}

	return &plugin{
		Bin:    bin,
		Args:   args,
		Env:    env,
		Proc:   cmd.Process,
		Stdin:  in,
		Stdout: out,
		Stderr: err,
	}, nil
}

func (p *plugin) Close() error {
	if p.Conn != nil {
		contract.IgnoreClose(p.Conn)
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
