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
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
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
	var binary string
	var root string
	flag.StringVar(&tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
	flag.StringVar(&binary, "binary", "", "A relative or an absolute path to a precompiled .NET assembly to execute")
	flag.StringVar(&root, "root", "", "Project root path to use")

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
	switch {
	case givenExecutor != "":
		logging.V(3).Infof("language host asked to use specific executor: `%s`", givenExecutor)
		dotnetExec = givenExecutor
	case binary != "" && !strings.HasSuffix(binary, ".dll"):
		logging.V(3).Info("language host requires no .NET SDK for a self-contained binary")
	default:
		pathExec, err := exec.LookPath("dotnet")
		if err != nil {
			err = errors.Wrap(err, "could not find `dotnet` on the $PATH")
			cmdutil.Exit(err)
		}

		logging.V(3).Infof("language host identified executor from path: `%s`", pathExec)
		dotnetExec = pathExec
	}

	// Optionally pluck out the engine so we can do logging, etc.
	var engineAddress string
	if len(args) > 0 {
		engineAddress = args[0]
	}

	// Fire up a gRPC server, letting the kernel choose a free port.
	port, done, err := rpcutil.Serve(0, nil, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			host := newLanguageHost(dotnetExec, engineAddress, tracing, binary)
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
	binary        string
}

func newLanguageHost(exec, engineAddress, tracing string, binary string) pulumirpc.LanguageRuntimeServer {

	return &dotnetLanguageHost{
		exec:          exec,
		engineAddress: engineAddress,
		tracing:       tracing,
		binary:        binary,
	}
}

// GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
func (host *dotnetLanguageHost) GetRequiredPlugins(
	ctx context.Context,
	req *pulumirpc.GetRequiredPluginsRequest) (*pulumirpc.GetRequiredPluginsResponse, error) {

	logging.V(5).Infof("GetRequiredPlugins: %v", req.GetProgram())

	if host.binary != "" {
		logging.V(5).Infof("GetRequiredPlugins: no plugins can be listed when a binary is specified")
		return &pulumirpc.GetRequiredPluginsResponse{}, nil
	}

	// Make a connection to the real engine that we will log messages to.
	conn, err := grpc.Dial(
		host.engineAddress,
		grpc.WithInsecure(),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "language host could not make connection to engine")
	}

	// Make a client around that connection.  We can then make our own server that will act as a
	// monitor for the sdk and forward to the real monitor.
	engineClient := pulumirpc.NewEngineClient(conn)

	// First do a `dotnet build`.  This will ensure that all the nuget dependencies of the project
	// are restored and locally available for us.
	if err := host.DotnetBuild(ctx, req, engineClient); err != nil {
		return nil, err
	}

	// now, introspect the user project to see which pulumi resource packages it references.
	pulumiPackages, err := host.DeterminePulumiPackages(ctx, engineClient)
	if err != nil {
		return nil, err
	}

	// Ensure we know where the local nuget package cache directory is.  User can specify where that
	// is located, so this makes sure we respect any custom location they may have.
	packageDir, err := host.DetermineDotnetPackageDirectory(ctx, engineClient)
	if err != nil {
		return nil, err
	}

	// Now that we know the set of pulumi packages referenced and we know where packages have been restored to,
	// we can examine each package to determine the corresponding resource-plugin for it.

	plugins := []*pulumirpc.PluginDependency{}
	packageToVersion := make(map[string]string)
	for _, parts := range pulumiPackages {
		packageName := parts[0]
		packageVersion := parts[1]

		if existingVersion := packageToVersion[packageName]; existingVersion == packageVersion {
			// only include distinct dependencies.
			continue
		}

		packageToVersion[packageName] = packageVersion

		plugin, err := host.DeterminePluginDependency(ctx, packageDir, packageName, packageVersion)
		if err != nil {
			return nil, err
		}

		if plugin != nil {
			plugins = append(plugins, plugin)
		}
	}

	return &pulumirpc.GetRequiredPluginsResponse{Plugins: plugins}, nil
}

