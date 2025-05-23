// Copyright 2024, Pulumi Corporation.
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

package tests

import (
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
)

func init() {
	LanguageTests["l2-provider-grpc-config-secret"] = LanguageTest{
		// Check what schemaprov received in CheckRequest.
		Providers: []plugin.Provider{&providers.ConfigGrpcProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					g := &grpcTestContext{l: l, s: snap}

					// Now check first-class secrets for programsecretprov.
					r := g.CheckConfigReq("config")

					// These asserts do not look right, but are based on Go behavior. Should SECRET
					// be wrapped in secret tags instead when passing to CheckConfig? Or not?
					assert.Equal(l, "SECRET", r.News.Fields["string1"].AsInterface(), "string1")
					AssertEqualOrJSONEncoded(l, float64(1234567890), r.News.Fields["int1"].AsInterface(), "int1")
					AssertEqualOrJSONEncoded(l, float64(123456.789), r.News.Fields["num1"].AsInterface(), "num1")
					AssertEqualOrJSONEncoded(l, true, r.News.Fields["bool1"].AsInterface(), "bool1")
					AssertEqualOrJSONEncoded(l, []any{"SECRET", "SECRET2"},
						r.News.Fields["listString1"].AsInterface(), "listString1")
					AssertEqualOrJSONEncoded(l, []any{"VALUE", "SECRET"},
						r.News.Fields["listString2"].AsInterface(), "listString2")
					AssertEqualOrJSONEncoded(l, map[string]any{"key1": "value1", "key2": "SECRET"},
						r.News.Fields["mapString2"].AsInterface(), "mapString2")
					AssertEqualOrJSONEncoded(l, map[string]any{"x": "SECRET"},
						r.News.Fields["objString2"].AsInterface(), "objString2")

					// The secret versions have two options, JSON-encoded or not. Languages do not
					// agree yet on which form to use.
					c := g.ConfigureReq("config")
					assert.Equal(l, Secret("SECRET"), c.Args.Fields["string1"].AsInterface(), "string1")

					AssertEqualOrJSONEncodedSecret(l,
						Secret(float64(1234567890)),
						float64(1234567890),
						c.Args.Fields["int1"].AsInterface(), "int1")

					AssertEqualOrJSONEncodedSecret(l,
						Secret(float64(123456.789)),
						float64(123456.789),
						c.Args.Fields["num1"].AsInterface(), "num1")

					AssertEqualOrJSONEncodedSecret(l,
						Secret(true),
						true,
						c.Args.Fields["bool1"].AsInterface(), "bool1")

					AssertEqualOrJSONEncodedSecret(l,
						Secret([]any{"SECRET", "SECRET2"}),
						[]any{"SECRET", "SECRET2"},
						c.Args.Fields["listString1"].AsInterface(), "listString1")

					// Secret floating happened here, perhaps []any{"VALUE", Secret("SECRET")}
					// would be preferable instead at some point.
					AssertEqualOrJSONEncodedSecret(l,
						Secret([]any{"VALUE", "SECRET"}),
						[]any{"VALUE", "SECRET"},
						c.Args.Fields["listString2"].AsInterface(), "listString2")

					AssertEqualOrJSONEncodedSecret(l,
						map[string]any{"key1": "value1", "key2": Secret("SECRET")},
						map[string]any{"key1": "value1", "key2": "SECRET"},
						c.Args.Fields["mapString2"].AsInterface(), "mapString2")

					AssertEqualOrJSONEncodedSecret(l,
						map[string]any{"x": Secret("SECRET")},
						map[string]any{"x": "SECRET"},
						c.Args.Fields["objString2"].AsInterface(), "objString2")

					// Secretness is not exposed in GetVariables. Instead the data is JSON-encoded.
					v := c.GetVariables()
					assert.Equal(l, "SECRET", v["config-grpc:config:string1"], "string1")
					assert.Equal(l, "1234567890", v["config-grpc:config:int1"], "int1")
					assert.Equal(l, "123456.789", v["config-grpc:config:num1"], "num1")
					assert.Equal(l, "true", v["config-grpc:config:bool1"], "bool1")
					assert.JSONEq(l, "[\"SECRET\",\"SECRET2\"]", v["config-grpc:config:listString1"], "listString1")
					assert.JSONEq(l, "[\"VALUE\",\"SECRET\"]", v["config-grpc:config:listString2"], "listString2")
					assert.JSONEq(l, "{\"key1\":\"value1\",\"key2\":\"SECRET\"}", v["config-grpc:config:mapString2"], "mapString2")
					assert.JSONEq(l, "{\"x\":\"SECRET\"}", v["config-grpc:config:objString2"], "objString2")

					AssertNoSecretLeaks(l, snap, AssertNoSecretLeaksOpts{
						// ConfigFetcher is a test helper that retains secret material in its
						// state by design, and should not be part of the check.
						IgnoreResourceTypes: []tokens.Type{"config-grpc:index:ConfigFetcher"},
						Secrets:             []string{"SECRET", "SECRET2"},
					})
				},
			},
		},
	}
}
