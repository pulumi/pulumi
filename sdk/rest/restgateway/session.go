// Copyright 2016-2026, Pulumi Corporation.
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

package restgateway

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// Session represents a single running Pulumi operation (up or preview).
// It holds the gRPC connections to the engine's ResourceMonitor and Engine services.
type Session struct {
	ID          string
	StackURN    string
	Project     string
	Stack       string
	Monitor     pulumirpc.ResourceMonitorClient
	monitorConn *grpc.ClientConn
	Engine      pulumirpc.EngineClient
	engineConn  *grpc.ClientConn
	cmd         *exec.Cmd
	tmpDir      string
	ready       chan struct{} // closed when gRPC clients are connected
	finished    chan struct{} // closed on shutdown, unblocks Run()
	done        chan struct{} // closed when Close() completes, signals interactive mode to exit
	// ownsProcess indicates whether this session started the pulumi subprocess
	// (and should kill it on close). False for sessions created from an existing monitor.
	ownsProcess bool
}

// Done returns a channel that is closed when the session is closed.
// Use this to block until the session is shut down (e.g. in interactive mode).
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// langRuntime implements the LanguageRuntime gRPC server. The Pulumi CLI calls Run() on this
// server, which gives us the ResourceMonitor address to connect to.
type langRuntime struct {
	pulumirpc.UnimplementedLanguageRuntimeServer
	session *Session
}

// NewSession creates a new session, starts the Pulumi CLI subprocess, and waits for the
// engine to call Run() so we can connect to the ResourceMonitor.
func NewSession(ctx context.Context, project, stack string, preview bool) (*Session, error) {
	return newSession(ctx, "pulumi", project, stack, preview, nil)
}

// NewSessionFromMonitor creates a session that wraps an existing ResourceMonitor connection.
// This is used by the language host during conformance tests, where the engine has already
// started and provides the monitor address directly.
func NewSessionFromMonitor(
	ctx context.Context,
	monitorAddr, engineAddr string,
	project, stack string,
) (*Session, error) {
	id := uuid.New().String()[:8]

	monitorConn, err := grpc.NewClient(
		monitorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, fmt.Errorf("connecting to monitor: %w", err)
	}
	monitor := pulumirpc.NewResourceMonitorClient(monitorConn)

	sess := &Session{
		ID:          id,
		Project:     project,
		Stack:       stack,
		Monitor:     monitor,
		monitorConn: monitorConn,
		finished:    make(chan struct{}),
		done:        make(chan struct{}),
		ownsProcess: false,
	}

	if engineAddr != "" {
		engineConn, err := grpc.NewClient(
			engineAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			rpcutil.GrpcChannelOptions(),
		)
		if err != nil {
			monitorConn.Close()
			return nil, fmt.Errorf("connecting to engine: %w", err)
		}
		sess.engineConn = engineConn
		sess.Engine = pulumirpc.NewEngineClient(engineConn)
	}

	// Register the root Stack resource.
	stackName := fmt.Sprintf("%s-%s", project, stack)
	resp, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:                    "pulumi:pulumi:Stack",
		Name:                    stackName,
		AcceptSecrets:           true,
		AcceptResources:         true,
		SupportsResultReporting: true,
	})
	if err != nil {
		monitorConn.Close()
		return nil, fmt.Errorf("registering stack resource: %w", err)
	}
	sess.StackURN = resp.Urn

	return sess, nil
}

