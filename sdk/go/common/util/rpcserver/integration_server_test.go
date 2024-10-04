// Copyright 2016-2024, Pulumi Corporation.
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

package rpcserver_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcserver"
	"github.com/spf13/pflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	pingpb "github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcserver/mockGRPC"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

const (
	tracingFlag              = "--tracing"
	engineAddrField          = "EngineAddr"
	pluginPathField          = "PluginPath"
	healthCheckIntervalField = "HealthCheckInterval"

	pluginPath   = "plugin/path"
	tracingName  = "tracing-name"
	rootSpanName = "root-span-name"

	healthCheckInterval = time.Second

	ENGINE_ADDR  = "ENGINE_ADDR"
	TRACING_ADDR = "TRACING_ADDR"

	overrideTracingName  = "GDvveMJ8"
	overrideRootSpanName = "Sz7JghpR"

	finishFuncMessage = "FINISH_FUNC_MESSAGE"
)

func findFlagValue(args []string, flag string) (bool, string) {
	// Iterate over the args slice to find the flag
	for i, arg := range args {
		if arg == flag {
			return true, args[i+1] // it will panic if test input is invalid
		}
	}
	return false, ""
}

func findPluginPathValue(args []string) (bool, string) {
	flagSet := pflag.NewFlagSet("", pflag.ContinueOnError)
	flagSet.ParseErrorsWhitelist.UnknownFlags = true
	flagSet.Parse(args)
	if len(flagSet.Args()) >= 2 {
		return true, flagSet.Args()[1]
	}

	return false, ""
}

var standardFunc = func(s *rpcserver.Server) {
	s.FinishFunc = func() {
		fmt.Println(finishFuncMessage)
	}

	s.Run(func(server *grpc.Server) error {
		pingpb.RegisterPingServiceServer(server, &PingServer{s: s})
		return nil
	})
}

var tests = map[string]struct {
	config rpcserver.Config
	give   []string

	f func(s *rpcserver.Server)

	timeOutBefore    time.Duration
	checkHealthCheck bool

	tracingWarning   string
	tracingOverrides bool
}{
	"simplest_run": {
		config: rpcserver.Config{},
		give:   []string{ENGINE_ADDR},
		f:      standardFunc,
	},
	"run_with_tracing_plugin_path": {
		config: rpcserver.Config{
			HealthcheckInterval: time.Minute,
			TracingName:         tracingName, RootSpanName: rootSpanName,
		},
		give: []string{ENGINE_ADDR, pluginPath, tracingFlag, TRACING_ADDR},
		f:    standardFunc,
	},
	"ensure_tracing": {
		config: rpcserver.Config{
			HealthcheckInterval: time.Minute,
			TracingName:         tracingName, RootSpanName: rootSpanName,
		},
		give: []string{ENGINE_ADDR, tracingFlag, TRACING_ADDR},
		f:    standardFunc,
	},
	"ensure_tracing_warning": {
		config:         rpcserver.Config{HealthcheckInterval: time.Minute},
		give:           []string{ENGINE_ADDR, tracingFlag, TRACING_ADDR},
		f:              standardFunc,
		tracingWarning: "Tracing disabled.",
	},
	"ensure_tracing_override": {
		config: rpcserver.Config{
			HealthcheckInterval: time.Minute,
			TracingName:         tracingName, RootSpanName: rootSpanName,
		},
		give: []string{ENGINE_ADDR, tracingFlag, TRACING_ADDR},
		f: func(s *rpcserver.Server) {
			s.FinishFunc = func() {
				fmt.Println(finishFuncMessage)
			}
			s.SetTracingNames(overrideTracingName, overrideRootSpanName)
			s.Run(func(server *grpc.Server) error {
				pingpb.RegisterPingServiceServer(server, &PingServer{s: s})
				return nil
			})
		},
		tracingOverrides: true,
	},
	"engine_stopped_healtcheck_shutdown": {
		config:           rpcserver.Config{HealthcheckInterval: 500 * time.Millisecond},
		give:             []string{ENGINE_ADDR},
		f:                standardFunc,
		timeOutBefore:    2 * time.Second,
		checkHealthCheck: true,
	},
	"healtcheck_valid": {
		config:        rpcserver.Config{HealthcheckInterval: 500 * time.Millisecond},
		give:          []string{ENGINE_ADDR},
		f:             standardFunc,
		timeOutBefore: 10 * time.Second,
	},
}

