# Executable Policy Packs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `runtime: executable` policy packs — self-contained per-platform binaries the CLI execs directly, with no language toolchain, dependency install, or Docker on the consumer machine.

**Architecture:** A third boot branch in `plugin.NewPolicyAnalyzer` execs the host platform's binary from the pack's `binaries` map via the existing `newPlugin` exec/port-handshake machinery. Install short-circuits dependency installation. Publish validates the platform matrix, runs conformance against the host binary (boot + `GetAnalyzerInfo` + synthetic `Analyze`), builds one tarball per platform (manifest + that platform's binary), and uploads each to a per-platform presigned URL defined by new `apitype` fields. Download picks the host platform's artifact from a new `PackLocations` map.

**Tech Stack:** Go (multi-module repo: `sdk/`, `pkg/`), gRPC (`pulumirpc.Analyzer`), testify.

**Spec:** `docs/superpowers/specs/2026-07-09-executable-policy-packs-design.md` — read it before starting any task.

## Global Constraints

- Run all make targets as `mise exec -- make <target>` from the repo root.
- New files get the Apache copyright header stamped `2026` (copy the header from any existing file, change the year).
- `sdk/` and `pkg/` are separate Go modules. `pkg` imports `sdk`. After touching `go.mod`/`go.sum` in either, run `mise exec -- make tidy`.
- No comments restating what code does; comments only for non-obvious constraints.
- Errors must be loud and actionable — never silently skip a policy pack (compliance feature).
- Platform strings are always `<GOOS>-<GOARCH>`, e.g. `linux-amd64`. `linux-amd64` is mandatory at publish. Valid platforms: `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`, `windows-amd64`, `windows-arm64`.
- Legacy pack behavior (`nodejs`, `python`, `opa`, …) must be byte-for-byte unchanged.
- Do NOT run `git push`. Commit locally only. All commits end with `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`.

---

### Task 1: Workspace accessor for the `binaries` runtime option

**Files:**
- Create: `sdk/go/common/workspace/policyexecutable.go`
- Test: `sdk/go/common/workspace/policyexecutable_test.go`

**Interfaces:**
- Consumes: `PolicyPackProject` and `ProjectRuntimeInfo` from `sdk/go/common/workspace/project.go` (`proj.Runtime.Options()` returns `map[string]any`; `proj.Runtime.Name()` returns the runtime name).
- Produces (later tasks call these exactly):
  - `const workspace.PolicyRuntimeExecutable = "executable"`
  - `const workspace.PlatformLinuxAmd64 = "linux-amd64"`
  - `func workspace.CurrentPlatform() string` — returns `runtime.GOOS + "-" + runtime.GOARCH`
  - `func (proj *PolicyPackProject) ExecutableBinaries() (map[string]string, error)` — validated platform → relative binary path map

- [ ] **Step 1: Write the failing test**

`sdk/go/common/workspace/policyexecutable_test.go` (with 2026 copyright header):

```go
package workspace

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func policyPackWithBinaries(t *testing.T, binaries map[string]any) *PolicyPackProject {
	proj := &PolicyPackProject{Runtime: NewProjectRuntimeInfo("executable", map[string]any{
		"binaries": binaries,
	})}
	require.NoError(t, proj.Validate())
	return proj
}

func TestExecutableBinaries(t *testing.T) {
	t.Parallel()

	t.Run("valid map", func(t *testing.T) {
		t.Parallel()
		proj := policyPackWithBinaries(t, map[string]any{
			"linux-amd64":   "bin/pack-linux-amd64",
			"darwin-arm64":  "bin/pack-darwin-arm64",
			"windows-amd64": "bin/pack-windows-amd64.exe",
		})
		binaries, err := proj.ExecutableBinaries()
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"linux-amd64":   "bin/pack-linux-amd64",
			"darwin-arm64":  "bin/pack-darwin-arm64",
			"windows-amd64": "bin/pack-windows-amd64.exe",
		}, binaries)
	})

	t.Run("missing binaries option", func(t *testing.T) {
		t.Parallel()
		proj := &PolicyPackProject{Runtime: NewProjectRuntimeInfo("executable", nil)}
		_, err := proj.ExecutableBinaries()
		assert.ErrorContains(t, err, "'binaries'")
	})

	t.Run("empty map", func(t *testing.T) {
		t.Parallel()
		proj := policyPackWithBinaries(t, map[string]any{})
		_, err := proj.ExecutableBinaries()
		assert.ErrorContains(t, err, "'binaries'")
	})

	t.Run("unknown platform key", func(t *testing.T) {
		t.Parallel()
		proj := policyPackWithBinaries(t, map[string]any{"linux-mips": "bin/p"})
		_, err := proj.ExecutableBinaries()
		assert.ErrorContains(t, err, "linux-mips")
	})

	t.Run("absolute path", func(t *testing.T) {
		t.Parallel()
		abs := "/usr/bin/pack"
		if runtime.GOOS == "windows" {
			abs = `C:\bin\pack.exe`
		}
		proj := policyPackWithBinaries(t, map[string]any{"linux-amd64": abs})
		_, err := proj.ExecutableBinaries()
		assert.ErrorContains(t, err, "relative")
	})

	t.Run("path escaping pack directory", func(t *testing.T) {
		t.Parallel()
		proj := policyPackWithBinaries(t, map[string]any{"linux-amd64": "../outside/pack"})
		_, err := proj.ExecutableBinaries()
		assert.ErrorContains(t, err, "escape")
	})

	t.Run("non-string value", func(t *testing.T) {
		t.Parallel()
		proj := policyPackWithBinaries(t, map[string]any{"linux-amd64": 42})
		_, err := proj.ExecutableBinaries()
		assert.ErrorContains(t, err, "linux-amd64")
	})
}

func TestCurrentPlatform(t *testing.T) {
	t.Parallel()
	assert.Equal(t, fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH), CurrentPlatform())
}
```

Note: check that `NewProjectRuntimeInfo(name string, options map[string]any) ProjectRuntimeInfo` exists in `project.go` (it does — used broadly). If its options parameter type differs, adapt the test helper, not the production signature.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd sdk && go test ./go/common/workspace/ -run 'TestExecutableBinaries|TestCurrentPlatform' -v`
Expected: FAIL — `proj.ExecutableBinaries undefined`, `undefined: CurrentPlatform`

- [ ] **Step 3: Write the implementation**

`sdk/go/common/workspace/policyexecutable.go` (with 2026 copyright header):

```go
package workspace

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// PolicyRuntimeExecutable is the policy pack runtime whose packs are pre-built
// per-platform binaries serving the analyzer gRPC protocol.
const PolicyRuntimeExecutable = "executable"

// PlatformLinuxAmd64 is mandatory for published executable packs: server-side
// policy evaluation runs on linux-amd64.
const PlatformLinuxAmd64 = "linux-amd64"

var validExecutablePlatforms = map[string]bool{
	"linux-amd64":   true,
	"linux-arm64":   true,
	"darwin-amd64":  true,
	"darwin-arm64":  true,
	"windows-amd64": true,
	"windows-arm64": true,
}

// CurrentPlatform returns the host platform in the "<os>-<arch>" form used to
// key executable policy pack binaries and artifacts.
func CurrentPlatform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// ExecutableBinaries returns the validated platform-to-binary-path map from an
// executable policy pack's runtime options. Paths are relative to the pack
// directory, in the platform's native separator form.
func (proj *PolicyPackProject) ExecutableBinaries() (map[string]string, error) {
	raw, has := proj.Runtime.Options()["binaries"]
	if !has {
		return nil, errors.New(
			"executable policy packs require a 'binaries' map of platform to binary path in the runtime options")
	}
	m, ok := raw.(map[string]any)
	if !ok || len(m) == 0 {
		return nil, errors.New(
			"the 'binaries' runtime option must be a non-empty map of platform to binary path")
	}

	binaries := make(map[string]string, len(m))
	for platform, v := range m {
		if !validExecutablePlatforms[platform] {
			return nil, fmt.Errorf("unknown platform %q in 'binaries'; valid platforms are: %s",
				platform, strings.Join(sortedPlatforms(validExecutablePlatforms), ", "))
		}
		path, ok := v.(string)
		if !ok || path == "" {
			return nil, fmt.Errorf("binary path for platform %q must be a non-empty string", platform)
		}
		if filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
			return nil, fmt.Errorf("binary path for platform %q must be relative to the policy pack directory", platform)
		}
		clean := filepath.Clean(filepath.FromSlash(path))
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("binary path for platform %q must not escape the policy pack directory", platform)
		}
		binaries[platform] = clean
	}
	return binaries, nil
}

