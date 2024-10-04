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

// Package rpcCmd provides functionality to create a standard server
// for language hosts, policy analyzers, and plugins.
// This package is primarily used to manage gRPC (Google Remote Procedure Call)
// server lifecycle, tracing, health checks,
// and configuration related to server startup.
//
// The Server struct in rpcCmd handles core server logic, including:
// 1. Parsing and managing flags for configuration.
// 2. Performing health checks to ensure the server is responsive.
// 3. Initializing and managing gRPC connections with configurable server options.
// 4. Supporting distributed tracing via a Zipkin-compatible endpoint.
//
// Additionally, this package provides configurable options for running and shutting down the server
// gracefully upon receiving signals like SIGINT.
package rpcserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
)

const DefaultHealthCheck = 5 * time.Minute

type Server struct {
	// Flag is the FlagSet containing registered rpcCmd flags. By default, it's set to pflag.ExitOnError.
	// If a Flag is provided in the config, that one is used, but with rpcCmd flags registered as well.
	Flag *pflag.FlagSet

	// FinishFunc is a function that is executed after the server shuts down.
	// It should contain any necessary cleanup logic to be performed after server shutdown.
	FinishFunc func()

	// tracing is the endpoint for emitting Zipkin-compatible tracing data.
	tracing string

	// engineAddr is the RPC address for connecting to the Pulumi engine.
	engineAddr string

	// pluginPath is the path to the plugin source.
	pluginPath string

	config      Config
	grpcOptions []grpc.ServerOption
}

type Config struct {
	// Flag allows specifying a custom FlagSet if behavior different from the default flag.ExitOnError is required.
	Flag *pflag.FlagSet

	// TracingName and RootSpanName are required if tracing is enabled.
	TracingName  string
	RootSpanName string

	// Healthcheck interval duration.
	HealthcheckInterval time.Duration

	// EngineAddressOptional indicates that the engine address is optional. This is rarely the case.
	EngineAddressOptional bool
}

// errW wraps an error with a message.
func errW(err error) error {
	return fmt.Errorf("rpcCmd initialization failed: %w", err)
}

// NewServer creates a new instance of Server.
func NewServer(c Config) (*Server, error) {
	s := &Server{config: c}

	// Server parses flags with a private instance of FlagSet.
	s.Flag = pflag.NewFlagSet("", pflag.ContinueOnError)
	// Filter out unknown flags, caller can register any flags later
	s.Flag.ParseErrorsWhitelist.UnknownFlags = true
	s.registerFlags()
	if err := s.Flag.Parse(os.Args[1:]); err != nil {
		return nil, errW(err)
	}
	// Set arguments.
	args := s.Flag.Args()
	if len(args) == 0 && !s.config.EngineAddressOptional {
		return nil, errW(errors.New("missing required engine RPC address argument"))
	}
	if len(args) != 0 {
		s.engineAddr = args[0]
	}

	// plugin path is the third argument.
	if len(args) >= 2 {
		s.pluginPath = args[1]
	}

	// rpcCmd has already parsed private flags; it needs to register them again for parsing on the caller side.
	s.Flag = getConfiguredFlagSet(s.config.Flag)

	return s, nil
}

func getConfiguredFlagSet(f *pflag.FlagSet) *pflag.FlagSet {
	s := Server{}
	s.Flag = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	if f != nil {
		s.Flag = f
	}
	s.registerFlags()
	return s.Flag
}

// registerFlags registers flags related to RPC server logic.
func (s *Server) registerFlags() {
	s.Flag.StringVar(&s.tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
}

// getHealthcheckD returns the health check duration.
func (s *Server) getHealthcheckD() time.Duration {
	if s.config.HealthcheckInterval != 0 {
		return s.config.HealthcheckInterval
	}
	return DefaultHealthCheck
}

// InitFunc defines the type of function passed to rpcutil.ServeWithOptions.
type InitFunc func(*grpc.Server) error

// Run starts server. It blocks workflow and waits for syscall.SIGINT to shut down.
func (s *Server) Run(iFunc InitFunc) {
	var err error

	// Ensure the finish function is executed.
	// Do not intercept panic; this runs as a separate command so the panic will be shown.
	defer func() {
		if s.FinishFunc != nil {
			s.FinishFunc()
		}
		if err != nil {
			cmdutil.Exit(err)
		}
	}()

	if s.tracing != "" {
		// TracingName and RootSpanName are required if tracing is enabled.
		if s.config.TracingName == "" || s.config.RootSpanName == "" {
			// Lack of tracing configuration is a warning
			// Print the warnings to stderr as the executor expects only the port value in stdout
			fmt.Fprintln(os.Stderr, "Tracing disabled.")
			fmt.Fprintln(os.Stderr, "--tracing is set to "+s.tracing+", but")
			fmt.Fprintln(os.Stderr, "required tracing configuration is missing: TracingName or RootSpanName.")
			fmt.Fprintln(os.Stderr, "Provide them in the configuration,")
			fmt.Fprintln(os.Stderr, "or set them using SetTracingNames.")
		} else {
			cmdutil.InitTracing(s.config.TracingName, s.config.RootSpanName, s.GetTracing())
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	// Map the context's Done channel to the rpcutil boolean cancel channel.
	cancelChannel := make(chan bool)
	go func() {
		<-ctx.Done()
		cancel() // Deregister handler so we don't catch another interrupt.
		close(cancelChannel)
	}()

	if !(s.config.EngineAddressOptional && s.engineAddr == "") {
		err = rpcutil.Healthcheck(ctx, s.engineAddr, s.getHealthcheckD(), cancel)
		if err != nil {
			err = fmt.Errorf("error starting server: %w", err)
			return
		}
	}

	// Fire up a gRPC server, letting the kernel choose a free port.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel:  cancelChannel,
		Init:    iFunc,
		Options: s.getGrpcOptions(),
	})
	if err != nil {
		err = fmt.Errorf("could not start language host RPC server: %w", err)
		return
	}

	// Print the port so that the spawner knows how to reach the server.
	fmt.Fprintf(os.Stdout, "%d\n", handle.Port)

	// Wait for the server to stop serving. If an error occurs, it will be handled in defer.
	if err = <-handle.Done; err != nil {
		err = fmt.Errorf("could not start language host RPC server: %w", err)
	}
}

// GetEngineAddress returns the engine address for the server.
func (s *Server) GetEngineAddress() string {
	return s.engineAddr
}

// GetPluginPath returns the plugin path for the server.
func (s *Server) GetPluginPath() string {
	return s.pluginPath
}

// GetTracing returns the tracing endpoint.
func (s *Server) GetTracing() string {
	return s.tracing
}

// getGrpcOptions returns the gRPC server options.
// Tip: If you want to suppress OpenTracing options but don't need to provide any other options,
// you can pass an array with a mock grpc.ServerOption implementation.
func (s *Server) getGrpcOptions() []grpc.ServerOption {
	if len(s.grpcOptions) == 0 {
		return rpcutil.OpenTracingServerInterceptorOptions(nil)
	}
	return s.grpcOptions
}

// SetGrpcOptions allows overriding the default gRPC server options.
// This should only be used if you need custom gRPC configurations.
func (s *Server) SetGrpcOptions(opts []grpc.ServerOption) {
	s.grpcOptions = opts
}

// SetTracingNames sets TracingName and RootSpanName
func (s *Server) SetTracingNames(tracingName, rootSpanName string) {
	s.config.RootSpanName = rootSpanName
	s.config.TracingName = tracingName
}
