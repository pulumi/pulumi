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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

const testPack = `
globalThis.__pulumiPolicyPack = {
  abiVersion: 1,
  name: "test-pack",
  version: "0.0.1",
  policies: [
    {
      name: "bucket-versioning",
      description: "Buckets must be versioned.",
      severity: "high",
      enforcementLevel: "mandatory",
      kind: "resource",
      resourceTypes: ["aws:s3/bucket:Bucket"],
      validate: function (input, ctx) {
        if (!input.props.versioning || !input.props.versioning.enabled) {
          ctx.reportViolation("versioning is off for " + input.name);
        }
      },
    },
    {
      name: "name-length",
      description: "Names must be short.",
      configSchema: { properties: { max: { type: "integer", default: 5 } }, required: ["max"] },
      kind: "resource",
      validate: function (input, ctx) {
        var max = input.config.max === undefined ? 100 : input.config.max;
        if (input.name.length > max) ctx.reportViolation("too long");
      },
    },
    {
      name: "only-buckets",
      description: "NA test",
      kind: "resource",
      validate: function (input, ctx) {
        if (input.type !== "aws:s3/bucket:Bucket") ctx.notApplicable("not a bucket");
      },
    },
    {
      name: "reads-arn",
      description: "Reads an unknown.",
      kind: "resource",
      resourceTypes: ["aws:s3/bucket:Bucket"],
      validate: function (input, ctx) {
        ctx.reportViolation("before");
        if (input.props.arn.indexOf("x") >= 0) ctx.reportViolation("after");
      },
    },
    {
      name: "values",
      description: "Value conversion.",
      kind: "resource",
      resourceTypes: ["aws:ec2/instance:Instance"],
      validate: function (input, ctx) {
        var p = input.props;
        var ud = Buffer.from(p.userDataBase64, "base64").toString("utf8");
        if (ud.indexOf("AKIA") >= 0) ctx.reportViolation("secret in user data");
        if (atob(btoa("hi")) !== "hi") ctx.reportViolation("atob broken");
        var u = new URL(p.repo);
        if (u.hostname !== "github.com" || u.protocol !== "https:") ctx.reportViolation("bad url " + u.hostname);
        if (!/(?<!a)b/.test("cb")) ctx.reportViolation("lookbehind broken");
        if (p.password !== "hunter2") ctx.reportViolation("secret not unwrapped");
        if (p.subnet !== "subnet-123") ctx.reportViolation("resource reference not its id");
        if (p.empty !== null) ctx.reportViolation("null not kept");
        if ("__defaults" in p) ctx.reportViolation("internal key kept");
        if (p.ids.length !== 2 || p.ids[0] !== "a") ctx.reportViolation("array with unknown: " + p.ids.length);
        if (Object.keys(p).join(",") !== "empty,ids,password,repo,subnet,userDataBase64") {
          ctx.reportViolation("keys " + Object.keys(p).join(","));
        }
        console.warn("hello", { a: 1 });
        p.ids[1];
      },
    },
    {
      name: "object-violation",
      description: "Object form.",
      kind: "resource",
      resourceTypes: ["aws:ec2/instance:Instance"],
      validate: function (input, ctx) {
        ctx.reportViolation({ message: "custom", severity: "low", urn: "urn:custom" });
      },
    },
    {
      name: "stack-deps",
      description: "Stack policy.",
      kind: "stack",
      validate: function (input, ctx) {
        input.resources.forEach(function (r) {
          if (r.parent) {
            ctx.reportViolation(r.name + " parent=" + r.parent + " deps=" + r.dependencies.join(",") +
              " bucketDeps=" + (r.propertyDependencies.bucket || []).join(","), r.urn);
          }
        });
      },
    },
    {
      name: "stack-typed",
      description: "Only when there are queues.",
      kind: "stack",
      resourceTypes: ["aws:sqs/queue:Queue"],
      validate: function (input, ctx) { ctx.reportViolation("queues!"); },
    },
    {
      name: "throws",
      description: "Throws.",
      kind: "resource",
      resourceTypes: ["test:index:Boom"],
      validate: function () { throw new Error("boom"); },
    },
  ],
};
`

