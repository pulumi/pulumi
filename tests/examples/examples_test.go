// Copyright 2016-2022, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package examples

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

//nolint:paralleltest // uses parallel programtest
func TestAccMinimal(t *testing.T) {
	test := getBaseOptions().
		With(integration.ProgramTestOptions{
			Dir: filepath.Join(getCwd(t), "minimal"),
			Config: map[string]string{
				"name": "Pulumi",
			},
			Secrets: map[string]string{
				"secret": "this is my secret message",
			},
			ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
				// Simple runtime validation that just ensures the checkpoint was written and read.
				require.NotNil(t, stackInfo.Deployment)
			},
			RunBuild: true,
		})

	integration.ProgramTest(t, &test)
}

//nolint:paralleltest // uses parallel programtest
func TestAccDynamicProviderSimple(t *testing.T) {
	test := getBaseOptions().
		With(integration.ProgramTestOptions{
			Dir: filepath.Join(getCwd(t), "dynamic-provider/simple"),
			Config: map[string]string{
				"simple:config:w": "1",
				"simple:config:x": "1",
				"simple:config:y": "1",
			},
		})

	integration.ProgramTest(t, &test)
}

//nolint:paralleltest // uses parallel programtest
func TestAccDynamicProviderClassWithComments(t *testing.T) {
	test := getBaseOptions().
		With(integration.ProgramTestOptions{
			Dir: filepath.Join(getCwd(t), "dynamic-provider/class-with-comments"),
		})

	integration.ProgramTest(t, &test)
}

//nolint:paralleltest // uses parallel programtest
func TestAccDynamicProviderClassWithComments_withLocalState(t *testing.T) {
	test := getBaseOptions().
		With(integration.ProgramTestOptions{
			Dir:      filepath.Join(getCwd(t), "dynamic-provider/class-with-comments"),
			CloudURL: integration.MakeTempBackend(t),
		})

	integration.ProgramTest(t, &test)
}

//nolint:paralleltest // uses parallel programtest
func TestAccDynamicProviderMultipleTurns(t *testing.T) {
	test := getBaseOptions().
		With(integration.ProgramTestOptions{
			Dir: filepath.Join(getCwd(t), "dynamic-provider/multiple-turns"),
			ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
				for _, res := range stackInfo.Deployment.Resources {
					if !providers.IsProviderType(res.Type) && res.Parent == "" {
						assert.Equal(t, stackInfo.RootResource.URN, res.URN,
							"every resource but the root resource should have a parent, but %v didn't", res.URN)
					}
				}
			},
		})

	integration.ProgramTest(t, &test)
}

//nolint:paralleltest // uses parallel programtest
func TestAccDynamicProviderMultipleTurns_withLocalState(t *testing.T) {
	test := getBaseOptions().
		With(integration.ProgramTestOptions{
			Dir: filepath.Join(getCwd(t), "dynamic-provider/multiple-turns"),
			ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
				for _, res := range stackInfo.Deployment.Resources {
					if !providers.IsProviderType(res.Type) && res.Parent == "" {
						assert.Equal(t, stackInfo.RootResource.URN, res.URN,
							"every resource but the root resource should have a parent, but %v didn't", res.URN)
					}
				}
			},
			CloudURL: integration.MakeTempBackend(t),
		})

	integration.ProgramTest(t, &test)
}

//nolint:paralleltest // uses parallel programtest
func TestAccDynamicProviderMultipleTurns2(t *testing.T) {
	test := getBaseOptions().
		With(integration.ProgramTestOptions{
			Dir: filepath.Join(getCwd(t), "dynamic-provider/multiple-turns-2"),
		})

	integration.ProgramTest(t, &test)
}

//nolint:paralleltest // uses parallel programtest
func TestAccDynamicProviderMultipleTurns2_withLocalState(t *testing.T) {
	test := getBaseOptions().
		With(integration.ProgramTestOptions{
			Dir:      filepath.Join(getCwd(t), "dynamic-provider/multiple-turns-2"),
			CloudURL: integration.MakeTempBackend(t),
		})

	integration.ProgramTest(t, &test)
}

//nolint:paralleltest // uses parallel programtest
func TestAccDynamicProviderSecrets(t *testing.T) {
	test := getBaseOptions().
		With(integration.ProgramTestOptions{
			Dir: filepath.Join(getCwd(t), "dynamic-provider/secrets"),
			Secrets: map[string]string{
				"password": "s3cret",
			},
			ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
				// Ensure the __provider input (and corresponding output) was marked secret
				dynRes := stackInfo.Deployment.Resources[2]
				for _, providerVal := range []interface{}{dynRes.Inputs["__provider"], dynRes.Outputs["__provider"]} {
					switch v := providerVal.(type) {
					case string:
						assert.Fail(t, "__provider was not a secret")
					case map[string]interface{}:
						assert.Equal(t, resource.SecretSig, v[resource.SigKey])
					}
				}
				// Ensure the resulting output had the expected value
				code, ok := stackInfo.Outputs["out"].(string)
				assert.True(t, ok)
				assert.Equal(t, "200", code)
			},
		})

	integration.ProgramTest(t, &test)
}

//nolint:paralleltest // uses parallel programtest
func TestAccDynamicProviderDerivedInputs(t *testing.T) {
	test := getBaseOptions().
		With(integration.ProgramTestOptions{
			Dir: filepath.Join(getCwd(t), "dynamic-provider/derived-inputs"),
		})

	integration.ProgramTest(t, &test)
}

//nolint:paralleltest // uses parallel programtest
func TestDynamicProviderGenericTypes(t *testing.T) {
	test := getBaseOptions().
		With(integration.ProgramTestOptions{
			Dir: filepath.Join(getCwd(t), "dynamic-provider/generic-types"),
		})

	integration.ProgramTest(t, &test)
}

//nolint:paralleltest // uses parallel programtest
func TestAccDynamicProviderDerivedInputs_withLocalState(t *testing.T) {
	test := getBaseOptions().
		With(integration.ProgramTestOptions{
			Dir:      filepath.Join(getCwd(t), "dynamic-provider/derived-inputs"),
			CloudURL: integration.MakeTempBackend(t),
		})

	integration.ProgramTest(t, &test)
}

//nolint:paralleltest // uses parallel programtest
func TestAccFormattable(t *testing.T) {
	var formattableStdout, formattableStderr bytes.Buffer
	test := getBaseOptions().
		With(integration.ProgramTestOptions{
			Dir: filepath.Join(getCwd(t), "formattable"),
			ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
				// Note that we're abusing this hook to validate stdout. We don't actually care about the checkpoint.
				stdout := formattableStdout.String()
				assert.False(t, strings.Contains(stdout, "MISSING"))
			},
			Stdout: &formattableStdout,
			Stderr: &formattableStderr,
		})

	integration.ProgramTest(t, &test)
}

//nolint:paralleltest // uses parallel programtest
func TestAccSecrets(t *testing.T) {
	test := getBaseOptions().
		With(integration.ProgramTestOptions{
			Dir: filepath.Join(getCwd(t), "secrets"),
			Config: map[string]string{
				"message": "plaintext message",
			},
			Secrets: map[string]string{
				"apiKey": "FAKE_API_KEY_FOR_TESTING",
			},
			Quick: true,
			ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
				require.NotNil(t, stackInfo.Deployment.SecretsProviders, "Deployment should have a secrets provider")

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

	integration.ProgramTest(t, &test)
}
