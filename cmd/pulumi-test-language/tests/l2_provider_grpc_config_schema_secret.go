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
	LanguageTests["l2-provider-grpc-config-schema-secret"] = LanguageTest{
		// This test checks how SDKs propagate properties marked as secret to the provider Configure on the gRPC level.
		Providers: []plugin.Provider{&providers.ConfigGrpcProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					g := &grpcTestContext{l: l, s: snap}

					// Verify the CheckConfig request received by the provider.
					r := g.CheckConfigReq("config")

					// TODO[pulumi/pulumi#16876]: CheckConfig request gets the secrets in the plain.
					// This is suspect, probably has to do with secret negotiation happening later
					// in the gRPC provider cycle.
					assert.Equal(l, "SECRET",
						r.News.Fields["secretString1"].AsInterface(), "secretString1")

					AssertEqualOrJSONEncoded(l, float64(16),
						r.News.Fields["secretInt1"].AsInterface(), "secretInt1")

					AssertEqualOrJSONEncoded(l, float64(123456.7890),
						r.News.Fields["secretNum1"].AsInterface(), "secretNum1")

					AssertEqualOrJSONEncoded(l, true,
						r.News.Fields["secretBool1"].AsInterface(), "secretBool1")

					AssertEqualOrJSONEncoded(l, []any{"SECRET", "SECRET2"},
						r.News.Fields["listSecretString1"].AsInterface(), "listSecretString1")

					AssertEqualOrJSONEncoded(l, map[string]any{"key1": "SECRET", "key2": "SECRET2"},
						r.News.Fields["mapSecretString1"].AsInterface(), "mapSecretString1")

					// Now verify the Configure request.
					c := g.ConfigureReq("config")

					// All the fields are coming in as secret-wrapped fields into Configure.
					assert.Equal(l, Secret("SECRET"),
						c.Args.Fields["secretString1"].AsInterface(), "secretString1")

					AssertEqualOrJSONEncodedSecret(l,
						Secret(float64(16)), float64(16),
						c.Args.Fields["secretInt1"].AsInterface(), "secretInt1")

					AssertEqualOrJSONEncodedSecret(l,
						Secret(float64(123456.7890)), float64(123456.7890),
						c.Args.Fields["secretNum1"].AsInterface(), "secretNum1")

					AssertEqualOrJSONEncodedSecret(l,
						Secret(true), true,
						c.Args.Fields["secretBool1"].AsInterface(), "secretBool1")

					AssertEqualOrJSONEncodedSecret(l,
						Secret([]any{"SECRET", "SECRET2"}),
						[]any{"SECRET", "SECRET2"},
						c.Args.Fields["listSecretString1"].AsInterface(), "listSecretString1")

					AssertEqualOrJSONEncodedSecret(l,
						Secret(map[string]any{"key1": "SECRET", "key2": "SECRET2"}),
						map[string]any{"key1": "SECRET", "key2": "SECRET2"},
						c.Args.Fields["mapSecretString1"].AsInterface(), "mapSecretString1")

					// Secretness is not exposed in GetVariables. Instead the data is JSON-encoded.
					v := c.GetVariables()
					assert.Equal(l, "SECRET", v["config-grpc:config:secretString1"], "secretString1")
					assert.JSONEq(l, "16", v["config-grpc:config:secretInt1"], "secretInt1")
					assert.JSONEq(l, "123456.7890", v["config-grpc:config:secretNum1"], "secretNum1")
					assert.JSONEq(l, "true", v["config-grpc:config:secretBool1"], "secretBool1")
					assert.JSONEq(l, `["SECRET", "SECRET2"]`, v["config-grpc:config:listSecretString1"], "listSecretString1")

					assert.JSONEq(l, `{"key1":"SECRET","key2":"SECRET2"}`,
						v["config-grpc:config:mapSecretString1"], "mapSecretString1")

					// TODO[pulumi/pulumi#17652] Languages do not agree on the object property
					// casing sent to CheckConfig, Node and Go send "secretX", Python sends
					// "secret_x" though.
					//
					// AssertEqualOrJSONEncoded(l, map[string]any{"secretX": "SECRET"},
					// 	r.News.Fields["objSecretString1"].AsInterface(), "objSecretString1")
					// AssertEqualOrJSONEncodedSecret(l,
					//      map[string]any{"secretX": Secret("SECRET")}, map[string]any{"secretX": "SECRET"},
					// 	r.Args.Fields["objSecretString1"].AsInterface(), "objSecretString1")
					// assert.JSONEq(l, `{"secretX":"SECRET"}`,
					// 	v["config-grpc:config:objectSecretString1"], "objSecretString1")

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