const unknown = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"

func newTestAnalyzer(t testing.TB, src string) *analyzer {
	pk, err := loadTestPack(t, src)
	require.NoError(t, err)
	return &analyzer{pack: pk}
}

func loadTestPack(t testing.TB, src string) (*pack, error) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "policies.js"), []byte(src), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PulumiPolicy.yaml"),
		[]byte("runtime: purejs\nversion: 1.2.3\ndescription: yaml description\n"), 0o600))
	return loadPack(dir)
}

func props(t testing.TB, m map[string]any) *structpb.Struct {
	s, err := structpb.NewStruct(m)
	require.NoError(t, err)
	return s
}

func bucket(t testing.TB, name string, m map[string]any) *pulumirpc.AnalyzeRequest {
	return &pulumirpc.AnalyzeRequest{
		Urn:        "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::b",
		Type:       "aws:s3/bucket:Bucket",
		Name:       name,
		Properties: props(t, m),
	}
}

func byPolicy(resp *pulumirpc.AnalyzeResponse) (map[string][]*pulumirpc.AnalyzeDiagnostic, map[string]string) {
	diags := map[string][]*pulumirpc.AnalyzeDiagnostic{}
	for _, d := range resp.GetDiagnostics() {
		diags[d.GetPolicyName()] = append(diags[d.GetPolicyName()], d)
	}
	na := map[string]string{}
	for _, n := range resp.GetNotApplicable() {
		na[n.GetPolicyName()] = n.GetReason()
	}
	return diags, na
}

func TestAnalyzerInfo(t *testing.T) {
	t.Parallel()
	a := newTestAnalyzer(t, testPack)
	info, err := a.GetAnalyzerInfo(t.Context(), &emptypb.Empty{})
	require.NoError(t, err)
	assert.Equal(t, "test-pack", info.GetName())
	assert.Equal(t, "1.2.3", info.GetVersion(), "PulumiPolicy.yaml version wins")
	assert.Equal(t, "yaml description", info.GetDescription())
	assert.True(t, info.GetSupportsConfig())
	require.Len(t, info.GetPolicies(), 9)
	first := info.GetPolicies()[0]
	assert.Equal(t, "bucket-versioning", first.GetName(), "sorted by name")
	assert.Equal(t, pulumirpc.EnforcementLevel_MANDATORY, first.GetEnforcementLevel())
	assert.Equal(t, pulumirpc.PolicySeverity_POLICY_SEVERITY_HIGH, first.GetSeverity())
	assert.Contains(t, first.GetConfigSchema().GetProperties().AsMap(), "enforcementLevel")
	for _, p := range info.GetPolicies() {
		if p.GetName() == "name-length" {
			assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, p.GetEnforcementLevel(), "defaults to advisory")
			assert.Equal(t, map[string]any{"type": "integer", "default": 5.0},
				p.GetConfigSchema().GetProperties().AsMap()["max"])
			assert.Equal(t, []string{"max"}, p.GetConfigSchema().GetRequired())
		}
		if strings.HasPrefix(p.GetName(), "stack-") {
			assert.Equal(t, pulumirpc.PolicyType_POLICY_TYPE_STACK, p.GetPolicyType())
		} else {
			assert.Equal(t, pulumirpc.PolicyType_POLICY_TYPE_RESOURCE, p.GetPolicyType())
		}
	}
}