func (host *dotnetLanguageHost) DeterminePulumiPackages(
	ctx context.Context, engineClient pulumirpc.EngineClient) ([][]string, error) {

	logging.V(5).Infof("GetRequiredPlugins: Determining pulumi packages")

	// Run the `dotnet list package --include-transitive` command.  Importantly, do not clutter the
	// stream with the extra steps we're performing. This is just so we can determine the required
	// plugins.  And, after the first time we do this, subsequent runs will see that the plugin is
	// installed locally and not need to do anything.
	args := []string{"list", "package", "--include-transitive"}
	commandStr := strings.Join(args, " ")
	commandOutput, err := host.RunDotnetCommand(ctx, engineClient, args, false /*logToUser*/)
	if err != nil {
		return nil, err
	}

	// expected output should be like so:
	//
	//    Project 'Aliases' has the following package references
	//    [netcoreapp3.1]:
	//    Top-level Package      Requested                        Resolved
	//    > Pulumi               1.5.0-preview-alpha.1572911568   1.5.0-preview-alpha.1572911568
	//
	//    Transitive Package                                       Resolved
	//    > Google.Protobuf                                        3.10.0
	//    > Grpc                                                   2.24.0
	outputLines := strings.Split(strings.Replace(commandOutput, "\r\n", "\n", -1), "\n")

	sawPulumi := false
	pulumiPackages := [][]string{}
	for _, line := range outputLines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		// Has to start with `>` and have at least 3 chunks:
		//
		//    > name requested_ver? resolved_ver
		if fields[0] != ">" {
			continue
		}

		// We only care about `Pulumi.` packages
		packageName := fields[1]
		if packageName == "Pulumi" {
			sawPulumi = true
			continue
		}

		if !strings.HasPrefix(packageName, "Pulumi.") {
			continue
		}

		version := fields[len(fields)-1]
		pulumiPackages = append(pulumiPackages, []string{packageName, version})
	}

	if !sawPulumi && len(pulumiPackages) == 0 {
		return nil, errors.Errorf(
			"Unexpected output from 'dotnet %v'. Program does not appear to reference any 'Pulumi.*' packages.",
			commandStr)
	}

	logging.V(5).Infof("GetRequiredPlugins: Pulumi packages: %#v", pulumiPackages)

	return pulumiPackages, nil
}

func (host *dotnetLanguageHost) DetermineDotnetPackageDirectory(
	ctx context.Context, engineClient pulumirpc.EngineClient) (string, error) {

	logging.V(5).Infof("GetRequiredPlugins: Determining package directory")

	// Run the `dotnet nuget locals global-packages --list` command.  Importantly, do not clutter
	// the stream with the extra steps we're performing. This is just so we can determine the
	// required plugins.  And, after the first time we do this, subsequent runs will see that the
	// plugin is installed locally and not need to do anything.
	args := []string{"nuget", "locals", "global-packages", "--list"}
	commandStr := strings.Join(args, " ")
	commandOutput, err := host.RunDotnetCommand(ctx, engineClient, args, false /*logToUser*/)
	if err != nil {
		return "", err
	}

	// expected output should be like so: "info : global-packages: /home/cyrusn/.nuget/packages/"
	// so grab the portion after "global-packages:"
	index := strings.Index(commandOutput, "global-packages:")
	if index < 0 {
		return "", errors.Errorf("Unexpected output from 'dotnet %v': %v", commandStr, commandOutput)
	}

	dir := strings.TrimSpace(commandOutput[index+len("global-packages:"):])
	logging.V(5).Infof("GetRequiredPlugins: Package directory: %v", dir)

	return dir, nil
}

