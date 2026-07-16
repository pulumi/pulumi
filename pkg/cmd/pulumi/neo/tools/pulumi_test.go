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

package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestPulumi_NewPulumiRejectsMissingWorkspace(t *testing.T) {
	t.Parallel()

	_, err := NewPulumi(t.TempDir(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace")
}

func TestPulumi_NewPulumiHappyPath(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	ws := &pkgWorkspace.MockContext{}
	sink := &PulumiSink{}

	pu, err := NewPulumi(cwd, ws, sink)
	require.NoError(t, err)
	require.NotNil(t, pu)

	// Cwd is canonicalized; on macOS /var → /private/var via symlink, so we
	// can't compare the raw input. Instead require the result is absolute and
	// resolves the same as canonicalRoot would on the same input.
	want, err := canonicalRoot(cwd)
	require.NoError(t, err)
	assert.Equal(t, want, pu.Cwd)

	assert.Same(t, ws, pu.Workspace)
	assert.Same(t, sink, pu.Sink)
}

func TestPulumi_InvokeUnknownMethod(t *testing.T) {
	t.Parallel()

	p := &Pulumi{Cwd: t.TempDir()}
	_, err := p.Invoke(t.Context(), "pulumi_destroy", json.RawMessage(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown pulumi method")
}

func TestPulumi_InvokeRejectsBadJSON(t *testing.T) {
	t.Parallel()

	p := &Pulumi{Cwd: t.TempDir()}
	_, err := p.Invoke(t.Context(), "pulumi_preview", json.RawMessage(`not json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding")
}

// assertFailedResult checks that the handler's returned value is a pulumiResult
// that self-describes the failure — Status="failed" plus the error text in Logs.
func assertFailedResult(t *testing.T, value any, errSubstr string) {
	t.Helper()
	res, ok := value.(pulumiResult)
	require.True(t, ok, "expected pulumiResult, got %T", value)
	assert.Equal(t, "failed", res.Status)
	assert.Contains(t, res.Logs, errSubstr)
}

func TestPulumi_RunRejectsMissingStackName(t *testing.T) {
	t.Parallel()

	p := &Pulumi{Cwd: t.TempDir()}
	value, err := p.Invoke(t.Context(), "pulumi_preview",
		json.RawMessage(`{"project_name":"p","local_pulumi_dir":"/tmp"}`))
	require.Error(t, err)
	assertFailedResult(t, value, "stack_name is required")
}

func TestPulumi_RunRejectsMissingLocalDir(t *testing.T) {
	t.Parallel()

	p := &Pulumi{Cwd: t.TempDir()}
	value, err := p.Invoke(t.Context(), "pulumi_preview",
		json.RawMessage(`{"project_name":"p","stack_name":"dev"}`))
	require.Error(t, err)
	assertFailedResult(t, value, "local_pulumi_dir is required")
}

func TestPulumi_RunRejectsRelativeLocalDir(t *testing.T) {
	t.Parallel()

	p := &Pulumi{Cwd: t.TempDir()}
	value, err := p.Invoke(t.Context(), "pulumi_preview",
		json.RawMessage(`{"project_name":"p","stack_name":"dev","local_pulumi_dir":"relative/path"}`))
	require.Error(t, err)
	assertFailedResult(t, value, "must be an absolute path")
}

func TestPulumi_RunRejectsEscapingLocalDir(t *testing.T) {
	t.Parallel()

	sandbox, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)

	outside, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)

	p := &Pulumi{Cwd: sandbox}
	args, err := json.Marshal(map[string]any{
		"project_name":     "p",
		"stack_name":       "dev",
		"local_pulumi_dir": outside,
	})
	require.NoError(t, err)
	value, err := p.Invoke(t.Context(), "pulumi_preview", args)
	require.Error(t, err)
	assertFailedResult(t, value, "outside")
}

func TestPulumi_RunRejectsMissingPulumiYaml(t *testing.T) {
	t.Parallel()

	root, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)

	p := &Pulumi{Cwd: root}
	args, err := json.Marshal(map[string]any{
		"project_name":     "p",
		"stack_name":       "dev",
		"local_pulumi_dir": root,
	})
	require.NoError(t, err)
	value, err := p.Invoke(t.Context(), "pulumi_preview", args)
	require.Error(t, err)
	assertFailedResult(t, value, "Pulumi.yaml not found")
}

func TestFailedResult(t *testing.T) {
	t.Parallel()

	res, err := failedResult(
		pulumiArgs{ProjectName: "proj", StackName: "dev"},
		"/tmp/events.ndjson",
		errors.New("kaboom"),
	)
	require.ErrorIs(t, err, errToolFailed)
	assert.Equal(t, "failed", res.Status)
	assert.Equal(t, "proj", res.ProjectName)
	assert.Equal(t, "dev", res.StackName)
	assert.Equal(t, "/tmp/events.ndjson", res.EventsFile)
	assert.Equal(t, "error: kaboom\n", res.Logs)
}

func TestEnvValUnmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    envVal
		wantErr bool
	}{
		{name: "plain", input: `"hello"`, want: envVal{Plain: "hello"}},
		{name: "secret", input: `{"secret":"shh"}`, want: envVal{Secret: "shh"}},
		{name: "empty_secret", input: `{"secret":""}`, wantErr: true},
		{name: "number", input: `42`, wantErr: true},
		{name: "object_no_secret", input: `{"foo":"bar"}`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got envVal
			err := json.Unmarshal([]byte(tc.input), &got)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestEnvValValue(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "plain", envVal{Plain: "plain"}.Value())
	assert.Equal(t, "secret", envVal{Secret: "secret"}.Value())
	// Secret takes precedence over Plain.
	assert.Equal(t, "secret", envVal{Plain: "plain", Secret: "secret"}.Value())
}

func TestApplyEnvVarsSetsAndRestores(t *testing.T) {
	// t.Setenv precludes t.Parallel, which is what we want here — the test mutates
	// process-global state.
	const presentKey = "PULUMI_NEO_TEST_PRESENT"
	const absentKey = "PULUMI_NEO_TEST_ABSENT"

	t.Setenv(presentKey, "original")
	require.NoError(t, os.Unsetenv(absentKey))
	t.Cleanup(func() { _ = os.Unsetenv(absentKey) })

	restore := applyEnvVars(map[string]envVal{
		presentKey: {Plain: "modified"},
		absentKey:  {Secret: "secret-val"},
	})

	assert.Equal(t, "modified", os.Getenv(presentKey))
	assert.Equal(t, "secret-val", os.Getenv(absentKey))

	restore()

	assert.Equal(t, "original", os.Getenv(presentKey))
	_, absentStillSet := os.LookupEnv(absentKey)
	assert.False(t, absentStillSet, "absent key should be cleared after restore")
}

func TestApplyEnvVarsNoopOnEmpty(t *testing.T) {
	t.Parallel()

	restore := applyEnvVars(nil)
	require.NotNil(t, restore)
	restore()
}

func TestFormatChangeCounts(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", FormatChangeCounts(nil, ", "))

	// Same-only counts produce an empty summary.
	assert.Equal(t, "", FormatChangeCounts(display.ResourceChanges{
		deploy.OpSame: 5,
	}, ", "))

	// Zero-count ops are filtered; ordering is semantic (creates first, then
	// updates, replaces, deletes…).
	got := FormatChangeCounts(display.ResourceChanges{
		deploy.OpUpdate:  2,
		deploy.OpCreate:  3,
		deploy.OpDelete:  0,
		deploy.OpReplace: 1,
		deploy.OpSame:    5,
	}, ", ")
	assert.Equal(t, "3 create, 2 update, 1 replace", got)

	// Joiner is configurable so the TUI can use " · " and the agent-facing
	// summary can use ", ".
	dot := FormatChangeCounts(display.ResourceChanges{
		deploy.OpCreate: 1,
		deploy.OpDelete: 1,
	}, " · ")
	assert.Equal(t, "1 create · 1 delete", dot)
}

func TestFormatUpdateSummary(t *testing.T) {
	t.Parallel()

	out := formatUpdateSummary(
		"dev",
		display.ResourceChanges{deploy.OpCreate: 1},
		3*time.Second,
	)
	assert.Contains(t, out, "stack: dev (3s)")
	assert.Contains(t, out, "changes: 1 create")
}

func TestFormatUpdateSummaryNoChanges(t *testing.T) {
	t.Parallel()

	out := formatUpdateSummary("dev", nil, time.Second)
	assert.Contains(t, out, "changes: none")
}

func TestFormatLogs(t *testing.T) {
	t.Parallel()

	// Empty inputs produce an empty string.
	assert.Equal(t, "", formatLogs(nil, nil))

	// Counts and diags compose; counts come first.
	got := formatLogs(
		display.ResourceChanges{deploy.OpCreate: 2, deploy.OpUpdate: 1},
		[]string{"warning: deprecated foo", "error: bad config"},
	)
	assert.Equal(t,
		"summary: 2 create, 1 update\nwarning: deprecated foo\nerror: bad config\n",
		got)
}

func TestOpSortRank(t *testing.T) {
	t.Parallel()

	// Creates sort before updates, replaces before deletes.
	assert.Less(t, OpSortRank(deploy.OpCreate), OpSortRank(deploy.OpUpdate))
	assert.Less(t, OpSortRank(deploy.OpReplace), OpSortRank(deploy.OpDelete))
	// Same lands at the bottom; an unknown StepOp sits between the known set
	// and same so the ordering stays stable when the engine adds new ops.
	assert.Less(t, OpSortRank(deploy.OpRefresh), OpSortRank("bogus"))
	assert.Less(t, OpSortRank("bogus"), OpSortRank(deploy.OpSame))
}

// TestPulumi_InvokeRoutesPreviewAndUp confirms the method dispatch in Invoke
// reaches both run() entry points. Observing the same downstream
// arg-validation failure from each arm proves the switch wires both, not a
// single method via a fallthrough.
func TestPulumi_InvokeRoutesPreviewAndUp(t *testing.T) {
	t.Parallel()

	for _, method := range []string{"pulumi_preview", "pulumi_up"} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			p := &Pulumi{Cwd: t.TempDir()}
			value, err := p.Invoke(t.Context(), method,
				json.RawMessage(`{"project_name":"p","stack_name":"dev"}`))
			require.Error(t, err)
			assertFailedResult(t, value, "local_pulumi_dir is required")
		})
	}
}

// newProjectDir creates a canonical sandbox dir containing a minimal Pulumi.yaml so
// run() gets past its Pulumi.yaml existence check and into backend resolution.
func newProjectDir(t *testing.T) string {
	t.Helper()
	dir, err := canonicalRoot(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "Pulumi.yaml"), []byte("name: p\nruntime: nodejs\n"), 0o600))
	return dir
}

// TestPulumi_Run_ResolvesBackendFromLiveEnv proves the tool resolves its backend
// via cmdBackend.CurrentBackend against the live (post-applyEnvVars) environment, so
// an agent-injected PULUMI_ACCESS_TOKEN is honored rather than a token frozen at neo
// startup — keeping the tool's identity in lockstep with `pulumi preview`.
//
//nolint:paralleltest // mutates the global cmdBackend.DefaultLoginManager and process env
func TestPulumi_Run_ResolvesBackendFromLiveEnv(t *testing.T) {
	dir := newProjectDir(t)

	// Ensure PULUMI_ACCESS_TOKEN is unset in the ambient environment (CI sets it) so the
	// "previously unset is unset on restore" behavior is what we actually exercise below.
	if orig, ok := os.LookupEnv("PULUMI_ACCESS_TOKEN"); ok {
		require.NoError(t, os.Unsetenv("PULUMI_ACCESS_TOKEN"))
		// t.Setenv can't express "restore the prior value" (there is no t.Unsetenv), so
		// restore it ourselves.
		t.Cleanup(func() { _ = os.Setenv("PULUMI_ACCESS_TOKEN", orig) }) //nolint:usetesting
	}

	var capturedToken string
	var resolved bool
	be := &backend.MockBackend{
		ParseStackReferenceF: func(string) (backend.StackReference, error) {
			resolved = true
			// The agent-injected PULUMI_ACCESS_TOKEN must be visible here, proving the
			// backend was resolved after applyEnvVars. Stop run() early with an error.
			capturedToken = os.Getenv("PULUMI_ACCESS_TOKEN")
			return nil, errors.New("stop after resolution")
		},
	}
	prev := cmdBackend.DefaultLoginManager
	cmdBackend.DefaultLoginManager = &cmdBackend.MockLoginManager{
		CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool,
		) (backend.Backend, error) {
			return be, nil
		},
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool,
			insecure bool, color colors.Colorization,
		) (backend.Backend, error) {
			return be, nil
		},
	}
	t.Cleanup(func() { cmdBackend.DefaultLoginManager = prev })

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(_ string) (*workspace.Project, string, error) {
			return &workspace.Project{Name: "p"}, dir, nil
		},
	}
	p := &Pulumi{Cwd: dir, Workspace: ws}

	args, err := json.Marshal(map[string]any{
		"project_name":     "p",
		"stack_name":       "dev",
		"local_pulumi_dir": dir,
		"environment_variables": map[string]any{
			"PULUMI_ACCESS_TOKEN": map[string]any{"secret": "live-token"},
		},
	})
	require.NoError(t, err)

	value, err := p.Invoke(t.Context(), "pulumi_preview", args)
	require.Error(t, err)
	assert.True(t, resolved, "freshly resolved backend was never consulted")
	assert.Equal(t, "live-token", capturedToken)
	assertFailedResult(t, value, "parsing stack reference")
	// The injected env var is restored after the call.
	_, present := os.LookupEnv("PULUMI_ACCESS_TOKEN")
	assert.False(t, present, "PULUMI_ACCESS_TOKEN should be restored after the tool call")
}

// TestPulumi_Run_PreviewAndUpResolveBackend proves both pulumi_preview and pulumi_up
// share the same fresh backend resolution (they share run()).
//
//nolint:paralleltest // mutates the global cmdBackend.BackendInstance
func TestPulumi_Run_PreviewAndUpResolveBackend(t *testing.T) {
	for _, method := range []string{"pulumi_preview", "pulumi_up"} {
		//nolint:paralleltest // mutates the global cmdBackend.DefaultLoginManager
		t.Run(method, func(t *testing.T) {
			dir := newProjectDir(t)

			var resolved bool
			be := &backend.MockBackend{
				ParseStackReferenceF: func(string) (backend.StackReference, error) {
					resolved = true
					return nil, errors.New("stop after resolution")
				},
			}

			prev := cmdBackend.DefaultLoginManager
			cmdBackend.DefaultLoginManager = &cmdBackend.MockLoginManager{
				CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
					url string, project *workspace.Project, setCurrent bool,
				) (backend.Backend, error) {
					return be, nil
				},
				LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
					url string, project *workspace.Project, setCurrent bool,
					insecure bool, color colors.Colorization,
				) (backend.Backend, error) {
					return be, nil
				},
			}
			t.Cleanup(func() { cmdBackend.DefaultLoginManager = prev })

			ws := &pkgWorkspace.MockContext{
				ReadProjectF: func(_ string) (*workspace.Project, string, error) {
					return &workspace.Project{Name: "p"}, dir, nil
				},
			}
			p := &Pulumi{Cwd: dir, Workspace: ws}

			args, err := json.Marshal(map[string]any{
				"project_name":     "p",
				"stack_name":       "dev",
				"local_pulumi_dir": dir,
			})
			require.NoError(t, err)

			value, err := p.Invoke(t.Context(), method, args)
			require.Error(t, err)
			assert.True(t, resolved, "backend was not resolved via CurrentBackend")
			assertFailedResult(t, value, "parsing stack reference")
		})
	}
}

// TestPulumi_Run_PopulatesConsoleURLAndDeploymentID proves run() wires
// backendDisplay.Options.OnPermalink through to pulumiResult.ConsoleURL /
// DeploymentID for both preview and up, using the same identifier convention
// as the Pulumi Console: the preview's UpdateID (a UUID) for previews, and
// the update's decimal version number for updates. The fake backend's
// PreviewF/UpdateF invoke OnPermalink itself, mirroring what
// cloudBackend.apply does in the real httpstate backend.
//
//nolint:paralleltest // mutates the global cmdBackend.DefaultLoginManager
func TestPulumi_Run_PopulatesConsoleURLAndDeploymentID(t *testing.T) {
	tests := []struct {
		method       string
		wantPreview  bool
		wantDeployID string
	}{
		{method: "pulumi_preview", wantPreview: true, wantDeployID: "11111111-2222-3333-4444-555555555555"},
		{method: "pulumi_up", wantPreview: false, wantDeployID: "42"},
	}

	for _, tc := range tests {
		//nolint:paralleltest // mutates the global cmdBackend.DefaultLoginManager
		t.Run(tc.method, func(t *testing.T) {
			dir := newProjectDir(t)

			const wantURL = "https://app.pulumi.com/o/proj/dev/updates/42"
			fireOnPermalink := func(op backend.UpdateOperation) {
				if op.Opts.Display.OnPermalink == nil {
					return
				}
				if tc.wantPreview {
					op.Opts.Display.OnPermalink(wantURL, tc.wantDeployID, 0, true)
				} else {
					op.Opts.Display.OnPermalink(wantURL, "11111111-2222-3333-4444-555555555555", 42, false)
				}
			}

			stackRef := &backend.MockStackReference{
				NameV:               tokens.MustParseStackName("dev"),
				FullyQualifiedNameV: tokens.QName("organization/p/dev"),
			}

			var stk backend.Stack
			be := &backend.MockBackend{
				ParseStackReferenceF: func(name string) (backend.StackReference, error) {
					return stackRef, nil
				},
				GetStackF: func(_ context.Context, ref backend.StackReference) (backend.Stack, error) {
					return stk, nil
				},
				PreviewF: func(_ context.Context, _ backend.Stack, op backend.UpdateOperation,
				) (*deploy.Plan, display.ResourceChanges, error) {
					fireOnPermalink(op)
					return nil, display.ResourceChanges{}, nil
				},
				UpdateF: func(_ context.Context, _ backend.Stack, op backend.UpdateOperation,
				) (display.ResourceChanges, error) {
					fireOnPermalink(op)
					return display.ResourceChanges{}, nil
				},
			}
			stk = &backend.MockStack{
				RefF:     func() backend.StackReference { return stackRef },
				BackendF: func() backend.Backend { return be },
				DefaultSecretManagerF: func(_ context.Context, _ *workspace.ProjectStack) (secrets.Manager, error) {
					return b64.NewBase64SecretsManager(), nil
				},
			}

			prev := cmdBackend.DefaultLoginManager
			cmdBackend.DefaultLoginManager = &cmdBackend.MockLoginManager{
				CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
					url string, project *workspace.Project, setCurrent bool,
				) (backend.Backend, error) {
					return be, nil
				},
				LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
					url string, project *workspace.Project, setCurrent bool,
					insecure bool, color colors.Colorization,
				) (backend.Backend, error) {
					return be, nil
				},
			}
			t.Cleanup(func() { cmdBackend.DefaultLoginManager = prev })

			ws := &pkgWorkspace.MockContext{
				ReadProjectF: func(_ string) (*workspace.Project, string, error) {
					return &workspace.Project{Name: "p"}, dir, nil
				},
			}
			p := &Pulumi{Cwd: dir, Workspace: ws}

			args, err := json.Marshal(map[string]any{
				"project_name":     "p",
				"stack_name":       "dev",
				"local_pulumi_dir": dir,
			})
			require.NoError(t, err)

			value, err := p.Invoke(t.Context(), tc.method, args)
			require.NoError(t, err)
			res, ok := value.(pulumiResult)
			require.True(t, ok, "expected pulumiResult, got %T", value)
			assert.Equal(t, "succeeded", res.Status)
			assert.Equal(t, wantURL, res.ConsoleURL)
			assert.Equal(t, tc.wantDeployID, res.DeploymentID)
		})
	}
}

// recorder collects invocations into the PulumiSink so tests can assert which
// callbacks fired and with what arguments. It's a struct of slices rather than
// counters so the test can inspect the exact event order.
type sinkRecorder struct {
	starts    []recordedStart
	resources []recordedResource
	diags     []recordedDiag
	ends      []recordedEnd
}

type recordedStart struct {
	toolName, stack string
	isPreview       bool
}

type recordedResource struct {
	toolName, urn, typ, status string
	op                         display.StepOp
}

type recordedDiag struct {
	toolName, severity, message, urn string
}

type recordedEnd struct {
	toolName, err, elapsed string
	counts                 display.ResourceChanges
}

func (r *sinkRecorder) sink() *PulumiSink {
	return &PulumiSink{
		OnStart: func(tn, sn string, p bool) {
			r.starts = append(r.starts, recordedStart{toolName: tn, stack: sn, isPreview: p})
		},
		OnResource: func(tn string, op display.StepOp, urn, typ, status string) {
			r.resources = append(r.resources, recordedResource{
				toolName: tn, op: op, urn: urn, typ: typ, status: status,
			})
		},
		OnDiag: func(tn, sev, msg, urn string) {
			r.diags = append(r.diags, recordedDiag{toolName: tn, severity: sev, message: msg, urn: urn})
		},
		OnEnd: func(tn, e string, c display.ResourceChanges, el string) {
			r.ends = append(r.ends, recordedEnd{toolName: tn, err: e, counts: c, elapsed: el})
		},
	}
}

// testURN constructs a syntactically-valid resource URN for fixture events.
// drainEvents reads URN.Type() and stringifies the URN; both paths require a
// well-formed URN.
func testURN(name string) resource.URN {
	return resource.NewURN(
		tokens.QName("dev"), tokens.PackageName("p"),
		"" /*parentType*/, tokens.Type("aws:s3/Bucket:Bucket"), name,
	)
}

func TestPulumi_DrainEvents_ResourcePre_PreviewMode(t *testing.T) {
	t.Parallel()

	rec := &sinkRecorder{}
	p := &Pulumi{Sink: rec.sink()}
	urn := testURN("my-bucket")

	ev := engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: engine.StepEventMetadata{
			Op: deploy.OpCreate, URN: urn, Type: tokens.Type("aws:s3/Bucket:Bucket"),
		},
	})

	ch := make(chan engine.Event, 1)
	ch <- ev
	close(ch)

	var buf bytes.Buffer
	diags := p.drainEvents("pulumi__pulumi_preview", true, ch, &buf)

	assert.Empty(t, diags, "preview-mode pre-event must not produce diag lines")
	require.Len(t, rec.resources, 1)
	got := rec.resources[0]
	assert.Equal(t, "pulumi__pulumi_preview", got.toolName)
	assert.Equal(t, deploy.OpCreate, got.op)
	assert.Equal(t, string(urn), got.urn)
	assert.Equal(t, "aws:s3/Bucket:Bucket", got.typ)
	assert.Equal(t, "planned", got.status, "preview-mode status must be planned")

	// NDJSON line is produced for the event.
	assert.NotEmpty(t, buf.String(), "events file must receive a line for the event")
	assert.True(t, json.Valid(bytes.TrimSpace(buf.Bytes())),
		"NDJSON line must be valid JSON")
}

func TestPulumi_DrainEvents_ResourcePre_UpMode(t *testing.T) {
	t.Parallel()

	rec := &sinkRecorder{}
	p := &Pulumi{Sink: rec.sink()}

	ev := engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: engine.StepEventMetadata{
			Op: deploy.OpCreate, URN: testURN("b"), Type: tokens.Type("aws:s3/Bucket:Bucket"),
		},
	})
	ch := make(chan engine.Event, 1)
	ch <- ev
	close(ch)

	var buf bytes.Buffer
	_ = p.drainEvents("pulumi__pulumi_up", false, ch, &buf)

	require.Len(t, rec.resources, 1)
	assert.Equal(t, "running", rec.resources[0].status,
		"up-mode pre-event must use 'running' status, not 'planned'")
}

func TestPulumi_DrainEvents_ResourcePre_FiltersInternalAndSame(t *testing.T) {
	t.Parallel()

	rec := &sinkRecorder{}
	p := &Pulumi{Sink: rec.sink()}

	ch := make(chan engine.Event, 2)
	// Internal events are engine-private and must not surface to the user.
	ch <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: engine.StepEventMetadata{
			Op: deploy.OpCreate, URN: testURN("internal"),
		},
		Internal: true,
	})
	// OpSame is the no-op step; the live block only shows actual changes.
	ch <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: engine.StepEventMetadata{
			Op: deploy.OpSame, URN: testURN("same"),
		},
	})
	close(ch)

	_ = p.drainEvents("pulumi__pulumi_preview", true, ch, &bytes.Buffer{})
	assert.Empty(t, rec.resources, "Internal and OpSame must not invoke OnResource")
}

func TestPulumi_DrainEvents_ResourceOutputs_UpMarksDone(t *testing.T) {
	t.Parallel()

	rec := &sinkRecorder{}
	p := &Pulumi{Sink: rec.sink()}

	ch := make(chan engine.Event, 1)
	ch <- engine.NewEvent(engine.ResourceOutputsEventPayload{
		Metadata: engine.StepEventMetadata{
			Op: deploy.OpUpdate, URN: testURN("u"), Type: tokens.Type("aws:s3/Bucket:Bucket"),
		},
	})
	close(ch)

	_ = p.drainEvents("pulumi__pulumi_up", false, ch, &bytes.Buffer{})
	require.Len(t, rec.resources, 1)
	assert.Equal(t, "done", rec.resources[0].status)
	assert.Equal(t, deploy.OpUpdate, rec.resources[0].op)
}

func TestPulumi_DrainEvents_ResourceOutputs_PreviewSkipped(t *testing.T) {
	t.Parallel()

	// Preview never runs resources, so an outputs event there would only
	// duplicate the row already emitted by ResourcePre. Skip silently.
	rec := &sinkRecorder{}
	p := &Pulumi{Sink: rec.sink()}

	ch := make(chan engine.Event, 1)
	ch <- engine.NewEvent(engine.ResourceOutputsEventPayload{
		Metadata: engine.StepEventMetadata{Op: deploy.OpCreate, URN: testURN("c")},
	})
	close(ch)

	_ = p.drainEvents("pulumi__pulumi_preview", true, ch, &bytes.Buffer{})
	assert.Empty(t, rec.resources, "outputs in preview mode must not invoke OnResource")
}

func TestPulumi_DrainEvents_ResourceOutputs_FiltersInternalAndSame(t *testing.T) {
	t.Parallel()

	rec := &sinkRecorder{}
	p := &Pulumi{Sink: rec.sink()}

	ch := make(chan engine.Event, 2)
	ch <- engine.NewEvent(engine.ResourceOutputsEventPayload{
		Metadata: engine.StepEventMetadata{Op: deploy.OpUpdate, URN: testURN("i")},
		Internal: true,
	})
	ch <- engine.NewEvent(engine.ResourceOutputsEventPayload{
		Metadata: engine.StepEventMetadata{Op: deploy.OpSame, URN: testURN("s")},
	})
	close(ch)

	_ = p.drainEvents("pulumi__pulumi_up", false, ch, &bytes.Buffer{})
	assert.Empty(t, rec.resources)
}

func TestPulumi_DrainEvents_ResourceFailed_MarksFailed(t *testing.T) {
	t.Parallel()

	rec := &sinkRecorder{}
	p := &Pulumi{Sink: rec.sink()}

	ch := make(chan engine.Event, 1)
	ch <- engine.NewEvent(engine.ResourceOperationFailedPayload{
		Metadata: engine.StepEventMetadata{
			Op: deploy.OpCreate, URN: testURN("f"), Type: tokens.Type("aws:s3/Bucket:Bucket"),
		},
	})
	close(ch)

	_ = p.drainEvents("pulumi__pulumi_up", false, ch, &bytes.Buffer{})
	require.Len(t, rec.resources, 1)
	assert.Equal(t, "failed", rec.resources[0].status)
}

func TestPulumi_DrainEvents_Diag_WarningAndError(t *testing.T) {
	t.Parallel()

	rec := &sinkRecorder{}
	p := &Pulumi{Sink: rec.sink()}

	ch := make(chan engine.Event, 2)
	// Color markers (<{%fg%}>) are stripped by colors.Never before the
	// callback fires — the TUI paints its own row colour.
	ch <- engine.NewEvent(engine.DiagEventPayload{
		Severity: diag.Warning,
		URN:      testURN("w"),
		Message:  "<{%fg 3%}>deprecated foo<{%reset%}>",
	})
	ch <- engine.NewEvent(engine.DiagEventPayload{
		Severity: diag.Error,
		Message:  "config invalid",
	})
	close(ch)

	diags := p.drainEvents("pulumi__pulumi_up", false, ch, &bytes.Buffer{})
	require.Len(t, rec.diags, 2)
	assert.Equal(t, "warning", rec.diags[0].severity)
	assert.Equal(t, "deprecated foo", rec.diags[0].message,
		"color markers must be stripped from the message")
	assert.NotEmpty(t, rec.diags[0].urn, "resource-scoped warning must carry the URN")
	assert.Equal(t, "error", rec.diags[1].severity)
	assert.Empty(t, rec.diags[1].urn, "stack-level diag has empty URN")

	// Returned diags slice mirrors what's threaded into Logs.
	require.Len(t, diags, 2)
	assert.Contains(t, diags[0], "deprecated foo")
	assert.Contains(t, diags[1], "config invalid")
}

func TestPulumi_DrainEvents_Diag_FiltersEphemeralAndInfo(t *testing.T) {
	t.Parallel()

	rec := &sinkRecorder{}
	p := &Pulumi{Sink: rec.sink()}

	ch := make(chan engine.Event, 2)
	// Ephemeral: progress chatter the engine emits while a resource is
	// running. Skipping these matches the rest of the CLI's display.
	ch <- engine.NewEvent(engine.DiagEventPayload{
		Severity: diag.Warning, Message: "tick", Ephemeral: true,
	})
	// Info severity isn't surfaced to the user — only Warning/Error gate the
	// Neo block from getting noisy.
	ch <- engine.NewEvent(engine.DiagEventPayload{
		Severity: diag.Info, Message: "fyi",
	})
	close(ch)

	diags := p.drainEvents("pulumi__pulumi_up", false, ch, &bytes.Buffer{})
	assert.Empty(t, rec.diags)
	assert.Empty(t, diags)
}

func TestPulumi_DrainEvents_NilSinkSafe(t *testing.T) {
	t.Parallel()

	// drainEvents must tolerate p.Sink == nil (non-interactive mode) without
	// panicking; events still need to land in the NDJSON file because the
	// agent reads it via the filesystem tool regardless of TUI state.
	p := &Pulumi{Sink: nil}

	ch := make(chan engine.Event, 4)
	ch <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: engine.StepEventMetadata{Op: deploy.OpCreate, URN: testURN("a")},
	})
	ch <- engine.NewEvent(engine.ResourceOutputsEventPayload{
		Metadata: engine.StepEventMetadata{Op: deploy.OpCreate, URN: testURN("a")},
	})
	ch <- engine.NewEvent(engine.ResourceOperationFailedPayload{
		Metadata: engine.StepEventMetadata{Op: deploy.OpCreate, URN: testURN("b")},
	})
	ch <- engine.NewEvent(engine.DiagEventPayload{
		Severity: diag.Warning, Message: "warn",
	})
	close(ch)

	var buf bytes.Buffer
	require.NotPanics(t, func() {
		_ = p.drainEvents("pulumi__pulumi_up", false, ch, &buf)
	})
	assert.NotEmpty(t, buf.String(), "NDJSON output must still be written when sink is nil")
}

func TestPulumi_DrainEvents_PartialSinkSafe(t *testing.T) {
	t.Parallel()

	// A PulumiSink with a nil OnResource (caller only wired diags, say) must
	// not panic when a resource event arrives — every callback is independently
	// guarded.
	calledDiag := 0
	p := &Pulumi{Sink: &PulumiSink{
		OnDiag: func(_, _, _, _ string) { calledDiag++ },
		// OnStart, OnResource, OnEnd intentionally nil.
	}}

	ch := make(chan engine.Event, 2)
	ch <- engine.NewEvent(engine.ResourcePreEventPayload{
		Metadata: engine.StepEventMetadata{Op: deploy.OpCreate, URN: testURN("a")},
	})
	ch <- engine.NewEvent(engine.DiagEventPayload{
		Severity: diag.Warning, Message: "w",
	})
	close(ch)

	require.NotPanics(t, func() {
		_ = p.drainEvents("pulumi__pulumi_up", false, ch, &bytes.Buffer{})
	})
	assert.Equal(t, 1, calledDiag, "OnDiag must still fire when other callbacks are nil")
}

func TestPulumi_DrainEvents_NDJSONIsValidEngineEvent(t *testing.T) {
	t.Parallel()

	// The NDJSON file is what the agent reads via the filesystem tool — each
	// line must round-trip through apitype.EngineEvent so downstream consumers
	// can re-hydrate it. Verify with a representative diag event.
	p := &Pulumi{}

	ch := make(chan engine.Event, 1)
	ch <- engine.NewEvent(engine.DiagEventPayload{
		Severity: diag.Warning,
		URN:      testURN("d"),
		Message:  "deprecated",
	})
	close(ch)

	var buf bytes.Buffer
	_ = p.drainEvents("pulumi__pulumi_up", false, ch, &buf)

	line := bytes.TrimSpace(buf.Bytes())
	require.NotEmpty(t, line)
	var apiEv apitype.EngineEvent
	require.NoError(t, json.Unmarshal(line, &apiEv))
	require.NotNil(t, apiEv.DiagnosticEvent, "NDJSON must carry the DiagnosticEvent payload")
	assert.Equal(t, "warning", apiEv.DiagnosticEvent.Severity)
}

func TestNewPulumiResultUsesParsedNames(t *testing.T) {
	t.Parallel()

	// Regression: pulumiResult must echo the canonical project name and the
	// bare stack name back to callers, not the raw input strings. The Neo
	// agent's entity extraction treats the returned stack_name as a bare
	// name; if we returned the input verbatim a fully-qualified
	// "<org>/<project>/<stack>" form would propagate into a malformed
	// StackEntity.
	proj := &workspace.Project{Name: tokens.PackageName("real-proj")}
	stackRef := &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}

	res := newPulumiResult(proj, stackRef, "/tmp/events.ndjson", "https://app.pulumi.com/o/p/s/updates/3", "3")

	assert.Equal(t, "real-proj", res.ProjectName)
	assert.Equal(t, "dev", res.StackName)
	assert.Equal(t, "/tmp/events.ndjson", res.EventsFile)
	assert.Equal(t, "https://app.pulumi.com/o/p/s/updates/3", res.ConsoleURL)
	assert.Equal(t, "3", res.DeploymentID)
}

func TestAutonamingStackContextFor_NonHTTPStateStack(t *testing.T) {
	t.Parallel()

	// A plain backend.Stack (not the cloud httpstate.Stack) cannot supply an
	// org name — autonamingStackContextFor must fall back to the placeholder
	// "organization" so autonaming config still loads.
	proj := &workspace.Project{Name: tokens.PackageName("my-proj")}
	stk := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}
		},
	}

	got := autonamingStackContextFor(proj, stk)
	assert.Equal(t, "organization", got.Organization)
	assert.Equal(t, "my-proj", got.Project)
	assert.Equal(t, "dev", got.Stack)
}

func TestAutonamingStackContextFor_HTTPStateStackUsesOrgName(t *testing.T) {
	t.Parallel()

	// httpstate.Stack callers expose the real organization via OrgName(); that
	// value must propagate into the autonaming context.
	proj := &workspace.Project{Name: tokens.PackageName("my-proj")}
	stk := &fakeHTTPStack{
		MockStack: &backend.MockStack{
			RefF: func() backend.StackReference {
				return &backend.MockStackReference{NameV: tokens.MustParseStackName("prod")}
			},
		},
		orgName: "my-org",
	}

	got := autonamingStackContextFor(proj, stk)
	assert.Equal(t, "my-org", got.Organization)
	assert.Equal(t, "my-proj", got.Project)
	assert.Equal(t, "prod", got.Stack)
}

// fakeHTTPStack satisfies httpstate.Stack so tests can exercise the
// type-assertion branch in autonamingStackContextFor. Methods other than the
// two cloud-only ones delegate to the embedded MockStack.
type fakeHTTPStack struct {
	*backend.MockStack
	orgName string
}

func (f *fakeHTTPStack) OrgName() string {
	return f.orgName
}

func (f *fakeHTTPStack) CurrentOperation() *apitype.OperationStatus {
	return nil
}

func (f *fakeHTTPStack) StackIdentifier() client.StackIdentifier {
	return client.StackIdentifier{}
}

// Compile-time assertion: fakeHTTPStack must satisfy httpstate.Stack so the
// type assertion in autonamingStackContextFor succeeds.
var _ httpstate.Stack = (*fakeHTTPStack)(nil)