func TestAnalyze(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	a := newTestAnalyzer(t, testPack)

	resp, err := a.Analyze(ctx, bucket(t, "my-bucket", map[string]any{"arn": unknown}))
	require.NoError(t, err)
	diags, na := byPolicy(resp)

	require.Len(t, diags["bucket-versioning"], 1)
	d := diags["bucket-versioning"][0]
	assert.Equal(t, "Buckets must be versioned.\nversioning is off for my-bucket", d.GetMessage())
	assert.Equal(t, pulumirpc.EnforcementLevel_MANDATORY, d.GetEnforcementLevel())
	assert.Equal(t, "test-pack", d.GetPolicyPackName())
	assert.Equal(t, "1.2.3", d.GetPolicyPackVersion())
	assert.Equal(t, pulumirpc.PolicySeverity_POLICY_SEVERITY_HIGH, d.GetSeverity())

	// name-length has no config yet, so the fallback max (100) applies.
	assert.Empty(t, diags["name-length"])

	// Reading the unknown keeps the earlier violation and adds an advisory "can't run" diagnostic.
	require.Len(t, diags["reads-arn"], 2)
	assert.Equal(t, "Reads an unknown.\nbefore", diags["reads-arn"][0].GetMessage())
	assert.Equal(t,
		"can't run policy 'reads-arn' from policy pack 'test-pack@v1.2.3' during preview: "+
			"string value at .arn can't be known during preview",
		diags["reads-arn"][1].GetMessage())
	assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, diags["reads-arn"][1].GetEnforcementLevel())

	assert.Equal(t, "Policy only applies to resources of type 'aws:ec2/instance:Instance'", na["values"])
	assert.NotContains(t, na, "only-buckets")

	resp, err = a.Analyze(ctx, &pulumirpc.AnalyzeRequest{Urn: "urn:x", Type: "aws:sqs/queue:Queue", Name: "q"})
	require.NoError(t, err)
	_, na = byPolicy(resp)
	assert.Equal(t, "not a bucket", na["only-buckets"])
}