func sortedPlatforms(set map[string]bool) []string {
	platforms := make([]string, 0, len(set))
	for p := range set {
		platforms = append(platforms, p)
	}
	slices.Sort(platforms)
	return platforms
}
```

Add `"slices"` to the imports.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd sdk && go test ./go/common/workspace/ -run 'TestExecutableBinaries|TestCurrentPlatform' -v`
Expected: PASS (all subtests)

- [ ] **Step 5: Commit**

```bash
git add sdk/go/common/workspace/policyexecutable.go sdk/go/common/workspace/policyexecutable_test.go
git commit -m "Add workspace accessor for executable policy pack binaries map

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 2: Engine boot branch for executable packs

**Files:**
- Modify: `sdk/go/common/resource/plugin/analyzer_plugin.go:143-216` (the boot-mode selection inside `NewPolicyAnalyzer`)
- Create: `pkg/host/testdata/analyzer-executable/main.go`
- Test: `pkg/host/analyzer_test.go` (append two tests)

**Interfaces:**
- Consumes: `workspace.PolicyRuntimeExecutable`, `workspace.CurrentPlatform()`, `(*workspace.PolicyPackProject).ExecutableBinaries()` from Task 1; existing `newPlugin`, `constructEnv`, `analyzerPluginDialOptions`, `handshake` in `analyzer_plugin.go`.
- Produces: `runtime: executable` packs boot via `plugin.NewPolicyAnalyzer` with no signature change. Later tasks (publish conformance) rely on this exact path.

- [ ] **Step 1: Write the failing tests**

Create `pkg/host/testdata/analyzer-executable/main.go` — a minimal analyzer that any Go toolchain can compile at test time (with 2026 copyright header):

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type analyzer struct {
	pulumirpc.UnimplementedAnalyzerServer
}

func (a *analyzer) Handshake(
	ctx context.Context, req *pulumirpc.AnalyzerHandshakeRequest,
) (*pulumirpc.AnalyzerHandshakeResponse, error) {
	return &pulumirpc.AnalyzerHandshakeResponse{}, nil
}

func (a *analyzer) GetAnalyzerInfo(ctx context.Context, req *emptypb.Empty) (*pulumirpc.AnalyzerInfo, error) {
	return &pulumirpc.AnalyzerInfo{Name: "executable-test-pack", Version: "0.0.1"}, nil
}

func (a *analyzer) Analyze(ctx context.Context, req *pulumirpc.AnalyzeRequest) (*pulumirpc.AnalyzeResponse, error) {
	return &pulumirpc.AnalyzeResponse{}, nil
}

func (a *analyzer) Cancel(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func main() {
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterAnalyzerServer(srv, &analyzer{})
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		fmt.Printf("fatal: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%d\n", handle.Port)
	if err := <-handle.Done; err != nil {
		fmt.Printf("fatal: %v\n", err)
		os.Exit(1)
	}
}
```

Check the exact proto signatures against `pkg/host/testdata/analyzer/main.go` and `sdk/proto/go` — `GetAnalyzerInfo`/`Analyze` request types must match the generated `AnalyzerServer` interface (adjust if e.g. `GetAnalyzerInfo` takes `*emptypb.Empty`).

Append to `pkg/host/analyzer_test.go`:

```go
func buildExecutableTestPack(t *testing.T) (packDir, binRel string) {
	packDir = t.TempDir()
	binRel = filepath.Join("bin", "policy")
	if goruntime.GOOS == "windows" {
		binRel += ".exe"
	}
	binPath := filepath.Join(packDir, binRel)
	cmd := exec.Command("go", "build", "-o", binPath, "./testdata/analyzer-executable")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return packDir, binRel
}

func writeExecutableManifest(t *testing.T, packDir string, binaries map[string]string) {
	var sb strings.Builder
	sb.WriteString("runtime:\n  name: executable\n  options:\n    binaries:\n")
	for _, platform := range slices.Sorted(maps.Keys(binaries)) {
		fmt.Fprintf(&sb, "      %s: %s\n", platform, filepath.ToSlash(binaries[platform]))
	}
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "PulumiPolicy.yaml"), []byte(sb.String()), 0o600))
}

func TestAnalyzerSpawnExecutable(t *testing.T) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)

	packDir, binRel := buildExecutableTestPack(t)
	writeExecutableManifest(t, packDir, map[string]string{workspace.CurrentPlatform(): binRel})

	analyzer, err := plugin.NewPolicyAnalyzer(ctx.Host, ctx, "policypack", packDir, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, analyzer.Close()) }()

	info, err := analyzer.GetAnalyzerInfo(t.Context())
	require.NoError(t, err)
	require.Equal(t, "executable-test-pack", info.Name)
}

func TestAnalyzerSpawnExecutableMissingPlatform(t *testing.T) {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, h.Close()) }()
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)

	otherPlatform := "linux-amd64"
	if workspace.CurrentPlatform() == otherPlatform {
		otherPlatform = "darwin-arm64"
	}
	packDir := t.TempDir()
	writeExecutableManifest(t, packDir, map[string]string{otherPlatform: "bin/policy"})

	_, err = plugin.NewPolicyAnalyzer(ctx.Host, ctx, "policypack", packDir, nil, nil)
	require.ErrorContains(t, err, "does not provide a binary for "+workspace.CurrentPlatform())
	require.ErrorContains(t, err, otherPlatform)
}
```

