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

package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestSecretProvider_Check(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		props    resource.PropertyMap
		wantFail bool
	}{
		{
			name: "valid input",
			props: resource.PropertyMap{
				"private": resource.MakeSecret(resource.NewProperty("hidden")),
				"public":  resource.NewProperty("visible"),
				"privateData": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("inner-hidden"),
					"public":  resource.NewProperty("inner-visible"),
				})),
				"publicData": resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("data-hidden"),
					"public":  resource.NewProperty("data-visible"),
				}),
			},
		},
		{
			name: "publicData.private as secret<string>",
			props: resource.PropertyMap{
				"private": resource.MakeSecret(resource.NewProperty("hidden")),
				"public":  resource.NewProperty("visible"),
				"privateData": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("inner-hidden"),
					"public":  resource.NewProperty("inner-visible"),
				})),
				"publicData": resource.NewProperty(resource.PropertyMap{
					"private": resource.MakeSecret(resource.NewProperty("data-hidden")),
					"public":  resource.NewProperty("data-visible"),
				}),
			},
		},
		{
			name: "privateData.private as secret<string>",
			props: resource.PropertyMap{
				"private": resource.MakeSecret(resource.NewProperty("hidden")),
				"public":  resource.NewProperty("visible"),
				"privateData": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"private": resource.MakeSecret(resource.NewProperty("inner-hidden")),
					"public":  resource.NewProperty("inner-visible"),
				})),
				"publicData": resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("data-hidden"),
					"public":  resource.NewProperty("data-visible"),
				}),
			},
		},
		{
			name: "public as secret<string>",
			props: resource.PropertyMap{
				"private": resource.MakeSecret(resource.NewProperty("hidden")),
				"public":  resource.MakeSecret(resource.NewProperty("visible")),
				"privateData": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("inner-hidden"),
					"public":  resource.NewProperty("inner-visible"),
				})),
				"publicData": resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("data-hidden"),
					"public":  resource.NewProperty("data-visible"),
				}),
			},
		},
		{
			name: "publicData.public as secret<string>",
			props: resource.PropertyMap{
				"private": resource.MakeSecret(resource.NewProperty("hidden")),
				"public":  resource.NewProperty("visible"),
				"privateData": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("inner-hidden"),
					"public":  resource.NewProperty("inner-visible"),
				})),
				"publicData": resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("data-hidden"),
					"public":  resource.MakeSecret(resource.NewProperty("data-visible")),
				}),
			},
		},
		{
			name: "privateData.public as secret<string>",
			props: resource.PropertyMap{
				"private": resource.MakeSecret(resource.NewProperty("hidden")),
				"public":  resource.NewProperty("visible"),
				"privateData": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("inner-hidden"),
					"public":  resource.MakeSecret(resource.NewProperty("inner-visible")),
				})),
				"publicData": resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("data-hidden"),
					"public":  resource.NewProperty("data-visible"),
				}),
			},
		},
		{
			name: "publicData.private as secret<secret<string>>",
			props: resource.PropertyMap{
				"private": resource.MakeSecret(resource.NewProperty("hidden")),
				"public":  resource.NewProperty("visible"),
				"privateData": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("inner-hidden"),
					"public":  resource.NewProperty("inner-visible"),
				})),
				"publicData": resource.NewProperty(resource.PropertyMap{
					"private": resource.MakeSecret(resource.MakeSecret(resource.NewProperty("data-hidden"))),
					"public":  resource.NewProperty("data-visible"),
				}),
			},
		},
		{
			name: "private is secret<secret<string>>",
			props: resource.PropertyMap{
				"private": resource.MakeSecret(resource.MakeSecret(resource.NewProperty("hidden"))),
				"public":  resource.NewProperty("visible"),
				"privateData": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("inner-hidden"),
					"public":  resource.NewProperty("inner-visible"),
				})),
				"publicData": resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("data-hidden"),
					"public":  resource.NewProperty("data-visible"),
				}),
			},
		},
		{
			name: "privateData is secret<secret<object>>",
			props: resource.PropertyMap{
				"private": resource.MakeSecret(resource.NewProperty("hidden")),
				"public":  resource.NewProperty("visible"),
				"privateData": resource.MakeSecret(resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("inner-hidden"),
					"public":  resource.NewProperty("inner-visible"),
				}))),
				"publicData": resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("data-hidden"),
					"public":  resource.NewProperty("data-visible"),
				}),
			},
		},
		{
			name: "all fields deeply nested in secrets",
			props: resource.PropertyMap{
				"private": resource.MakeSecret(resource.MakeSecret(resource.MakeSecret(resource.NewProperty("hidden")))),
				"public":  resource.MakeSecret(resource.MakeSecret(resource.NewProperty("visible"))),
				"privateData": resource.MakeSecret(resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"private": resource.MakeSecret(resource.MakeSecret(resource.NewProperty("inner-hidden"))),
					"public":  resource.MakeSecret(resource.NewProperty("inner-visible")),
				}))),
				"publicData": resource.NewProperty(resource.PropertyMap{
					"private": resource.MakeSecret(resource.MakeSecret(resource.MakeSecret(resource.NewProperty("data-hidden")))),
					"public":  resource.MakeSecret(resource.MakeSecret(resource.NewProperty("data-visible"))),
				}),
			},
		},
		{
			name: "private is plain string, not secret",
			props: resource.PropertyMap{
				"private": resource.NewProperty("not-a-secret"),
				"public":  resource.NewProperty("visible"),
				"privateData": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("inner-hidden"),
					"public":  resource.NewProperty("inner-visible"),
				})),
				"publicData": resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("data-hidden"),
					"public":  resource.NewProperty("data-visible"),
				}),
			},
			wantFail: true,
		},
		{
			name: "privateData is plain object, not secret",
			props: resource.PropertyMap{
				"private": resource.MakeSecret(resource.NewProperty("hidden")),
				"public":  resource.NewProperty("visible"),
				"privateData": resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("inner-hidden"),
					"public":  resource.NewProperty("inner-visible"),
				}),
				"publicData": resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("data-hidden"),
					"public":  resource.NewProperty("data-visible"),
				}),
			},
			wantFail: true,
		},
		{
			name: "publicData is secret<object>",
			props: resource.PropertyMap{
				"private": resource.MakeSecret(resource.NewProperty("hidden")),
				"public":  resource.NewProperty("visible"),
				"privateData": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("inner-hidden"),
					"public":  resource.NewProperty("inner-visible"),
				})),
				"publicData": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"private": resource.NewProperty("data-hidden"),
					"public":  resource.NewProperty("data-visible"),
				})),
			},
			wantFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &SecretProvider{}
			resp, err := p.Check(t.Context(), plugin.CheckRequest{
				URN:  resource.NewURN("test-stack", "test-project", "", "secret:index:Resource", "res"),
				News: tt.props,
			})
			require.NoError(t, err)
			if tt.wantFail {
				assert.NotEmpty(t, resp.Failures)
			} else {
				assert.Empty(t, resp.Failures)
			}
		})
	}
}