// newSession is the internal implementation that accepts a custom pulumi binary path and
// extra environment variables for testing.
func newSession(ctx context.Context, pulumiBin, project, stack string, preview bool, extraEnv []string) (*Session, error) {
	id := uuid.New().String()[:8]

	tmpDir, err := os.MkdirTemp("", "pulumi-rest-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	// Write a minimal Pulumi.yaml.
	pulumiYaml := fmt.Sprintf("name: %s\nruntime:\n  name: rest\n", project)
	if err := os.WriteFile(filepath.Join(tmpDir, "Pulumi.yaml"), []byte(pulumiYaml), 0o644); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("writing Pulumi.yaml: %w", err)
	}

	// Initialize the stack if it doesn't exist.
	env := append(os.Environ(), extraEnv...)
	initCmd := exec.CommandContext(ctx, pulumiBin, "stack", "init", stack, "--non-interactive")
	initCmd.Dir = tmpDir
	initCmd.Env = env
	// Ignore error — stack may already exist.
	initCmd.Run()

	sess := &Session{
		ID:          id,
		Project:     project,
		Stack:       stack,
		tmpDir:      tmpDir,
		ready:       make(chan struct{}),
		finished:    make(chan struct{}),
		done:        make(chan struct{}),
		ownsProcess: true,
	}

	// Start the LanguageRuntime gRPC server.
	lang := &langRuntime{session: sess}
	cancel := make(chan bool)
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, lang)
			return nil
		},
		Options: rpcutil.TracingServerInterceptorOptions(nil),
	})
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("starting language runtime server: %w", err)
	}

	langAddr := fmt.Sprintf("127.0.0.1:%d", handle.Port)

	// Build CLI args.
	var args []string
	if preview {
		args = []string{"preview", "--non-interactive"}
	} else {
		args = []string{"up", "--yes", "--skip-preview", "--non-interactive"}
	}
	args = append(args, "--client="+langAddr, "--exec-kind=auto.inline", "--stack="+stack)

	// Start the Pulumi CLI subprocess.
	cmd := exec.CommandContext(ctx, pulumiBin, args...)
	cmd.Dir = tmpDir
	cmd.Env = env
	cmd.Stdout = os.Stderr // Forward CLI output to our stderr for debugging.
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		os.RemoveAll(tmpDir)
		cancel <- true
		return nil, fmt.Errorf("starting pulumi subprocess: %w", err)
	}
	sess.cmd = cmd

	// Wait for the engine to call Run() on our LanguageRuntime server,
	// which will connect to the ResourceMonitor and close the ready channel.
	select {
	case <-sess.ready:
		// Session is ready.
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("timed out waiting for engine to connect")
	case <-ctx.Done():
		cmd.Process.Kill()
		os.RemoveAll(tmpDir)
		return nil, ctx.Err()
	}

	return sess, nil
}

// Close shuts down the session: registers stack outputs, signals shutdown, and cleans up.
func (s *Session) Close(ctx context.Context, exports map[string]interface{}) error {
	// Register stack outputs if any.
	if len(exports) > 0 {
		outs, err := structpb.NewStruct(exports)
		if err != nil {
			return fmt.Errorf("marshaling exports: %w", err)
		}
		if _, err := s.Monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
			Urn:     s.StackURN,
			Outputs: outs,
		}); err != nil {
			return fmt.Errorf("registering stack outputs: %w", err)
		}
	} else {
		// Must register empty outputs for the stack resource.
		if _, err := s.Monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
			Urn:     s.StackURN,
			Outputs: &structpb.Struct{Fields: map[string]*structpb.Value{}},
		}); err != nil {
			return fmt.Errorf("registering empty stack outputs: %w", err)
		}
	}

	// Signal that no more resources will be registered.
	if _, err := s.Monitor.SignalAndWaitForShutdown(ctx, &emptypb.Empty{}); err != nil {
		log.Printf("SignalAndWaitForShutdown error (may be expected for older engines): %v", err)
	}

	if s.ownsProcess {
		// Unblock the Run() method so the subprocess can exit.
		close(s.finished)

		// Wait for subprocess to exit.
		if err := s.cmd.Wait(); err != nil {
			log.Printf("pulumi subprocess exited with error: %v", err)
		}
	}

	// Close gRPC connections.
	if s.monitorConn != nil {
		s.monitorConn.Close()
	}
	if s.engineConn != nil {
		s.engineConn.Close()
	}

	// Clean up temp directory.
	if s.tmpDir != "" {
		os.RemoveAll(s.tmpDir)
	}

	// Signal that the session is fully closed.
	close(s.done)

	return nil
}

// --- LanguageRuntime gRPC server methods ---