New imports for `pkg/host/analyzer_test.go`: `"fmt"`, `"maps"`, `goruntime "runtime"`, `"slices"`, `"strings"`, `"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"` (keep existing imports; `os`, `os/exec`, `path/filepath` already imported).

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd pkg && go test ./host/ -run 'TestAnalyzerSpawnExecutable' -v`
Expected: FAIL — the language-plugin/shim fallback fires: error mentions `pulumi-analyzer-policy-executable` not found (the new branch doesn't exist yet).

- [ ] **Step 3: Implement the boot branch**

In `sdk/go/common/resource/plugin/analyzer_plugin.go`, restructure the boot-mode selection (currently lines 143–216). Keep the existing shim and language-plugin code verbatim; wrap them in the `else` of a new executable check so `hasPlugin` is never probed for executable packs:

```go
	var plug *Plugin
	if proj.Runtime.Name() == workspace.PolicyRuntimeExecutable {
		var binaries map[string]string
		binaries, err = proj.ExecutableBinaries()
		if err != nil {
			return nil, fmt.Errorf("policy pack %q: %w", string(name), err)
		}
		platform := workspace.CurrentPlatform()
		binaryRel, ok := binaries[platform]
		if !ok {
			return nil, fmt.Errorf(
				"policy pack %q does not provide a binary for %s; it supports: %s. "+
					"The pack must be republished with a %s binary to run on this machine",
				string(name), platform, strings.Join(slices.Sorted(maps.Keys(binaries)), ", "), platform)
		}

		var environment env.Env
		environment, err = constructEnv(opts, proj.Runtime.Name())
		if err != nil {
			return nil, err
		}

		plug, _, err = newPlugin(ctx, policyPackPath, filepath.Join(policyPackPath, binaryRel),
			fmt.Sprintf("%v (analyzer)", name),
			apitype.AnalyzerPlugin, []string{host.ServerAddr()}, environment, handshake,
			analyzerPluginDialOptions(ctx, string(name)),
			host.AttachDebugger(DebugSpec{Type: DebugTypePlugin, Name: string(name)}))
	} else {
		// This section is a back compatibility bit ... (existing comment)
		if hasPlugin == nil {
			// ... existing default hasPlugin, unchanged ...
		}
		foundLanguagePlugin := hasPlugin(workspace.PluginDescriptor{Name: proj.Runtime.Name(), Kind: apitype.LanguagePlugin})

		if !foundLanguagePlugin {
			// ... existing shim branch body, unchanged ...
		} else {
			// ... existing language-plugin branch body, unchanged ...
		}
	}
```

The `var plug *Plugin` declaration already exists at line 161 — move it above the new `if`. The shared error handling after the branches (`errRunPolicyModuleNotFound` npm hint, `errPluginNotFound`, generic "failed to start") stays as-is; the npm hint cannot trigger for executable packs because that sentinel is only produced by the node shim path.

Add imports if missing: `"maps"`, `"slices"` (`"strings"`, `"path/filepath"`, `env` are already imported — verify).

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd pkg && go test ./host/ -run 'TestAnalyzerSpawn' -v`
Expected: PASS — including the three pre-existing `TestAnalyzerSpawn*` tests (regression check on legacy branches).

