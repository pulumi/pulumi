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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
)

func init() {
	LanguageTests["l2-provider-grpc-config"] = LanguageTest{
		Providers: []plugin.Provider{&providers.ConfigGrpcProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					g := &grpcTestContext{l: l, s: snap}

					r := g.CheckConfigReq("config")
					assert.Equal(l, "", r.News.Fields["string1"].AsInterface(), "string1")
					assert.Equal(l, "x", r.News.Fields["string2"].AsInterface(), "string2")
					assert.Equal(l, "{}", r.News.Fields["string3"].AsInterface(), "string3")
					AssertEqualOrJSONEncoded(l, float64(0), r.News.Fields["int1"].AsInterface(), "int1")
					AssertEqualOrJSONEncoded(l, float64(42), r.News.Fields["int2"].AsInterface(), "int2")
					AssertEqualOrJSONEncoded(l, float64(0), r.News.Fields["num1"].AsInterface(), "num1")
					AssertEqualOrJSONEncoded(l, float64(42.42), r.News.Fields["num2"].AsInterface(), "num2")
					AssertEqualOrJSONEncoded(l, true, r.News.Fields["bool1"].AsInterface(), "bool1")
					AssertEqualOrJSONEncoded(l, false, r.News.Fields["bool2"].AsInterface(), "bool2")
					AssertEqualOrJSONEncoded(l, []any{}, r.News.Fields["listString1"].AsInterface(), "listString1")
					AssertEqualOrJSONEncoded(l, []any{"", "foo"}, r.News.Fields["listString2"].AsInterface(), "listString2")
					AssertEqualOrJSONEncoded(l,
						[]any{float64(1), float64(2)},
						r.News.Fields["listInt1"].AsInterface(), "listInt1")

					AssertEqualOrJSONEncoded(l, map[string]any{}, r.News.Fields["mapString1"].AsInterface(), "mapString1")

					AssertEqualOrJSONEncoded(l,
						map[string]any{"key1": "value1", "key2": "value2"},
						r.News.Fields["mapString2"].AsInterface(), "mapString2")

					AssertEqualOrJSONEncoded(l,
						map[string]any{"key1": float64(0), "key2": float64(42)},
						r.News.Fields["mapInt1"].AsInterface(), "mapInt1")

					AssertEqualOrJSONEncoded(l, map[string]any{}, r.News.Fields["objString1"].AsInterface(), "objString1")

					AssertEqualOrJSONEncoded(l, map[string]any{"x": "x-value"},
						r.News.Fields["objString2"].AsInterface(), "objString2")

					AssertEqualOrJSONEncoded(l,
						map[string]any{"x": float64(42)},
						r.News.Fields["objInt1"].AsInterface(), "objInt1")

					// Check what schemaprov received in ConfigureRequest.
					c := g.ConfigureReq("config")
					assert.Equal(l, "", c.Args.Fields["string1"].AsInterface(), "string1")
					assert.Equal(l, "x", c.Args.Fields["string2"].AsInterface(), "string2")
					assert.Equal(l, "{}", c.Args.Fields["string3"].AsInterface(), "string3")
					AssertEqualOrJSONEncoded(l, float64(0), c.Args.Fields["int1"].AsInterface(), "int1")
					AssertEqualOrJSONEncoded(l, float64(42), c.Args.Fields["int2"].AsInterface(), "int2")
					AssertEqualOrJSONEncoded(l, float64(0), c.Args.Fields["num1"].AsInterface(), "num1")
					AssertEqualOrJSONEncoded(l, float64(42.42), c.Args.Fields["num2"].AsInterface(), "num2")
					AssertEqualOrJSONEncoded(l, true, c.Args.Fields["bool1"].AsInterface(), "bool1")
					AssertEqualOrJSONEncoded(l, false, c.Args.Fields["bool2"].AsInterface(), "bool2")
					AssertEqualOrJSONEncoded(l, []any{}, c.Args.Fields["listString1"].AsInterface(), "listString1")
					AssertEqualOrJSONEncoded(l, []any{"", "foo"}, c.Args.Fields["listString2"].AsInterface(), "listString2")
					AssertEqualOrJSONEncoded(l,
						[]any{float64(1), float64(2)},
						c.Args.Fields["listInt1"].AsInterface(), "listInt1")

					AssertEqualOrJSONEncoded(l, map[string]any{}, c.Args.Fields["mapString1"].AsInterface(), "mapString1")

					AssertEqualOrJSONEncoded(l,
						map[string]any{"key1": "value1", "key2": "value2"},
						c.Args.Fields["mapString2"].AsInterface(), "mapString2")

					AssertEqualOrJSONEncoded(l,
						map[string]any{"key1": float64(0), "key2": float64(42)},
						c.Args.Fields["mapInt1"].AsInterface(), "mapInt1")

					AssertEqualOrJSONEncoded(l, map[string]any{}, c.Args.Fields["objString1"].AsInterface(), "objString1")

					AssertEqualOrJSONEncoded(l, map[string]any{"x": "x-value"},
						c.Args.Fields["objString2"].AsInterface(), "objString2")

					AssertEqualOrJSONEncoded(l,
						map[string]any{"x": float64(42)},
						c.Args.Fields["objInt1"].AsInterface(), "objInt1")

					v := c.GetVariables()
					assert.Equal(l, "", v["config-grpc:config:string1"], "string1")
					assert.Equal(l, "x", v["config-grpc:config:string2"], "string2")
					assert.Equal(l, "{}", v["config-grpc:config:string3"], "string3")
					assert.Equal(l, "0", v["config-grpc:config:int1"], "int1")
					assert.Equal(l, "42", v["config-grpc:config:int2"], "int2")
					assert.Equal(l, "0", v["config-grpc:config:num1"], "num1")
					assert.Equal(l, "42.42", v["config-grpc:config:num2"], "num2")
					assert.Equal(l, "true", v["config-grpc:config:bool1"], "bool1")
					assert.Equal(l, "false", v["config-grpc:config:bool2"], "bool2")
					assert.JSONEq(l, "[]", v["config-grpc:config:listString1"], "listString1")
					assert.JSONEq(l, "[\"\",\"foo\"]", v["config-grpc:config:listString2"], "listString2")
					assert.JSONEq(l, "[1,2]", v["config-grpc:config:listInt1"], "listInt1")
					assert.JSONEq(l, "{}", v["config-grpc:config:mapString1"], "mapString1")
					assert.JSONEq(l, "{\"key1\":\"value1\",\"key2\":\"value2\"}", v["config-grpc:config:mapString2"], "mapString2")
					assert.JSONEq(l, "{\"key1\":0,\"key2\":42}", v["config-grpc:config:mapInt1"], "mapInt1")
					assert.JSONEq(l, "{}", v["config-grpc:config:objString1"], "objString1")
					assert.JSONEq(l, "{\"x\":\"x-value\"}", v["config-grpc:config:objString2"], "objString2")
					assert.JSONEq(l, "{\"x\":42}", v["config-grpc:config:objInt1"], "objInt1")

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
