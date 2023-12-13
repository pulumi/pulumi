// Copyright 2016-2022, Pulumi Corporation.
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	opentracing "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// PulumiPluginJSON represents additional information about a package's associated Pulumi plugin.
// For Python, the content is inside a pulumi-plugin.json file inside the package.
// For Node.js, the content is within the package.json file, under the "pulumi" node.
// For .NET, the content is inside a pulumi-plugin.json file inside the NuGet package.
// For Go, the content is inside a pulumi-plugin.json file inside the module.
type PulumiPluginJSON struct {
	// Indicates whether the package has an associated resource plugin. Set to false to indicate no plugin.
	Resource bool `json:"resource"`
	// Optional plugin name. If not set, the plugin name is derived from the package name.
	Name string `json:"name,omitempty"`
	// Optional plugin version. If not set, the version is derived from the package version (if possible).
	Version string `json:"version,omitempty"`
	// Optional plugin server. If not set, the default server is used when installing the plugin.
	Server string `json:"server,omitempty"`
}

func (plugin *PulumiPluginJSON) JSON() ([]byte, error) {
	json, err := json.MarshalIndent(plugin, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(json, '\n'), nil
}

func LoadPulumiPluginJSON(path string) (*PulumiPluginJSON, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		// Deliberately not wrapping the error here so that os.IsNotExist checks can be used to determine
		// if the file could not be opened due to it not existing.
		return nil, err
	}

	plugin := &PulumiPluginJSON{}
	if err := json.Unmarshal(b, plugin); err != nil {
		return nil, err
	}

	return plugin, nil
}

type plugin struct {
	stdoutDone <-chan bool
	stderrDone <-chan bool

	Bin  string
	Args []string
	// Env specifies the environment of the plugin in the same format as go's os/exec.Cmd.Env
	// https://golang.org/pkg/os/exec/#Cmd (each entry is of the form "key=value").
	Env  []string
	Conn *grpc.ClientConn
	// Function to trigger the plugin to be killed. If the plugin is a process this will just SIGKILL it.
	Kill   func() error
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

func dialPlugin(portNum int, bin, prefix string, dialOptions []grpc.DialOption) (*grpc.ClientConn, error) {
	port := strconv.Itoa(portNum)

	// Now that we have the port, go ahead and create a gRPC client connection to it.
	conn, err := grpc.Dial("127.0.0.1:"+port, dialOptions...)
	if err != nil {
		return nil, fmt.Errorf("could not dial plugin [%v] over RPC: %w", bin, err)
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
				}

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
				return nil, fmt.Errorf("%v plugin [%v] did not come alive: %w", prefix, bin, err)
			}
			break
		}
		// Not ready yet; ask the gRPC client APIs to block until the state transitions again so we can retry.
		if !conn.WaitForStateChange(timeout, s) {
			return nil, fmt.Errorf("%v plugin [%v] did not begin responding to RPC connections", prefix, bin)
		}
	}

	return conn, nil
}

func newPlugin(ctx *Context, pwd, bin, prefix string, kind workspace.PluginKind,
	args, env []string, dialOptions []grpc.DialOption,
) (*plugin, error) {
	if logging.V(9) {
		var argstr string
		for i, arg := range args {
			if i > 0 {
				argstr += ","
			}
			argstr += arg
		}
		logging.V(9).Infof("newPlugin(): Launching plugin '%v' from '%v' with args: %v", prefix, bin, argstr)
	}

	// Create a span for the plugin initialization
	opts := []opentracing.StartSpanOption{
		opentracing.Tag{Key: "prefix", Value: prefix},
		opentracing.Tag{Key: "bin", Value: bin},
		opentracing.Tag{Key: "pulumi-decorator", Value: prefix + ":" + bin},
	}
	if ctx != nil && ctx.tracingSpan != nil {
		opts = append(opts, opentracing.ChildOf(ctx.tracingSpan.Context()))
	}
	tracingSpan := opentracing.StartSpan("newPlugin", opts...)
	defer tracingSpan.Finish()

	// Try to execute the binary.
	plug, err := execPlugin(ctx, bin, prefix, kind, args, pwd, env)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin %s: %w", bin, err)
	}
	contract.Assertf(plug != nil, "plugin %v canot be nil", bin)

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

			// Even if we've hit the end of the stream, we want to check for non-empty content.
			// The reason is that if the last line is missing a \n, we still want to include it.
			if strings.TrimSpace(msg) != "" {
				// We may be trying to run a plugin that isn't present in the SDK installed with the Policy Pack.
				// e.g. the stack's package.json does not contain a recent enough @pulumi/pulumi.
				//
				// Rather than fail with an opaque error because we didn't get the gRPC port, inspect if it
				// is a well-known problem and return a better error as appropriate.
				if strings.Contains(msg, "Cannot find module '@pulumi/pulumi/cmd/run-policy-pack'") {
					sawPolicyModuleNotFoundErr = true
				}

				if stderr {
					ctx.Diag.Infoerrf(diag.StreamMessage("" /*urn*/, msg, errStreamID))
				} else {
					ctx.Diag.Infof(diag.StreamMessage("" /*urn*/, msg, outStreamID))
				}
			}

			// If we've hit the end of the stream, break out and close the channel.
			if readerr != nil {
				break
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
	var portString string
	b := make([]byte, 1)
	for {
		n, readerr := plug.Stdout.Read(b)
		if readerr != nil {
			killerr := plug.Kill()
			contract.IgnoreError(killerr) // We are ignoring because the readerr trumps it.

			// If from the output we have seen, return a specific error if possible.
			if sawPolicyModuleNotFoundErr {
				return nil, errRunPolicyModuleNotFound
			}

			// Fall back to a generic, opaque error.
			if portString == "" {
				return nil, fmt.Errorf("could not read plugin [%v] stdout: %w", bin, readerr)
			}
			return nil, fmt.Errorf("failure reading plugin [%v] stdout (read '%v'): %w",
				bin, portString, readerr)
		}
		if n > 0 && b[0] == '\n' {
			break
		}
		portString += string(b[:n])
	}
	// Trim any whitespace from the first line (this is to handle things like windows that will write
	// "1234\r\n", or slightly odd providers that might add whitespace like "1234 ")
	portString = strings.TrimSpace(portString)

	// Parse the output line (minus the '\n') to ensure it's a numeric port.
	var port int
	if port, err = strconv.Atoi(portString); err != nil {
		killerr := plug.Kill()
		contract.IgnoreError(killerr) // ignoring the error because the existing one trumps it.
		return nil, fmt.Errorf(
			"%v plugin [%v] wrote a non-numeric port to stdout ('%v'): %w", prefix, bin, port, err)
	}

	// After reading the port number, set up a tracer on stdout just so other output doesn't disappear.
	stdoutDone := make(chan bool)
	plug.stdoutDone = stdoutDone
	go runtrace(plug.Stdout, false, stdoutDone)

	conn, err := dialPlugin(port, bin, prefix, dialOptions)
	if err != nil {
		return nil, err
	}

	// Done; store the connection and return the plugin info.
	plug.Conn = conn
	return plug, nil
}

