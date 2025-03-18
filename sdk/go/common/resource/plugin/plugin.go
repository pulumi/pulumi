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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	opentracing "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type PulumiParameterizationJSON struct {
	// The name of the parameterized package.
	Name string `json:"name"`
	// The version of the parameterized package.
	Version string `json:"version"`
	// The parameter value of the parameterized package.
	Value []byte `json:"value"`
}

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
	// Parameterization information for the package.
	Parameterization *PulumiParameterizationJSON `json:"parameterization,omitempty"`
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

	// The unstructured output of the process.
	//
	// unstructuredOutput is only non-nil if Pulumi launched the process and is hiding
	// unstructured output.
	unstructuredOutput *unstructuredOutput

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

type unstructuredOutput struct {
	output bytes.Buffer
	// outputLock prevents concurrent writes to output.
	outputLock sync.Mutex
	diag       diag.Sink

	// done is true when the output has already been written to the user.
	done atomic.Bool
}

// WriteString a string of unstructured output.
//
// WriteString is safe to call concurrently.
func (uo *unstructuredOutput) WriteString(msg string) {
	uo.outputLock.Lock()
	defer uo.outputLock.Unlock()
	uo.output.WriteString(msg)
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

func dialPlugin[T any](
	portNum int,
	bin string,
	prefix string,
	handshake func(context.Context, string, string, *grpc.ClientConn) (*T, error),
	dialOptions []grpc.DialOption,
) (*grpc.ClientConn, *T, error) {
	port := strconv.Itoa(portNum)

	// Now that we have the port, go ahead and create a gRPC client connection to it.
	conn, err := grpc.NewClient("127.0.0.1:"+port, dialOptions...)
	if err != nil {
		return nil, nil, fmt.Errorf("could not dial plugin [%v] over RPC: %w", bin, err)
	}

	// We want to wait for the gRPC connection to the plugin to become ready before we proceed. To this end, we'll
	// manually kick off a Connect() call and then wait until the state of the connection becomes Ready.
	conn.Connect()

	var handshakeRes *T

	// TODO[pulumi/pulumi#337]: in theory, this should be unnecessary.  gRPC's default WaitForReady behavior
	//     should auto-retry appropriately.  On Linux, however, we are observing different behavior.  In the meantime
	//     while this bug exists, we'll simply do a bit of waiting of our own up front.
	timeout, _ := context.WithTimeout(context.Background(), pluginRPCConnectionTimeout)
	for {
		s := conn.GetState()
		if s == connectivity.Ready {
			// The connection is supposedly ready; but we will make sure it is *actually* ready by sending a dummy
			// method invocation to the server.  Until it responds successfully, we can't safely proceed.
			for {
				handshakeRes, err = handshake(timeout, bin, prefix, conn)
				if err == nil {
					break // successful connect
				}

				// We have an error; see if it's a known status and, if so, react appropriately.
				status, ok := status.FromError(err)
				if ok && status.Code() == codes.Unavailable {
					// The server is unavailable.  This is the Linux bug.  Wait a little and retry.
					time.Sleep(time.Millisecond * 10)
					continue // keep retrying
				}

				// Unexpected error; get outta dodge.
				return nil, nil, fmt.Errorf("%v plugin [%v] did not come alive: %w", prefix, bin, err)
			}
			break
		}
		// Not ready yet; ask the gRPC client APIs to block until the state transitions again so we can retry.
		if !conn.WaitForStateChange(timeout, s) {
			return nil, nil, fmt.Errorf("%v plugin [%v] did not begin responding to RPC connections", prefix, bin)
		}
	}

	return conn, handshakeRes, nil
}

func testConnection(ctx context.Context, bin string, prefix string, conn *grpc.ClientConn) (*struct{}, error) {
	err := conn.Invoke(ctx, "", nil, nil)
	if err != nil {
		status, ok := status.FromError(err)
		if ok && status.Code() != codes.Unavailable {
			return &struct{}{}, nil
		}
	}
	return &struct{}{}, err
}

func newPlugin[T any](
	ctx *Context,
	pwd string,
	bin string,
	prefix string,
	kind apitype.PluginKind,
	args []string,
	env []string,
	handshake func(context.Context, string, string, *grpc.ClientConn) (*T, error),
	dialOptions []grpc.DialOption,
) (*plugin, *T, error) {
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
		return nil, nil, fmt.Errorf("failed to load plugin %s: %w", bin, err)
	}
	contract.Assertf(plug != nil, "plugin %v canot be nil", bin)

	// If we did not successfully launch the plugin, we still need to wait for stderr and stdout to drain.
	defer func() {
		if plug.Conn == nil {
			contract.IgnoreError(plug.Close())
		}
	}()

	type streamID int32
	outStreamID := streamID(atomic.AddInt32(&nextStreamID, 1))
	errStreamID := streamID(atomic.AddInt32(&nextStreamID, 1))

	// For now, we will spawn goroutines that will spew STDOUT/STDERR to the relevant diag streams.
	var sawPolicyModuleNotFoundErr bool
	if kind == apitype.ResourcePlugin && !isDynamicPluginBinary(bin) {
		logging.Infof("Hiding logs from %q:%q", prefix, bin)
		plug.unstructuredOutput = &unstructuredOutput{diag: ctx.Diag}
	}
	runtrace := func(t io.Reader, streamID streamID, done chan<- bool) {
		reader := bufio.NewReader(t)

		for {
			msg, readerr := reader.ReadString('\n')
			if plug.unstructuredOutput != nil {
				plug.unstructuredOutput.WriteString(msg)
			}

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

				var log func(*diag.Diag, ...interface{})
				if plug.unstructuredOutput != nil {
					log = ctx.Diag.Debugf
				} else if streamID == outStreamID {
					log = ctx.Diag.Infof
				} else {
					contract.Assertf(streamID == errStreamID, "invalid")
					log = ctx.Diag.Infoerrf
				}
				log(diag.StreamMessage("" /*urn*/, msg, int32(streamID)))
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
	go runtrace(plug.Stderr, errStreamID, stderrDone)

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
				return nil, nil, errRunPolicyModuleNotFound
			}

			// Fall back to a generic, opaque error.
			if portString == "" {
				return nil, nil, fmt.Errorf("could not read plugin [%v] stdout: %w", bin, readerr)
			}
			return nil, nil, fmt.Errorf("failure reading plugin [%v] stdout (read '%v'): %w",
				bin, portString, readerr)
		}
		if n > 0 && b[0] == '\n' {
			break
		}
		portString += string(b[:n])
	}
	// Parse the output line to ensure it's a numeric port.
	var port int
	if port, err = parsePort(portString); err != nil {
		killerr := plug.Kill()
		contract.IgnoreError(killerr) // ignoring the error because the existing one trumps it.
		return nil, nil, fmt.Errorf(
			"%v plugin [%v] wrote an invalid port to stdout: %w", prefix, bin, err)
	}

	// After reading the port number, set up a tracer on stdout just so other output doesn't disappear.
	stdoutDone := make(chan bool)
	plug.stdoutDone = stdoutDone
	go runtrace(plug.Stdout, outStreamID, stdoutDone)

	conn, handshakeRes, err := dialPlugin(port, bin, prefix, handshake, dialOptions)
	if err != nil {
		return nil, nil, err
	}

	// Done; store the connection and return the plugin info.
	plug.Conn = conn
	return plug, handshakeRes, nil
}

