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

package pulumi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	backendDisplay "github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/filestate"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	enginerpc "github.com/pulumi/pulumi/sdk/v3/proto/go/engine"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type EngineServer interface {
	enginerpc.EngineServer

	// Address returns the address at which the engine's RPC server may be reached.
	Address() string

	// Cancel signals that the engine should be terminated, awaits its termination, and returns any errors that result.
	Cancel() error

	// Done awaits the engines termination, and returns any errors that result.
	Done() error
}

func Start(ctx context.Context) (EngineServer, error) {
	// New up an engine RPC server.
	engine := &engineServer{
		ctx:    ctx,
		cancel: make(chan bool),
	}

	// Fire up a gRPC server and start listening for incomings.
	port, done, err := rpcutil.Serve(0, engine.cancel, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			enginerpc.RegisterEngineServer(srv, engine)
			return nil
		},
	}, nil)
	if err != nil {
		return nil, err
	}

	engine.addr = fmt.Sprintf("127.0.0.1:%d", port)
	engine.done = done

	return engine, nil
}

// engineServer is the server side of the engine RPC machinery.
type engineServer struct {
	enginerpc.UnsafeEngineServer

	ctx    context.Context
	cancel chan bool
	done   chan error
	addr   string
}

func (eng *engineServer) Address() string {
	return eng.addr
}

func (eng *engineServer) Cancel() error {
	eng.cancel <- true
	return <-eng.done
}

func (eng *engineServer) Done() error {
	return <-eng.done
}

type languageTest struct {
	program   string
	config    config.Map
	providers []plugin.Provider
	assert    func(result.Result, *deploy.Snapshot, display.ResourceChanges) (bool, string)
}

func expectStackResource(res result.Result, changes display.ResourceChanges) (bool, string) {
	if res != nil {
		return false, fmt.Sprintf("expected no error, got %v", res)
	}

	if len(changes) != 1 {
		return false, fmt.Sprintf("expected 1 StepOp, got %v", changes)
	}

	if changes[deploy.OpCreate] != 1 {
		return false, fmt.Sprintf("expected 1 Create, got %v", changes[deploy.OpCreate])
	}

	return true, ""
}

var languageTests = map[string]languageTest{
	"l0-empty": {
		program: "",
		assert: func(res result.Result, snap *deploy.Snapshot, changes display.ResourceChanges) (bool, string) {
			return expectStackResource(res, changes)
		},
	},
	"l0-output_bool": {
		program: `
			output "output_true" "bool" {
				value = true
			}

			output "output_false" "bool" {
				value = false
			}
		`,
		assert: func(res result.Result, snap *deploy.Snapshot, changes display.ResourceChanges) (bool, string) {
			ok, msg := expectStackResource(res, changes)
			if !ok {
				return ok, msg
			}

			// Check we have two outputs in the stack for true and false
			stack := snap.Resources[0]
			if stack.Type != resource.RootStackType {
				return false, "expected a stack resource"
			}
			outputs := stack.Outputs
			var trueOut, falseOut resource.PropertyValue
			var has bool
			if trueOut, has = outputs[resource.PropertyKey("output_true")]; !has {
				return false, "expected output_true to be in stack outputs"
			}
			if falseOut, has = outputs[resource.PropertyKey("output_false")]; !has {
				return false, "expected output_false to be in stack outputs"
			}

			if trueOut != resource.NewBoolProperty(true) {
				return false, "expected output_true to be true"
			}

			if falseOut != resource.NewBoolProperty(false) {
				return false, "expected output_false to be false"
			}

			return true, ""
		},
	},
	"l1-resource-simple": {
		program: `
				resource "res" "simple:index:Resource" {
					value = true
				}
			`,
		providers: []plugin.Provider{&simpleProvider{}},
		assert: func(res result.Result, snap *deploy.Snapshot, changes display.ResourceChanges) (bool, string) {
			if res != nil {
				return false, fmt.Sprintf("expected no error, got %v", res)
			}

			if len(changes) != 0 {
				return false, fmt.Sprintf("expected no resources, got %v", changes)
			}

			return true, ""
		},
	},
}

func (eng *engineServer) GetLanguageTests(ctx context.Context, req *enginerpc.GetLanguageTestsRequest) (*enginerpc.GetLanguageTestsResponse, error) {
	tests := make([]string, 0, len(languageTests))
	for testName := range languageTests {
		tests = append(tests, testName)
	}

	return &enginerpc.GetLanguageTestsResponse{
		Tests: tests,
	}, nil
}

func makeTestResponse(msg string) *enginerpc.RunLanguageTestResponse {
	return &enginerpc.RunLanguageTestResponse{
		Success: false,
		Message: msg,
	}
}

