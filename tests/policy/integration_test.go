// Copyright 2016-2025, Pulumi Corporation.
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

package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/assert"
)

type Runtime int

const (
	NodeJS Runtime = iota
	Python
)

type PolicyConfig map[string]interface{}

// policyTestScenario describes an iteration of the
type policyTestScenario struct {
	// WantErrors is the error message we expect to see in the command's output.
	WantErrors []string
	// Whether the error messages are advisory, and don't actually fail the operation.
	Advisory bool
	// The Policy Pack configuration to use for the test scenario.
	PolicyPackConfig map[string]PolicyConfig
}

// runPolicyPackIntegrationTest creates a new Pulumi stack and then runs through
// a sequence of test scenarios where a configuration value is set and then
// the stack is updated or previewed, confirming the expected result.
func runPolicyPackIntegrationTest(
	t *testing.T, testDirName string,
	initialConfig map[string]string, scenarios []policyTestScenario,
) {
	t.Logf("Running Policy Pack Integration Test from directory %q", testDirName)

	// Get the directory for the policy pack to run.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting working directory")
	}
	rootDir := filepath.Join(cwd, testDirName)

	// The Pulumi project name matches the test dir name in these tests.
	t.Setenv("PULUMI_TEST_PROJECT", testDirName)

	// Copy the root directory to /tmp and run various operations within that directory.
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	e.ImportDirectory(rootDir)

	// Change to the Policy Pack directory.
	packDirs := []string{filepath.Join(e.RootPath, "policy-pack"), filepath.Join(e.RootPath, "policy-pack-python")}

	for _, packDir := range packDirs {
		e.CWD = packDir

		// Get dependencies.
		e.RunCommand("pulumi", "install")

		// Change to the Pulumi program directory.
		programDir := filepath.Join(e.RootPath, "program")
		e.CWD = programDir

		stackName := fmt.Sprintf("%s-%s", testDirName, ptesting.RandomStackName())
		t.Setenv("PULUMI_TEST_STACK", stackName)

		// Create the stack.
		e.RunCommand("pulumi", "login", "--local")
		e.RunCommand("pulumi", "stack", "init", stackName)
		e.RunCommand("pulumi", "install")

		// Initial configuration.
		for k, v := range initialConfig {
			e.RunCommand("pulumi", "config", "set", k, v)
		}

		// After this point, we want be sure to cleanup the stack, so we don't accidentally leak
		// any cloud resources.
		defer func() {
			t.Log("Cleaning up Stack")
			e.RunCommand("pulumi", "destroy", "--yes")
			e.RunCommand("pulumi", "stack", "rm", "--yes")
		}()

		assert.True(t, len(scenarios) > 0, "no test scenarios provided")
		runScenarios := func(policyPackDirectoryPath string) {
			t.Run(policyPackDirectoryPath, func(t *testing.T) {
				e.T = t

				// Clean up the stack after running through the scenarios, so that subsequent runs
				// begin on a clean slate.
				defer func() {
					e.RunCommand("pulumi", "destroy", "--yes")
				}()

				for idx, scenario := range scenarios {
					// Create a sub-test so go test will output data incrementally, which will let
					// a CI system like Travis know not to kill the job if no output is sent after 10m.
					// idx+1 to make it 1-indexed.
					scenarioName := fmt.Sprintf("scenario_%d", idx+1)
					t.Run(scenarioName, func(t *testing.T) {
						e.T = t

						e.RunCommand("pulumi", "config", "set", "scenario", strconv.Itoa(idx+1))

						cmd := "pulumi"
						args := []string{"up", "--yes", "--policy-pack", policyPackDirectoryPath}

						// If there is config for the scenario, write it out to a file and pass the file path
						// as a --policy-pack-config argument.
						if len(scenario.PolicyPackConfig) > 0 {
							// Marshal the config to JSON, with indentation for easier debugging.
							bytes, err := json.MarshalIndent(scenario.PolicyPackConfig, "", "    ")
							if err != nil {
								t.Fatalf("error marshalling policy config to JSON: %v", err)
							}

							// Change to the config directory.
							configDir := filepath.Join(e.RootPath, "config", scenarioName)
							e.CWD = configDir

							// Write the JSON to a file.
							filename := "policy-config.json"
							e.WriteTestFile(filename, string(bytes))

							// Add the policy config argument.
							policyConfigFile := filepath.Join(configDir, filename)
							args = append(args, "--policy-pack-config", policyConfigFile)

							// Change back to the program directory to proceed with the update.
							e.CWD = programDir
						}

						if len(scenario.WantErrors) == 0 {
							t.Log("No errors are expected.")
							e.RunCommand(cmd, args...)
						} else {
							var stdout, stderr string
							if scenario.Advisory {
								stdout, stderr = e.RunCommand(cmd, args...)
							} else {
								stdout, stderr = e.RunCommandExpectError(cmd, args...)
							}

							for _, wantErr := range scenario.WantErrors {
								inSTDOUT := strings.Contains(stdout, wantErr)
								inSTDERR := strings.Contains(stderr, wantErr)

								if !inSTDOUT && !inSTDERR {
									t.Errorf("Did not find expected error %q", wantErr)
								}
							}

							if t.Failed() {
								t.Logf("Command output:\nSTDOUT:\n%v\n\nSTDERR:\n%v\n\n", stdout, stderr)
							}
						}
					})
				}
			})
		}
		runScenarios(packDir)

		e.T = t
		t.Log("Finished test scenarios.")
		// Cleanup already registered via defer.
	}
}

