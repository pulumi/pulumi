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
	"embed"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lorem is a long string used for testing large string values.
const lorem string = "Lorem ipsum dolor sit amet, consectetur adipiscing elit," +
	" sed do eiusmod tempor incididunt ut labore et dolore magna aliqua." +
	" Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat." +
	" Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur." +
	" Excepteur sint occaecat cupidatat non proident," +
	" sunt in culpa qui officia deserunt mollit anim id est laborum."

//go:embed testdata
var LanguageTestdata embed.FS

var LanguageTests = map[string]LanguageTest{
	// ==========
	// L2 (Tests using providers)
	// ==========
	"l2-explicit-provider": {
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:simple", provider.Type.String(), "expected simple provider")
					assert.Equal(l, "prov", provider.URN.Name(), "expected explicit provider resource")

					simple := snap.Resources[2]
					assert.Equal(l, "simple:index:Resource", simple.Type.String(), "expected simple resource")
					assert.Equal(l, string(provider.URN)+"::"+string(provider.ID), simple.Provider)

					want := resource.NewPropertyMapFromMap(map[string]any{"value": true})
					assert.Equal(l, want, simple.Inputs, "expected inputs to be {value: true}")
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	},
	"l2-engine-update-options": {
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []TestRun{
			{
				UpdateOptions: engine.UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{
						"**target**",
					}),
				},
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 3, "expected 2 resource in snapshot")

					// Check that we have the target in the snapshot, but not the other resource.
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")
					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:simple", provider.Type.String(), "expected simple provider")
					target := snap.Resources[2]
					require.Equal(l, "simple:index:Resource", target.Type.String(), "expected simple resource")
					require.Equal(l, "target", target.URN.Name(), "expected target resource")
				},
			},
		},
	},
	"l2-destroy": {
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					// check that both expected resources are in the snapshot
					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:simple", provider.Type.String(), "expected simple provider")

					// Make sure we can assert the resource names in a consistent order
					sort.Slice(snap.Resources[2:4], func(i, j int) bool {
						i = i + 2
						j = j + 2
						return snap.Resources[i].URN.Name() < snap.Resources[j].URN.Name()
					})

					simple := snap.Resources[2]
					assert.Equal(l, "simple:index:Resource", simple.Type.String(), "expected simple resource")
					assert.Equal(l, "aresource", simple.URN.Name(), "expected aresource resource")
					simple2 := snap.Resources[3]
					assert.Equal(l, "simple:index:Resource", simple2.Type.String(), "expected simple resource")
					assert.Equal(l, "other", simple2.URN.Name(), "expected other resource")
				},
			},
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					assert.Equal(l, 1, changes[deploy.OpDelete], "expected a delete operation")
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					// No need to sort here, since we have only resources that depend on each other in a chain.
					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:simple", provider.Type.String(), "expected simple provider")
					// check that only the expected resource is left in the snapshot
					simple := snap.Resources[2]
					assert.Equal(l, "simple:index:Resource", simple.Type.String(), "expected simple resource")
					assert.Equal(l, "aresource", simple.URN.Name(), "expected aresource resource")
				},
			},
		},
	},
	"l2-target-up-with-new-dependency": {
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")
					err = snap.VerifyIntegrity()
					require.NoError(l, err, "expected snapshot to be valid")

					sort.Slice(snap.Resources, func(i, j int) bool {
						return snap.Resources[i].URN.Name() < snap.Resources[j].URN.Name()
					})

					target := snap.Resources[2]
					require.Equal(l, "simple:index:Resource", target.Type.String(), "expected simple resource")
					require.Equal(l, "targetOnly", target.URN.Name(), "expected target resource")
					unrelated := snap.Resources[3]
					require.Equal(l, "simple:index:Resource", unrelated.Type.String(), "expected simple resource")
					require.Equal(l, "unrelated", unrelated.URN.Name(), "expected target resource")
					require.Equal(l, 0, len(unrelated.Dependencies), "expected no dependencies")
				},
			},
			{
				UpdateOptions: engine.UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{
						"**targetOnly**",
					}),
				},
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					sort.Slice(snap.Resources, func(i, j int) bool {
						return snap.Resources[i].URN.Name() < snap.Resources[j].URN.Name()
					})

					target := snap.Resources[2]
					require.Equal(l, "simple:index:Resource", target.Type.String(), "expected simple resource")
					require.Equal(l, "targetOnly", target.URN.Name(), "expected target resource")
					unrelated := snap.Resources[3]
					require.Equal(l, "simple:index:Resource", unrelated.Type.String(), "expected simple resource")
					require.Equal(l, "unrelated", unrelated.URN.Name(), "expected target resource")
					require.Equal(l, 0, len(unrelated.Dependencies), "expected still no dependencies")
				},
			},
		},
	},
	"l2-failed-create-continue-on-error": {
		Providers: []plugin.Provider{&providers.SimpleProvider{}, &providers.FailOnCreateProvider{}},
		Runs: []TestRun{
			{
				UpdateOptions: engine.UpdateOptions{
					ContinueOnError: true,
				},
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					require.True(l, result.IsBail(err), "expected a bail result")
					require.Equal(l, 1, len(changes), "expected 1 StepOp")
					require.Equal(l, 2, changes[deploy.OpCreate], "expected 2 Creates")
					require.NotNil(l, snap, "expected snapshot to be non-nil")
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot") // 1 stack, 2 providers, 1 resource
					require.NoError(l, snap.VerifyIntegrity(), "expected snapshot to be valid")

					sort.Slice(snap.Resources, func(i, j int) bool {
						return snap.Resources[i].URN.Name() < snap.Resources[j].URN.Name()
					})

					require.Equal(l, "independent", snap.Resources[2].URN.Name(), "expected independent resource")
				},
			},
		},
	},
	"l2-large-string": {
		Providers: []plugin.Provider{&providers.LargeProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					// Check that the large string is in the snapshot
					largeString := resource.NewStringProperty(strings.Repeat("hello world", 9532509))
					large := snap.Resources[2]
					require.Equal(l, "large:index:String", large.Type.String(), "expected large string resource")
					require.Equal(l,
						resource.NewStringProperty("hello world"),
						large.Inputs["value"],
					)
					require.Equal(l,
						largeString,
						large.Outputs["value"],
					)

					// Check the stack output value is as well
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")
					require.Equal(l, largeString, stack.Outputs["output"], "expected large string stack output")
				},
			},
		},
	},
	"l2-provider-grpc-config": {
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
	},
	// Looks like in the test setup, proper partitioning of provider space is not yet working and Configure calls
	// race with Create calls when talking to a provider. It makes it too difficult to test more than one explicit
	// provider per test case. To compensate, more test cases are added.
	"l2-provider-grpc-config-secret": {
		// Check what schemaprov received in CheckRequest.
		Providers: []plugin.Provider{&providers.ConfigGrpcProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
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
	},
	// This test checks how SDKs propagate properties marked as secret to the provider Configure on the gRPC level.
	"l2-provider-grpc-config-schema-secret": {
		Providers: []plugin.Provider{&providers.ConfigGrpcProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
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
	},
	"l2-primitive-ref": {
		Providers: []plugin.Provider{&providers.PrimitiveRefProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:primitive-ref", provider.Type.String(), "expected primitive-ref provider")

					simple := snap.Resources[2]
					assert.Equal(l, "primitive-ref:index:Resource", simple.Type.String(), "expected primitive-ref resource")

					want := resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"boolean":   false,
							"float":     2.17,
							"integer":   -12,
							"string":    "Goodbye",
							"boolArray": []interface{}{false, true},
							"stringMap": map[string]interface{}{
								"two":   "turtle doves",
								"three": "french hens",
							},
						}),
					})
					assert.Equal(l, want, simple.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	},
	"l2-ref-ref": {
		Providers: []plugin.Provider{&providers.RefRefProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:ref-ref", provider.Type.String(), "expected ref-ref provider")

					simple := snap.Resources[2]
					assert.Equal(l, "ref-ref:index:Resource", simple.Type.String(), "expected ref-ref resource")

					want := resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"innerData": resource.NewPropertyMapFromMap(map[string]any{
								"boolean":   false,
								"float":     2.17,
								"integer":   -12,
								"string":    "Goodbye",
								"boolArray": []interface{}{false, true},
								"stringMap": map[string]interface{}{
									"two":   "turtle doves",
									"three": "french hens",
								},
							}),
							"boolean":   true,
							"float":     4.5,
							"integer":   1024,
							"string":    "Hello",
							"boolArray": []interface{}{},
							"stringMap": map[string]interface{}{
								"x": "100",
								"y": "200",
							},
						}),
					})
					assert.Equal(l, want, simple.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	},
	"l2-plain": {
		Providers: []plugin.Provider{&providers.PlainProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:plain", provider.Type.String(), "expected plain provider")

					plain := snap.Resources[2]
					assert.Equal(l, "plain:index:Resource", plain.Type.String(), "expected plain resource")

					want := resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"innerData": resource.NewPropertyMapFromMap(map[string]any{
								"boolean":   false,
								"float":     2.17,
								"integer":   -12,
								"string":    "Goodbye",
								"boolArray": []interface{}{false, true},
								"stringMap": map[string]interface{}{
									"two":   "turtle doves",
									"three": "french hens",
								},
							}),
							"boolean":   true,
							"float":     4.5,
							"integer":   1024,
							"string":    "Hello",
							"boolArray": []interface{}{true, false},
							"stringMap": map[string]interface{}{
								"x": "100",
								"y": "200",
							},
						}),
					})
					assert.Equal(l, want, plain.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, plain.Inputs, plain.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	},
	"l2-parameterized-resource": {
		Providers: []plugin.Provider{&providers.ParameterizedProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")
					require.Equal(l,
						resource.NewStringProperty("HelloWorld"),
						stack.Outputs["parameterValue"],
						"parameter value should be correct")
				},
			},
		},
	},
	"l2-map-keys": {
		Providers: []plugin.Provider{
			&providers.PrimitiveProvider{}, &providers.PrimitiveRefProvider{},
			&providers.RefRefProvider{}, &providers.PlainProvider{},
		},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 9, "expected 9 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:primitive")
					primResource := RequireSingleResource(l, snap.Resources, "primitive:index:Resource")
					RequireSingleResource(l, snap.Resources, "pulumi:providers:primitive-ref")
					refResource := RequireSingleResource(l, snap.Resources, "primitive-ref:index:Resource")
					RequireSingleResource(l, snap.Resources, "pulumi:providers:ref-ref")
					rrefResource := RequireSingleResource(l, snap.Resources, "ref-ref:index:Resource")
					RequireSingleResource(l, snap.Resources, "pulumi:providers:plain")
					plainResource := RequireSingleResource(l, snap.Resources, "plain:index:Resource")

					want := resource.NewPropertyMapFromMap(map[string]any{
						"boolean":     false,
						"float":       2.17,
						"integer":     -12,
						"string":      "Goodbye",
						"numberArray": []interface{}{0, 1},
						"booleanMap": map[string]interface{}{
							"my key": false,
							"my.key": true,
							"my-key": false,
							"my_key": true,
							"MY_KEY": false,
							"myKey":  true,
						},
					})
					assert.Equal(l, want, primResource.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, primResource.Inputs, primResource.Outputs, "expected inputs and outputs to match")

					want = resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"boolean":   false,
							"float":     2.17,
							"integer":   -12,
							"string":    "Goodbye",
							"boolArray": []interface{}{false, true},
							"stringMap": map[string]interface{}{
								"my key": "one",
								"my.key": "two",
								"my-key": "three",
								"my_key": "four",
								"MY_KEY": "five",
								"myKey":  "six",
							},
						}),
					})
					assert.Equal(l, want, refResource.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, refResource.Inputs, refResource.Outputs, "expected inputs and outputs to match")

					want = resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"innerData": resource.NewPropertyMapFromMap(map[string]any{
								"boolean":   false,
								"float":     -2.17,
								"integer":   123,
								"string":    "Goodbye",
								"boolArray": []interface{}{},
								"stringMap": map[string]interface{}{
									"my key": "one",
									"my.key": "two",
									"my-key": "three",
									"my_key": "four",
									"MY_KEY": "five",
									"myKey":  "six",
								},
							}),
							"boolean":   true,
							"float":     4.5,
							"integer":   1024,
							"string":    "Hello",
							"boolArray": []interface{}{},
							"stringMap": map[string]interface{}{
								"my key": "one",
								"my.key": "two",
								"my-key": "three",
								"my_key": "four",
								"MY_KEY": "five",
								"myKey":  "six",
							},
						}),
					})
					assert.Equal(l, want, rrefResource.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, rrefResource.Inputs, rrefResource.Outputs, "expected inputs and outputs to match")

					want = resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"innerData": resource.NewPropertyMapFromMap(map[string]any{
								"boolean":   false,
								"float":     2.17,
								"integer":   -12,
								"string":    "Goodbye",
								"boolArray": []interface{}{false, true},
								"stringMap": map[string]interface{}{
									"my key": "one",
									"my.key": "two",
									"my-key": "three",
									"my_key": "four",
									"MY_KEY": "five",
									"myKey":  "six",
								},
							}),
							"boolean":   true,
							"float":     4.5,
							"integer":   1024,
							"string":    "Hello",
							"boolArray": []interface{}{true, false},
							"stringMap": map[string]interface{}{
								"my key": "one",
								"my.key": "two",
								"my-key": "three",
								"my_key": "four",
								"MY_KEY": "five",
								"myKey":  "six",
							},
						}),
						"nonPlainData": resource.NewPropertyMapFromMap(map[string]any{
							"innerData": resource.NewPropertyMapFromMap(map[string]any{
								"boolean":   false,
								"float":     2.17,
								"integer":   -12,
								"string":    "Goodbye",
								"boolArray": []interface{}{false, true},
								"stringMap": map[string]interface{}{
									"my key": "one",
									"my.key": "two",
									"my-key": "three",
									"my_key": "four",
									"MY_KEY": "five",
									"myKey":  "six",
								},
							}),
							"boolean":   true,
							"float":     4.5,
							"integer":   1024,
							"string":    "Hello",
							"boolArray": []interface{}{true, false},
							"stringMap": map[string]interface{}{
								"my key": "one",
								"my.key": "two",
								"my-key": "three",
								"my_key": "four",
								"MY_KEY": "five",
								"myKey":  "six",
							},
						}),
					})
					assert.Equal(l, want, plainResource.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, plainResource.Inputs, plainResource.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	},
	"l2-explicit-parameterized-provider": {
		Providers: []plugin.Provider{&providers.ParameterizedProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					// Check we have the one resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")
					require.Equal(l,
						resource.NewStringProperty("Goodbye World"),
						stack.Outputs["parameterValue"],
						"parameter value and provider config should be correct")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:goodbye", provider.Type.String(), "expected goodbye provider")
					assert.Equal(l, "prov", provider.URN.Name(), "expected explicit provider resource")

					simple := snap.Resources[2]
					assert.Equal(l, "goodbye:index:Goodbye", simple.Type.String(), "expected Goodbye resource")
					assert.Equal(l, string(provider.URN)+"::"+string(provider.ID), simple.Provider)
				},
			},
		},
	},
}
