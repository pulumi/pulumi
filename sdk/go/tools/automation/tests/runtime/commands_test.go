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

//go:build automation_runtime

// Package runtime_test drives the generated command methods against the
// testing boilerplate, which renders each invocation to a deterministic
// string instead of executing the CLI. Each test asserts on that string.
//
// The file is guarded by a build tag so a default `go test ./...` of the
// surrounding tree stays green without needing the output/ tree to exist
// yet. The outer generator tests regenerate output/ and spawn this package
// with the tag enabled.
// and the API singleton; running them in parallel provides no benefit and
// just muddles the output.
//
//nolint:paralleltest // These tests share the generated output directory
package runtime_test

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/tools/automation/output/automation"
	"github.com/pulumi/pulumi/sdk/v3/go/tools/automation/output/optcancel"
	"github.com/pulumi/pulumi/sdk/v3/go/tools/automation/output/optorgsearch"
	"github.com/pulumi/pulumi/sdk/v3/go/tools/automation/output/optorgsearchai"
	"github.com/pulumi/pulumi/sdk/v3/go/tools/automation/output/optstatemove"
)

// newAPI constructs an API wired to the testing boilerplate, which
// renders invocations instead of executing them. Using the New constructor
// keeps the public surface stable if we ever change the struct's fields.
func newAPI() *automation.API { return automation.New(nil) }

func strPtr(s string) *string { return &s }

// runStdout invokes fn and returns the rendered CLI command line. It is a
// thin wrapper that fails the test on error so individual cases stay
// focussed on the command string they care about.
func runStdout(t *testing.T, fn func() (stdout string, err error)) string {
	t.Helper()
	out, err := fn()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return out
}

func TestCancel_Empty(t *testing.T) {
	api := newAPI()
	got := runStdout(t, func() (string, error) {
		r, err := api.Cancel(context.Background(), nil)
		return r.Stdout, err
	})
	want := "pulumi cancel --yes"
	if got != want {
		t.Fatalf("Cancel() = %q, want %q", got, want)
	}
}

func TestCancel_WithStackName(t *testing.T) {
	api := newAPI()
	got := runStdout(t, func() (string, error) {
		r, err := api.Cancel(context.Background(), strPtr("my-stack"))
		return r.Stdout, err
	})
	want := "pulumi cancel --yes -- my-stack"
	if got != want {
		t.Fatalf("Cancel(my-stack) = %q, want %q", got, want)
	}
}

func TestCancel_WithStackFlag(t *testing.T) {
	api := newAPI()
	got := runStdout(t, func() (string, error) {
		r, err := api.Cancel(context.Background(), nil, optcancel.Stack("dev"))
		return r.Stdout, err
	})
	want := "pulumi cancel --yes --stack dev"
	if got != want {
		t.Fatalf("Cancel(Stack=dev) = %q, want %q", got, want)
	}
}

func TestOrg_ExecutableMenu(t *testing.T) {
	api := newAPI()
	got := runStdout(t, func() (string, error) {
		r, err := api.Org(context.Background())
		return r.Stdout, err
	})
	want := "pulumi org"
	if got != want {
		t.Fatalf("Org() = %q, want %q", got, want)
	}
}

func TestOrgGetDefault(t *testing.T) {
	api := newAPI()
	got := runStdout(t, func() (string, error) {
		r, err := api.OrgGetDefault(context.Background())
		return r.Stdout, err
	})
	want := "pulumi org get-default"
	if got != want {
		t.Fatalf("OrgGetDefault() = %q, want %q", got, want)
	}
}

func TestOrgSetDefault(t *testing.T) {
	api := newAPI()
	got := runStdout(t, func() (string, error) {
		r, err := api.OrgSetDefault(context.Background(), "my-org")
		return r.Stdout, err
	})
	want := "pulumi org set-default -- my-org"
	if got != want {
		t.Fatalf("OrgSetDefault(my-org) = %q, want %q", got, want)
	}
}

func TestOrgSearch_RepeatableQuery(t *testing.T) {
	api := newAPI()
	got := runStdout(t, func() (string, error) {
		r, err := api.OrgSearch(
			context.Background(),
			optorgsearch.Query([]string{"foo", "bar"}),
		)
		return r.Stdout, err
	})
	want := "pulumi org search --query foo --query bar"
	if got != want {
		t.Fatalf("OrgSearch(--query foo --query bar) = %q, want %q", got, want)
	}
}

func TestOrgSearchAi_SingleQuery(t *testing.T) {
	api := newAPI()
	got := runStdout(t, func() (string, error) {
		r, err := api.OrgSearchAi(
			context.Background(),
			optorgsearchai.Query("hello"),
		)
		return r.Stdout, err
	})
	want := "pulumi org search ai --query hello"
	if got != want {
		t.Fatalf("OrgSearchAi(query=hello) = %q, want %q", got, want)
	}
}

func TestStateMove_VariadicOnly(t *testing.T) {
	api := newAPI()
	got := runStdout(t, func() (string, error) {
		r, err := api.StateMove(context.Background(), []string{"urn:1", "urn:2"})
		return r.Stdout, err
	})
	want := "pulumi state move --yes -- urn:1 urn:2"
	if got != want {
		t.Fatalf("StateMove([urn:1,urn:2]) = %q, want %q", got, want)
	}
}

func TestStateMove_EmptyVariadic(t *testing.T) {
	api := newAPI()
	got := runStdout(t, func() (string, error) {
		r, err := api.StateMove(context.Background(), nil)
		return r.Stdout, err
	})
	want := "pulumi state move --yes"
	if got != want {
		t.Fatalf("StateMove(nil) = %q, want %q", got, want)
	}
}

func TestStateMove_WithBooleanFlag(t *testing.T) {
	api := newAPI()
	got := runStdout(t, func() (string, error) {
		r, err := api.StateMove(
			context.Background(),
			[]string{"urn:1"},
			optstatemove.IncludeParents(true),
		)
		return r.Stdout, err
	})
	want := "pulumi state move --yes --include-parents -- urn:1"
	if got != want {
		t.Fatalf("StateMove(IncludeParents=true) = %q, want %q", got, want)
	}
}

func TestStateMove_WithSourceAndDest(t *testing.T) {
	api := newAPI()
	got := runStdout(t, func() (string, error) {
		r, err := api.StateMove(
			context.Background(),
			[]string{"urn:1"},
			optstatemove.Source("dev"),
			optstatemove.Dest("prod"),
		)
		return r.Stdout, err
	})
	want := "pulumi state move --yes --dest prod --source dev -- urn:1"
	if got != want {
		t.Fatalf("StateMove(source=dev, dest=prod) = %q, want %q", got, want)
	}
}