func (eng *engineServer) RunLanguageTest(ctx context.Context, req *enginerpc.RunLanguageTestRequest) (*enginerpc.RunLanguageTestResponse, error) {
	test, has := languageTests[req.Test]
	if !has {
		return nil, errors.New("unknown test")
	}

	// Create a temp project dir for the test to run in
	rootDir, err := os.MkdirTemp("", "pulumi-language-test-root")
	if err != nil {
		return nil, fmt.Errorf("create temp project dir: %w", err)
	}
	defer os.RemoveAll(rootDir)

	// Create a diagnostics sink for the test
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	snk := diag.DefaultSink(stdout, stderr, diag.FormatOptions{
		Color: colors.Never,
	})

	// Start up a plugin context
	pctx, err := plugin.NewContextWithContext(ctx, snk, snk, nil, rootDir, rootDir, nil, false, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("setup plugin context: %w", err)
	}

	// Connect to the language host
	conn, err := grpc.Dial(req.LanguagePluginAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial language plugin: %w", err)
	}

	languageClient := plugin.NewLanguageRuntimeClient(pctx, "uut", pulumirpc.NewLanguageRuntimeClient(conn))

	// Generate SDKs for all the providers we need
	sdksDir, err := os.MkdirTemp("", "pulumi-language-test-sdks")
	if err != nil {
		return nil, fmt.Errorf("create temp sdks dir: %w", err)
	}
	defer os.RemoveAll(sdksDir)

	localDependencies := make(map[string]string)
	for _, provider := range test.providers {
		pkg := string(provider.Pkg())
		sdkDir := filepath.Join(sdksDir, pkg)
		localDependencies[pkg] = sdkDir

		schema, err := provider.GetSchema(0)
		if err != nil {
			return nil, fmt.Errorf("get schema for provider %s: %w", pkg, err)
		}

		err = languageClient.GeneratePackage(sdkDir, string(schema), nil)
		if err != nil {
			return makeTestResponse(fmt.Sprintf("generate package %s: %v", pkg, err)), nil
		}
	}

	// Generate the project and read in the Pulumi.yaml
	projectJson := fmt.Sprintf(`{
			"name": "%s",
			"runtime": {
				"name": "client",
				"options": {
					"address": "%s"
				}
			}
		}`, req.Test, req.LanguagePluginAddress)

	err = languageClient.GenerateProject(
		rootDir, projectJson,
		map[string]string{
			"main.pp": test.program,
		},
		localDependencies,
	)
	if err != nil {
		return makeTestResponse(fmt.Sprintf("generate project: %v", err)), nil
	}

	// TODO: We don't capture stdout/stderr from the language plugin, so we can't show it back to the test.
	err = languageClient.InstallDependencies(rootDir)
	if err != nil {
		return makeTestResponse(fmt.Sprintf("generate project: %v", err)), nil
	}

	project, err := workspace.LoadProject(filepath.Join(rootDir, "Pulumi.yaml"))
	if err != nil {
		return makeTestResponse(fmt.Sprintf("load project: %v", err)), nil
	}

	// Create a temp dir for the a filestate backend to run in for the test
	backendDir, err := os.MkdirTemp("", "pulumi-language-test-backend")
	if err != nil {
		return nil, fmt.Errorf("create temp backend dir: %w", err)
	}
	defer os.RemoveAll(backendDir)
	testBackend, err := filestate.New(ctx, snk, "file://"+backendDir, project)
	if err != nil {
		return nil, fmt.Errorf("create filestate backend: %w", err)
	}

	// Create a new stack for the test
	stackReference, err := testBackend.ParseStackReference("test")
	if err != nil {
		return nil, fmt.Errorf("parse test stack reference: %w", err)
	}
	s, err := testBackend.CreateStack(ctx, stackReference, rootDir, nil)
	if err != nil {
		return nil, fmt.Errorf("create test stack: %w", err)
	}

	// Set up the stack and engine configuration
	opts := backend.UpdateOptions{
		AutoApprove: true,
		SkipPreview: true,
		Display: backendDisplay.Options{
			Color:  colors.Never,
			Stdout: stdout,
			Stderr: stderr,
		},
	}
	sm := b64.NewBase64SecretsManager()
	dec, err := sm.Decrypter()
	contract.AssertNoErrorf(err, "base64 must be able to create a Decrypter")

	cfg := backend.StackConfiguration{
		Config:    test.config,
		Decrypter: dec,
	}

	changes, res := s.Update(ctx, backend.UpdateOperation{
		Proj:               project,
		Root:               rootDir,
		Opts:               opts,
		M:                  &backend.UpdateMetadata{},
		StackConfiguration: cfg,
		SecretsManager:     sm,
		SecretsProvider:    stack.DefaultSecretsProvider,
		Scopes:             backend.CancellationScopes,
	})

	var snap *deploy.Snapshot
	if res == nil {
		// Refetch the stack so we can get the snapshot
		s, err = testBackend.GetStack(ctx, stackReference)
		if err != nil {
			return nil, fmt.Errorf("get stack: %w", err)
		}

		snap, err = s.Snapshot(ctx, stack.DefaultSecretsProvider)
		if err != nil {
			return nil, fmt.Errorf("snapshot: %w", err)
		}
	}

	success, message := test.assert(res, snap, changes)

	return &enginerpc.RunLanguageTestResponse{
		Success: success,
		Message: message,
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
	}, nil
}