Also run: `cd sdk && go test ./go/common/resource/plugin/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add sdk/go/common/resource/plugin/analyzer_plugin.go pkg/host/testdata/analyzer-executable/main.go pkg/host/analyzer_test.go
git commit -m "Boot executable policy packs by exec'ing the host platform binary

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 3: apitype wire contract for per-platform artifacts

**Files:**
- Modify: `sdk/go/common/apitype/policy.go` (`CreatePolicyPackRequest` ~line 25, `CreatePolicyPackResponse` ~line 69, `RequiredPolicy` ~line 79)
- Test: `sdk/go/common/apitype/policy_test.go` (create if absent)

**Interfaces:**
- Produces (Tasks 4 and 7 consume these exactly):
  - `CreatePolicyPackRequest.Platforms []string` — json `platforms,omitempty`
  - `CreatePolicyPackResponse.PlatformUploadURIs map[string]PolicyPackUpload` — json `platformUploadURIs,omitempty`
  - `type PolicyPackUpload struct { UploadURI string; RequiredHeaders map[string]string }` — json `uploadURI` / `requiredHeaders,omitempty`
  - `RequiredPolicy.PackLocations map[string]string` — json `packLocations,omitempty`

- [ ] **Step 1: Write the failing test**

In `sdk/go/common/apitype/policy_test.go` (create with 2026 header if the file doesn't exist; otherwise append):

```go
func TestPolicyPackPlatformFieldsRoundTrip(t *testing.T) {
	t.Parallel()

	req := CreatePolicyPackRequest{Name: "p", Platforms: []string{"linux-amd64", "darwin-arm64"}}
	b, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"platforms":["linux-amd64","darwin-arm64"]`)

	respJSON := `{"version":3,"platformUploadURIs":{"linux-amd64":{"uploadURI":"https://u","requiredHeaders":{"k":"v"}}}}`
	var resp CreatePolicyPackResponse
	require.NoError(t, json.Unmarshal([]byte(respJSON), &resp))
	assert.Equal(t, "https://u", resp.PlatformUploadURIs["linux-amd64"].UploadURI)
	assert.Equal(t, map[string]string{"k": "v"}, resp.PlatformUploadURIs["linux-amd64"].RequiredHeaders)

	rpJSON := `{"name":"p","version":1,"versionTag":"0.0.1","displayName":"p","packLocations":{"linux-amd64":"https://d"}}`
	var rp RequiredPolicy
	require.NoError(t, json.Unmarshal([]byte(rpJSON), &rp))
	assert.Equal(t, map[string]string{"linux-amd64": "https://d"}, rp.PackLocations)

	legacy := CreatePolicyPackRequest{Name: "p"}
	b, err = json.Marshal(legacy)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "platforms")
}
```

Imports: `encoding/json`, `testing`, testify `assert`/`require`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd sdk && go test ./go/common/apitype/ -run TestPolicyPackPlatformFieldsRoundTrip -v`
Expected: FAIL — unknown fields `Platforms`, `PlatformUploadURIs`, `PackLocations`.

- [ ] **Step 3: Add the fields**

In `sdk/go/common/apitype/policy.go`:

At the end of `CreatePolicyPackRequest` (after `Metadata`):

```go
	// Platforms lists the platforms ("linux-amd64", ...) for which this pack
	// provides per-platform artifacts. Empty for legacy single-tarball packs.
	Platforms []string `json:"platforms,omitempty"`
```

At the end of `CreatePolicyPackResponse`:

```go
	// PlatformUploadURIs maps platform to a presigned upload for that
	// platform's artifact. Set when the request declared Platforms.
	PlatformUploadURIs map[string]PolicyPackUpload `json:"platformUploadURIs,omitempty"`
```

After `CreatePolicyPackResponse`:

```go
// PolicyPackUpload describes the presigned upload destination for one
// platform's policy pack artifact.
type PolicyPackUpload struct {
	UploadURI string `json:"uploadURI"`
	// RequiredHeaders represents headers that the CLI must set in order
	// for the upload to succeed.
	RequiredHeaders map[string]string `json:"requiredHeaders,omitempty"`
}
```

In `RequiredPolicy`, after `PackLocation`:

```go
	// PackLocations maps platform ("linux-amd64", ...) to a download URL for
	// that platform's artifact. Set only for packs published with
	// per-platform artifacts; PackLocation is unset for such packs.
	PackLocations map[string]string `json:"packLocations,omitempty"`
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd sdk && go test ./go/common/apitype/ -run TestPolicyPackPlatformFieldsRoundTrip -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add sdk/go/common/apitype/policy.go sdk/go/common/apitype/policy_test.go
git commit -m "Add apitype fields for per-platform policy pack artifacts

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 4: Platform-aware download in cloudRequiredPolicy

**Files:**
- Modify: `pkg/backend/httpstate/policypack.go:104-114` (`Download`)
- Test: `pkg/backend/httpstate/policypack_executable_test.go` (create)

**Interfaces:**
- Consumes: `RequiredPolicy.PackLocations` (Task 3), `workspace.CurrentPlatform()` (Task 1).
- Produces: `func (rp *cloudRequiredPolicy) packLocation() (string, error)` (unexported), used by `Download`. No exported surface change.

- [ ] **Step 1: Write the failing test**

Create `pkg/backend/httpstate/policypack_executable_test.go` (2026 header, `package httpstate`):

```go
package httpstate

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackLocationPlatformSelection(t *testing.T) {
	t.Parallel()

	t.Run("legacy single location", func(t *testing.T) {
		t.Parallel()
		rp := &cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name: "pack", PackLocation: "https://legacy",
		}}
		loc, err := rp.packLocation()
		require.NoError(t, err)
		assert.Equal(t, "https://legacy", loc)
	})

	t.Run("platform map picks host platform", func(t *testing.T) {
		t.Parallel()
		rp := &cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name: "pack",
			PackLocations: map[string]string{
				workspace.CurrentPlatform(): "https://mine",
				"made-up-platform":          "https://other",
			},
		}}
		loc, err := rp.packLocation()
		require.NoError(t, err)
		assert.Equal(t, "https://mine", loc)
	})

	t.Run("host platform missing is a loud error", func(t *testing.T) {
		t.Parallel()
		rp := &cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name:          "pack",
			PackLocations: map[string]string{"made-up-platform": "https://other"},
		}}
		_, err := rp.packLocation()
		require.Error(t, err)
		assert.ErrorContains(t, err, "pack")
		assert.ErrorContains(t, err, workspace.CurrentPlatform())
		assert.ErrorContains(t, err, "made-up-platform")
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd pkg && go test ./backend/httpstate/ -run TestPackLocationPlatformSelection -v`
Expected: FAIL — `rp.packLocation undefined`

- [ ] **Step 3: Implement**

In `pkg/backend/httpstate/policypack.go`, add below `policyPath()`:

```go
func (rp *cloudRequiredPolicy) packLocation() (string, error) {
	if len(rp.PackLocations) == 0 {
		return rp.PackLocation, nil
	}
	platform := workspace.CurrentPlatform()
	loc, ok := rp.PackLocations[platform]
	if !ok {
		return "", fmt.Errorf(
			"policy pack %q does not provide an artifact for %s; it supports: %s. "+
				"The pack must be republished with a %s binary to run on this machine",
			rp.RequiredPolicy.Name, platform,
			strings.Join(slices.Sorted(maps.Keys(rp.PackLocations)), ", "), platform)
	}
	return loc, nil
}
```

And change `Download` to use it:

```go
func (rp *cloudRequiredPolicy) Download(
	ctx context.Context,
	wrapper func(stream io.ReadCloser, size int64) io.ReadCloser,
) (io.ReadCloser, int64, error) {
	location, err := rp.packLocation()
	if err != nil {
		return nil, 0, err
	}
	tarball, size, err := rp.client.DownloadPolicyPack(ctx, location)
	if err != nil {
		return nil, 0, err
	}
	return wrapper(tarball, size), size, nil
}
```

(`maps`, `slices`, `strings`, `fmt` are already imported in this file — verify.)

- [ ] **Step 4: Run test to verify it passes**

Run: `cd pkg && go test ./backend/httpstate/ -run TestPackLocationPlatformSelection -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/backend/httpstate/policypack.go pkg/backend/httpstate/policypack_executable_test.go
git commit -m "Select the host platform artifact when downloading executable policy packs

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 5: Install short-circuit for executable packs

**Files:**
- Modify: `pkg/backend/httpstate/policypack.go:499-577` (`installRequiredPolicy`)
- Test: `pkg/backend/httpstate/policypack_executable_test.go` (append)

**Interfaces:**
- Consumes: `workspace.PolicyRuntimeExecutable`, `(*workspace.PolicyPackProject).ExecutableBinaries()`, `workspace.CurrentPlatform()`.
- Produces: no signature change; `installRequiredPolicy` returns after extraction (+`chmod` on non-Windows) for executable packs, never touching `ctx.Host`.

- [ ] **Step 1: Write the failing test**

Append to `pkg/backend/httpstate/policypack_executable_test.go`:

```go
func TestInstallRequiredPolicyExecutable(t *testing.T) {
	t.Parallel()

	packDir := t.TempDir()
	binRel := filepath.Join("bin", "policy")
	require.NoError(t, os.MkdirAll(filepath.Join(packDir, "bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(packDir, binRel), []byte("#!/bin/sh\nexit 0\n"), 0o644))
	manifest := fmt.Sprintf(
		"runtime:\n  name: executable\n  options:\n    binaries:\n      %s: bin/policy\n",
		workspace.CurrentPlatform())
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "PulumiPolicy.yaml"), []byte(manifest), 0o600))

	tgz, err := archive.TGZ(packDir, "package", false)
	require.NoError(t, err)

	finalDir := filepath.Join(t.TempDir(), "pulumi-analyzer-pack-v1")
	// The plugin context is never used on the executable path; a zero value
	// guarantees the language-runtime/dependency-install path is not reached.
	err = installRequiredPolicy(&plugin.Context{}, finalDir,
		io.NopCloser(bytes.NewReader(tgz)), io.Discard, io.Discard)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(finalDir, "PulumiPolicy.yaml"))
	info, err := os.Stat(filepath.Join(finalDir, binRel))
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.NotZero(t, info.Mode()&0o111, "installed binary must be executable")
	}
}
```

Additional imports for the test file: `"bytes"`, `"fmt"`, `"io"`, `"os"`, `"path/filepath"`, `"runtime"`, `"github.com/pulumi/pulumi/pkg/v3/resource/plugin"`, `"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd pkg && go test ./backend/httpstate/ -run TestInstallRequiredPolicyExecutable -v`
Expected: FAIL — nil-pointer panic or error from `ctx.Host.LanguageRuntime` (the executable short-circuit doesn't exist, so the zero `plugin.Context` hits the language-runtime path).

- [ ] **Step 3: Implement**

In `installRequiredPolicy`, immediately after the `LoadPolicyPack` call (line ~547, before the python virtualenv workaround):

```go
	if proj.Runtime.Name() == workspace.PolicyRuntimeExecutable {
		binaries, err := proj.ExecutableBinaries()
		if err != nil {
			return fmt.Errorf("invalid executable policy pack at %s: %w", finalDir, err)
		}
		if bin, ok := binaries[workspace.CurrentPlatform()]; ok && goruntime.GOOS != "windows" {
			binPath := filepath.Join(finalDir, bin)
			if _, err := os.Stat(binPath); err == nil {
				if err := os.Chmod(binPath, 0o755); err != nil {
					return fmt.Errorf("marking policy pack binary executable: %w", err)
				}
			}
		}
		return nil
	}
```

Add import `goruntime "runtime"` to `pkg/backend/httpstate/policypack.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd pkg && go test ./backend/httpstate/ -run TestInstallRequiredPolicyExecutable -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/backend/httpstate/policypack.go pkg/backend/httpstate/policypack_executable_test.go
git commit -m "Skip dependency installation when installing executable policy packs

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 6: Per-platform artifact tarball builder and matrix validation

**Files:**
- Create: `pkg/backend/httpstate/policypack_executable.go`
- Test: `pkg/backend/httpstate/policypack_executable_test.go` (append)

**Interfaces:**
- Consumes: `archive.TGZ(dir, prefixPathInsideTar string, useDefaultExcludes bool) ([]byte, error)`; `workspace.PlatformLinuxAmd64`, `workspace.CurrentPlatform()`.
- Produces (Task 7's publish flow calls these exactly):
  - `func validateExecutableMatrix(packDir string, binaries map[string]string) error`
  - `func buildExecutablePlatformTarball(packDir, binaryRelPath string) ([]byte, error)`

- [ ] **Step 1: Write the failing tests**

Append to `pkg/backend/httpstate/policypack_executable_test.go`:

```go
func writeExecutablePack(t *testing.T, binaries map[string]string) string {
	packDir := t.TempDir()
	var sb strings.Builder
	sb.WriteString("runtime:\n  name: executable\n  options:\n    binaries:\n")
	for platform, rel := range binaries {
		fmt.Fprintf(&sb, "      %s: %s\n", platform, filepath.ToSlash(rel))
	}
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "PulumiPolicy.yaml"), []byte(sb.String()), 0o600))
	for _, rel := range binaries {
		path := filepath.Join(packDir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("binary"), 0o755))
	}
	return packDir
}

