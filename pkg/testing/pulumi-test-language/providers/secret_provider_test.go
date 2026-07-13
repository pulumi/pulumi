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

	newData := func() resource.PropertyValue {
		return resource.NewProperty(resource.PropertyMap{
			"private": resource.NewProperty("inner-hidden"),
			"public":  resource.NewProperty("inner-visible"),
		})
	}

	// validProps returns a fresh property map that Check accepts; each test case mutates its
	// own copy.
	validProps := func() resource.PropertyMap {
		return resource.PropertyMap{
			"private":     resource.MakeSecret(resource.NewProperty("hidden")),
			"public":      resource.NewProperty("visible"),
			"privateData": resource.MakeSecret(newData()),
			"publicData": resource.NewProperty(resource.PropertyMap{
				"private": resource.NewProperty("data-hidden"),
				"public":  resource.NewProperty("data-visible"),
			}),
			"privateArray": resource.MakeSecret(resource.NewProperty([]resource.PropertyValue{
				resource.NewProperty("array-hidden"),
			})),
			"privateMap": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
				"key": resource.NewProperty("map-hidden"),
			})),
			"privateDataArray": resource.MakeSecret(resource.NewProperty([]resource.PropertyValue{
				newData(),
			})),
			"privateDataMap": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
				"key": newData(),
			})),
		}
	}

	tests := []struct {
		name     string
		mutate   func(props resource.PropertyMap)
		wantFail bool
	}{
		{
			name:   "valid input",
			mutate: func(resource.PropertyMap) {},
		},
		{
			name: "publicData.private as secret<string>",
			mutate: func(props resource.PropertyMap) {
				props["publicData"].ObjectValue()["private"] = resource.MakeSecret(resource.NewProperty("data-hidden"))
			},
		},
		{
			name: "privateData.private as secret<string>",
			mutate: func(props resource.PropertyMap) {
				data := props["privateData"].SecretValue().Element.ObjectValue()
				data["private"] = resource.MakeSecret(resource.NewProperty("inner-hidden"))
			},
		},
		{
			name: "public as secret<string>",
			mutate: func(props resource.PropertyMap) {
				props["public"] = resource.MakeSecret(resource.NewProperty("visible"))
			},
		},
		{
			name: "publicData.public as secret<string>",
			mutate: func(props resource.PropertyMap) {
				props["publicData"].ObjectValue()["public"] = resource.MakeSecret(resource.NewProperty("data-visible"))
			},
		},
		{
			name: "privateData.public as secret<string>",
			mutate: func(props resource.PropertyMap) {
				data := props["privateData"].SecretValue().Element.ObjectValue()
				data["public"] = resource.MakeSecret(resource.NewProperty("inner-visible"))
			},
		},
		{
			name: "publicData.private as secret<secret<string>>",
			mutate: func(props resource.PropertyMap) {
				data := props["publicData"].ObjectValue()
				data["private"] = resource.MakeSecret(resource.MakeSecret(resource.NewProperty("data-hidden")))
			},
		},
		{
			name: "private is secret<secret<string>>",
			mutate: func(props resource.PropertyMap) {
				props["private"] = resource.MakeSecret(resource.MakeSecret(resource.NewProperty("hidden")))
			},
		},
		{
			name: "privateData is secret<secret<object>>",
			mutate: func(props resource.PropertyMap) {
				props["privateData"] = resource.MakeSecret(resource.MakeSecret(newData()))
			},
		},
		{
			name: "all fields deeply nested in secrets",
			mutate: func(props resource.PropertyMap) {
				props["private"] = resource.MakeSecret(resource.MakeSecret(resource.MakeSecret(resource.NewProperty("hidden"))))
				props["public"] = resource.MakeSecret(resource.MakeSecret(resource.NewProperty("visible")))
				props["privateData"] = resource.MakeSecret(resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"private": resource.MakeSecret(resource.MakeSecret(resource.NewProperty("inner-hidden"))),
					"public":  resource.MakeSecret(resource.NewProperty("inner-visible")),
				})))
				props["publicData"] = resource.NewProperty(resource.PropertyMap{
					"private": resource.MakeSecret(resource.MakeSecret(resource.MakeSecret(resource.NewProperty("data-hidden")))),
					"public":  resource.MakeSecret(resource.MakeSecret(resource.NewProperty("data-visible"))),
				})
				props["privateArray"] = resource.MakeSecret(resource.MakeSecret(resource.NewProperty([]resource.PropertyValue{
					resource.MakeSecret(resource.NewProperty("array-hidden")),
				})))
				props["privateMap"] = resource.MakeSecret(resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"key": resource.MakeSecret(resource.NewProperty("map-hidden")),
				})))
				props["privateDataArray"] = resource.MakeSecret(resource.MakeSecret(resource.NewProperty(
					[]resource.PropertyValue{resource.MakeSecret(newData())})))
				props["privateDataMap"] = resource.MakeSecret(resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"key": resource.MakeSecret(newData()),
				})))
			},
		},
		{
			name: "private is plain string, not secret",
			mutate: func(props resource.PropertyMap) {
				props["private"] = resource.NewProperty("not-a-secret")
			},
			wantFail: true,
		},
		{
			name: "privateData is plain object, not secret",
			mutate: func(props resource.PropertyMap) {
				props["privateData"] = newData()
			},
			wantFail: true,
		},
		{
			name: "publicData is secret<object>",
			mutate: func(props resource.PropertyMap) {
				props["publicData"] = resource.MakeSecret(props["publicData"])
			},
			wantFail: true,
		},
		{
			name: "privateArray is plain array, not secret",
			mutate: func(props resource.PropertyMap) {
				props["privateArray"] = resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty("array-hidden"),
				})
			},
			wantFail: true,
		},
		{
			name: "privateArray element is not a string",
			mutate: func(props resource.PropertyMap) {
				props["privateArray"] = resource.MakeSecret(resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty(true),
				}))
			},
			wantFail: true,
		},
		{
			name: "privateMap is plain map, not secret",
			mutate: func(props resource.PropertyMap) {
				props["privateMap"] = resource.NewProperty(resource.PropertyMap{
					"key": resource.NewProperty("map-hidden"),
				})
			},
			wantFail: true,
		},
		{
			name: "privateDataArray is plain array, not secret",
			mutate: func(props resource.PropertyMap) {
				props["privateDataArray"] = resource.NewProperty([]resource.PropertyValue{newData()})
			},
			wantFail: true,
		},
		{
			name: "privateDataArray element is not a Data object",
			mutate: func(props resource.PropertyMap) {
				props["privateDataArray"] = resource.MakeSecret(resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty(resource.PropertyMap{
						"private": resource.NewProperty("inner-hidden"),
					}),
				}))
			},
			wantFail: true,
		},
		{
			name: "privateDataMap is plain map, not secret",
			mutate: func(props resource.PropertyMap) {
				props["privateDataMap"] = resource.NewProperty(resource.PropertyMap{
					"key": newData(),
				})
			},
			wantFail: true,
		},
		{
			name: "privateDataMap value is not a Data object",
			mutate: func(props resource.PropertyMap) {
				props["privateDataMap"] = resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
					"key": resource.NewProperty("not-a-data-object"),
				}))
			},
			wantFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			props := validProps()
			tt.mutate(props)
			p := &SecretProvider{}
			resp, err := p.Check(t.Context(), plugin.CheckRequest{
				URN:  resource.NewURN("test-stack", "test-project", "", "secret:index:Resource", "res"),
				News: props,
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
