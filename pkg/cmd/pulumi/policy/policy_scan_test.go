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

package policy

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func runScanCmd(t *testing.T, analyzers []plugin.Analyzer, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := newPolicyScanCmd(stubLoadAnalyzers(analyzers))
	cmd.SilenceUsage = true
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(t.Context())
	return stdout.String(), stderr.String(), err
}

func TestPolicyScanCmd_RequiresPolicyPack(t *testing.T) {
	t.Parallel()
	_, _, err := runScanCmd(t, nil, t.TempDir())
	assert.ErrorContains(t, err, "at least one --policy-pack")
}

func TestPolicyScanCmd_OnlyNodeJS(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: p\nruntime: python\n"), 0o600))
	_, _, err := runScanCmd(t, nil, dir, "--policy-pack", "./pack")
	assert.ErrorContains(t, err, `supports only Node.js programs`)
}

func TestWriteScanConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.dev.yaml"), []byte(`config:
  p:fromStack: stack
  p:password:
    secure: AAABAKd
  aws:region: us-east-1
  p:unset: null
`), 0o600))
	proj := &workspace.Project{
		Name: "p",
		Config: map[string]workspace.ProjectConfigType{
			"fromProject": {Default: "project"},
			"fromStack":   {Default: "project"},
			"object":      {Default: map[string]any{"a": 1}},
		},
	}
	fixtures := scanFixtures{
		Config:  map[string]any{"fixture": "fixture", "aws:region": "eu-west-1"},
		Invokes: map[string]any{"getAccount": map[string]any{"id": "1"}},
	}

	path, err := writeScanConfig(dir, proj, dir, "dev", []string{"fromProject=flag"}, fixtures)
	require.NoError(t, err)
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	var got struct {
		Values     map[string]string `json:"values"`
		SecretKeys []string          `json:"secretKeys"`
		Fixtures   map[string]any    `json:"fixtures"`
	}
	require.NoError(t, json.Unmarshal(b, &got))

	assert.Equal(t, map[string]string{
		"p:fromProject": "flag",      // --config wins
		"p:fromStack":   "stack",     // the stack file beats the project's default
		"p:object":      `{"a":1}`,   // objects are JSON
		"p:password":    "[secret]",  // secure values can't be decrypted
		"p:fixture":     "fixture",   // the fixtures' config
		"aws:region":    "eu-west-1", // ... comes after the stack's
	}, got.Values)
	assert.Equal(t, []string{"p:password"}, got.SecretKeys)
	assert.Equal(t, map[string]any{
		"invokes":         map[string]any{"getAccount": map[string]any{"id": "1"}},
		"stackReferences": map[string]any{},
	}, got.Fixtures)
}

func TestAnalyzeScanGraph(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stackURN := "urn:pulumi:scan::p::pulumi:pulumi:Stack::p-scan"
	g := &scanGraph{
		Project:    "p",
		ProgramDir: root,
		Resources: []scanResource{
			{URN: stackURN, Type: "pulumi:pulumi:Stack", Name: "p-scan"},
			{
				URN: "urn:pulumi:scan::p::test:index:Bucket::a", Type: "test:index:Bucket", Name: "a",
				Parent:         stackURN,
				SourcePosition: &scanPosition{URI: "file://" + filepath.Join(root, "index.js"), Line: 7, Column: 1},
				Object:         map[string]any{"acl": "public-read"},
			},
			{
				URN: "urn:pulumi:scan::p::test:index:Bucket::b", Type: "test:index:Bucket", Name: "b",
				Parent: stackURN,
				// Positions outside the program's own code are dropped.
				SourcePosition: &scanPosition{
					URI: "file://" + filepath.Join(root, "node_modules", "lib", "index.js"), Line: 3, Column: 1,
				},
				Object: map[string]any{"acl": "04da6b54-80e4-46f7-96ec-b56ff0331ba9"}, // unknown
			},
		},
		Errors: []scanMessage{{Message: "boom"}},
	}

	diag := func(policy, msg string) plugin.AnalyzeDiagnostic {
		return plugin.AnalyzeDiagnostic{
			PolicyName: policy, PolicyPackName: "pack", PolicyPackVersion: "1.0.0",
			Message: msg, EnforcementLevel: apitype.Mandatory,
		}
	}
	analyzer := &deploytest.Analyzer{
		AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			acl := r.Properties.Get("acl")
			switch {
			case acl.IsComputed():
				return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{
					diag("no-public-buckets", "can't run policy 'no-public-buckets' during preview"),
				}}, nil
			case acl.IsString() && acl.AsString() == "public-read":
				return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{
					diag("no-public-buckets", "bucket is public"),
				}}, nil
			}
			return plugin.AnalyzeResponse{}, nil
		},
		AnalyzeStackF: func(resources []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
			require.Len(t, resources, 3)
			return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{
				diag("stack-policy", "stack-wide"), // no URN: reported on the Stack
			}}, nil
		},
	}

	report, err := analyzeScanGraph(t.Context(), g, []plugin.Analyzer{analyzer})
	require.NoError(t, err)

	assert.Equal(t, 3, report.Resources)
	assert.Equal(t, []string{"boom"}, report.Errors)
	require.Len(t, report.Findings, 2)
	// Findings without a position sort first.
	assert.Equal(t, "stack-policy", report.Findings[0].Policy)
	assert.Equal(t, stackURN, report.Findings[0].URN)
	assert.Empty(t, report.Findings[0].File)
	assert.Equal(t, "no-public-buckets", report.Findings[1].Policy)
	assert.Equal(t, "index.js", report.Findings[1].File)
	assert.Equal(t, 7, report.Findings[1].Line)
	assert.Equal(t, "a", report.Findings[1].Name)

	require.Len(t, report.Undecided, 1)
	assert.Equal(t, "b", report.Undecided[0].Name)
	assert.Empty(t, report.Undecided[0].File)
}

// TestPolicyScanCmd_Program runs the driver on a small JavaScript program with the SDK built in this repository.
// It isn't parallel: it sets PULUMI_HOME.
func TestPolicyScanCmd_Program(t *testing.T) {
	sdk, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "sdk", "nodejs", "bin"))
	require.NoError(t, err)
	if _, err := os.Stat(filepath.Join(sdk, "runtime", "policyScan.js")); err != nil {
		t.Skip("the Node.js SDK isn't built (sdk/nodejs: make build_package)")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node isn't on the PATH")
	}
	t.Setenv("PULUMI_HOME", t.TempDir())

	var seen []plugin.AnalyzerResource
	analyzer := &deploytest.Analyzer{
		AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			seen = append(seen, r)
			if acl := r.Properties.Get("acl"); acl.IsString() && acl.AsString() == "public-read" {
				return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{{
					PolicyName: "no-public-buckets", PolicyPackName: "pack", PolicyPackVersion: "1.0.0",
					Message: "bucket is public", EnforcementLevel: apitype.Mandatory,
				}}}, nil
			}
			return plugin.AnalyzeResponse{}, nil
		},
	}

	var stack []plugin.AnalyzerStackResource
	analyzer.AnalyzeStackF = func(resources []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
		stack = resources
		// A join on an output nothing has deployed yet: the private bucket names the public one.
		ids := map[string]string{}
		for _, r := range resources {
			if id := r.Properties.Get("id"); id.IsString() {
				ids[id.AsString()] = r.Name
			}
		}
		var resp plugin.AnalyzeResponse
		for _, r := range resources {
			if src := r.Properties.Get("source"); src.IsString() && ids[src.AsString()] == "public" {
				resp.Diagnostics = append(resp.Diagnostics, plugin.AnalyzeDiagnostic{
					PolicyName: "no-public-sources", PolicyPackName: "pack", PolicyPackVersion: "1.0.0",
					Message: "source " + src.AsString() + " is public", EnforcementLevel: apitype.Advisory,
					URN: r.URN,
				})
			}
		}
		return resp, nil
	}

	stdout, stderr, err := runScanCmd(t, []plugin.Analyzer{analyzer},
		filepath.Join("testdata", "scan"), "--policy-pack", "./pack", "--sdk", sdk, "--json")
	require.ErrorContains(t, err, "mandatory policy violations", stderr)

	var report scanReport
	require.NoError(t, json.Unmarshal([]byte(stdout), &report), stdout)
	assert.Empty(t, report.Errors)
	assert.Equal(t, 3, report.Resources) // the Stack and two buckets
	require.Len(t, report.Findings, 2)
	f := report.Findings[0]
	assert.Equal(t, "public", f.Name)
	assert.Equal(t, "index.js", f.File)
	assert.Equal(t, 22, f.Line)
	join := report.Findings[1]
	assert.Equal(t, "private", join.Name)
	assert.Equal(t, "source <output public.id> is public", join.Message)
	assert.Equal(t, 1, report.Probed)
	assert.Equal(t, 1, report.Fallbacks)

	assert.Equal(t, []string{"scan-test:owner"}, report.Missing)
	assert.Equal(t, []string{"invoke getAccount"}, report.Fixtures)
	assert.Equal(t, []string{"test:index:getNetwork"}, report.Unmocked)
	assert.Equal(t, []string{"process git rev-parse HEAD"}, report.Blocked)

	byName := map[string]plugin.AnalyzerResource{}
	for _, r := range seen {
		byName[r.Name] = r
	}
	public := byName["public"].Properties
	assert.Equal(t, "[missing config scan-test:owner]", public.Get("owner").AsString())
	assert.Equal(t, "123456789012", public.Get("accountId").AsString())
	assert.Equal(t, "", public.Get("commit").AsString())
	private := byName["private"].Properties
	assert.True(t, private.Get("vpcId").IsComputed(), "a data source without a fixture is unknown")
	ref := scanRefFor(string(byName["public"].URN), "id")
	assert.Equal(t, ref, private.Get("source").AsString(), "another resource's ID is its identifiable unknown")
	assert.Equal(t, `{"Resource":"`+ref+`/*"}`, private.Get("policy").AsString(), "a probed apply's document")
	assert.True(t, private.Get("region").IsComputed(), "an apply that can't run on a probe is unknown")
	_, hasID := byName["public"].Properties.GetOk("id")
	assert.False(t, hasID, "resource policies see the inputs only")

	require.Len(t, stack, 3)
	for _, r := range stack {
		if r.Name == "public" {
			assert.Equal(t, ref, r.Properties.Get("id").AsString(), "stack policies see the ID as its identifiable unknown")
		}
	}
}

func TestScanUnknownsRewrite(t *testing.T) {
	t.Parallel()

	const bucket = "urn:pulumi:scan::p::test:index:Bucket::b"
	out := func(fields map[string]any) map[string]any {
		fields[resource.SigKey] = resource.OutputValueSig
		return fields
	}
	src := func(urn, prop string) map[string]any {
		return out(map[string]any{"source": map[string]any{"urn": urn, "property": prop}})
	}
	g := &scanGraph{Resources: []scanResource{
		{URN: bucket, Name: "b", Object: map[string]any{}},
		{URN: "urn:pulumi:scan::p::test:index:Policy::p", Name: "p", Object: map[string]any{
			"bucket": src(bucket, "id"),
			"arn":    out(map[string]any{"template": map[string]any{"concat": []any{"arn:", src(bucket, "arn"), "/*"}}}),
			"doc": out(map[string]any{"template": map[string]any{"json": map[string]any{
				"Resource": src(bucket, "arn"), "Count": 1.0,
			}}}),
			"derived": out(map[string]any{"dependencies": []any{bucket}}),
			"partly":  out(map[string]any{"template": map[string]any{"concat": []any{"x", plugin.UnknownStringValue}}}),
			"other":   src("urn:pulumi:scan::p::test:index:Bucket::elsewhere", "id"),
		}},
	}}
	u := newScanUnknowns(g)
	got := u.rewrite(g.Resources[1].Object).(map[string]any)
	id, arn := scanRefFor(bucket, "id"), scanRefFor(bucket, "arn")
	assert.Equal(t, id, got["bucket"])
	assert.Equal(t, "arn:"+arn+"/*", got["arn"])
	assert.Equal(t, `{"Count":1,"Resource":"`+arn+`"}`, got["doc"])
	assert.Equal(t, plugin.UnknownStringValue, got["derived"], "derived some other way: still unknown")
	assert.Equal(t, plugin.UnknownStringValue, got["partly"], "a template with an unsourced part")
	assert.Equal(t, plugin.UnknownStringValue, got["other"], "an output of a resource not in the program")
	assert.Equal(t, 3, u.linked)
	assert.Equal(t, 3, u.unlinked)

	// Stack policies also see the bucket's id, and its arn since another resource reads it.
	assert.Equal(t, map[string]any{"id": id, "arn": arn}, u.stackOutputs(&g.Resources[0], map[string]any{}))
	assert.Equal(t, "bucket <output b.arn>/*", u.describe("bucket "+arn+"/*"))
}
