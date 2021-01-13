// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.
// +build go all

package ints

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pulumi/pulumi/pkg/v2/testing/integration"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/stretchr/testify/assert"
)

// TestEmptyGo simply tests that we can build and run an empty Go project.
func TestEmptyGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("empty", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v2",
		},
		Quick: true,
	})
}

// TestEmptyGoRun exercises the 'go run' invocation path that doesn't require an explicit build step.
func TestEmptyGoRun(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("empty", "gorun"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v2",
		},
		Quick: true,
	})
}

// TestEmptyGoRunMain exercises the 'go run' invocation path with a 'main' entrypoint specified in Pulumi.yml
func TestEmptyGoRunMain(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("empty", "gorun_main"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v2",
		},
		Quick: true,
	})
}

// Tests basic configuration from the perspective of a Pulumi Go program.
func TestConfigBasicGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: filepath.Join("config_basic", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v2",
		},
		Quick: true,
		Config: map[string]string{
			"aConfigValue": "this value is a value",
		},
		Secrets: map[string]string{
			"bEncryptedSecret": "this super secret is encrypted",
		},
		OrderedConfig: []integration.ConfigValue{
			{Key: "outer.inner", Value: "value", Path: true},
			{Key: "names[0]", Value: "a", Path: true},
			{Key: "names[1]", Value: "b", Path: true},
			{Key: "names[2]", Value: "c", Path: true},
			{Key: "names[3]", Value: "super secret name", Path: true, Secret: true},
			{Key: "servers[0].port", Value: "80", Path: true},
			{Key: "servers[0].host", Value: "example", Path: true},
			{Key: "a.b[0].c", Value: "true", Path: true},
			{Key: "a.b[1].c", Value: "false", Path: true},
			{Key: "tokens[0]", Value: "shh", Path: true, Secret: true},
			{Key: "foo.bar", Value: "don't tell", Path: true, Secret: true},
		},
	})
}

// Tests that stack references work in Go.
func TestStackReferenceGo(t *testing.T) {
	if runtime.GOOS == WindowsOS {
		t.Skip("Temporarily skipping test on Windows - pulumi/pulumi#3811")
	}
	if owner := os.Getenv("PULUMI_TEST_OWNER"); owner == "" {
		t.Skipf("Skipping: PULUMI_TEST_OWNER is not set")
	}

	opts := &integration.ProgramTestOptions{
		Dir: filepath.Join("stack_reference", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v2",
		},
		Quick: true,
		Config: map[string]string{
			"org": os.Getenv("PULUMI_TEST_OWNER"),
		},
		EditDirs: []integration.EditDir{
			{
				Dir:      "step1",
				Additive: true,
			},
			{
				Dir:      "step2",
				Additive: true,
			},
		},
	}
	integration.ProgramTest(t, opts)
}

// Tests a resource with a large (>4mb) string prop in Go
func TestLargeResourceGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v2",
		},
		Dir: filepath.Join("large_resource", "go"),
	})
}

// Test remote component construction in Go.
func TestConstructGo(t *testing.T) {
	pathEnv, err := testComponentPathEnv()
	if err != nil {
		t.Fatalf("failed to build test component PATH: %v", err)
	}

	// TODO[pulumi/pulumi#5455]: Dynamic providers fail to load when used from multi-lang components.
	// Until we've addressed this, set PULUMI_TEST_YARN_LINK_PULUMI, which tells the integration test
	// module to run `yarn install && yarn link @pulumi/pulumi` in the Go program's directory, allowing
	// the Node.js dynamic provider plugin to load.
	// When the underlying issue has been fixed, the use of this environment variable inside the integration
	// test module should be removed.
	const testYarnLinkPulumiEnv = "PULUMI_TEST_YARN_LINK_PULUMI=true"

	var opts *integration.ProgramTestOptions
	opts = &integration.ProgramTestOptions{
		Env: []string{pathEnv, testYarnLinkPulumiEnv},
		Dir: filepath.Join("construct_component", "go"),
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v2",
		},
		Quick: true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Deployment)
			if assert.Equal(t, 9, len(stackInfo.Deployment.Resources)) {
				stackRes := stackInfo.Deployment.Resources[0]
				assert.NotNil(t, stackRes)
				assert.Equal(t, resource.RootStackType, stackRes.Type)
				assert.Equal(t, "", string(stackRes.Parent))

				// Check that dependencies flow correctly between the originating program and the remote component
				// plugin.
				urns := make(map[string]resource.URN)
				for _, res := range stackInfo.Deployment.Resources[1:] {
					assert.NotNil(t, res)

					urns[string(res.URN.Name())] = res.URN
					switch res.URN.Name() {
					case "child-a":
						for _, deps := range res.PropertyDependencies {
							assert.Empty(t, deps)
						}
					case "child-b":
						assert.Equal(t, []resource.URN{urns["a"]}, res.PropertyDependencies["echo"])
					case "child-c":
						assert.ElementsMatch(t, []resource.URN{urns["child-a"], urns["a"]},
							res.PropertyDependencies["echo"])
					}
				}
			}
		},
	}
	integration.ProgramTest(t, opts)
}

func TestGetResourceGo(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dependencies: []string{
			"github.com/pulumi/pulumi/sdk/v2",
		},
		Dir:                      filepath.Join("get_resource", "go"),
		AllowEmptyPreviewChanges: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stack.Outputs)
			assert.Equal(t, float64(2), stack.Outputs["getPetLength"])
		},
	})
}