func parsePort(portString string) (int, error) {
	// Workaround for https://github.com/dotnet/sdk/issues/44610
	// In .NET 9.0 `dotnet run` will print progress indicators to the terminal,
	// even though it should not do this when the output is redirected.
	// We strip the control characters here to ensure that the port number is parsed correctly.
	//nolint:lll
	// https://github.com/dotnet/sdk/pull/42240/files#diff-6860155f1838e13335d417fc2fed7b13ac5ddf3b95d3548c6646618bc59e89e7R11
	portString = strings.ReplaceAll(portString, "\x1b]9;4;3;\x1b\\", "")
	portString = strings.ReplaceAll(portString, "\x1b]9;4;0;\x1b\\", "")

	// Trim any whitespace from the first line (this is to handle things like windows that will write
	// "1234\r\n", or slightly odd providers that might add whitespace like "1234 ")
	portString = strings.TrimSpace(portString)

	// Parse the output line to ensure it's a numeric port.
	port, err := strconv.Atoi(portString)
	if err != nil {
		// strconv.Atoi already includes the string we tried to parse
		return 0, fmt.Errorf("could not parse port: %w", err)
	}
	if port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid port number: %v", port)
	}
	return port, nil
}

// execPlugin starts the plugin executable.
func execPlugin(ctx *Context, bin, prefix string, kind apitype.PluginKind,
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
		if kind == apitype.ResourcePlugin || kind == apitype.ConverterPlugin {
			proj, err := workspace.LoadPluginProject(filepath.Join(pluginDir, "PulumiPlugin.yaml"))
			if err != nil {
				if os.IsNotExist(err) {
					// Apply heuristics to infer the runtime if the PulumiPlugin.yaml file is not found
					runtimeInfo, err = inferRuntime(pluginDir)
					if err != nil {
						return nil, err
					}
				} else {
					return nil, fmt.Errorf("loading PulumiPlugin.yaml: %w", err)
				}
			} else {
				runtimeInfo = proj.Runtime
			}
		} else if kind == apitype.AnalyzerPlugin {
			proj, err := workspace.LoadPluginProject(filepath.Join(pluginDir, "PulumiPolicy.yaml"))
			if err != nil {
				return nil, fmt.Errorf("loading PulumiPolicy.yaml: %w", err)
			}
			runtimeInfo = proj.Runtime
		} else {
			return nil, errors.New("language plugins must be executable binaries")
		}

		logging.V(9).Infof("Launching plugin '%v' from '%v' via runtime '%s'", prefix, pluginDir, runtimeInfo.Name())

		// ProgramInfo needs pluginDir to be an absolute path
		pluginDir, err = filepath.Abs(pluginDir)
		if err != nil {
			return nil, fmt.Errorf("getting absolute path for plugin directory: %w", err)
		}

		info := NewProgramInfo(pluginDir, pluginDir, ".", runtimeInfo.Options())
		runtime, err := ctx.Host.LanguageRuntime(runtimeInfo.Name(), info)
		if err != nil {
			return nil, fmt.Errorf("loading runtime: %w", err)
		}

		stdout, stderr, kill, err := runtime.RunPlugin(RunPluginInfo{
			Info:             info,
			WorkingDirectory: ctx.Pwd,
			Args:             args,
			Env:              env,
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

func inferRuntime(pluginDir string) (workspace.ProjectRuntimeInfo, error) {
	// Define the file-to-runtime mapping
	runtimeFiles := map[string]string{
		"package.json":     "nodejs",
		"requirements.txt": "python",
		"*.csproj":         "dotnet",
		"*.fsproj":         "dotnet",
		"go.mod":           "go",
		"pom.xml":          "java",
		"build.gradle":     "java",
	}

	// Use a map to track unique runtimes
	detectedRuntimes := make(map[string]bool)

	// Check for the presence of each file
	for file, runtime := range runtimeFiles {
		matches, err := filepath.Glob(filepath.Join(pluginDir, file))
		if err != nil {
			return workspace.ProjectRuntimeInfo{}, fmt.Errorf("inferring runtime: checking for %s: %w", file, err)
		}
		if len(matches) > 0 {
			detectedRuntimes[runtime] = true
		}
	}

	if len(detectedRuntimes) != 1 {
		return workspace.ProjectRuntimeInfo{}, errors.New("could not infer Plugin runtime")
	}

	// Get the single runtime (there's exactly one key in the map)
	var runtime string
	for r := range detectedRuntimes {
		runtime = r
		break
	}

	return workspace.NewProjectRuntimeInfo(runtime, nil), nil
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

func (p *plugin) healthCheck() bool {
	if p.Conn == nil {
		return false
	}

	// Check that the plugin looks alive by calling gRPC's Health Check service.
	// Most plugins don't actually implement this service, which is OK as we treat
	// an unimplemented status as OK.

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	healthy := make(chan bool, 1)
	go func() {
		health := grpc_health_v1.NewHealthClient(p.Conn)
		req := &grpc_health_v1.HealthCheckRequest{}

		resp, err := health.Check(ctx, req)
		if err != nil {
			// Treat this as healthy as most plugins don't implement gRPC's Health
			// Check service. An unimplemented status is enough for us to know the
			// plugin is alive.
			if status.Code(err) == codes.Unimplemented {
				healthy <- true
				return
			}

			logging.V(9).Infof("healthCheck(): failed with: %v", err)
			healthy <- false
			return
		}

		healthy <- resp.Status == grpc_health_v1.HealthCheckResponse_SERVING
	}()

	select {
	case result := <-healthy:
		return result
	case <-ctx.Done(): // hit deadline
		return false
	}
}

func (p *plugin) Close() error {
	// Something has gone wrong with the plugin if it is not healthy and we have not yet
	// shut it down.
	pluginCrashed := !p.healthCheck()

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

	// If the plugin has crashed and p.unstructuredOutput != nil is non-nil, then we
	// have not displayed any unstructured output to the user - including any
	// potential stack trace.
	//
	// To help debug (and to avoid attempting to detect the stack trace), we dump the captured stdout.
	if pluginCrashed && p.unstructuredOutput != nil && p.unstructuredOutput.done.CompareAndSwap(false, true) {
		id := atomic.AddInt32(&nextStreamID, 1)
		d := p.unstructuredOutput.diag
		// This outputs an error block:
		//
		//	error:
		//
		//	        Detected that <bin> exited prematurely.
		//	        This is *always* a bug in the provider. Please report the issue to the provider author as appropriate.
		//
		//	To assist with debugging we have dumped the STDOUT and STDERR streams of the plugin:
		//
		//	<output>
		d.Errorf(diag.StreamMessage("", fmt.Sprintf("\n\n         Detected that %s exited prematurely.\n", p.Bin), id))
		d.Errorf(diag.StreamMessage("",
			"         This is *always* a bug in the provider. "+
				"Please report the issue to the provider author as appropriate.\n\n", id))
		d.Errorf(diag.StreamMessage("",
			"To assist with debugging we have dumped the STDOUT and STDERR streams of the plugin:\n\n", id))
		p.unstructuredOutput.outputLock.Lock()
		defer p.unstructuredOutput.outputLock.Unlock()
		d.Errorf(diag.StreamMessage("", p.unstructuredOutput.output.String(), id))
	}

	return result
}

func isDynamicPluginBinary(path string) bool {
	return strings.HasSuffix(path, "pulumi-resource-pulumi-nodejs") ||
		strings.HasSuffix(path, "pulumi-resource-pulumi-python")
}