func (host *dotnetLanguageHost) DeterminePluginDependency(
	ctx context.Context, packageDir, packageName, packageVersion string) (*pulumirpc.PluginDependency, error) {

	logging.V(5).Infof("GetRequiredPlugins: Determining plugin dependency: %v, %v, %v",
		packageDir, packageName, packageVersion)

	// Check for a `~/.nuget/packages/package_name/package_version/content/version.txt` file.

	versionFilePath := path.Join(packageDir, strings.ToLower(packageName), packageVersion, "content", "version.txt")
	logging.V(5).Infof("GetRequiredPlugins: version file path: %v", versionFilePath)

	if _, err := os.Stat(versionFilePath); err != nil {
		if os.IsNotExist(err) {
			// Pulumi package doesn't contain a version.txt file.  This is not a resource-plugin.
			// just ignore it.
			logging.V(5).Infof("GetRequiredPlugins: No such file")
			return nil, nil
		}

		// some other error.  report it as it means we can't read this important file.
		logging.V(5).Infof("GetRequiredPlugins: err: %v", err)
		return nil, err
	}

	b, err := ioutil.ReadFile(versionFilePath)
	if err != nil {
		logging.V(5).Infof("GetRequiredPlugins: err: %v", err)
		return nil, err
	}

	// Given a package name like "Pulumi.Azure" lowercase the part after Pulumi. to get the plugin name "azure".
	name := strings.ToLower(packageName[len("Pulumi."):])

	version := strings.TrimSpace(bytes.NewBuffer(b).String())
	parts := strings.SplitN(version, "\n", 2)
	if len(parts) == 2 {
		// version.txt may contain two lines, in which case it's "plugin name\nversion"
		name = strings.TrimSpace(parts[0])
		version = strings.TrimSpace(parts[1])
	}

	if !strings.HasPrefix(version, "v") {
		// Version file has stripped off the "v" that we need. So add it back here.
		version = fmt.Sprintf("v%v", version)
	}

	result := pulumirpc.PluginDependency{
		Name:    name,
		Version: version,
		Kind:    "resource",
	}

	logging.V(5).Infof("GetRequiredPlugins: Determining plugin dependency: %#v", result)
	return &result, nil
}

func (host *dotnetLanguageHost) DotnetBuild(
	ctx context.Context, req *pulumirpc.GetRequiredPluginsRequest, engineClient pulumirpc.EngineClient) error {

	args := []string{"build", "-nologo"}

	if req.GetProgram() != "" {
		args = append(args, req.GetProgram())
	}

	// Run the `dotnet build` command.  Importantly, report the output of this to the user
	// (ephemerally) as it is happening so they're aware of what's going on and can see the progress
	// of things.
	_, err := host.RunDotnetCommand(ctx, engineClient, args, true /*logToUser*/)
	if err != nil {
		return err
	}

	return nil
}

func (host *dotnetLanguageHost) RunDotnetCommand(
	ctx context.Context, engineClient pulumirpc.EngineClient, args []string, logToUser bool) (string, error) {

	commandStr := strings.Join(args, " ")
	if logging.V(5) {
		logging.V(5).Infoln("Language host launching process: ", host.exec, commandStr)
	}

	// Buffer the writes we see from dotnet from its stdout and stderr streams. We will display
	// these ephemerally as `dotnet build` runs.  If the build does fail though, we will dump
	// messages back to our own stdout/stderr so they get picked up and displayed to the user.
	streamID := rand.Int31() //nolint:gosec

	infoBuffer := &bytes.Buffer{}
	errorBuffer := &bytes.Buffer{}

	infoWriter := &logWriter{
		ctx:          ctx,
		logToUser:    logToUser,
		engineClient: engineClient,
		streamID:     streamID,
		buffer:       infoBuffer,
		severity:     pulumirpc.LogSeverity_INFO,
	}

	errorWriter := &logWriter{
		ctx:          ctx,
		logToUser:    logToUser,
		engineClient: engineClient,
		streamID:     streamID,
		buffer:       errorBuffer,
		severity:     pulumirpc.LogSeverity_ERROR,
	}

	// Now simply spawn a process to execute the requested program, wiring up stdout/stderr directly.
	cmd := exec.Command(host.exec, args...) // nolint: gas // intentionally running dynamic program name.

	cmd.Stdout = infoWriter
	cmd.Stderr = errorWriter

	_, err := infoWriter.LogToUser(fmt.Sprintf("running 'dotnet %v'", commandStr))
	if err != nil {
		return "", err
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
				return "", errors.Errorf(
					"'dotnet %v' exited with non-zero exit code: %d", commandStr, status.ExitStatus())
			}

			return "", errors.Wrapf(exiterr, "'dotnet %v' exited unexpectedly", commandStr)
		}

		// Otherwise, we didn't even get to run the program.  This ought to never happen unless there's
		// a bug or system condition that prevented us from running the language exec.  Issue a scarier error.
		return "", errors.Wrapf(err, "Problem executing 'dotnet %v'", commandStr)
	}

	_, err = infoWriter.LogToUser(fmt.Sprintf("'dotnet %v' completed successfully", commandStr))
	return infoBuffer.String(), err
}

