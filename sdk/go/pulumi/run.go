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

package pulumi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	multierror "github.com/hashicorp/go-multierror"

	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

var ErrPlugins = errors.New("pulumi: plugins requested")

// A RunOption is used to control the behavior of Run and RunErr.
type RunOption func(*RunInfo)

// Run executes the body of a Pulumi program, granting it access to a deployment context that it may use
// to register resources and orchestrate deployment activities.  This connects back to the Pulumi engine using gRPC.
// If the program fails, the process will be terminated and the function will not return.
func Run(body RunFunc, opts ...RunOption) {
	if err := RunErr(body, opts...); err != nil {
		if err != ErrPlugins {
			fmt.Fprintf(os.Stderr, "error: program failed: %v\n", err)
			os.Exit(1)
		}

		printRequiredPlugins()
		os.Exit(0)
	}
}

// RunErr executes the body of a Pulumi program, granting it access to a deployment context that it may use
// to register resources and orchestrate deployment activities.  This connects back to the Pulumi engine using gRPC.
func RunErr(body RunFunc, opts ...RunOption) error {
	// Parse the info out of environment variables.  This is a lame contract with the caller, but helps to keep
	// boilerplate to a minimum in the average Pulumi Go program.
	info := getEnvInfo()
	if info.getPlugins {
		return ErrPlugins
	}

	for _, o := range opts {
		o(&info)
	}

	// Validate some properties.
	if info.Project == "" {
		return errors.New("missing project name")
	} else if info.Stack == "" {
		return errors.New("missing stack name")
	} else if info.MonitorAddr == "" && info.Mocks == nil {
		return errors.New("missing resource monitor RPC address")
	} else if info.EngineAddr == "" && info.Mocks == nil {
		return errors.New("missing engine RPC address")
	}

	// Create a fresh context.
	ctx, err := NewContext(context.TODO(), info)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(ctx)

	return RunWithContext(ctx, body)
}

// RunWithContext runs the body of a Pulumi program using the given Context for information about the target stack,
// configuration, and engine connection.
func RunWithContext(ctx *Context, body RunFunc) error {
	info := ctx.info

	// Create a root stack resource that we'll parent everything to.
	var stack ResourceState
	err := ctx.RegisterResource(
		"pulumi:pulumi:Stack", fmt.Sprintf("%s-%s", info.Project, info.Stack), nil, &stack)
	if err != nil {
		return err
	}
	ctx.stack = &stack

	// Execute the body.
	var result error
	if err = body(ctx); err != nil {
		result = multierror.Append(result, err)
	}

	// Register all the outputs to the stack object.
	if err = ctx.RegisterResourceOutputs(ctx.stack, Map(ctx.exports)); err != nil {
		result = multierror.Append(result, err)
	}

	// Ensure all outstanding RPCs have completed before proceeding. Also, prevent any new RPCs from happening.
	ctx.waitForRPCs()
	if ctx.rpcError != nil {
		return ctx.rpcError
	}

	// Propagate the error from the body, if any.
	return result
}

// RunFunc executes the body of a Pulumi program.  It may register resources using the deployment context
// supplied as an arguent and any non-nil return value is interpreted as a program error by the Pulumi runtime.
type RunFunc func(ctx *Context) error

// RunInfo contains all the metadata about a run request.
type RunInfo struct {
	Project     string
	Stack       string
	Config      map[string]string
	Parallel    int
	DryRun      bool
	MonitorAddr string
	EngineAddr  string
	Mocks       MockResourceMonitor
	getPlugins  bool
}

// getEnvInfo reads various program information from the process environment.
func getEnvInfo() RunInfo {
	// Most of the variables are just strings, and we can read them directly.  A few of them require more parsing.
	parallel, _ := strconv.Atoi(os.Getenv(EnvParallel))
	dryRun, _ := strconv.ParseBool(os.Getenv(EnvDryRun))
	getPlugins, _ := strconv.ParseBool(os.Getenv(envPlugins))

	var config map[string]string
	if cfg := os.Getenv(EnvConfig); cfg != "" {
		_ = json.Unmarshal([]byte(cfg), &config)
	}

	return RunInfo{
		Project:     os.Getenv(EnvProject),
		Stack:       os.Getenv(EnvStack),
		Config:      config,
		Parallel:    parallel,
		DryRun:      dryRun,
		MonitorAddr: os.Getenv(EnvMonitor),
		EngineAddr:  os.Getenv(EnvEngine),
		getPlugins:  getPlugins,
	}
}

const (
	// EnvProject is the envvar used to read the current Pulumi project name.
	EnvProject = "PULUMI_PROJECT"
	// EnvStack is the envvar used to read the current Pulumi stack name.
	EnvStack = "PULUMI_STACK"
	// EnvConfig is the envvar used to read the current Pulumi configuration variables.
	EnvConfig = "PULUMI_CONFIG"
	// EnvParallel is the envvar used to read the current Pulumi degree of parallelism.
	EnvParallel = "PULUMI_PARALLEL"
	// EnvDryRun is the envvar used to read the current Pulumi dry-run setting.
	EnvDryRun = "PULUMI_DRY_RUN"
	// EnvMonitor is the envvar used to read the current Pulumi monitor RPC address.
	EnvMonitor = "PULUMI_MONITOR"
	// EnvEngine is the envvar used to read the current Pulumi engine RPC address.
	EnvEngine = "PULUMI_ENGINE"
	// envPlugins is the envvar used to request that the Pulumi program print its set of required plugins and exit.
	envPlugins = "PULUMI_PLUGINS"
)

type PackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Server  string `json:"server,omitempty"`
}

var packageRegistry = map[PackageInfo]struct{}{}

func RegisterPackage(info PackageInfo) {
	packageRegistry[info] = struct{}{}
}

func printRequiredPlugins() {
	plugins := []PackageInfo{}
	for info := range packageRegistry {
		plugins = append(plugins, info)
	}

	err := json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"plugins": plugins})
	contract.IgnoreError(err)
}
