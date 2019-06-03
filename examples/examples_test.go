// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package examples

import (
	"bytes"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func TestExamples(t *testing.T) {
	cwd, err := os.Getwd()
	if !assert.NoError(t, err, "expected a valid working directory: %v", err) {
		return
	}

	getExamples := func() []integration.ProgramTestOptions {
		var formattableStdout, formattableStderr bytes.Buffer
		return []integration.ProgramTestOptions{
			{
				Dir:          path.Join(cwd, "minimal"),
				Dependencies: []string{"@pulumi/pulumi"},
				Config: map[string]string{
					"name": "Pulumi",
				},
				Secrets: map[string]string{
					"secret": "this is my secret message",
				},
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					// Simple runtime validation that just ensures the checkpoint was written and read.
					assert.NotNil(t, stackInfo.Deployment)
				},
				RunBuild: true,
			},
			{
				Dir:          path.Join(cwd, "dynamic-provider/simple"),
				Dependencies: []string{"@pulumi/pulumi"},
				Config: map[string]string{
					"simple:config:w": "1",
					"simple:config:x": "1",
					"simple:config:y": "1",
				},
			},
			{
				Dir:          path.Join(cwd, "dynamic-provider/class-with-comments"),
				Dependencies: []string{"@pulumi/pulumi"},
				Config:       map[string]string{},
			},
			{
				Dir:          path.Join(cwd, "dynamic-provider/multiple-turns"),
				Dependencies: []string{"@pulumi/pulumi"},
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					for _, res := range stackInfo.Deployment.Resources {
						if !providers.IsProviderType(res.Type) && res.Parent == "" {
							assert.Equal(t, stackInfo.RootResource.URN, res.URN,
								"every resource but the root resource should have a parent, but %v didn't", res.URN)
						}
					}
				},
			},
			{
				Dir:          path.Join(cwd, "dynamic-provider/derived-inputs"),
				Dependencies: []string{"@pulumi/pulumi"},
			},
			{
				Dir:          path.Join(cwd, "formattable"),
				Dependencies: []string{"@pulumi/pulumi"},
				ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
					// Note that we're abusing this hook to validate stdout. We don't actually care about the checkpoint.
					stdout := formattableStdout.String()
					assert.False(t, strings.Contains(stdout, "MISSING"))
				},
				Stdout: &formattableStdout,
				Stderr: &formattableStderr,
			},
			{
				Dir:          path.Join(cwd, "dynamic-provider/multiple-turns-2"),
				Dependencies: []string{"@pulumi/pulumi"},
			},
		}
	}

	// Get the entire set of examples first.
	examples := getExamples()

	// Now, add them again, this time using a local-login path. This helps test all the same
	// scenarios against local paths to validate that they are working properly.
	for _, test := range getExamples() {
		examples = append(examples, test.With(integration.ProgramTestOptions{
			CloudURL: "file://~",
		}))
	}

	// Add a secrets example:  This deploys a program that spins up a bunch of custom resources with different sets
	// of secret inputs.
	examples = append(examples, integration.ProgramTestOptions{
		Dir:          path.Join(cwd, "secrets"),
		Dependencies: []string{"@pulumi/pulumi"},
		Config: map[string]string{
			"message": "plaintext message",
		},
		Secrets: map[string]string{
			"apiKey": "FAKE_API_KEY_FOR_TESTING",
		},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Deployment.SecretsProviders, "Deployment should have a secrets provider")

			isEncrypted := func(v interface{}) bool {
				if m, ok := v.(map[string]interface{}); ok {
					sigKey := m[resource.SigKey]
					if sigKey == nil {
						return false
					}

					v, vOk := sigKey.(string)
					if !vOk {
						return false
					}

					if v != resource.SecretSig {
						return false
					}

					ciphertext := m["ciphertext"]
					if ciphertext == nil {
						return false
					}

					_, cOk := ciphertext.(string)
					return cOk
				}

				return false
			}

			assertEncryptedValue := func(m map[string]interface{}, key string) {
				assert.Truef(t, isEncrypted(m[key]), "%s value should be encrypted", key)
			}

			assertPlaintextValue := func(m map[string]interface{}, key string) {
				assert.Truef(t, !isEncrypted(m[key]), "%s value should not encrypted", key)
			}

			for _, res := range stackInfo.Deployment.Resources {
				if res.Type == "pulumi-nodejs:dynamic:Resource" {
					switch res.URN.Name() {
					case "sValue", "sApply", "cValue", "cApply":
						assertEncryptedValue(res.Inputs, "value")
						assertEncryptedValue(res.Outputs, "value")
					case "pValue", "pApply":
						assertPlaintextValue(res.Inputs, "value")
						assertPlaintextValue(res.Outputs, "value")
					case "pDummy":
						assertPlaintextValue(res.Outputs, "value")
					case "sDummy":
						// Creation of this resource passes in a custom resource options to ensure that "value" is
						// treated as secret.  In the state file, we'll see this as an uncrypted input with an
						// encrypted output.
						assertEncryptedValue(res.Outputs, "value")
					case "rValue":
						assertEncryptedValue(res.Inputs["value"].(map[string]interface{}), "secret")
						assertEncryptedValue(res.Outputs["value"].(map[string]interface{}), "secret")
						assertPlaintextValue(res.Inputs["value"].(map[string]interface{}), "plain")
						assertPlaintextValue(res.Outputs["value"].(map[string]interface{}), "plain")
					default:
						contract.Assertf(false, "unknown name type: %s", res.URN.Name())
					}
				}
			}

			assertEncryptedValue(stackInfo.Outputs, "combinedApply")
			assertEncryptedValue(stackInfo.Outputs, "combinedMessage")
			assertPlaintextValue(stackInfo.Outputs, "plaintextApply")
			assertPlaintextValue(stackInfo.Outputs, "plaintextMessage")
			assertEncryptedValue(stackInfo.Outputs, "secretApply")
			assertEncryptedValue(stackInfo.Outputs, "secretMessage")
			assertEncryptedValue(stackInfo.Outputs, "richStructure")
		},
	})

	// The compat test only works on Node 6.10.X because its uses the old 0.10.0 pulumi package, which only supported
	// a single node version, since it had the native runtime component.
	if nodeVer, err := getNodeVersion(); err != nil && nodeVer.Major == 6 && nodeVer.Minor == 10 {
		examples = append(examples, integration.ProgramTestOptions{
			Dir: path.Join(cwd, "compat/v0.10.0/minimal"),
			Config: map[string]string{
				"name": "Pulumi",
			},
			Secrets: map[string]string{
				"secret": "this is my secret message",
			},
			RunBuild: true,
		})
	} else {
		t.Log("Skipping 0.10.0 compat tests, because current node version is not 6.10.X")
	}

	for _, example := range examples {
		ex := example
		t.Run(example.Dir, func(t *testing.T) {
			integration.ProgramTest(t, &ex)
		})
	}
}

func getNodeVersion() (semver.Version, error) {
	var buf bytes.Buffer

	nodeVersionCmd := exec.Command("node", "--version")
	nodeVersionCmd.Stdout = &buf
	if err := nodeVersionCmd.Run(); err != nil {
		return semver.Version{}, errors.Wrap(err, "running node --version")
	}

	return semver.ParseTolerant(buf.String())
}
