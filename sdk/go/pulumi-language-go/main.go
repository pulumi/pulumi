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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/buildutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/goversion"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func findProgram(binary string) (*exec.Cmd, error) {
	// we default to execution via `go run`
	// the user can explicitly opt in to using a binary executable by specifying
	// runtime.options.binary in the Pulumi.yaml
	if binary != "" {
		program, err := executable.FindExecutable(binary)
		if err != nil {
			return nil, errors.Wrap(err, "expected to find prebuilt executable")
		}
		return exec.Command(program), nil
	}

	// Fall back to 'go run' style executions
	logging.V(5).Infof("No prebuilt executable specified, attempting invocation via 'go run'")
	program, err := executable.FindExecutable("go")
	if err != nil {
		return nil, errors.Wrap(err, "problem executing program (could not run language executor)")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get current working directory")
	}

	goFileSearchPattern := filepath.Join(cwd, "*.go")
	if matches, err := filepath.Glob(goFileSearchPattern); err != nil || len(matches) == 0 {
		return nil, errors.Errorf("Failed to find go files for 'go run' matching %s", goFileSearchPattern)
	}

	return exec.Command(program, "run", cwd), nil
}

// Launches the language host, which in turn fires up an RPC server implementing the LanguageRuntimeServer endpoint.
func main() {
	var tracing string
	var binary string
	var root string
	flag.StringVar(&tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
	flag.StringVar(&binary, "binary", "", "Look on path for a binary executable with this name")
	flag.StringVar(&root, "root", "", "Project root path to use")

	flag.Parse()
	args := flag.Args()
	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing("pulumi-language-go", "pulumi-language-go", tracing)

	// Pluck out the engine so we can do logging, etc.
	if len(args) == 0 {
		cmdutil.Exit(errors.New("missing required engine RPC address argument"))
	}
	engineAddress := args[0]

	// Fire up a gRPC server, letting the kernel choose a free port.
	port, done, err := rpcutil.Serve(0, nil, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			host := newLanguageHost(engineAddress, tracing, binary)
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

// goLanguageHost implements the LanguageRuntimeServer interface for use as an API endpoint.
type goLanguageHost struct {
	engineAddress string
	tracing       string
	binary        string
}

func newLanguageHost(engineAddress, tracing, binary string) pulumirpc.LanguageRuntimeServer {
	return &goLanguageHost{
		engineAddress: engineAddress,
		tracing:       tracing,
		binary:        binary,
	}
}

// modInfo is the useful portion of the output from `go list -m -json all`
// with respect to plugin acquisition
type modInfo struct {
	Path    string
	Version string
}

func (m *modInfo) getPlugin() (*pulumirpc.PluginDependency, error) {
	if !strings.HasPrefix(m.Path, "github.com/pulumi/pulumi-") {
		return nil, errors.New("module is not a pulumi provider")
	}

	// github.com/pulumi/pulumi-aws/sdk/... => aws
	pluginPart := strings.Split(m.Path, "/")[2]
	name := strings.SplitN(pluginPart, "-", 2)[1]

	v, err := semver.ParseTolerant(m.Version)
	if err != nil {
		return nil, errors.New("module does not have semver compatible version")
	}
	version := m.Version

	// psuedoversions are commits that don't have a corresponding tag at the specified git hash
	// https://golang.org/cmd/go/#hdr-Pseudo_versions
	// pulumi-aws v1.29.1-0.20200403140640-efb5e2a48a86 (first commit after 1.29.0 release)
	if buildutil.IsPseudoVersion(version) {
		// no prior tag means there was never a release build
		if v.Major == 0 && v.Minor == 0 && v.Patch == 0 {
			return nil, errors.New("invalid pseduoversion with no prior tag")
		}
		// patch is typically bumped from the previous tag when using pseudo version
		// downgrade the patch by 1 to make sure we match a release that exists
		patch := v.Patch
		if patch > 0 {
			patch--
		}
		version = fmt.Sprintf("v%v.%v.%v", v.Major, v.Minor, patch)
	}

	plugin := &pulumirpc.PluginDependency{
		Name:    name,
		Version: version,
		Kind:    "resource",
	}

	return plugin, nil
}

// GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
// We're lenient here as this relies on the `go list` command and the use of modules.
// If the consumer insists on using some other form of dependency management tool like
// dep or glide, the list command fails with "go list -m: not using modules".
// However, we do enforce that go 1.14.0 or higher is installed.
func (host *goLanguageHost) GetRequiredPlugins(ctx context.Context,
	req *pulumirpc.GetRequiredPluginsRequest) (*pulumirpc.GetRequiredPluginsResponse, error) {

	logging.V(5).Infof("GetRequiredPlugins: Determining pulumi packages")

	gobin, err := executable.FindExecutable("go")
	if err != nil {
		return nil, errors.Wrap(err, "couldn't find go binary")
	}

	if err = goversion.CheckMinimumGoVersion(gobin); err != nil {
		return nil, err
	}

	args := []string{"list", "-m", "-json", "-mod=mod", "all"}

	tracingSpan, _ := opentracing.StartSpanFromContext(ctx,
		fmt.Sprintf("%s %s", gobin, strings.Join(args, " ")),
		opentracing.Tag{Key: "component", Value: "exec.Command"},
		opentracing.Tag{Key: "command", Value: gobin},
		opentracing.Tag{Key: "args", Value: args})

	// don't wire up stderr so non-module users don't see error output from list
	cmd := exec.Command(gobin, args...)
	cmd.Env = os.Environ()
	stdout, err := cmd.Output()

	tracingSpan.Finish()

	if err != nil {
		logging.V(5).Infof("GetRequiredPlugins: Error discovering plugin requirements using go modules: %s", err.Error())
		return &pulumirpc.GetRequiredPluginsResponse{}, nil
	}

	plugins := []*pulumirpc.PluginDependency{}

	dec := json.NewDecoder(bytes.NewReader(stdout))
	for {
		var m modInfo
		if err := dec.Decode(&m); err != nil {
			if err == io.EOF {
				break
			}
			logging.V(5).Infof("GetRequiredPlugins: Error parsing list output: %s", err.Error())
			return &pulumirpc.GetRequiredPluginsResponse{}, nil
		}

		plugin, err := m.getPlugin()
		if err == nil {
			logging.V(5).Infof("GetRequiredPlugins: Found plugin name: %s, version: %s", plugin.Name, plugin.Version)
			plugins = append(plugins, plugin)
		} else {
			logging.V(5).Infof(
				"GetRequiredPlugins: Ignoring dependency: %s, version: %s, error: %s",
				m.Path,
				m.Version,
				err.Error(),
			)
		}
	}

	return &pulumirpc.GetRequiredPluginsResponse{
		Plugins: plugins,
	}, nil
}

// RPC endpoint for LanguageRuntimeServer::Run
func (host *goLanguageHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	// Create the environment we'll use to run the process.  This is how we pass the RunInfo to the actual
	// Go program runtime, to avoid needing any sort of program interface other than just a main entrypoint.
	env, err := host.constructEnv(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare environment")
	}

	cmd, err := findProgram(host.binary)
	if err != nil {
		return nil, err
	}
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	var errResult string
	if err := cmd.Run(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// If the program ran, but exited with a non-zero error code.  This will happen often, since user
			// errors will trigger this.  So, the error message should look as nice as possible.
			if status, stok := exiterr.Sys().(syscall.WaitStatus); stok {
				err = errors.Errorf("program exited with non-zero exit code: %d", status.ExitStatus())
			} else {
				err = errors.Wrapf(exiterr, "program exited unexpectedly")
			}
		} else {
			// Otherwise, we didn't even get to run the program.  This ought to never happen unless there's
			// a bug or system condition that prevented us from running the language exec.  Issue a scarier error.
			err = errors.Wrapf(err, "problem executing program (could not run language executor)")
		}

		errResult = err.Error()
	}

	return &pulumirpc.RunResponse{Error: errResult}, nil
}

// constructEnv constructs an environment for a Go progam by enumerating all of the optional and non-optional
// arguments present in a RunRequest.
func (host *goLanguageHost) constructEnv(req *pulumirpc.RunRequest) ([]string, error) {
	config, err := host.constructConfig(req)
	if err != nil {
		return nil, err
	}
	configSecretKeys, err := host.constructConfigSecretKeys(req)
	if err != nil {
		return nil, err
	}

	env := os.Environ()
	maybeAppendEnv := func(k, v string) {
		if v != "" {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	maybeAppendEnv(pulumi.EnvProject, req.GetProject())
	maybeAppendEnv(pulumi.EnvStack, req.GetStack())
	maybeAppendEnv(pulumi.EnvConfig, config)
	maybeAppendEnv(pulumi.EnvConfigSecretKeys, configSecretKeys)
	maybeAppendEnv(pulumi.EnvDryRun, fmt.Sprintf("%v", req.GetDryRun()))
	maybeAppendEnv(pulumi.EnvParallel, fmt.Sprint(req.GetParallel()))
	maybeAppendEnv(pulumi.EnvMonitor, req.GetMonitorAddress())
	maybeAppendEnv(pulumi.EnvEngine, host.engineAddress)

	return env, nil
}

// constructConfig JSON-serializes the configuration data given as part of a RunRequest.
func (host *goLanguageHost) constructConfig(req *pulumirpc.RunRequest) (string, error) {
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
func (host *goLanguageHost) constructConfigSecretKeys(req *pulumirpc.RunRequest) (string, error) {
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

func (host *goLanguageHost) GetPluginInfo(ctx context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: version.Version,
	}, nil
}