// PingServer implements the PingService.
type PingServer struct {
	pingpb.UnimplementedPingServiceServer

	s *rpcserver.Server
}

// Ping method returns a "Pong" response.
func (s *PingServer) Ping(ctx context.Context, req *pingpb.PingRequest) (*pingpb.PingResponse, error) {
	var msg string
	switch req.Message {
	case "Ping":
		msg = "Pong"
	case tracingFlag:
		msg = s.s.GetTracing()
	case engineAddrField:
		msg = s.s.GetEngineAddress()
	case pluginPathField:
		msg = s.s.GetPluginPath()
		// case healthCheckIntervalField:
		//	msg = s.s.getHealthcheckD().String()
	}
	return &pingpb.PingResponse{Reply: msg}, nil
}

func RequestTheServer(t *testing.T, client pingpb.PingServiceClient, requested, expected string) {
	// Send a Ping request
	req := &pingpb.PingRequest{Message: requested}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := client.Ping(ctx, req)
	assert.NoError(t, err)

	// Assert the response
	assert.Equal(t, expected, resp.Reply, fmt.Sprintf("for requested %s expected %s, got %s", requested,
		expected, resp.Reply))
}

func checkExitCode(t *testing.T, err error) {
	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ExitCode() != 0 {
			t.Fatalf("Subprocess exited with non-zero exit code: %d", exitError.ExitCode())
		}
	} else if err != nil {
		t.Fatalf("Subprocess finished with error: %v", err)
	}
}

func substituteArg(args []string, sub, val string) []string {
	for i := range args {
		if args[i] == sub {
			args[i] = val
		}
	}
	return args
}