func TestValidateExecutableMatrix(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		binaries := map[string]string{
			"linux-amd64":               filepath.Join("bin", "linux"),
			workspace.CurrentPlatform(): filepath.Join("bin", "host"),
		}
		packDir := writeExecutablePack(t, binaries)
		require.NoError(t, validateExecutableMatrix(packDir, binaries))
	})

	t.Run("missing linux-amd64", func(t *testing.T) {
		t.Parallel()
		binaries := map[string]string{workspace.CurrentPlatform(): filepath.Join("bin", "host")}
		packDir := writeExecutablePack(t, binaries)
		err := validateExecutableMatrix(packDir, binaries)
		assert.ErrorContains(t, err, "linux-amd64")
	})

	t.Run("declared binary missing on disk", func(t *testing.T) {
		t.Parallel()
		binaries := map[string]string{
			"linux-amd64":               filepath.Join("bin", "linux"),
			workspace.CurrentPlatform(): filepath.Join("bin", "host"),
		}
		packDir := writeExecutablePack(t, binaries)
		require.NoError(t, os.Remove(filepath.Join(packDir, "bin", "linux")))
		err := validateExecutableMatrix(packDir, binaries)
		assert.ErrorContains(t, err, "linux-amd64")
		assert.ErrorContains(t, err, filepath.Join("bin", "linux"))
	})

	t.Run("host platform not declared", func(t *testing.T) {
		t.Parallel()
		if workspace.CurrentPlatform() == "linux-amd64" {
			t.Skip("host platform is the mandatory platform")
		}
		binaries := map[string]string{"linux-amd64": filepath.Join("bin", "linux")}
		packDir := writeExecutablePack(t, binaries)
		err := validateExecutableMatrix(packDir, binaries)
		assert.ErrorContains(t, err, workspace.CurrentPlatform())
		assert.ErrorContains(t, err, "conformance")
	})
}

