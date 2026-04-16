// Copyright 2026, Pulumi Corporation.
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

package httpstate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEscValueToInterface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    esc.Value
		expected any
	}{
		{
			name:     "string value",
			input:    esc.Value{Value: "hello"},
			expected: "hello",
		},
		{
			name:     "bool value",
			input:    esc.Value{Value: true},
			expected: true,
		},
		{
			name:     "nil value",
			input:    esc.Value{Value: nil},
			expected: nil,
		},
		{
			name: "map value",
			input: esc.Value{Value: map[string]esc.Value{
				"key": {Value: "val"},
			}},
			expected: map[string]any{"key": "val"},
		},
		{
			name: "slice value",
			input: esc.Value{Value: []esc.Value{
				{Value: "a"},
				{Value: "b"},
			}},
			expected: []any{"a", "b"},
		},
		{
			name: "nested map with slice",
			input: esc.Value{Value: map[string]esc.Value{
				"list": {Value: []esc.Value{
					{Value: "x"},
				}},
				"nested": {Value: map[string]esc.Value{
					"deep": {Value: "value"},
				}},
			}},
			expected: map[string]any{
				"list":   []any{"x"},
				"nested": map[string]any{"deep": "value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := escValueToInterface(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEscValueToConfigMap(t *testing.T) {
	t.Parallel()

	t.Run("valid policy config", func(t *testing.T) {
		t.Parallel()
		input := esc.Value{Value: map[string]esc.Value{
			"my-policy": {Value: map[string]esc.Value{
				"enforcement": {Value: "mandatory"},
			}},
		}}

		result, err := escValueToConfigMap(input)
		require.NoError(t, err)
		require.Contains(t, result, "my-policy")

		var parsed map[string]any
		err = json.Unmarshal(*result["my-policy"], &parsed)
		require.NoError(t, err)
		assert.Equal(t, "mandatory", parsed["enforcement"])
	})

	t.Run("non-map returns error", func(t *testing.T) {
		t.Parallel()
		input := esc.Value{Value: "not a map"}

		_, err := escValueToConfigMap(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected policyConfig to be a map")
	})

	t.Run("empty map", func(t *testing.T) {
		t.Parallel()
		input := esc.Value{Value: map[string]esc.Value{}}

		result, err := escValueToConfigMap(input)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// mockEnvironmentsBackend implements backend.EnvironmentsBackend for testing.
type mockEnvironmentsBackend struct {
	openYAMLEnvironmentF func(
		ctx context.Context, org string, yaml []byte, duration time.Duration,
	) (*esc.Environment, apitype.EnvironmentDiagnostics, error)
}

func (m *mockEnvironmentsBackend) CreateEnvironment(
	context.Context, string, string, string, []byte,
) (apitype.EnvironmentDiagnostics, error) {
	return nil, nil
}

func (m *mockEnvironmentsBackend) CheckYAMLEnvironment(
	context.Context, string, []byte,
) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
	return nil, nil, nil
}

func (m *mockEnvironmentsBackend) OpenYAMLEnvironment(
	ctx context.Context, org string, yaml []byte, duration time.Duration,
) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
	return m.openYAMLEnvironmentF(ctx, org, yaml, duration)
}

func TestResolveEnvironments(t *testing.T) {
	t.Parallel()

	t.Run("no environments returns nil", func(t *testing.T) {
		t.Parallel()
		rp := &cloudRequiredPolicy{
			RequiredPolicy: apitype.RequiredPolicy{
				Name: "test-pack",
			},
		}

		result, err := rp.ResolveEnvironments(t.Context())
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("resolves policyConfig and environmentVariables", func(t *testing.T) {
		t.Parallel()
		mock := &mockEnvironmentsBackend{
			openYAMLEnvironmentF: func(
				_ context.Context, org string, yaml []byte, _ time.Duration,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				assert.Equal(t, "test-org", org)
				assert.Contains(t, string(yaml), "prod/policy-config")
				return &esc.Environment{
					Properties: map[string]esc.Value{
						"policyConfig": {Value: map[string]esc.Value{
							"cost-policy": {Value: map[string]esc.Value{
								"maxMonthlyCost": {Value: json.Number("1000")},
							}},
						}},
						"environmentVariables": {Value: map[string]esc.Value{
							"AWS_REGION": {Value: "us-west-2"},
						}},
					},
				}, nil, nil
			},
		}

		rp := &cloudRequiredPolicy{
			envs:    mock,
			orgName: "test-org",
			RequiredPolicy: apitype.RequiredPolicy{
				Name:         "test-pack",
				Environments: []string{"prod/policy-config"},
			},
		}

		result, err := rp.ResolveEnvironments(t.Context())
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify policyConfig was extracted.
		require.Contains(t, result.Config, "cost-policy")
		var configVal map[string]any
		err = json.Unmarshal(*result.Config["cost-policy"], &configVal)
		require.NoError(t, err)
		assert.Equal(t, float64(1000), configVal["maxMonthlyCost"])

		// Verify environmentVariables were extracted via PrepareEnvironment.
		assert.Equal(t, "us-west-2", result.EnvironmentVariables["AWS_REGION"])
	})

	t.Run("OpenYAMLEnvironment error", func(t *testing.T) {
		t.Parallel()
		mock := &mockEnvironmentsBackend{
			openYAMLEnvironmentF: func(
				context.Context, string, []byte, time.Duration,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				return nil, nil, errors.New("connection refused")
			},
		}

		rp := &cloudRequiredPolicy{
			envs:    mock,
			orgName: "test-org",
			RequiredPolicy: apitype.RequiredPolicy{
				Name:         "test-pack",
				Environments: []string{"env1"},
			},
		}

		_, err := rp.ResolveEnvironments(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "opening ESC environments")
		assert.Contains(t, err.Error(), "connection refused")
	})

	t.Run("diagnostics returned as error", func(t *testing.T) {
		t.Parallel()
		mock := &mockEnvironmentsBackend{
			openYAMLEnvironmentF: func(
				context.Context, string, []byte, time.Duration,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				return nil, apitype.EnvironmentDiagnostics{
					{Summary: "unknown environment 'prod/missing'"},
				}, nil
			},
		}

		rp := &cloudRequiredPolicy{
			envs:    mock,
			orgName: "test-org",
			RequiredPolicy: apitype.RequiredPolicy{
				Name:         "test-pack",
				Environments: []string{"prod/missing"},
			},
		}

		_, err := rp.ResolveEnvironments(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown environment")
	})

	t.Run("only policyConfig no envVars", func(t *testing.T) {
		t.Parallel()
		mock := &mockEnvironmentsBackend{
			openYAMLEnvironmentF: func(
				context.Context, string, []byte, time.Duration,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				return &esc.Environment{
					Properties: map[string]esc.Value{
						"policyConfig": {Value: map[string]esc.Value{
							"p1": {Value: "simple"},
						}},
					},
				}, nil, nil
			},
		}

		rp := &cloudRequiredPolicy{
			envs:    mock,
			orgName: "org",
			RequiredPolicy: apitype.RequiredPolicy{
				Name:         "pack",
				Environments: []string{"env"},
			},
		}

		result, err := rp.ResolveEnvironments(t.Context())
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.Config)
		assert.Nil(t, result.EnvironmentVariables)
	})

	t.Run("only envVars no policyConfig", func(t *testing.T) {
		t.Parallel()
		mock := &mockEnvironmentsBackend{
			openYAMLEnvironmentF: func(
				context.Context, string, []byte, time.Duration,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				return &esc.Environment{
					Properties: map[string]esc.Value{
						"environmentVariables": {Value: map[string]esc.Value{
							"KEY": {Value: "val"},
						}},
					},
				}, nil, nil
			},
		}

		rp := &cloudRequiredPolicy{
			envs:    mock,
			orgName: "org",
			RequiredPolicy: apitype.RequiredPolicy{
				Name:         "pack",
				Environments: []string{"env"},
			},
		}

		result, err := rp.ResolveEnvironments(t.Context())
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Nil(t, result.Config)
		assert.Equal(t, map[string]string{"KEY": "val"}, result.EnvironmentVariables)
	})

	t.Run("resolves version-pinned environment references", func(t *testing.T) {
		t.Parallel()
		mock := &mockEnvironmentsBackend{
			openYAMLEnvironmentF: func(
				_ context.Context, org string, yaml []byte, _ time.Duration,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				assert.Equal(t, "test-org", org)
				yamlStr := string(yaml)
				assert.Contains(t, yamlStr, "prod/policy-config@v2.1.0")
				assert.Contains(t, yamlStr, "shared/base@stable")
				return &esc.Environment{
					Properties: map[string]esc.Value{
						"policyConfig": {Value: map[string]esc.Value{
							"cost-policy": {Value: map[string]esc.Value{
								"maxMonthlyCost": {Value: json.Number("500")},
							}},
						}},
					},
				}, nil, nil
			},
		}

		rp := &cloudRequiredPolicy{
			envs:    mock,
			orgName: "test-org",
			RequiredPolicy: apitype.RequiredPolicy{
				Name:         "test-pack",
				Environments: []string{"prod/policy-config@v2.1.0", "shared/base@stable"},
			},
		}

		result, err := rp.ResolveEnvironments(t.Context())
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Contains(t, result.Config, "cost-policy")
	})

	t.Run("namespaced policyConfig keys pass through", func(t *testing.T) {
		t.Parallel()
		mock := &mockEnvironmentsBackend{
			openYAMLEnvironmentF: func(
				context.Context, string, []byte, time.Duration,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				return &esc.Environment{
					Properties: map[string]esc.Value{
						"policyConfig": {Value: map[string]esc.Value{
							"my-pack:cost-policy": {Value: map[string]esc.Value{
								"maxCost": {Value: json.Number("500")},
							}},
							"naming-policy": {Value: map[string]esc.Value{
								"enforcement": {Value: "mandatory"},
							}},
						}},
					},
				}, nil, nil
			},
		}

		rp := &cloudRequiredPolicy{
			envs:    mock,
			orgName: "org",
			RequiredPolicy: apitype.RequiredPolicy{
				Name:         "my-pack",
				Environments: []string{"env"},
			},
		}

		result, err := rp.ResolveEnvironments(t.Context())
		require.NoError(t, err)
		require.NotNil(t, result)
		// Raw keys pass through as-is; namespace parsing happens during merge in update.go.
		assert.Contains(t, result.Config, "my-pack:cost-policy")
		assert.Contains(t, result.Config, "naming-policy")
	})

	t.Run("files exports create temp files and inject paths", func(t *testing.T) {
		t.Parallel()
		mock := &mockEnvironmentsBackend{
			openYAMLEnvironmentF: func(
				context.Context, string, []byte, time.Duration,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				return &esc.Environment{
					Properties: map[string]esc.Value{
						"files": {Value: map[string]esc.Value{
							"KUBECONFIG": {Value: "apiVersion: v1\nkind: Config\n"},
							"TLS_CERT":   {Value: "-----BEGIN CERTIFICATE-----\nMIIB...", Secret: true},
						}},
					},
				}, nil, nil
			},
		}

		rp := &cloudRequiredPolicy{
			envs:    mock,
			orgName: "org",
			RequiredPolicy: apitype.RequiredPolicy{
				Name:         "pack",
				Environments: []string{"env"},
			},
		}

		result, err := rp.ResolveEnvironments(t.Context())
		require.NoError(t, err)
		require.NotNil(t, result)

		// File paths should be injected as environment variables.
		kubePath := result.EnvironmentVariables["KUBECONFIG"]
		tlsPath := result.EnvironmentVariables["TLS_CERT"]
		require.NotEmpty(t, kubePath, "KUBECONFIG should have a temp file path")
		require.NotEmpty(t, tlsPath, "TLS_CERT should have a temp file path")

		// Temp files should exist and contain the expected content.
		kubeContent, err := os.ReadFile(kubePath)
		require.NoError(t, err)
		assert.Equal(t, "apiVersion: v1\nkind: Config\n", string(kubeContent))

		tlsContent, err := os.ReadFile(tlsPath)
		require.NoError(t, err)
		assert.Equal(t, "-----BEGIN CERTIFICATE-----\nMIIB...", string(tlsContent))

		// Secret file content should be tracked.
		assert.Contains(t, result.Secrets, "-----BEGIN CERTIFICATE-----\nMIIB...")

		// Clean up temp files.
		os.Remove(kubePath)
		os.Remove(tlsPath)
	})

	t.Run("files and env vars are merged", func(t *testing.T) {
		t.Parallel()
		mock := &mockEnvironmentsBackend{
			openYAMLEnvironmentF: func(
				context.Context, string, []byte, time.Duration,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				return &esc.Environment{
					Properties: map[string]esc.Value{
						"environmentVariables": {Value: map[string]esc.Value{
							"AWS_REGION": {Value: "us-west-2"},
						}},
						"files": {Value: map[string]esc.Value{
							"GOOGLE_CREDENTIALS": {Value: `{"type":"service_account"}`},
						}},
					},
				}, nil, nil
			},
		}

		rp := &cloudRequiredPolicy{
			envs:    mock,
			orgName: "org",
			RequiredPolicy: apitype.RequiredPolicy{
				Name:         "pack",
				Environments: []string{"env"},
			},
		}

		result, err := rp.ResolveEnvironments(t.Context())
		require.NoError(t, err)
		require.NotNil(t, result)

		// Both env vars and file paths should be present.
		assert.Equal(t, "us-west-2", result.EnvironmentVariables["AWS_REGION"])
		credPath := result.EnvironmentVariables["GOOGLE_CREDENTIALS"]
		require.NotEmpty(t, credPath)

		content, err := os.ReadFile(credPath)
		require.NoError(t, err)
		assert.Equal(t, `{"type":"service_account"}`, string(content))

		os.Remove(credPath)
	})

	t.Run("secrets are returned from environment", func(t *testing.T) {
		t.Parallel()
		mock := &mockEnvironmentsBackend{
			openYAMLEnvironmentF: func(
				context.Context, string, []byte, time.Duration,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				return &esc.Environment{
					Properties: map[string]esc.Value{
						"environmentVariables": {Value: map[string]esc.Value{
							"SECRET_KEY": {
								Value:  "super-secret-value",
								Secret: true,
							},
							"PLAIN_KEY": {Value: "plain-value"},
						}},
					},
				}, nil, nil
			},
		}

		rp := &cloudRequiredPolicy{
			envs:    mock,
			orgName: "org",
			RequiredPolicy: apitype.RequiredPolicy{
				Name:         "pack",
				Environments: []string{"env"},
			},
		}

		result, err := rp.ResolveEnvironments(t.Context())
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Contains(t, result.EnvironmentVariables, "SECRET_KEY")
		assert.Contains(t, result.EnvironmentVariables, "PLAIN_KEY")
		assert.Contains(t, result.Secrets, "super-secret-value")
	})
}

func TestLocalPolicyEnvironmentResolver(t *testing.T) {
	t.Parallel()

	t.Run("no environments returns nil", func(t *testing.T) {
		t.Parallel()
		resolver := NewLocalPolicyEnvironmentResolver(&mockEnvironmentsBackend{}, "org")
		result, err := resolver.ResolveEnvironments(t.Context(), nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("empty environments returns nil", func(t *testing.T) {
		t.Parallel()
		resolver := NewLocalPolicyEnvironmentResolver(&mockEnvironmentsBackend{}, "org")
		result, err := resolver.ResolveEnvironments(t.Context(), []string{})
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("resolves policyConfig and environmentVariables", func(t *testing.T) {
		t.Parallel()
		mock := &mockEnvironmentsBackend{
			openYAMLEnvironmentF: func(
				_ context.Context, org string, yaml []byte, _ time.Duration,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				assert.Equal(t, "test-org", org)
				assert.Contains(t, string(yaml), "org/policy-secrets")
				assert.Contains(t, string(yaml), "org/compliance-config")
				return &esc.Environment{
					Properties: map[string]esc.Value{
						"policyConfig": {Value: map[string]esc.Value{
							"cost-policy": {Value: map[string]esc.Value{
								"maxMonthlyCost": {Value: json.Number("1000")},
							}},
						}},
						"environmentVariables": {Value: map[string]esc.Value{
							"VALIDATOR_TOKEN": {Value: "secret-token", Secret: true},
							"API_URL":         {Value: "https://example.com"},
						}},
					},
				}, nil, nil
			},
		}

		resolver := NewLocalPolicyEnvironmentResolver(mock, "test-org")
		result, err := resolver.ResolveEnvironments(
			t.Context(), []string{"org/policy-secrets", "org/compliance-config"})
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify policyConfig was extracted.
		require.Contains(t, result.Config, "cost-policy")
		var configVal map[string]any
		err = json.Unmarshal(*result.Config["cost-policy"], &configVal)
		require.NoError(t, err)
		assert.Equal(t, float64(1000), configVal["maxMonthlyCost"])

		// Verify environment variables.
		assert.Equal(t, "secret-token", result.EnvironmentVariables["VALIDATOR_TOKEN"])
		assert.Equal(t, "https://example.com", result.EnvironmentVariables["API_URL"])

		// Verify secrets are tracked.
		assert.Contains(t, result.Secrets, "secret-token")
	})

	t.Run("OpenYAMLEnvironment error propagates", func(t *testing.T) {
		t.Parallel()
		mock := &mockEnvironmentsBackend{
			openYAMLEnvironmentF: func(
				context.Context, string, []byte, time.Duration,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				return nil, nil, errors.New("connection refused")
			},
		}

		resolver := NewLocalPolicyEnvironmentResolver(mock, "org")
		_, err := resolver.ResolveEnvironments(t.Context(), []string{"env/missing"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "opening ESC environments")
		assert.Contains(t, err.Error(), "connection refused")
	})

	t.Run("diagnostics returned as error", func(t *testing.T) {
		t.Parallel()
		mock := &mockEnvironmentsBackend{
			openYAMLEnvironmentF: func(
				context.Context, string, []byte, time.Duration,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				return nil, apitype.EnvironmentDiagnostics{
					{Summary: "unknown environment 'env/missing'"},
				}, nil
			},
		}

		resolver := NewLocalPolicyEnvironmentResolver(mock, "org")
		_, err := resolver.ResolveEnvironments(t.Context(), []string{"env/missing"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown environment")
	})

	t.Run("only envVars no policyConfig", func(t *testing.T) {
		t.Parallel()
		mock := &mockEnvironmentsBackend{
			openYAMLEnvironmentF: func(
				context.Context, string, []byte, time.Duration,
			) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
				return &esc.Environment{
					Properties: map[string]esc.Value{
						"environmentVariables": {Value: map[string]esc.Value{
							"KEY": {Value: "val"},
						}},
					},
				}, nil, nil
			},
		}

		resolver := NewLocalPolicyEnvironmentResolver(mock, "org")
		result, err := resolver.ResolveEnvironments(t.Context(), []string{"env"})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Nil(t, result.Config)
		assert.Equal(t, map[string]string{"KEY": "val"}, result.EnvironmentVariables)
	})
}