func TestConfigure(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	a := newTestAnalyzer(t, testPack)
	_, err := a.Configure(ctx, &pulumirpc.ConfigureAnalyzerRequest{PolicyConfig: map[string]*pulumirpc.PolicyConfig{
		"bucket-versioning": {EnforcementLevel: pulumirpc.EnforcementLevel_REMEDIATE},
		"name-length": {
			EnforcementLevel: pulumirpc.EnforcementLevel_MANDATORY,
			Properties:       props(t, map[string]any{"max": 3}),
		},
		"reads-arn": {EnforcementLevel: pulumirpc.EnforcementLevel_DISABLED},
	}})
	require.NoError(t, err)

	resp, err := a.Analyze(ctx, bucket(t, "long-name", map[string]any{
		"arn":        "x",
		"versioning": map[string]any{"enabled": true},
	}))
	require.NoError(t, err)
	diags, _ := byPolicy(resp)
	assert.NotContains(t, diags, "reads-arn", "disabled policies don't run")
	assert.NotContains(t, diags, "bucket-versioning")
	require.Contains(t, diags, "name-length")
	assert.Equal(t, pulumirpc.EnforcementLevel_MANDATORY, diags["name-length"][0].GetEnforcementLevel())

	resp, err = a.Analyze(ctx, bucket(t, "b", map[string]any{}))
	require.NoError(t, err)
	require.Len(t, resp.GetDiagnostics(), 1)
	assert.Equal(t, pulumirpc.EnforcementLevel_MANDATORY, resp.GetDiagnostics()[0].GetEnforcementLevel(),
		"remediate is reported as mandatory")

	info, err := a.GetAnalyzerInfo(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	for _, p := range info.GetPolicies() {
		if p.GetName() == "reads-arn" {
			assert.Equal(t, pulumirpc.EnforcementLevel_DISABLED, p.GetEnforcementLevel())
		}
	}
}

func TestValuesAndObjectViolations(t *testing.T) {
	t.Parallel()
	secret := map[string]any{sigKey: secretSig, "value": "hunter2"}
	ref := map[string]any{
		sigKey: resourceReference, "urn": "urn:pulumi:dev::proj::aws:ec2/subnet:Subnet::s", "id": "subnet-123",
	}
	a := newTestAnalyzer(t, testPack)
	resp, err := a.Analyze(t.Context(), &pulumirpc.AnalyzeRequest{
		Urn:  "urn:pulumi:dev::proj::aws:ec2/instance:Instance::i",
		Type: "aws:ec2/instance:Instance",
		Name: "i",
		Properties: props(t, map[string]any{
			"userDataBase64": "ZXhwb3J0IEtFWT1BS0lBMTIz", // "export KEY=AKIA123"
			"repo":           "https://github.com/pulumi/pulumi.git",
			"password":       secret,
			"subnet":         ref,
			"empty":          nil,
			"ids":            []any{"a", unknown},
			"__defaults":     []any{},
		}),
	})
	require.NoError(t, err)
	msgs := make([]string, 0, len(resp.GetDiagnostics()))
	for _, d := range resp.GetDiagnostics() {
		msgs = append(msgs, d.GetPolicyName()+": "+d.GetMessage())
		if d.GetPolicyName() == "object-violation" {
			assert.Equal(t, pulumirpc.PolicySeverity_POLICY_SEVERITY_LOW, d.GetSeverity())
			assert.Equal(t, "urn:custom", d.GetUrn())
		}
	}
	assert.ElementsMatch(t, []string{
		"values: Value conversion.\nsecret in user data",
		"values: can't run policy 'values' from policy pack 'test-pack@v1.2.3' during preview: " +
			"string value at .ids.1 can't be known during preview",
		"object-violation: custom",
	}, msgs)
}

func TestAnalyzeStack(t *testing.T) {
	t.Parallel()
	a := newTestAnalyzer(t, testPack)
	resp, err := a.AnalyzeStack(t.Context(), &pulumirpc.AnalyzeStackRequest{Resources: []*pulumirpc.AnalyzerResource{
		{Urn: "urn:p", Type: "pkg:index:Comp", Name: "p"},
		{
			Urn: "urn:c", Type: "aws:s3/bucket:Bucket", Name: "c", Parent: "urn:p", Dependencies: []string{"urn:p"},
			PropertyDependencies: map[string]*pulumirpc.AnalyzerPropertyDependencies{"bucket": {Urns: []string{"urn:p"}}},
		},
	}})
	require.NoError(t, err)
	require.Len(t, resp.GetDiagnostics(), 1)
	assert.Equal(t, "Stack policy.\nc parent=urn:p deps=urn:p bucketDeps=urn:p", resp.GetDiagnostics()[0].GetMessage())
	assert.Equal(t, "urn:c", resp.GetDiagnostics()[0].GetUrn())
	require.Len(t, resp.GetNotApplicable(), 1)
	assert.Equal(t, "stack-typed", resp.GetNotApplicable()[0].GetPolicyName())
}

func TestErrors(t *testing.T) {
	t.Parallel()
	a := newTestAnalyzer(t, testPack)
	_, err := a.Analyze(t.Context(), &pulumirpc.AnalyzeRequest{Urn: "urn:x", Type: "test:index:Boom", Name: "x"})
	require.ErrorContains(t, err, "Error validating resource with policy 'throws' from policy pack 'test-pack@v1.2.3'")
	require.ErrorContains(t, err, "boom")

	_, err = loadTestPack(t, `var x = 1;`)
	require.ErrorContains(t, err, "did not set globalThis.__pulumiPolicyPack")

	_, err = loadTestPack(t, `globalThis.__pulumiPolicyPack = {abiVersion: 2, name: "v2", policies: []};`)
	require.ErrorContains(t, err, "uses ABI version 2")
}

func TestConcurrentAnalyze(t *testing.T) {
	t.Parallel()
	a := newTestAnalyzer(t, testPack)
	reqs := make([]*pulumirpc.AnalyzeRequest, 8)
	for i := range reqs {
		reqs[i] = bucket(t, fmt.Sprintf("b%d", i), map[string]any{"arn": "arn:x"})
	}
	var wg sync.WaitGroup
	for _, req := range reqs {
		wg.Go(func() {
			for range 20 {
				_, err := a.Analyze(t.Context(), req)
				assert.NoError(t, err) //nolint:forbidigo // require must not be called from a goroutine
			}
		})
	}
	wg.Wait()
}