// Test basic resource validation.
//
//nolint:paralleltest // Not designed to be run in parallel.
func TestValidateResource(t *testing.T) {
	runPolicyPackIntegrationTest(t, "validate_resource", nil, []policyTestScenario{
		// Test scenario 1: no resources.
		{
			WantErrors: nil,
		},
		// Test scenario 2: no violations.
		{
			WantErrors: nil,
		},
		// Test scenario 3: violates the first policy.
		{
			WantErrors: []string{
				"validate-resource-test-policy@v0.0.1 ",
				"[mandatory]  dynamic-no-state-with-value-1  (pulumi-nodejs:dynamic:Resource: a)",
				"Prohibits setting state to 1 on dynamic resources.",
				"'state' must not have the value 1.",
			},
		},
		// Test scenario 4: violates the second policy.
		{
			WantErrors: []string{
				"validate-resource-test-policy@v0.0.1 ",
				"[mandatory]  dynamic-no-state-with-value-2  (pulumi-nodejs:dynamic:Resource: b)",
				"Prohibits setting state to 2 on dynamic resources.",
				"'state' must not have the value 2.",
			},
		},
		// Test scenario 5: violates the first validation function of the third policy.
		{
			WantErrors: []string{
				"validate-resource-test-policy@v0.0.1 ",
				"[mandatory]  dynamic-no-state-with-value-3-or-4  (pulumi-nodejs:dynamic:Resource: c)",
				"Prohibits setting state to 3 or 4 on dynamic resources.",
				"'state' must not have the value 3.",
			},
		},
		// Test scenario 6: violates the second validation function of the third policy.
		{
			WantErrors: []string{
				"validate-resource-test-policy@v0.0.1 ",
				"[mandatory]  dynamic-no-state-with-value-3-or-4  (pulumi-nodejs:dynamic:Resource: d)",
				"Prohibits setting state to 3 or 4 on dynamic resources.",
				"'state' must not have the value 4.",
			},
		},
		// Test scenario 7: violates the fourth policy.
		{
			WantErrors: []string{
				"validate-resource-test-policy@v0.0.1 ",
				"[mandatory]  randomuuid-no-keepers  (random:index/randomUuid:RandomUuid: r1)",
				"Prohibits creating a RandomUuid without any 'keepers'.",
				"RandomUuid must not have an empty 'keepers'.",
			},
		},
		// Test scenario 8: no violations.
		{
			WantErrors: nil,
		},
		// Test scenario 9: violates the fifth policy.
		{
			WantErrors: []string{
				"validate-resource-test-policy@v0.0.1 ",
				"[mandatory]  dynamic-no-state-with-value-5  (pulumi-nodejs:dynamic:Resource: e)",
				"Prohibits setting state to 5 on dynamic resources.",
				"'state' must not have the value 5.",
			},
		},
		// Test scenario 10: no violations.
		{
			WantErrors: nil,
		},
		// Test scenario 11: no violations.
		// Test the ability to send large gRPC messages (>4mb).
		// Issue: https://github.com/pulumi/pulumi/issues/4155
		{
			WantErrors: nil,
		},
	})
}