func (l *langRuntime) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	log.Printf("Engine called Run() - monitor=%s", req.GetMonitorAddress())

	// Connect to the ResourceMonitor.
	monitorConn, err := grpc.NewClient(
		req.GetMonitorAddress(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return &pulumirpc.RunResponse{Error: fmt.Sprintf("connecting to monitor: %v", err)}, nil
	}
	l.session.monitorConn = monitorConn
	l.session.Monitor = pulumirpc.NewResourceMonitorClient(monitorConn)

	// Connect to the Engine service (address is in Args[0]).
	if len(req.Args) > 0 {
		engineConn, err := grpc.NewClient(
			req.Args[0],
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			rpcutil.GrpcChannelOptions(),
		)
		if err != nil {
			return &pulumirpc.RunResponse{Error: fmt.Sprintf("connecting to engine: %v", err)}, nil
		}
		l.session.engineConn = engineConn
		l.session.Engine = pulumirpc.NewEngineClient(engineConn)
	}

	// Register the root Stack resource.
	stackName := fmt.Sprintf("%s-%s", l.session.Project, l.session.Stack)
	resp, err := l.session.Monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:                    "pulumi:pulumi:Stack",
		Name:                    stackName,
		AcceptSecrets:           true,
		AcceptResources:         true,
		SupportsResultReporting: true,
	})
	if err != nil {
		return &pulumirpc.RunResponse{Error: fmt.Sprintf("registering stack resource: %v", err)}, nil
	}
	l.session.StackURN = resp.Urn
	log.Printf("Stack registered: %s", resp.Urn)

	// Signal that the session is ready for HTTP requests.
	close(l.session.ready)

	// Block until the session is closed. This keeps the "program" alive
	// so the engine doesn't think it exited prematurely.
	<-l.session.finished

	return &pulumirpc.RunResponse{}, nil
}

func (l *langRuntime) GetRequiredPlugins(
	ctx context.Context, req *pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return &pulumirpc.GetRequiredPluginsResponse{}, nil
}

func (l *langRuntime) GetRequiredPackages(
	ctx context.Context, req *pulumirpc.GetRequiredPackagesRequest,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	return &pulumirpc.GetRequiredPackagesResponse{}, nil
}

func (l *langRuntime) GetPluginInfo(ctx context.Context, req *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: "1.0.0",
	}, nil
}

func (l *langRuntime) InstallDependencies(
	req *pulumirpc.InstallDependenciesRequest,
	server pulumirpc.LanguageRuntime_InstallDependenciesServer,
) error {
	return nil
}

func (l *langRuntime) RuntimeOptionsPrompts(
	ctx context.Context, req *pulumirpc.RuntimeOptionsRequest,
) (*pulumirpc.RuntimeOptionsResponse, error) {
	return &pulumirpc.RuntimeOptionsResponse{}, nil
}

func (l *langRuntime) About(
	ctx context.Context, req *pulumirpc.AboutRequest,
) (*pulumirpc.AboutResponse, error) {
	return &pulumirpc.AboutResponse{}, nil
}

func (l *langRuntime) GetProgramDependencies(
	ctx context.Context, req *pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	return &pulumirpc.GetProgramDependenciesResponse{}, nil
}

func (l *langRuntime) GenerateProgram(
	ctx context.Context, req *pulumirpc.GenerateProgramRequest,
) (*pulumirpc.GenerateProgramResponse, error) {
	return &pulumirpc.GenerateProgramResponse{}, nil
}

func (l *langRuntime) GenerateProject(
	ctx context.Context, req *pulumirpc.GenerateProjectRequest,
) (*pulumirpc.GenerateProjectResponse, error) {
	return &pulumirpc.GenerateProjectResponse{}, nil
}

func (l *langRuntime) GeneratePackage(
	ctx context.Context, req *pulumirpc.GeneratePackageRequest,
) (*pulumirpc.GeneratePackageResponse, error) {
	return &pulumirpc.GeneratePackageResponse{}, nil
}

func (l *langRuntime) Pack(
	ctx context.Context, req *pulumirpc.PackRequest,
) (*pulumirpc.PackResponse, error) {
	return &pulumirpc.PackResponse{}, nil
}