func TestBuildExecutablePlatformTarball(t *testing.T) {
	t.Parallel()

	binRel := filepath.Join("bin", "tool")
	packDir := writeExecutablePack(t, map[string]string{"linux-amd64": binRel})
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "index.ts"), []byte("// source"), 0o600))

	tgz, err := buildExecutablePlatformTarball(packDir, binRel)
	require.NoError(t, err)

	extractDir := t.TempDir()
	require.NoError(t, archive.ExtractTGZ(io.NopCloser(bytes.NewReader(tgz)), extractDir))

	assert.FileExists(t, filepath.Join(extractDir, "package", "PulumiPolicy.yaml"))
	info, err := os.Stat(filepath.Join(extractDir, "package", binRel))
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.NotZero(t, info.Mode()&0o111, "binary in artifact must keep the executable bit")
	}
	assert.NoFileExists(t, filepath.Join(extractDir, "package", "index.ts"),
		"per-platform artifacts must contain only the manifest and one binary")
}
```

Check `archive.ExtractTGZ`'s exact signature in `sdk/go/common/util/archive` (Task 5's install path calls it with `(io.ReadCloser, string)`) and adapt the call if it differs.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd pkg && go test ./backend/httpstate/ -run 'TestValidateExecutableMatrix|TestBuildExecutablePlatformTarball' -v`
Expected: FAIL — `undefined: validateExecutableMatrix`, `undefined: buildExecutablePlatformTarball`

- [ ] **Step 3: Implement**

Create `pkg/backend/httpstate/policypack_executable.go` (2026 header):

```go
package httpstate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// validateExecutableMatrix enforces the publish-time platform requirements for
// an executable policy pack: linux-amd64 is mandatory (server-side evaluation
// runs there), every declared binary must exist, and the publishing host's
// platform must be declared so conformance checks can boot it.
func validateExecutableMatrix(packDir string, binaries map[string]string) error {
	if _, ok := binaries[workspace.PlatformLinuxAmd64]; !ok {
		return fmt.Errorf(
			"executable policy packs must declare a %s binary: server-side policy evaluation runs on %s",
			workspace.PlatformLinuxAmd64, workspace.PlatformLinuxAmd64)
	}
	for platform, rel := range binaries {
		if _, err := os.Stat(filepath.Join(packDir, rel)); err != nil {
			return fmt.Errorf("the binary declared for %s was not found at %s: %w", platform, rel, err)
		}
	}
	hostPlatform := workspace.CurrentPlatform()
	if _, ok := binaries[hostPlatform]; !ok {
		return fmt.Errorf(
			"cannot publish from %s: the pack does not declare a %s binary, "+
				"which is required to run publish-time conformance checks",
			hostPlatform, hostPlatform)
	}
	return nil
}

// buildExecutablePlatformTarball builds the artifact for one platform of an
// executable policy pack: a gzipped tarball containing only the pack manifest
// and that platform's binary, nested under the standard "package" directory.
func buildExecutablePlatformTarball(packDir, binaryRelPath string) ([]byte, error) {
	stage, err := os.MkdirTemp("", "pulumi-policy-artifact-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)

	if err := copyFile(
		filepath.Join(packDir, "PulumiPolicy.yaml"),
		filepath.Join(stage, "PulumiPolicy.yaml"), 0o644); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(stage, binaryRelPath)), 0o755); err != nil {
		return nil, err
	}
	if err := copyFile(
		filepath.Join(packDir, binaryRelPath),
		filepath.Join(stage, binaryRelPath), 0o755); err != nil {
		return nil, err
	}

	return archive.TGZ(stage, packageDir, false /*useDefaultExcludes*/)
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
```