// execPlugin starts the plugin executable.
func execPlugin(ctx *Context, bin, prefix string, kind workspace.PluginKind,
	pluginArgs []string, pwd string, env []string,
) (*plugin, error) {
	args := buildPluginArguments(pluginArgumentOptions{
		pluginArgs:      pluginArgs,
		tracingEndpoint: cmdutil.TracingEndpoint,
		logFlow:         logging.LogFlow,
		logToStderr:     logging.LogToStderr,
		verbose:         logging.Verbose,
	})

	// Check to see if we have a binary we can invoke directly
	if _, err := os.Stat(bin); os.IsNotExist(err) {
		// If we don't have the expected binary, see if we have a "PulumiPlugin.yaml" or "PulumiPolicy.yaml"
		pluginDir := filepath.Dir(bin)

		var runtimeInfo workspace.ProjectRuntimeInfo
		if kind == workspace.ResourcePlugin || kind == workspace.ConverterPlugin {
			proj, err := workspace.LoadPluginProject(filepath.Join(pluginDir, "PulumiPlugin.yaml"))
			if err != nil {
				return nil, fmt.Errorf("loading PulumiPlugin.yaml: %w", err)
			}
			runtimeInfo = proj.Runtime
		} else if kind == workspace.AnalyzerPlugin {
			proj, err := workspace.LoadPluginProject(filepath.Join(pluginDir, "PulumiPolicy.yaml"))
			if err != nil {
				return nil, fmt.Errorf("loading PulumiPolicy.yaml: %w", err)
			}
			runtimeInfo = proj.Runtime
		} else {
			return nil, errors.New("language plugins must be executable binaries")
		}

		logging.V(9).Infof("Launching plugin '%v' from '%v' via runtime '%s'", prefix, pluginDir, runtimeInfo.Name())

		runtime, err := ctx.Host.LanguageRuntime(pluginDir, pluginDir, runtimeInfo.Name(), runtimeInfo.Options())
		if err != nil {
			return nil, fmt.Errorf("loading runtime: %w", err)
		}

		stdout, stderr, kill, err := runtime.RunPlugin(RunPluginInfo{
			Pwd:     pwd,
			Program: pluginDir,
			Args:    pluginArgs,
			Env:     env,
		})
		if err != nil {
			return nil, err
		}

		return &plugin{
			Bin:    bin,
			Args:   args,
			Env:    env,
			Kill:   func() error { kill(); return nil },
			Stdout: io.NopCloser(stdout),
			Stderr: io.NopCloser(stderr),
		}, nil
	}

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

	kill := func() error {
		var result *multierror.Error

		// On each platform, plugins are not loaded directly, instead a shell launches each plugin as a child process, so
		// instead we need to kill all the children of the PID we have recorded, as well. Otherwise we will block waiting
		// for the child processes to close.
		if err := cmdutil.KillChildren(cmd.Process.Pid); err != nil {
			result = multierror.Append(result, err)
		}

		// IDEA: consider a more graceful termination than just SIGKILL.
		if err := cmd.Process.Kill(); err != nil {
			result = multierror.Append(result, err)
		}

		return result.ErrorOrNil()
	}

	return &plugin{
		Bin:    bin,
		Args:   args,
		Env:    env,
		Kill:   kill,
		Stdin:  in,
		Stdout: out,
		Stderr: err,
	}, nil
}

type pluginArgumentOptions struct {
	pluginArgs           []string
	tracingEndpoint      string
	logFlow, logToStderr bool
	verbose              int
}

func buildPluginArguments(opts pluginArgumentOptions) []string {
	var args []string
	// Flow the logging information if set.
	if opts.logFlow {
		if opts.logToStderr {
			args = append(args, "--logtostderr")
		}
		if opts.verbose > 0 {
			args = append(args, "-v="+strconv.Itoa(opts.verbose))
		}
	}
	if opts.tracingEndpoint != "" {
		args = append(args, "--tracing", opts.tracingEndpoint)
	}
	args = append(args, opts.pluginArgs...)
	return args
}

func (p *plugin) Close() error {
	if p.Conn != nil {
		contract.IgnoreClose(p.Conn)
	}

	result := p.Kill()

	// Wait for stdout and stderr to drain if we attached to the plugin we won't have a stdout/err
	if p.stdoutDone != nil {
		<-p.stdoutDone
	}
	if p.stderrDone != nil {
		<-p.stderrDone
	}

	return result
}