//nolint:paralleltest
func TestSubprocess(t *testing.T) {
	for testCaseId, testCase := range tests {
		t.Run("Test Case "+testCaseId, func(t *testing.T) {
			engAddr, shutdownEngine := StartHealthCheckServer(t)
			defer shutdownEngine()
			substituteArg(testCase.give, ENGINE_ADDR, engAddr)

			tracingAddr, shutdownTracingAddr, tracingChan := StartMockTracingServer(t)
			defer shutdownTracingAddr()
			substituteArg(testCase.give, TRACING_ADDR, "http://"+tracingAddr)

			// Use os.Executable() to get the path to the current test binary
			executablePath, err := os.Executable()
			if err != nil {
				t.Fatalf("Failed to get current executable path: %v", err)
			}

			// Run the test in a subprocess
			cmd := exec.Command(executablePath, append([]string{"-test.run=TestCmd"}, testCase.give...)...)
			cmd.Env = append(os.Environ(), "TEST_CASE_ID="+testCaseId)

			// Capture stdout dynamically
			stdoutPipe, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatalf("Failed to get stdout pipe: %v", err)
			}
			// Capture stderr dynamically
			stderrPipe, err := cmd.StderrPipe()
			if err != nil {
				t.Fatalf("Failed to get stdout pipe: %v", err)
			}

			serverDone, notifyServerDone := context.WithCancel(context.Background())

			var errCmd error

			// Start the command
			go func() {
				if err := cmd.Start(); err != nil { // Use Start() instead of Run() here to avoid blocking
					t.Fatalf("Failed to start command: %v", err)
				}

				// Wait for the subprocess to finish
				errCmd = cmd.Wait()

				// Check the exit code
				notifyServerDone()
			}()

			// Read stdout to capture the port number
			portC := make(chan string, 10000)
			finishFunc := make(chan struct{})
			go func() {
				scanner := bufio.NewScanner(stdoutPipe)
				for scanner.Scan() {
					line := scanner.Text()
					fmt.Printf("%s\n", line)
					if line == finishFuncMessage {
						close(finishFunc)
						continue
					}
					portC <- line
				}
			}()

			tracingWarningCh := make(chan struct{})
			go func() {
				scanner := bufio.NewScanner(stderrPipe)
				for scanner.Scan() {
					line := scanner.Text()
					if testCase.tracingWarning != "" {
						if strings.Contains(line, testCase.tracingWarning) {
							close(tracingWarningCh)
						}
					}
				}
			}()

			// Wait for the port to be captured
			var port string
			select {
			case port = <-portC:
			case <-time.After(2 * time.Second):
				t.Fatal("Timeout waiting for the port to be printed")
			}

			// Check if the port is a valid integer
			_, err = strconv.Atoi(port)
			if err != nil {
				t.Fatalf("Expected port number, got: %s", port)
			}

			// Connect to the gRPC server using the captured port
			conn, err := grpc.Dial("localhost:"+port, grpc.WithInsecure())
			assert.NoError(t, err)
			defer conn.Close()

			client := pingpb.NewPingServiceClient(conn)
			RequestTheServer(t, client, "Ping", "Pong")

			if set, val := findFlagValue(testCase.give, tracingFlag); set {
				RequestTheServer(t, client, tracingFlag, val)

				// also check that mock tracing server actually get trace logs ONLY IF don't expect warning
				if testCase.tracingWarning == "" {
					select {
					case traceString := <-tracingChan:

						if testCase.tracingOverrides {
							assert.Contains(t, traceString, overrideTracingName)
							// TODO figure out why rootSpanName is not there. I assume it requires more complicated mock server?
							// assert.Contains(t, traceString, overrideRootSpanName)
						} else {
							assert.Contains(t, traceString, tracingName)
							// TODO figure out why rootSpanName is not there. I assume it requires more complicated mock server?
							// assert.Contains(t, traceString, rootSpanName)
						}

					case <-time.After(2 * time.Second):
						t.Fatalf("Didn't get expected tracing")
					}
				} else {
					select {
					case <-tracingWarningCh:
						// continue tracing misconfiguration MUST NOT interrupt workflow
					case <-time.After(2 * time.Second):
						t.Fatalf("Didn't get expected tracing warning")
					}
				}
			}

			if set, val := findPluginPathValue(testCase.give); set {
				RequestTheServer(t, client, pluginPathField, val)
			}

			// after we added _test to package name we are not able anymore to call private methods
			//if testCase.config.HealthcheckInterval != 0 {
			//	RequestTheServer(t, client, healthCheckIntervalField, testCase.config.HealthcheckInterval.String())
			//}

			// in case we're testing healthCheck scenarios
			if testCase.timeOutBefore != 0 {
				if testCase.checkHealthCheck {
					shutdownEngine()
				}
				time.Sleep(testCase.timeOutBefore)
				if testCase.checkHealthCheck {
					assert.Error(t, serverDone.Err(), "the healthcheck had to be triggered in this scenario")
					checkFinishFunc(t, finishFunc)
					checkExitCode(t, errCmd)
					return
				} else {
					assert.NoError(t, serverDone.Err(), "the healthcheck had to be passed in this scenario")
				}
			}

			// Simulate sending the os.Interrupt signal to the subprocess
			err = cmd.Process.Signal(os.Interrupt)
			if err != nil {
				t.Fatalf("Failed to send interrupt signal to the subprocess: %v", err)
			}

			// Wait for the server (subprocess) to shut down
			select {
			case <-serverDone.Done():
				fmt.Println("Server shutdown gracefully after receiving signal")
			case <-time.After(2 * time.Second):
				t.Fatalf("Server did not shutdown after receiving signal")
			}

			// Ensure that finish func was executed
			checkFinishFunc(t, finishFunc)
			checkExitCode(t, errCmd)
		})
	}
}

func checkFinishFunc(t *testing.T, finish chan struct{}) {
	select {
	case <-finish:
		fmt.Println("Finish func was executed")
	case <-time.After(2 * time.Second):
		t.Fatalf("Finish func wasn't executed")
	}
}