type logWriter struct {
	ctx          context.Context
	logToUser    bool
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

	return w.LogToUser(string(p))
}

func (w *logWriter) LogToUser(val string) (int, error) {
	if w.logToUser {
		_, err := w.engineClient.Log(w.ctx, &pulumirpc.LogRequest{
			Message:   strings.ToValidUTF8(val, "�"),
			Urn:       "",
			Ephemeral: true,
			StreamId:  w.streamID,
			Severity:  w.severity,
		})

		if err != nil {
			return 0, err
		}
	}

	return len(val), nil
}

// RPC endpoint for LanguageRuntimeServer::Run
func (host *dotnetLanguageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	config, err := host.constructConfig(req)
	if err != nil {
		err = errors.Wrap(err, "failed to serialize configuration")
		return nil, err
	}
	configSecretKeys, err := host.constructConfigSecretKeys(req)
	if err != nil {
		err = errors.Wrap(err, "failed to serialize configuration secret keys")
		return nil, err
	}

	executable := host.exec
	args := []string{}

	switch {
	case host.binary != "" && strings.HasSuffix(host.binary, ".dll"):
		// Portable pre-compiled dll: run `dotnet <name>.dll`
		args = append(args, host.binary)
	case host.binary != "":
		// Self-contained executable: run it directly.
		executable = host.binary
	default:
		// Run from source.
		args = append(args, "run")

		if req.GetProgram() != "" {
			args = append(args, req.GetProgram())
		}
	}

	if logging.V(5) {
		commandStr := strings.Join(args, " ")
		logging.V(5).Infoln("Language host launching process: ", host.exec, commandStr)
	}

	// Now simply spawn a process to execute the requested program, wiring up stdout/stderr directly.
	var errResult string
	cmd := exec.Command(executable, args...) // nolint: gas // intentionally running dynamic program name.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = host.constructEnv(req, config, configSecretKeys)
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

func (host *dotnetLanguageHost) constructEnv(req *pulumirpc.RunRequest, config, configSecretKeys string) []string {
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
	maybeAppendEnv("config_secret_keys", configSecretKeys)

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

// constructConfigSecretKeys JSON-serializes the list of keys that contain secret values given as part of
// a RunRequest.
func (host *dotnetLanguageHost) constructConfigSecretKeys(req *pulumirpc.RunRequest) (string, error) {
	configSecretKeys := req.GetConfigSecretKeys()
	if configSecretKeys == nil {
		return "[]", nil
	}

	configSecretKeysJSON, err := json.Marshal(configSecretKeys)
	if err != nil {
		return "", err
	}

	return string(configSecretKeysJSON), nil
}

func (host *dotnetLanguageHost) GetPluginInfo(ctx context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: version.Version,
	}, nil
}
