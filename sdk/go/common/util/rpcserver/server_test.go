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
	"errors"
	"flag"
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

	"google.golang.org/grpc/credentials/insecure"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcserver"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	pingpb "github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcserver/mockGRPC"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

const (
	tracingFlag     = "-tracing"
	helpFlag        = "-help"
	helpShortFlag   = "-h"
	engineAddrField = "EngineAddr"
	pluginPathField = "PluginPath"
	// healthCheckIntervalField = "HealthCheckInterval"

	pluginPath   = "plugin/path"
	tracingName  = "tracing-name"
	rootSpanName = "root-span-name"

	// healthCheckInterval = time.Second

	engineAddressSub = "ENGINE_ADDR_SUB"
	tracingSub       = "TRACING_ADDR"

	overrideTracingName  = "Zinder"
	overrideRootSpanName = "Kisii"

	finishFuncMessage = "FINISH_FUNC_MESSAGE"

	helpMessageSuccess = "this can be printed only in help of std flag"

	waitD = 2 * time.Second
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

func findPluginPathValue(args []string, val string) (bool, string) {
	// Iterate over the args slice to find the arg
	for _, arg := range args {
		if arg == val {
			return true, arg // it will panic if test input is invalid
		}
	}
	return false, ""
}

// RemoveAllDashPrefix removes all leading dashes from the string.
func RemoveAllDashPrefix(s string) string {
	for strings.HasPrefix(s, "-") {
		s = strings.TrimPrefix(s, "-")
	}
	return s
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

var helpTestFunc = func(s *rpcserver.Server) {
	s.Flag.String("aaa", "", helpMessageSuccess)
}

// helps to distinguish between not initialized 0s
func intPointer(i int) *int {
	return &i
}

var tests = map[string]struct {
	config rpcserver.Config
	give   []string

	f func(s *rpcserver.Server)

	beforeParseFlag func(s *rpcserver.Server)

	timeOutBefore    time.Duration
	checkHealthCheck bool

	expectExitMessage string
	expectExitCode    *int

	testFlags   []string
	unKnownFlag string

	tracingWarning   string
	tracingOverrides bool
}{
	"simplest_run": {
		config: rpcserver.Config{},
		give:   []string{engineAddressSub},
		f:      standardFunc,
	},
	"missed_required_engine": {
		config:            rpcserver.Config{},
		give:              []string{},
		f:                 standardFunc,
		expectExitMessage: "missing required engine RPC address argument",
		expectExitCode:    intPointer(255),
	},
	"optional_engine": {
		config: rpcserver.Config{EngineAddressOptional: true},
		give:   []string{},
		f:      standardFunc,
	},
	"run_with_tracing_plugin_path": {
		config: rpcserver.Config{
			HealthcheckInterval: time.Minute,
			TracingName:         tracingName, RootSpanName: rootSpanName,
		},
		give: []string{tracingFlag, tracingSub, engineAddressSub, pluginPath},
		f:    standardFunc,
	},
	"ensure_tracing": {
		config: rpcserver.Config{
			HealthcheckInterval: time.Minute,
			TracingName:         tracingName, RootSpanName: rootSpanName,
		},
		give: []string{tracingFlag, tracingSub, engineAddressSub},
		f:    standardFunc,
	},
	"pflag_ignore_help_flag": { // mustn't print help message
		config:            rpcserver.Config{},
		give:              []string{helpFlag, engineAddressSub},
		beforeParseFlag:   helpTestFunc,
		f:                 standardFunc,
		expectExitMessage: helpMessageSuccess,
		expectExitCode:    intPointer(255),
	},
	"pflag_ignore_short help_flag": { // mustn't print help message
		config:            rpcserver.Config{},
		give:              []string{helpShortFlag, engineAddressSub},
		beforeParseFlag:   helpTestFunc,
		f:                 standardFunc,
		expectExitMessage: helpMessageSuccess,
		expectExitCode:    intPointer(255),
	},
	"ensure_tracing_warning": {
		config:         rpcserver.Config{HealthcheckInterval: time.Minute},
		give:           []string{tracingFlag, tracingSub, engineAddressSub},
		f:              standardFunc,
		tracingWarning: "Tracing disabled.",
	},
	"ensure_tracing_override": {
		config: rpcserver.Config{
			HealthcheckInterval: time.Minute,
			TracingName:         tracingName, RootSpanName: rootSpanName,
		},
		give: []string{tracingFlag, tracingSub, engineAddressSub},
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
	"custom_flag_not_registered": {
		config:            rpcserver.Config{},
		give:              []string{"-unknown", "unknown", engineAddressSub},
		f:                 standardFunc,
		expectExitMessage: "Usage of",
		expectExitCode:    intPointer(2),
	},
	"custom_flag_registered": {
		config:    rpcserver.Config{},
		give:      []string{"-unknown", "unknown", engineAddressSub},
		testFlags: []string{"-unknown"},
		f:         standardFunc,
	},
	"engine_stopped_healthcheck_shutdown": {
		config:           rpcserver.Config{HealthcheckInterval: 500 * time.Millisecond},
		give:             []string{engineAddressSub},
		f:                standardFunc,
		timeOutBefore:    2 * time.Second,
		checkHealthCheck: true,
	},
	"healthcheck_valid": {
		config:        rpcserver.Config{HealthcheckInterval: 500 * time.Millisecond},
		give:          []string{engineAddressSub},
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
func (s *PingServer) Ping(_ context.Context, req *pingpb.PingRequest) (*pingpb.PingResponse, error) {
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

func checkExitCode(t *testing.T, serverDone <-chan struct{}, err error, expectedCode int) {
	waitForCondition(t, serverDone, time.Second, func(_ struct{}) {},
		"Cant check signal as didn't get signal that server finished")

	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			assert.Equal(t, expectedCode, exitError.ExitCode(), "Subprocess exited with non-zero exit code")
		} else {
			t.Fatalf("Subprocess finished with unexpected error: %v", err)
		}
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

// waitForCondition waits for a condition function to be satisfied or a timeout to occur.
// It takes a channel to listen on, a timeout duration, and a function that processes
// the channel data.
func waitForCondition[T any](t *testing.T, ch <-chan T, timeout time.Duration, f func(T), timeoutMessage string) {
	select {
	case data := <-ch:
		f(data)
	case <-time.After(timeout):
		t.Fatalf("Timeout: %s", timeoutMessage)
	}
}

//nolint:paralleltest
func TestSubprocess(t *testing.T) {
	for testCaseID, testCase := range tests {
		t.Run("Test Case "+testCaseID, func(t *testing.T) {
			engineAddress, shutdownEngine := StartHealthCheckServer(t)
			defer shutdownEngine()
			substituteArg(testCase.give, engineAddressSub, engineAddress)

			tracingAddr, shutdownTracingAddr, tracingChan := StartMockTracingServer(t)
			defer shutdownTracingAddr()
			substituteArg(testCase.give, tracingSub, "http://"+tracingAddr)

			// Use os.Executable() to get the path to the current test binary
			executablePath, err := os.Executable()
			assert.NoError(t, err, "failed to get current executable path")

			// Run the test in a subprocess
			cmd := exec.Command(executablePath, append([]string{"-test.run=TestCmd"}, testCase.give...)...)
			cmd.Env = append(os.Environ(), "TEST_CASE_ID="+testCaseID)

			// Capture stdout dynamically
			stdoutPipe, err := cmd.StdoutPipe()
			assert.NoError(t, err, "failed to get stdout pipe")
			// Capture stderr dynamically
			stderrPipe, err := cmd.StderrPipe()
			assert.NoError(t, err, "failed to get stderr pipe")

			serverShutdownComplete, fireServerDone := context.WithCancel(context.Background())

			// to suppress race detector we will control sending kill signal
			serverStarted, fireServerStarted := context.WithCancel(context.Background())

			var errCmd error

			// Start the command
			go func() {
				if err := cmd.Start(); err != nil { // Use Start() instead of Run() here to avoid blocking
					panic(fmt.Sprintf("Failed to start command: %v", err))
				}

				fireServerStarted()

				// Wait for the subprocess to finish
				errCmd = cmd.Wait()

				// Check the exit code
				fireServerDone()
			}()

			// Read stdout to capture the port number
			portC := make(chan string, 10000)
			finishFunc, fireFinishFunc := context.WithCancel(context.Background())
			go func() {
				scanner := bufio.NewScanner(stdoutPipe)
				for scanner.Scan() {
					line := scanner.Text()
					fmt.Printf("%s\n", line)
					if line == finishFuncMessage {
						fireFinishFunc()
						continue
					}
					portC <- line
				}
			}()

			tracingWarning, fireTracingWarning := context.WithCancel(context.Background())
			expectedExit, fireExpectedExit := context.WithCancel(context.Background())
			go func() {
				scanner := bufio.NewScanner(stderrPipe)
				for scanner.Scan() {
					line := scanner.Text()
					fmt.Printf("[STDERR] %s\n", line)
					if testCase.tracingWarning != "" {
						if strings.Contains(line, testCase.tracingWarning) {
							fireTracingWarning()
						}
					}
					if testCase.expectExitMessage != "" {
						if strings.Contains(line, testCase.expectExitMessage) {
							fireExpectedExit()
						}
					}
				}
			}()

			// Wait for the port to be captured
			var port string
			select {
			case port = <-portC:
			case <-time.After(waitD):
				if testCase.expectExitMessage != "" {
					waitForCondition(t, expectedExit.Done(), time.Second, func(_ struct{}) {
						checkExitCode(t, serverShutdownComplete.Done(), errCmd, *testCase.expectExitCode)
					}, "Didn't get expected missed engine exit")
					return
				}
				t.Fatal("Timeout waiting for the port to be printed")
			}

			// Check if the port is a valid integer
			_, err = strconv.Atoi(port)
			assert.NoError(t, err, "expected port number, got: ", port)

			// Connect to the gRPC server using the captured port
			conn, err := grpc.Dial("localhost:"+port, grpc.WithTransportCredentials(insecure.NewCredentials()))
			assert.NoError(t, err)
			defer conn.Close()

			client := pingpb.NewPingServiceClient(conn)
			RequestTheServer(t, client, "Ping", "Pong")

			if set, val := findFlagValue(testCase.give, tracingFlag); set {
				RequestTheServer(t, client, tracingFlag, val)

				// also check that mock tracing server actually get trace logs ONLY IF don't expect warning
				if testCase.tracingWarning == "" {
					waitForCondition(t, tracingChan, waitD, func(traceResponse TracingMockMessage) {
						assert.NoError(t, traceResponse.err, "tracing server returned error")
						if testCase.tracingOverrides {
							assert.Contains(t, traceResponse.msg, overrideTracingName)
							// TODO figure out why rootSpanName is not there. I assume it requires more complicated mock server?
							// assert.Contains(t, traceString, overrideRootSpanName)
						} else {
							assert.Contains(t, traceResponse.msg, tracingName)
							// TODO figure out why rootSpanName is not there. I assume it requires more complicated mock server?
							// assert.Contains(t, traceString, rootSpanName)
						}
					}, "Didn't get expected tracing")
				} else {
					waitForCondition(t, tracingWarning.Done(), waitD, func(_ struct{}) {
						// continue tracing; tracing misconfiguration MUST NOT interrupt workflow
					}, "Didn't get expected tracing warning")
				}
			}

			if set, _ := findPluginPathValue(testCase.give, pluginPath); set {
				RequestTheServer(t, client, pluginPathField, pluginPath)
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
					assert.Error(t, serverShutdownComplete.Err(), "the healthcheck had to be triggered in this scenario")
					checkFinishFunc(t, finishFunc.Done())
					checkExitCode(t, serverShutdownComplete.Done(), errCmd, 0)
					return
				}
				assert.NoError(t, serverShutdownComplete.Err(), "the healthcheck had to be passed in this scenario")
			}

			// we need to add this unused wait to calm down the race detector warnings
			waitForCondition(t, serverStarted.Done(), waitD, func(_ struct{}) {}, "")

			// Simulate sending the os.Interrupt signal to the subprocess
			err = cmd.Process.Signal(os.Interrupt)
			assert.NoError(t, err, "failed to send interrupt signal to the subprocess")

			// Wait for the server (subprocess) to shut down
			waitForCondition(t, serverShutdownComplete.Done(), waitD, func(_ struct{}) {
				fmt.Println("Server shutdown gracefully after receiving signal")
			}, "Server did not shutdown after receiving signal")

			// Ensure that finish func was executed
			checkFinishFunc(t, finishFunc.Done())
			checkExitCode(t, serverShutdownComplete.Done(), errCmd, 0)
		})
	}
}

func checkFinishFunc(t *testing.T, finish <-chan struct{}) {
	waitForCondition(t, finish, waitD, func(_ struct{}) {
		fmt.Println("Finish func was executed")
	}, "Finish func wasn't executed")
}

// SUBPROCESS
// The following code is split into two parts: TestMain (initialization) and TestCmd (execution).
// The key challenge is ensuring that custom flags are registered before the test execution begins.
// TestMain is responsible for initializing the test environment, including setting up the server
// and registering any custom flags that are needed for the specific test case. It uses the
// environment variable "TEST_CASE_ID" to identify if the test is being run in a subprocess.
//
// TestCmd is the function where the actual test logic is executed. It runs only in the subprocess,
// after TestMain has completed the initialization and custom flags have been parsed. If custom
// flags are required for a test case, they are registered in TestMain before TestCmd runs.
//
// The tricky part is that flag registration must happen in TestMain (before flag.Parse() is called),
// since Go’s testing framework automatically parses flags at the beginning of the test execution.
// Therefore, we set up the test environment, register the custom flags, and handle any potential
// server initialization errors before running the test logic in TestCmd.

var (
	server        *rpcserver.Server
	serverInitErr error
)

func TestMain(m *testing.M) {
	// In subprocess
	if testCaseID := os.Getenv("TEST_CASE_ID"); testCaseID != "" {
		fmt.Fprintf(os.Stderr, "start case %s\n", testCaseID)
		testCase := tests[testCaseID]
		testCase.config.Flag = flag.CommandLine
		server, serverInitErr = rpcserver.NewServer(testCase.config)
		if serverInitErr == nil {
			for _, testFlag := range testCase.testFlags {
				server.Flag.String(RemoveAllDashPrefix(testFlag), "", "test case")
			}
		} else {
			cmdutil.Exit(serverInitErr)
		}

		if testCase.beforeParseFlag != nil {
			fmt.Fprintf(os.Stderr, "beforeParseFlag is set and will be executed\n")
			testCase.beforeParseFlag(server) // Now call the function
		}

		fmt.Fprintf(os.Stderr, "ready to run case %s\n", testCaseID)
	}
	os.Exit(m.Run())
}

//nolint:paralleltest
func TestCmd(t *testing.T) {
	var testCaseID string
	// Only run this func in subprocess
	if testCaseID = os.Getenv("TEST_CASE_ID"); testCaseID == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "run case %s\n", testCaseID)
	testCase := tests[testCaseID]

	assert.NoError(t, server.Flag.Parse(os.Args[1:]))
	testCase.f(server)
}

// SUBPROCESS END

// HEALTHCHECK SERVER

// HealthServer implements the grpc_health_v1.HealthServer interface
type HealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
}

// Check returns the health status of the server
func (s *HealthServer) Check(_ context.Context, req *grpc_health_v1.HealthCheckRequest) (
	*grpc_health_v1.HealthCheckResponse, error,
) {
	if req.Service == "" {
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_SERVING,
		}, nil
	}
	return nil, status.Errorf(codes.NotFound, "unknown service: %s", req.Service)
}

// Watch is not implemented for this simple example
func (s *HealthServer) Watch(_ *grpc_health_v1.HealthCheckRequest, _ grpc_health_v1.Health_WatchServer) error {
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
			panic(fmt.Sprintf("Failed to serve: %v\n", err))
		}
	}()

	return listener.Addr().String(), func() {
		grpcServer.GracefulStop()
	}
}

// HEALTHCHECK SERVER END

// TRACING SERVER

type TracingMockMessage struct {
	err error
	msg string
}

// Tracing server impl
func StartMockTracingServer(t *testing.T) (string, func(), chan TracingMockMessage) {
	requestChan := make(chan TracingMockMessage, 100) // Channel to capture request data

	// Create a custom HTTP server
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read the body of the request
			body, err := io.ReadAll(r.Body)
			if err != nil {
				requestChan <- TracingMockMessage{err: err}
			}
			defer r.Body.Close()

			// TODO Try to decode the body as Thrift (Jaeger Span)
			//decodedSpan, err := decodeThriftSpan(body)
			//if err != nil {
			//	fmt.Printf("Failed to decode Thrift data: %v\n", err)
			//}

			// Send the trace data to the channel for further processing in tests
			requestChan <- TracingMockMessage{msg: string(body)}
			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: 5 * time.Second, // Prevent Slowloris attacks (golint error)
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
		if errS := server.Serve(listener); errS != nil && !errors.Is(errS, http.ErrServerClosed) {
			requestChan <- TracingMockMessage{err: err}
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

// TRACING SERVER END