(`packageDir` is the existing `const packageDir = "package"` in `policypack.go`.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd pkg && go test ./backend/httpstate/ -run 'TestValidateExecutableMatrix|TestBuildExecutablePlatformTarball' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/backend/httpstate/policypack_executable.go pkg/backend/httpstate/policypack_executable_test.go
git commit -m "Add matrix validation and per-platform artifact builder for executable packs

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 7: Client publish with per-platform uploads

**Files:**
- Modify: `pkg/backend/httpstate/client/client.go:1990-2092` (`PublishPolicyPack` — extract shared request builder; add new method)
- Test: `pkg/backend/httpstate/client/client_test.go` (append)

**Interfaces:**
- Consumes: `apitype.CreatePolicyPackRequest.Platforms`, `CreatePolicyPackResponse.PlatformUploadURIs`, `apitype.PolicyPackUpload` (Task 3).
- Produces (Task 8 calls this exactly):
  - `func (pc *Client) PublishPolicyPackPlatforms(ctx context.Context, orgName string, runtime string, analyzerInfo plugin.AnalyzerInfo, platformArchives map[string][]byte, metadata map[string]string) (string, error)`

- [ ] **Step 1: Write the failing tests**

Append to `pkg/backend/httpstate/client/client_test.go`:

```go
func TestPublishPolicyPackPlatforms(t *testing.T) {
	t.Parallel()

	uploads := map[string][]byte{}
	var createReq apitype.CreatePolicyPackRequest
	completeCalled := false

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/orgs/acme/policypacks", func(rw http.ResponseWriter, req *http.Request) {
		require.NoError(t, json.NewDecoder(req.Body).Decode(&createReq))
		resp := apitype.CreatePolicyPackResponse{
			Version: 1,
			PlatformUploadURIs: map[string]apitype.PolicyPackUpload{
				"linux-amd64":  {UploadURI: server.URL + "/upload/linux-amd64", RequiredHeaders: map[string]string{"x-test": "yes"}},
				"darwin-arm64": {UploadURI: server.URL + "/upload/darwin-arm64"},
			},
		}
		require.NoError(t, json.NewEncoder(rw).Encode(resp))
	})
	mux.HandleFunc("/upload/", func(rw http.ResponseWriter, req *http.Request) {
		require.Equal(t, http.MethodPut, req.Method)
		platform := strings.TrimPrefix(req.URL.Path, "/upload/")
		if platform == "linux-amd64" {
			require.Equal(t, "yes", req.Header.Get("x-test"))
		}
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		uploads[platform] = body
	})
	mux.HandleFunc("/api/orgs/acme/policypacks/mypack/versions/0.0.1/complete",
		func(rw http.ResponseWriter, req *http.Request) {
			completeCalled = true
		})

	client := newMockClient(server)
	version, err := client.PublishPolicyPackPlatforms(context.Background(), "acme", "executable",
		plugin.AnalyzerInfo{Name: "mypack", Version: "0.0.1"},
		map[string][]byte{
			"linux-amd64":  []byte("linux-bytes"),
			"darwin-arm64": []byte("darwin-bytes"),
		}, nil)
	require.NoError(t, err)

	assert.Equal(t, "0.0.1", version)
	assert.Equal(t, []string{"darwin-arm64", "linux-amd64"}, createReq.Platforms)
	assert.Equal(t, "executable", createReq.Runtime)
	assert.Equal(t, []byte("linux-bytes"), uploads["linux-amd64"])
	assert.Equal(t, []byte("darwin-bytes"), uploads["darwin-arm64"])
	assert.True(t, completeCalled)
}

func TestPublishPolicyPackPlatformsUnsupportedService(t *testing.T) {
	t.Parallel()

	server := newMockServer(200, `{"version":1,"uploadURI":"https://legacy-only"}`)
	defer server.Close()

	client := newMockClient(server)
	_, err := client.PublishPolicyPackPlatforms(context.Background(), "acme", "executable",
		plugin.AnalyzerInfo{Name: "mypack", Version: "0.0.1"},
		map[string][]byte{"linux-amd64": []byte("b")}, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "does not support executable policy packs")
}
```

Verify the publish-complete path against the `publishPolicyPackPublishComplete` helper (`pkg/backend/httpstate/client/api_endpoints.go` or wherever it's defined, ~client.go:517 per the codebase map) and fix the mux route to the exact path it builds. Add any missing imports (`context`, `encoding/json`, `io`, `strings`, `apitype`, `plugin`) — most already exist in the file.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd pkg && go test ./backend/httpstate/client/ -run TestPublishPolicyPackPlatforms -v`
Expected: FAIL — `client.PublishPolicyPackPlatforms undefined`

- [ ] **Step 3: Implement**

In `pkg/backend/httpstate/client/client.go`:

First extract the request construction shared with `PublishPolicyPack` (dedupe the `policies` conversion loop and `CreatePolicyPackRequest` literal from lines 2005–2039 into):

```go
func buildCreatePolicyPackRequest(
	runtime string, analyzerInfo plugin.AnalyzerInfo, metadata map[string]string, platforms []string,
) (apitype.CreatePolicyPackRequest, error) {
	policies := make([]apitype.Policy, len(analyzerInfo.Policies))
	for i, policy := range analyzerInfo.Policies {
		configSchema, err := convertPolicyConfigSchema(policy.ConfigSchema)
		if err != nil {
			return apitype.CreatePolicyPackRequest{}, err
		}
		policies[i] = apitype.Policy{
			Name:             policy.Name,
			DisplayName:      policy.DisplayName,
			Description:      policy.Description,
			EnforcementLevel: policy.EnforcementLevel,
			Message:          policy.Message,
			ConfigSchema:     configSchema,
			Severity:         policy.Severity,
			Framework:        convertPolicyComplianceFramework(policy.Framework),
			Tags:             policy.Tags,
			RemediationSteps: policy.RemediationSteps,
			URL:              policy.URL,
		}
	}
	return apitype.CreatePolicyPackRequest{
		Name:        analyzerInfo.Name,
		DisplayName: analyzerInfo.DisplayName,
		VersionTag:  analyzerInfo.Version,
		Policies:    policies,
		Description: analyzerInfo.Description,
		Readme:      analyzerInfo.Readme,
		Provider:    analyzerInfo.Provider,
		Tags:        analyzerInfo.Tags,
		Repository:  analyzerInfo.Repository,
		Runtime:     runtime,
		Metadata:    metadata,
		Platforms:   platforms,
	}, nil
}
```

Update `PublishPolicyPack` to call `buildCreatePolicyPackRequest(runtime, analyzerInfo, metadata, nil)` in place of the extracted code; everything else in it stays identical.

Then add:

```go
// PublishPolicyPackPlatforms publishes an executable policy pack with one
// artifact per platform. It returns the published version.
func (pc *Client) PublishPolicyPackPlatforms(ctx context.Context, orgName string,
	runtime string, analyzerInfo plugin.AnalyzerInfo, platformArchives map[string][]byte,
	metadata map[string]string,
) (string, error) {
	if err := validatePolicyPackVersion(analyzerInfo.Version); err != nil {
		return "", err
	}

	platforms := slices.Sorted(maps.Keys(platformArchives))
	req, err := buildCreatePolicyPackRequest(runtime, analyzerInfo, metadata, platforms)
	if err != nil {
		return "", err
	}

	fmt.Printf("Publishing %q - version %s to %q\n", analyzerInfo.Name, analyzerInfo.Version, orgName)

	var resp apitype.CreatePolicyPackResponse
	if err := pc.restCall(ctx, "POST", publishPolicyPackPath(orgName), nil, req, &resp); err != nil {
		return "", fmt.Errorf("Publish policy pack failed: %w", err)
	}

	if len(resp.PlatformUploadURIs) == 0 {
		return "", errors.New(
			"this Pulumi service version does not support executable policy packs; " +
				"upgrade the service or publish the pack with a language runtime instead")
	}

	for _, platform := range platforms {
		upload, ok := resp.PlatformUploadURIs[platform]
		if !ok {
			return "", fmt.Errorf("the service did not return an upload location for platform %s", platform)
		}
		putReq, err := http.NewRequest(http.MethodPut, upload.UploadURI, bytes.NewReader(platformArchives[platform]))
		if err != nil {
			return "", fmt.Errorf("failed to upload policy pack artifact for %s: %w", platform, err)
		}
		for k, v := range upload.RequiredHeaders {
			putReq.Header.Add(k, v)
		}
		if _, err := pc.restClient.HTTPClient().Do(putReq, retryAllMethods); err != nil {
			return "", fmt.Errorf("failed to upload policy pack artifact for %s: %w", platform, err)
		}
	}

	if err := pc.restCall(ctx, "POST",
		publishPolicyPackPublishComplete(orgName, analyzerInfo.Name, analyzerInfo.Version), nil, nil, nil); err != nil {
		return "", fmt.Errorf("Request to signal completion of the publish operation failed: %w", err)
	}

	return analyzerInfo.Version, nil
}
```

Add `"bytes"`, `"errors"`, `"maps"`, `"slices"` imports if not already present. Note: unlike legacy `PublishPolicyPack`, `analyzerInfo.Version` is required here (executable packs are new; there is no legacy no-version-tag client to support) — `validatePolicyPackVersion` already rejects empty versions, verify this and if it doesn't, add an explicit empty check.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd pkg && go test ./backend/httpstate/client/ -run 'TestPublishPolicyPackPlatforms' -v`
Expected: PASS (both tests)

Also run the whole package to catch refactor regressions: `cd pkg && go test ./backend/httpstate/client/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/backend/httpstate/client/client.go pkg/backend/httpstate/client/client_test.go
git commit -m "Add client support for publishing per-platform policy pack artifacts

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 8: Publish flow — conformance gate and wiring

**Files:**
- Modify: `pkg/backend/httpstate/policypack.go:397-461` (`cloudPolicyPack.Publish`)
- Modify: `pkg/backend/httpstate/policypack_executable.go` (add `publishExecutable`)

**Interfaces:**
- Consumes: `validateExecutableMatrix`, `buildExecutablePlatformTarball` (Task 6), `PublishPolicyPackPlatforms` (Task 7), the executable boot branch (Task 2 — conformance boots through the same `Host.PolicyAnalyzer` path consumers use), `plugin.Analyzer.Analyze(ctx, AnalyzerResource) (AnalyzeResponse, error)` and `GetAnalyzerInfo(ctx)`.
- Produces: `pulumi policy publish` on a `runtime: executable` pack validates, conformance-checks, and uploads per-platform artifacts. No CLI flag changes.

- [ ] **Step 1: Wire the branch and write publishExecutable**

At the top of `cloudPolicyPack.Publish` (before the "Obtaining policy metadata" print):

```go
	if op.PolicyPack.Runtime.Name() == workspace.PolicyRuntimeExecutable {
		return pack.publishExecutable(ctx, op)
	}
```

Add to `pkg/backend/httpstate/policypack_executable.go`:

```go
func (pack *cloudPolicyPack) publishExecutable(ctx context.Context, op backend.PublishOperation) error {
	packDir, err := filepath.Abs(op.PlugCtx.Pwd)
	if err != nil {
		return err
	}

	binaries, err := op.PolicyPack.ExecutableBinaries()
	if err != nil {
		return err
	}
	if err := validateExecutableMatrix(packDir, binaries); err != nil {
		return err
	}

	fmt.Println("Running conformance checks against the host platform binary")

	analyzer, err := op.PlugCtx.Host.PolicyAnalyzer(op.PlugCtx, tokens.QName(packDir), op.PlugCtx.Pwd, nil /*opts*/)
	if err != nil {
		return fmt.Errorf("conformance check failed: the %s binary did not boot: %w",
			workspace.CurrentPlatform(), err)
	}

	analyzerInfo, err := analyzer.GetAnalyzerInfo(ctx)
	if err != nil {
		return fmt.Errorf("conformance check failed: GetAnalyzerInfo: %w", err)
	}

	if _, err := analyzer.Analyze(ctx, plugin.AnalyzerResource{
		URN: resource.NewURN(
			tokens.QName("conformance"), tokens.PackageName("conformance"),
			tokens.Type(""), tokens.Type("pulumi:pulumi:Stack"), "conformance"),
		Type:       tokens.Type("pulumi:pulumi:Stack"),
		Name:       "conformance",
		Properties: resource.PropertyMap{},
	}); err != nil {
		return fmt.Errorf("conformance check failed: synthetic Analyze call: %w", err)
	}

	pack.ref.name = tokens.QName(analyzerInfo.Name)
	pack.ref.versionTag = analyzerInfo.Version

	fmt.Println("Building per-platform artifacts")

	archives := make(map[string][]byte, len(binaries))
	for platform, rel := range binaries {
		tarball, err := buildExecutablePlatformTarball(packDir, rel)
		if err != nil {
			return fmt.Errorf("building artifact for %s: %w", platform, err)
		}
		archives[platform] = tarball
	}

	fmt.Println("Uploading policy pack to Pulumi service")

	publishedVersion, err := pack.cl.PublishPolicyPackPlatforms(
		ctx, pack.ref.orgName, workspace.PolicyRuntimeExecutable, analyzerInfo, archives, op.Metadata)
	if err != nil {
		return err
	}

	fmt.Printf("\nPermalink: %s/%s\n", pack.ref.CloudConsoleURL(), publishedVersion)
	return nil
}
```

Add imports to `policypack_executable.go`: `"context"`, `"github.com/pulumi/pulumi/pkg/v3/backend"`, `"github.com/pulumi/pulumi/pkg/v3/resource/plugin"`, `"github.com/pulumi/pulumi/sdk/v3/go/common/resource"`, `"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"`.

Check `resource.NewURN`'s exact signature (`sdk/go/common/resource/urn.go` or `sdk/go/common/urn/`) and adjust the argument types; the intent is a synthetic `pulumi:pulumi:Stack` resource with no properties. Also verify `plugin.Analyzer.Analyze` returns `(plugin.AnalyzeResponse, error)` per `sdk/go/common/resource/plugin/analyzer.go:48`.

- [ ] **Step 2: Verify it compiles and existing tests pass**

Run: `cd pkg && go build ./... && go test ./backend/httpstate/... -count=1`
Expected: builds; all tests PASS (the matrix-validation and tarball tests from Task 6 cover this function's building blocks; the conformance boot path is covered by Task 2's `pkg/host` test).

- [ ] **Step 3: End-to-end smoke of the local path**

Verify the consumer-facing local path actually works with a real binary (this is the core deliverable):

Run: `cd pkg && go test ./host/ -run TestAnalyzerSpawnExecutable -count=1 -v`
Expected: PASS — a compiled binary boots through the exact branch `pulumi up --policy-pack <dir>` uses.

- [ ] **Step 4: Commit**

```bash
git add pkg/backend/httpstate/policypack.go pkg/backend/httpstate/policypack_executable.go
git commit -m "Publish executable policy packs with conformance gate and per-platform uploads

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 9: Changelog, lint, and full verification

**Files:**
- Create: `changelog/pending/20260709--cli--executable-policy-packs.yaml`

- [ ] **Step 1: Write the changelog entry**

```yaml
component: cli
kind: feature
body: "Add support for `runtime: executable` policy packs, which run pre-built per-platform binaries without requiring a language toolchain on the consumer machine"
time: 2026-07-09T00:00:00.000000+00:00
```

Match the exact schema of existing files in `changelog/pending/` (component/kind/body/time). No trailing punctuation. `custom.PR` is added later once the PR exists.

- [ ] **Step 2: Format, lint, tidy**

Run, in order:

```bash
mise exec -- make format
mise exec -- make lint
mise exec -- make tidy
```

Expected: format may rewrite files (re-stage them); lint and tidy exit 0. Fix any findings — do not skip lint.

- [ ] **Step 3: Run the affected test suites**

```bash
cd sdk && go test ./go/common/workspace/... ./go/common/apitype/... ./go/common/resource/plugin/... -count=1
cd ../pkg && go test ./host/... ./backend/httpstate/... -count=1
```

Expected: PASS. If anything fails, fix before proceeding (escalate after two failed debugging attempts, per repo instructions).

- [ ] **Step 4: Fast test sweep**

Run: `mise exec -- make test_fast`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "Add changelog entry for executable policy packs

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```