//nolint:paralleltest
func TestCmd(t *testing.T) {
	var testCaseId string
	// This is the function that will run in the subprocess
	if testCaseId = os.Getenv("TEST_CASE_ID"); testCaseId == "" {
		return
	}
	testCase := tests[testCaseId]

	s, err := rpcserver.NewServer(testCase.config)
	if err != nil {
		cmdutil.Exit(err)
	}

	testCase.f(s)
}

// HealthCheckImpl

// HealthServer implements the grpc_health_v1.HealthServer interface
type HealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
}

// Check returns the health status of the server
func (s *HealthServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	if req.Service == "" {
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_SERVING,
		}, nil
	}
	return nil, status.Errorf(codes.NotFound, "unknown service: %s", req.Service)
}

// Watch is not implemented for this simple example
func (s *HealthServer) Watch(req *grpc_health_v1.HealthCheckRequest, stream grpc_health_v1.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "Watch is not implemented")
}

func StartHealthCheckServer(t *testing.T) (string, func()) {
	// Create a new gRPC server
	grpcServer := grpc.NewServer()

	// Register the health service
	grpc_health_v1.RegisterHealthServer(grpcServer, &HealthServer{})

	// Listen on a TCP port
	listener, err := net.Listen("tcp", "")
	if err != nil {
		t.Fatalf("Failed to listen: %v\n", err)
	}

	// Start the server in a goroutine
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Fatalf("Failed to serve: %v\n", err)
		}
	}()

	return listener.Addr().String(), func() {
		grpcServer.GracefulStop()
	}
}

// Tracing server impl
func StartMockTracingServer(t *testing.T) (string, func(), chan string) {
	requestChan := make(chan string, 100) // Channel to capture request data

	// Create a custom HTTP server
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read the body of the request
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("could not read body")
			}
			defer r.Body.Close()

			// TODO Try to decode the body as Thrift (Jaeger Span)
			//decodedSpan, err := decodeThriftSpan(body)
			//if err != nil {
			//	fmt.Printf("Failed to decode Thrift data: %v\n", err)
			//}

			// Send the trace data to the channel for further processing in tests
			requestChan <- string(body)
			w.WriteHeader(http.StatusOK)
		}),
	}

	// Create a listener on a random available port
	listener, err := net.Listen("tcp", "localhost:0") // 0 means assign a random port
	if err != nil {
		t.Fatalf("Failed to start mock tracing server: %v", err)
	}

	// Get the address of the server
	serverAddr := listener.Addr().String()

	// Start the server in a goroutine
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			t.Fatalf("Server error: %v", err)
		}
	}()

	// Define a shutdown function to gracefully stop the server
	shutdownFunc := func() {
		if err := server.Close(); err != nil {
			t.Fatalf("Failed to shut down server: %v", err)
		}
	}

	return serverAddr, shutdownFunc, requestChan
}

// TODO I couldn't make following code to work yet
// decodeThriftSpan attempts to decode the given Thrift binary data into a Jaeger span
//func decodeThriftSpan(thriftData []byte) (*jaeger.Batch, error) {
//	// Create a Thrift transport and protocol for decoding
//	transport := thrift.NewTMemoryBufferLen(1024)
//	_, err := transport.Write(thriftData)
//	if err != nil {
//		return nil, fmt.Errorf("error writing to buffer: %w", err)
//	}
//	a := thrift.THeaderProtocolBinary
//	protocol := thrift.NewTBinaryProtocolConf(transport, &thrift.TConfiguration{
//		THeaderProtocolID: &a,
//	})
//
//	// Initialize a Jaeger span (as defined in jaeger.thrift)
//	span := jaeger.NewBatch()
//
//	// Use context.Background() to provide the required context
//	err = span.Read(context.Background(), protocol)
//	if err != nil {
//		return nil, fmt.Errorf("error decoding Thrift data: %w", err)
//	}
//
//	return span, nil
//}
