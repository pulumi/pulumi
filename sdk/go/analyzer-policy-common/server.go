// Copyright 2016-2023, Pulumi Corporation.
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

package policyAnalyzer

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
)

var actualStdOut = os.Stdout

// runParams defines the command line arguments accepted by this program.
type runParams struct {
	tracing       string
	engineAddress string
}

// parseRunParams parses the given arguments into a runParams structure,
// using the provided FlagSet.
func parseRunParams(flag *flag.FlagSet, args []string) (*runParams, error) {
	var p runParams
	flag.StringVar(&p.tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")

	if err := flag.Parse(args); err != nil {
		return nil, err
	}

	// Pluck out the engine so we can do logging, etc.
	args = flag.Args()
	if len(args) == 0 {
		return nil, errors.New("missing required engine RPC address argument")
	}
	p.engineAddress = args[0]

	return &p, nil
}

type MainConfig struct {
	GetAnalyzer             GetAnalyzerFunc
	AnalyzerGRPCWrapperFunc plugin.AnalyzerGRPCWrapperFunc
}

// Launches the language host, which in turn fires up an RPC server implementing the LanguageRuntimeServer endpoint.
func Main(c *MainConfig) {

	p, err := parseRunParams(flag.CommandLine, os.Args[1:])
	if err != nil {
		cmdutil.Exit(err)
	}

	logging.InitLogging(false, 0, false)
	cmdutil.InitTracing("pulumi-analyzer-policy-go", "pulumi-analyzer-policy-go", p.tracing)

	var cmd mainCmd

	cmd.GetAnalyzer = c.GetAnalyzer

	if err := cmd.Run(p); err != nil {
		cmdutil.Exit(err)
	}
}

type mainCmd struct {
	Stdout io.Writer              // == os.Stdout
	Getwd  func() (string, error) // == os.Getwd

	GetAnalyzer             GetAnalyzerFunc
	AnalyzerGRPCWrapperFunc plugin.AnalyzerGRPCWrapperFunc
}

func (cmd *mainCmd) init() {
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	if cmd.Getwd == nil {
		cmd.Getwd = os.Getwd
	}
}

func (cmd *mainCmd) Run(p *runParams) error {
	cmd.init()

	cwd, err := cmd.Getwd()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	// map the context Done channel to the rpcutil boolean cancel channel
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		cancel() // deregister handler so we don't catch another interrupt
		close(cancelChannel)
	}()
	err = rpcutil.Healthcheck(ctx, p.engineAddress, 5*time.Minute, cancel)
	if err != nil {
		return fmt.Errorf("could not start health check host RPC server: %w", err)
	}

	// The current directory is the policy directory
	plug, err := cmd.GetAnalyzer(ctx, &GetAnalyzerConfig{CompileConfig: &CompileConfig{
		ProgramDirectory: cwd,
		OutFile:          "",
	},
		PolicyPackPath: cwd,
	})
	if err != nil {
		return fmt.Errorf("could not compile policy: %w", err)
	}
	dialOpts := rpcutil.OpenTracingInterceptorDialOptions()

	dialOpts = append(dialOpts, grpc.WithInsecure()) // TODO it must be secure

	if cmd.AnalyzerGRPCWrapperFunc == nil {
		cmd.AnalyzerGRPCWrapperFunc = plugin.NewAnalyzerPluginProxy
	}

	analyzerServer, err := cmd.AnalyzerGRPCWrapperFunc(plug)
	if err != nil {
		return fmt.Errorf("could not compile policy: %w", err)
	}

	// Fire up a gRPC server, letting the kernel choose a free port.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancelChannel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterAnalyzerServer(srv, analyzerServer)
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})

	if err != nil {
		return fmt.Errorf("could not start polycy analyzer proxy RPC server: %w", err)
	}

	// Otherwise, print out the port so that the spawner knows how to reach us.
	fmt.Fprintf(actualStdOut, "%d\n", handle.Port)

	// And finally wait for the server to stop serving.
	if err := <-handle.Done; err != nil {
		return fmt.Errorf("analyzer proxy RPC stopped serving: %w", err)
	}

	if err := plug.Close(); err != nil {
		return fmt.Errorf("policy analyzer RPC closed with error: %w", err)
	}

	return nil
}
