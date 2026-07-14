# Policy Packs as Published Binaries — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `pulumi policy publish` publishes per-platform binaries alongside the source tarball; the CLI installs and execs the binary for its platform with no language toolchain, while every existing pack and old CLI keeps working unchanged.

**Architecture:** No new runtime — `PulumiPolicy.yaml` keeps `runtime: nodejs` etc. Binary-ness is a property of the published artifact, dispatched on shape (executable present → exec; else source path), mirroring how shipped providers are runtime-less binaries. Distribution is service-hosted per-platform blobs; publish dual-uploads source + binaries so old CLIs are unaffected. Spec: `docs/superpowers/specs/2026-07-14-policy-pack-binary-publish-design.md`.

**Tech Stack:** Go (multi-module: `sdk/`, `pkg/`), gRPC `pulumirpc.Analyzer`, Pulumi service REST API.

**Prior art:** Branch `dbiwer/executable-policy-packs` (local) implemented the earlier `runtime: executable` framing. Much of its code transplants directly — this plan embeds the adapted code, and cites `git show dbiwer/executable-policy-packs:<file>` where the implementer can diff for context. Do NOT copy its `runtime: executable` / `binaries:` manifest concepts or the `pulumi-language-executable` plugin — those are superseded.

## Global Constraints

- Run everything from the repo root: `/Users/danbiwer/dev/pulumi/.claude/worktrees/ethereal-baking-dijkstra`. Work on branch `policy-pack-binary-publish` (already checked out).
- Prefix make commands: `mise exec -- make …`.
- After any `.go` change, before the task's final commit: `mise exec -- make format` (and `mise exec -- make lint` in Task 8).
- New files get `// Copyright 2026, Pulumi Corporation.` Apache-2.0 headers (copy the 13-line header block from any recent file, year 2026).
- Comments: only where the *why* is non-obvious. No comments restating code or narrating the change.
- Go test invocations: run from the module dir (`cd sdk && go test …` or `cd pkg && go test …`) with `-count=1`.
- Commit messages end with `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`.
- Platform strings are `<GOOS>-<GOARCH>`: `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`, `windows-amd64`, `windows-arm64`. `linux-amd64` is mandatory at publish. Windows binaries carry `.exe`.
- Artifact layout (one tarball per platform): `package/PulumiPolicy.yaml` + `package/pulumi-analyzer-<name>[.exe]`.
- Author build convention: `bin/pulumi-analyzer-<name>-<os>-<arch>[.exe]` relative to the pack root. This convention is used for local dev exec (`PolicyPackBinary`) and to detect a leaked binary in the source tarball; it is **not** used at publish time — binary publishing only happens when the author passes explicit `--binary <os>-<arch>=<path>` flags.

---

### Task 1: Workspace binary conventions

**Files:**
- Create: `sdk/go/common/workspace/policybinary.go`
- Test: `sdk/go/common/workspace/policybinary_test.go`

**Interfaces:**
- Produces (all in package `workspace`, consumed by Tasks 3, 5, 6, 7):
  - `func CurrentPlatform() string` — `runtime.GOOS + "-" + runtime.GOARCH`
  - `const PlatformLinuxAmd64 = "linux-amd64"`
  - `func DiscoverPolicyBinaries(packDir string) (map[string]string, error)` — platform → pack-relative path; empty map (nil error) when none found
  - `func ParsePolicyBinaryOverrides(flags []string) (map[string]string, error)` — parses `<os>-<arch>=<path>` flag values
  - `func PolicyPackBinary(dir string) (string, bool)` — shape-dispatch lookup; absolute path of the pack's binary

- [ ] **Step 1: Write the failing tests**

`sdk/go/common/workspace/policybinary_test.go` (with 2026 copyright header):

```go
package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func touch(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("#!"), 0o755))
}

func TestDiscoverPolicyBinaries(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	touch(t, filepath.Join(dir, "bin", "pulumi-analyzer-mypack-linux-amd64"))
	touch(t, filepath.Join(dir, "bin", "pulumi-analyzer-mypack-darwin-arm64"))
	touch(t, filepath.Join(dir, "bin", "pulumi-analyzer-mypack-windows-amd64.exe"))
	// Non-matching files are ignored.
	touch(t, filepath.Join(dir, "bin", "pulumi-analyzer-mypack"))
	touch(t, filepath.Join(dir, "bin", "helper.sh"))

	binaries, err := DiscoverPolicyBinaries(dir)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"linux-amd64":   filepath.Join("bin", "pulumi-analyzer-mypack-linux-amd64"),
		"darwin-arm64":  filepath.Join("bin", "pulumi-analyzer-mypack-darwin-arm64"),
		"windows-amd64": filepath.Join("bin", "pulumi-analyzer-mypack-windows-amd64.exe"),
	}, binaries)
}

func TestDiscoverPolicyBinariesEmpty(t *testing.T) {
	t.Parallel()

	binaries, err := DiscoverPolicyBinaries(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, binaries)
}

func TestDiscoverPolicyBinariesMixedNames(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	touch(t, filepath.Join(dir, "bin", "pulumi-analyzer-alpha-linux-amd64"))
	touch(t, filepath.Join(dir, "bin", "pulumi-analyzer-beta-linux-amd64"))

	_, err := DiscoverPolicyBinaries(dir)
	require.ErrorContains(t, err, "alpha")
	require.ErrorContains(t, err, "beta")
}

func TestParsePolicyBinaryOverrides(t *testing.T) {
	t.Parallel()

	m, err := ParsePolicyBinaryOverrides([]string{
		"linux-amd64=out/a",
		"darwin-arm64=out/b",
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"linux-amd64": "out/a", "darwin-arm64": "out/b"}, m)

	_, err = ParsePolicyBinaryOverrides([]string{"freebsd-riscv=out/a"})
	require.ErrorContains(t, err, "freebsd-riscv")

	_, err = ParsePolicyBinaryOverrides([]string{"linux-amd64"})
	require.ErrorContains(t, err, "expected <os>-<arch>=<path>")

	_, err = ParsePolicyBinaryOverrides([]string{"linux-amd64=/abs/path"})
	require.ErrorContains(t, err, "relative")
}

func TestPolicyPackBinaryCanonical(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	name := "pulumi-analyzer-mypack"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	touch(t, filepath.Join(dir, name))

	bin, ok := PolicyPackBinary(dir)
	require.True(t, ok)
	assert.Equal(t, filepath.Join(dir, name), bin)
}

func TestPolicyPackBinaryConventionFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	name := "pulumi-analyzer-mypack-" + CurrentPlatform()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	touch(t, filepath.Join(dir, "bin", name))

	bin, ok := PolicyPackBinary(dir)
	require.True(t, ok)
	assert.Equal(t, filepath.Join(dir, "bin", name), bin)
}

func TestPolicyPackBinaryAbsent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	touch(t, filepath.Join(dir, "index.ts"))

	_, ok := PolicyPackBinary(dir)
	assert.False(t, ok)
}

func TestPolicyPackBinaryAmbiguous(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	touch(t, filepath.Join(dir, "pulumi-analyzer-a"))
	touch(t, filepath.Join(dir, "pulumi-analyzer-b"))

	_, ok := PolicyPackBinary(dir)
	assert.False(t, ok)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd sdk && go test ./go/common/workspace/ -run 'TestDiscoverPolicyBinaries|TestParsePolicyBinaryOverrides|TestPolicyPackBinary' -count=1`
Expected: FAIL — `undefined: DiscoverPolicyBinaries` etc.

- [ ] **Step 3: Implement**

`sdk/go/common/workspace/policybinary.go` (with 2026 copyright header):

```go
package workspace

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// PlatformLinuxAmd64 is mandatory for binary-published policy packs: server-side
// policy evaluation runs on linux-amd64.
const PlatformLinuxAmd64 = "linux-amd64"

const policyBinaryPrefix = "pulumi-analyzer-"

var validPolicyBinaryPlatforms = map[string]bool{
	"linux-amd64":   true,
	"linux-arm64":   true,
	"darwin-amd64":  true,
	"darwin-arm64":  true,
	"windows-amd64": true,
	"windows-arm64": true,
}

// CurrentPlatform returns the host platform in the "<os>-<arch>" form used to key
// policy pack binary artifacts.
func CurrentPlatform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// DiscoverPolicyBinaries scans a policy pack's bin/ directory for binaries built to
// the pulumi-analyzer-<name>-<os>-<arch>[.exe] convention and returns platform to
// pack-relative path. It returns an empty map when the pack has no binaries.
func DiscoverPolicyBinaries(packDir string) (map[string]string, error) {
	entries, err := os.ReadDir(filepath.Join(packDir, "bin"))
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}

	binaries := map[string]string{}
	names := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), policyBinaryPrefix) {
			continue
		}
		stem := strings.TrimSuffix(strings.TrimPrefix(e.Name(), policyBinaryPrefix), ".exe")
		parts := strings.Split(stem, "-")
		if len(parts) < 3 {
			continue
		}
		platform := parts[len(parts)-2] + "-" + parts[len(parts)-1]
		if !validPolicyBinaryPlatforms[platform] {
			continue
		}
		names[strings.Join(parts[:len(parts)-2], "-")] = true
		binaries[platform] = filepath.Join("bin", e.Name())
	}

	if len(names) > 1 {
		return nil, fmt.Errorf(
			"found binaries for more than one policy pack name in bin/: %s",
			strings.Join(slices.Sorted(maps.Keys(names)), ", "))
	}
	return binaries, nil
}

// ParsePolicyBinaryOverrides parses --binary flag values of the form
// "<os>-<arch>=<path>" into a platform-to-path map. Paths must be relative to the
// policy pack directory.
func ParsePolicyBinaryOverrides(flags []string) (map[string]string, error) {
	binaries := make(map[string]string, len(flags))
	for _, f := range flags {
		platform, path, ok := strings.Cut(f, "=")
		if !ok || path == "" {
			return nil, fmt.Errorf("invalid --binary value %q: expected <os>-<arch>=<path>", f)
		}
		if !validPolicyBinaryPlatforms[platform] {
			return nil, fmt.Errorf("unknown platform %q; valid platforms are: %s",
				platform, strings.Join(slices.Sorted(maps.Keys(validPolicyBinaryPlatforms)), ", "))
		}
		if filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
			return nil, fmt.Errorf("binary path for %q must be relative to the policy pack directory", platform)
		}
		binaries[platform] = filepath.Clean(filepath.FromSlash(path))
	}
	return binaries, nil
}

// PolicyPackBinary reports whether the policy pack at dir is a binary pack, and if
// so returns the path of the binary to exec. Installed packs carry the binary at the
// pack root as pulumi-analyzer-<name>; locally built packs carry it at the build
// convention path bin/pulumi-analyzer-<name>-<os>-<arch>.
func PolicyPackBinary(dir string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	var rootMatches []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), policyBinaryPrefix) {
			continue
		}
		rootMatches = append(rootMatches, filepath.Join(dir, e.Name()))
	}
	if len(rootMatches) == 1 {
		return rootMatches[0], true
	}
	if len(rootMatches) > 1 {
		return "", false
	}

	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	matches, err := filepath.Glob(
		filepath.Join(dir, "bin", policyBinaryPrefix+"*-"+CurrentPlatform()+suffix))
	if err != nil || len(matches) != 1 {
		return "", false
	}
	return matches[0], true
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd sdk && go test ./go/common/workspace/ -run 'TestDiscoverPolicyBinaries|TestParsePolicyBinaryOverrides|TestPolicyPackBinary' -count=1`
Expected: PASS

- [ ] **Step 5: Format and commit**

```bash
mise exec -- make format
git add sdk/go/common/workspace/policybinary.go sdk/go/common/workspace/policybinary_test.go
git commit -m "Add workspace conventions for policy pack binaries

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 2: apitype per-platform artifact fields

**Files:**
- Modify: `sdk/go/common/apitype/policy.go` (`CreatePolicyPackRequest` ~line 25, `CreatePolicyPackResponse` ~line 69, `RequiredPolicy` ~line 79)
- Test: `sdk/go/common/apitype/policy_test.go` (new)

**Interfaces:**
- Produces (consumed by Tasks 4, 7):
  - `CreatePolicyPackRequest.Platforms []string`
  - `CreatePolicyPackResponse.PlatformUploadURIs map[string]PolicyPackUpload`
  - `type PolicyPackUpload struct { UploadURI string; RequiredHeaders map[string]string }`
  - `RequiredPolicy.PackLocations map[string]string`

- [ ] **Step 1: Write the failing test**

`sdk/go/common/apitype/policy_test.go` (with 2026 copyright header):

```go
package apitype

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyPackPlatformFieldsRoundTrip(t *testing.T) {
	t.Parallel()

	req := CreatePolicyPackRequest{Name: "p", Platforms: []string{"linux-amd64"}}
	b, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"platforms":["linux-amd64"]`)

	var resp CreatePolicyPackResponse
	require.NoError(t, json.Unmarshal([]byte(`{
		"version": 2,
		"uploadURI": "https://legacy",
		"platformUploadURIs": {
			"linux-amd64": {"uploadURI": "https://x", "requiredHeaders": {"a": "b"}}
		}
	}`), &resp))
	assert.Equal(t, "https://legacy", resp.UploadURI)
	assert.Equal(t, "https://x", resp.PlatformUploadURIs["linux-amd64"].UploadURI)
	assert.Equal(t, map[string]string{"a": "b"}, resp.PlatformUploadURIs["linux-amd64"].RequiredHeaders)

	var rp RequiredPolicy
	require.NoError(t, json.Unmarshal([]byte(`{
		"name": "p",
		"packLocation": "legacy-key",
		"packLocations": {"linux-amd64": "key-linux"}
	}`), &rp))
	assert.Equal(t, "legacy-key", rp.PackLocation)
	assert.Equal(t, map[string]string{"linux-amd64": "key-linux"}, rp.PackLocations)
}

func TestPolicyPackPlatformFieldsOmitted(t *testing.T) {
	t.Parallel()

	b, err := json.Marshal(CreatePolicyPackRequest{Name: "p"})
	require.NoError(t, err)
	assert.NotContains(t, string(b), "platforms")

	b, err = json.Marshal(RequiredPolicy{Name: "p"})
	require.NoError(t, err)
	assert.NotContains(t, string(b), "packLocations")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd sdk && go test ./go/common/apitype/ -run TestPolicyPackPlatformFields -count=1`
Expected: FAIL — unknown fields `Platforms`, `PlatformUploadURIs`, `PackLocations`.

- [ ] **Step 3: Add the fields**

In `sdk/go/common/apitype/policy.go`, append to `CreatePolicyPackRequest` (after `Metadata`):

```go
	// Platforms lists the platforms ("linux-amd64", ...) for which this pack
	// provides per-platform binary artifacts, in addition to the source archive.
	// Empty for source-only packs.
	Platforms []string `json:"platforms,omitempty"`
```

Append to `CreatePolicyPackResponse` (after `RequiredHeaders`):

```go
	// PlatformUploadURIs maps platform to a presigned upload for that
	// platform's binary artifact. Set when the request declared Platforms.
	PlatformUploadURIs map[string]PolicyPackUpload `json:"platformUploadURIs,omitempty"`
```

Add after `CreatePolicyPackResponse`:

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

Append to `RequiredPolicy` (after `PackLocation`):

```go
	// PackLocations maps platform ("linux-amd64", ...) to a download location for
	// that platform's binary artifact. Set only for packs published with binary
	// artifacts; PackLocation remains the source archive.
	PackLocations map[string]string `json:"packLocations,omitempty"`
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd sdk && go test ./go/common/apitype/ -run TestPolicyPackPlatformFields -count=1`
Expected: PASS

- [ ] **Step 5: Format and commit**

```bash
mise exec -- make format
git add sdk/go/common/apitype/policy.go sdk/go/common/apitype/policy_test.go
git commit -m "Add apitype fields for per-platform policy pack artifacts

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 3: Engine shape dispatch in NewPolicyAnalyzer

**Files:**
- Modify: `sdk/go/common/resource/plugin/analyzer_plugin.go:143-216`
- Create: `pkg/host/testdata/analyzer-binary/main.go`
- Test: `pkg/host/analyzer_test.go` (append)

**Interfaces:**
- Consumes: `workspace.PolicyPackBinary(dir) (string, bool)` (Task 1)
- Produces: `NewPolicyAnalyzer` (signature unchanged) execs a pack binary when present, before any language-plugin resolution. Tasks 5 and 7 rely on this behavior via `Host.PolicyAnalyzer`.

- [ ] **Step 1: Add the Go test analyzer fixture**

`pkg/host/testdata/analyzer-binary/main.go` — copy verbatim from the prior branch (it is framing-independent), changing only the reported name to `binary-test-pack`:

```bash
git show dbiwer/executable-policy-packs:pkg/host/testdata/analyzer-executable/main.go > pkg/host/testdata/analyzer-binary/main.go
```

(Create the directory first: `mkdir -p pkg/host/testdata/analyzer-binary`.) Then edit the `GetAnalyzerInfo` return to `Name: "binary-test-pack"`. The fixture serves `pulumirpc.Analyzer` via `rpcutil.ServeWithOptions`, prints its port, and implements `Handshake`/`GetAnalyzerInfo`/`Analyze`/`Cancel`.

- [ ] **Step 2: Write the failing tests**

Append to `pkg/host/analyzer_test.go`:

```go
func buildBinaryTestPack(t *testing.T, binRel string) string {
	packDir := t.TempDir()
	binPath := filepath.Join(packDir, binRel)
	require.NoError(t, os.MkdirAll(filepath.Dir(binPath), 0o755))
	cmd := exec.Command("go", "build", "-o", binPath, "./testdata/analyzer-binary")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	// The manifest still names a language runtime; the binary must win dispatch.
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "PulumiPolicy.yaml"),
		[]byte("runtime: nodejs\n"), 0o600))
	return packDir
}

func newAnalyzerTestContext(t *testing.T) *plugin.Context {
	d := diagtest.LogSink(t)
	h, err := New(t.Context(), d, d, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, h.Close()) })
	ctx, err := plugin.NewContextWithHost(t.Context(), d, d, h, "", "", nil)
	require.NoError(t, err)
	return ctx
}

func TestAnalyzerSpawnBinaryCanonical(t *testing.T) {
	binName := "pulumi-analyzer-binary-test-pack"
	if goruntime.GOOS == "windows" {
		binName += ".exe"
	}
	packDir := buildBinaryTestPack(t, binName)
	ctx := newAnalyzerTestContext(t)

	analyzer, err := plugin.NewPolicyAnalyzer(ctx.Host, ctx, "policypack", packDir, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, analyzer.Close()) }()

	info, err := analyzer.GetAnalyzerInfo(t.Context())
	require.NoError(t, err)
	require.Equal(t, "binary-test-pack", info.Name)
}

func TestAnalyzerSpawnBinaryConvention(t *testing.T) {
	binName := "pulumi-analyzer-binary-test-pack-" + workspace.CurrentPlatform()
	if goruntime.GOOS == "windows" {
		binName += ".exe"
	}
	packDir := buildBinaryTestPack(t, filepath.Join("bin", binName))
	ctx := newAnalyzerTestContext(t)

	analyzer, err := plugin.NewPolicyAnalyzer(ctx.Host, ctx, "policypack", packDir, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, analyzer.Close()) }()

	info, err := analyzer.GetAnalyzerInfo(t.Context())
	require.NoError(t, err)
	require.Equal(t, "binary-test-pack", info.Name)
}
```

Add the imports the file needs (check its current import block first): `goruntime "runtime"`, `"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"`. `os`, `exec`, `filepath`, `plugin`, `diagtest`, `require` are already imported.

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd pkg && go test ./host/ -run TestAnalyzerSpawnBinary -count=1 -v`
Expected: FAIL — the nodejs runtime path is taken; boot fails or the analyzer name is wrong (no node policy module in the temp dir).

- [ ] **Step 4: Implement dispatch**

In `sdk/go/common/resource/plugin/analyzer_plugin.go`, restructure the branch beginning at the back-compat comment (~line 143). Insert a binary check that wins over both existing branches:

```go
	var plug *Plugin

	if binPath, ok := workspace.PolicyPackBinary(policyPackPath); ok {
		// The pack ships a pre-built analyzer binary: exec it directly, exactly as a
		// provider plugin would be, with no language host involved.
		analyzerEnv := env.Global()
		if opts != nil && len(opts.AdditionalEnv) > 0 {
			additionalStore := envutil.MapStore{}
			for k, v := range opts.AdditionalEnv {
				additionalStore[k] = v
			}
			analyzerEnv = envutil.NewEnv(envutil.JoinStore(additionalStore, env.Global().GetStore()))
		}

		plug, _, err = newPlugin(ctx, policyPackPath, binPath, fmt.Sprintf("%v (analyzer)", name),
			apitype.AnalyzerPlugin, []string{host.ServerAddr(), "."}, analyzerEnv,
			handshake, analyzerPluginDialOptions(ctx, string(name)),
			host.AttachDebugger(DebugSpec{Type: DebugTypePlugin, Name: string(name)}))
	} else {
		if hasPlugin == nil {
			hasPlugin = func(spec workspace.PluginDescriptor) bool {
				path, err := workspace.GetPluginPath(
					ctx.baseContext,
					ctx.Diag,
					spec,
					ctx.ProjectPlugins())
				return err == nil && path != ""
			}
		}
		foundLanguagePlugin := hasPlugin(
			workspace.PluginDescriptor{Name: proj.Runtime.Name(), Kind: apitype.LanguagePlugin})

		if !foundLanguagePlugin {
			// ... existing legacy pulumi-analyzer-policy-<runtime> branch, unchanged ...
		} else {
			// ... existing language RunPlugin branch, unchanged ...
		}
	}
```

Mechanically: move the existing `hasPlugin`/`foundLanguagePlugin` computation and the whole existing `if !foundLanguagePlugin { … } else { … }` block inside the new `else`, keep the existing back-compat comment where it now lives, and hoist `var plug *Plugin` above the new `if`. Delete the now-duplicate `var plug *Plugin` at old line 161. The existing error handling after the branches (`if err != nil { … }`) stays as is. The `envutil` import alias already exists in the file (check the import block; the modern branch at old line 203-210 uses it).

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd pkg && go test ./host/ -run TestAnalyzerSpawn -count=1 -v`
Expected: PASS — both new tests AND the pre-existing `TestAnalyzerSpawn*` tests (the back-compat guarantee).

- [ ] **Step 6: Format and commit**

```bash
mise exec -- make format
git add sdk/go/common/resource/plugin/analyzer_plugin.go pkg/host/analyzer_test.go pkg/host/testdata/analyzer-binary/main.go
git commit -m "Exec policy pack binaries directly in NewPolicyAnalyzer

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 4: Client dual-upload publish

**Files:**
- Modify: `pkg/backend/httpstate/client/client.go` (`PublishPolicyPack` ~line 2004)
- Test: `pkg/backend/httpstate/client/client_test.go` (append)

**Interfaces:**
- Consumes: apitype fields (Task 2)
- Produces (consumed by Task 5):
  - `func (pc *Client) PublishPolicyPackWithPlatforms(ctx context.Context, orgName string, runtime string, analyzerInfo plugin.AnalyzerInfo, sourceTarball []byte, platformArchives map[string][]byte, metadata map[string]string) (string, error)` — returns published version tag

- [ ] **Step 1: Write the failing tests**

Append to `pkg/backend/httpstate/client/client_test.go` (the prior branch's tests at `git show dbiwer/executable-policy-packs:pkg/backend/httpstate/client/client_test.go` are the template; these are the adapted versions — note the added source upload):

```go
func TestPublishPolicyPackWithPlatforms(t *testing.T) {
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
			Version:   1,
			UploadURI: server.URL + "/upload/source",
			PlatformUploadURIs: map[string]apitype.PolicyPackUpload{
				"linux-amd64": {
					UploadURI:       server.URL + "/upload/linux-amd64",
					RequiredHeaders: map[string]string{"x-test": "yes"},
				},
				"darwin-arm64": {UploadURI: server.URL + "/upload/darwin-arm64"},
			},
		}
		require.NoError(t, json.NewEncoder(rw).Encode(resp))
	})
	mux.HandleFunc("/upload/", func(rw http.ResponseWriter, req *http.Request) {
		require.Equal(t, http.MethodPut, req.Method)
		key := strings.TrimPrefix(req.URL.Path, "/upload/")
		if key == "linux-amd64" {
			require.Equal(t, "yes", req.Header.Get("x-test"))
		}
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		uploads[key] = body
	})
	mux.HandleFunc("/api/orgs/acme/policypacks/mypack/versions/0.0.1/complete",
		func(rw http.ResponseWriter, req *http.Request) {
			completeCalled = true
		})

	client := newMockClient(server)
	version, err := client.PublishPolicyPackWithPlatforms(t.Context(), "acme", "nodejs",
		plugin.AnalyzerInfo{Name: "mypack", Version: "0.0.1"},
		[]byte("source-bytes"),
		map[string][]byte{
			"linux-amd64":  []byte("linux-bytes"),
			"darwin-arm64": []byte("darwin-bytes"),
		}, nil)
	require.NoError(t, err)

	assert.Equal(t, "0.0.1", version)
	assert.Equal(t, []string{"darwin-arm64", "linux-amd64"}, createReq.Platforms)
	assert.Equal(t, "nodejs", createReq.Runtime)
	assert.Equal(t, []byte("source-bytes"), uploads["source"])
	assert.Equal(t, []byte("linux-bytes"), uploads["linux-amd64"])
	assert.Equal(t, []byte("darwin-bytes"), uploads["darwin-arm64"])
	assert.True(t, completeCalled)
}

func TestPublishPolicyPackWithPlatformsUploadRejected(t *testing.T) {
	t.Parallel()

	completeCalled := false

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/orgs/acme/policypacks", func(rw http.ResponseWriter, req *http.Request) {
		resp := apitype.CreatePolicyPackResponse{
			Version:   1,
			UploadURI: server.URL + "/upload/source",
			PlatformUploadURIs: map[string]apitype.PolicyPackUpload{
				"darwin-arm64": {UploadURI: server.URL + "/upload/darwin-arm64"},
				"linux-amd64":  {UploadURI: server.URL + "/upload/linux-amd64"},
			},
		}
		require.NoError(t, json.NewEncoder(rw).Encode(resp))
	})
	mux.HandleFunc("/upload/", func(rw http.ResponseWriter, req *http.Request) {
		if strings.HasSuffix(req.URL.Path, "darwin-arm64") {
			rw.WriteHeader(http.StatusForbidden)
		}
	})
	mux.HandleFunc("/api/orgs/acme/policypacks/mypack/versions/0.0.1/complete",
		func(rw http.ResponseWriter, req *http.Request) {
			completeCalled = true
		})

	client := newMockClient(server)
	_, err := client.PublishPolicyPackWithPlatforms(t.Context(), "acme", "nodejs",
		plugin.AnalyzerInfo{Name: "mypack", Version: "0.0.1"},
		[]byte("source-bytes"),
		map[string][]byte{
			"linux-amd64":  []byte("linux-bytes"),
			"darwin-arm64": []byte("darwin-bytes"),
		}, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "darwin-arm64")
	assert.False(t, completeCalled)
}

func TestPublishPolicyPackWithPlatformsUnsupportedService(t *testing.T) {
	t.Parallel()

	server := newMockServer(200, `{"version":1,"uploadURI":"https://legacy-only"}`)
	defer server.Close()

	client := newMockClient(server)
	_, err := client.PublishPolicyPackWithPlatforms(t.Context(), "acme", "nodejs",
		plugin.AnalyzerInfo{Name: "mypack", Version: "0.0.1"},
		[]byte("source-bytes"),
		map[string][]byte{"linux-amd64": []byte("b")}, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "does not support policy pack binaries")
	assert.ErrorContains(t, err, "--source-only")
}
```

Add missing imports to the test file if not present: `"github.com/pulumi/pulumi/pkg/v3/resource/plugin"` (check first; `http`, `httptest`, `io`, `json`, `strings` are already imported).

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd pkg && go test ./backend/httpstate/client/ -run TestPublishPolicyPackWithPlatforms -count=1`
Expected: FAIL — `undefined: client.PublishPolicyPackWithPlatforms` (compile error).

- [ ] **Step 3: Implement**

In `pkg/backend/httpstate/client/client.go`:

3a. Extract the request-building block of the existing `PublishPolicyPack` (the `policies := …` loop through the `req := apitype.CreatePolicyPackRequest{…}` literal) into a helper, and call it from `PublishPolicyPack` with `platforms = nil`. Copy the helper verbatim from the prior branch:

```bash
git show dbiwer/executable-policy-packs:pkg/backend/httpstate/client/client.go | grep -n "buildCreatePolicyPackRequest" # for reference
```

The helper (place it after `PublishPolicyPack`):

```go
// buildCreatePolicyPackRequest converts an analyzer's policy metadata into the wire request used
// to create a new PolicyPack version. platforms is non-nil only for packs that publish
// per-platform binary artifacts in addition to the source archive.
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

In `PublishPolicyPack`, replace the extracted block with:

```go
	req, err := buildCreatePolicyPackRequest(runtime, analyzerInfo, metadata, nil)
	if err != nil {
		return "", err
	}
```

(and change the later `err :=` on the `restCall` line to `err =`).

3b. Add a small upload helper and the new method (after `buildCreatePolicyPackRequest`). The source upload MUST mirror how the existing `PublishPolicyPack` uploads to `resp.UploadURI` — read that block (immediately after its `restCall`) and reuse its mechanics via this helper:

```go
func (pc *Client) uploadPolicyPackArtifact(
	uploadURI string, requiredHeaders map[string]string, artifact []byte, what string,
) error {
	putReq, err := http.NewRequest(http.MethodPut, uploadURI, bytes.NewReader(artifact))
	if err != nil {
		return fmt.Errorf("failed to upload policy pack artifact for %s: %w", what, err)
	}
	for k, v := range requiredHeaders {
		putReq.Header.Add(k, v)
	}
	resp, err := pc.restClient.HTTPClient().Do(putReq, retryAllMethods)
	if err != nil {
		return fmt.Errorf("failed to upload policy pack artifact for %s: %w", what, err)
	}
	contract.IgnoreClose(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to upload policy pack artifact for %s: upload returned status %s",
			what, resp.Status)
	}
	return nil
}

// PublishPolicyPackWithPlatforms publishes a policy pack with per-platform binary
// artifacts alongside the source archive, uploading each to a presigned URL returned
// by the service. It returns the published version. A version tag is required: packs
// published with binaries have no legacy client that omits it.
func (pc *Client) PublishPolicyPackWithPlatforms(ctx context.Context, orgName string,
	runtime string, analyzerInfo plugin.AnalyzerInfo, sourceTarball []byte,
	platformArchives map[string][]byte, metadata map[string]string,
) (string, error) {
	if analyzerInfo.Version == "" {
		return "", errors.New("a version is required to publish a policy pack with binaries")
	}
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
			"this Pulumi service version does not support policy pack binaries; " +
				"pass --source-only to publish without them")
	}

	if err := pc.uploadPolicyPackArtifact(
		resp.UploadURI, resp.RequiredHeaders, sourceTarball, "source"); err != nil {
		return "", err
	}
	for _, platform := range platforms {
		upload, ok := resp.PlatformUploadURIs[platform]
		if !ok {
			return "", fmt.Errorf("the service did not return an upload location for platform %s", platform)
		}
		if err := pc.uploadPolicyPackArtifact(
			upload.UploadURI, upload.RequiredHeaders, platformArchives[platform], platform); err != nil {
			return "", err
		}
	}

	if err := pc.restCall(ctx, "POST",
		publishPolicyPackPublishComplete(orgName, analyzerInfo.Name, analyzerInfo.Version), nil, nil, nil); err != nil {
		return "", fmt.Errorf("Request to signal completion of the publish operation failed: %w", err)
	}

	return analyzerInfo.Version, nil
}
```

Add imports if missing: `"maps"`, `"slices"`. Verify `retryAllMethods`, `validatePolicyPackVersion`, `publishPolicyPackPath`, `publishPolicyPackPublishComplete` all exist in the package (they are used by the existing `PublishPolicyPack`); if the existing source-upload block in `PublishPolicyPack` uses different mechanics than `uploadPolicyPackArtifact` (e.g. a different HTTP helper), match those mechanics instead and adjust the helper.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd pkg && go test ./backend/httpstate/client/ -run TestPublishPolicyPack -count=1`
Expected: PASS — new tests and any pre-existing `TestPublishPolicyPack*` tests.

- [ ] **Step 5: Format and commit**

```bash
mise exec -- make format
git add pkg/backend/httpstate/client/client.go pkg/backend/httpstate/client/client_test.go
git commit -m "Add client support for publishing policy packs with binary artifacts

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 5: Backend publish path — validation, artifacts, conformance

**Files:**
- Modify: `pkg/backend/policypack.go:26` (`PublishOperation`)
- Create: `pkg/backend/httpstate/policypack_binary.go`
- Modify: `pkg/backend/httpstate/policypack.go:397` (`cloudPolicyPack.Publish`)
- Test: `pkg/backend/httpstate/policypack_binary_test.go`

**Interfaces:**
- Consumes: `workspace.DiscoverPolicyBinaries`, `workspace.CurrentPlatform`, `workspace.PlatformLinuxAmd64` (Task 1); `PublishPolicyPackWithPlatforms` (Task 4); binary dispatch in `NewPolicyAnalyzer` (Task 3 — conformance boots the staged artifact through it)
- Produces (consumed by Task 6):
  - `backend.PublishOperation.Binaries map[string]string` and `.SourceOnly bool`

- [ ] **Step 1: Extend PublishOperation**

In `pkg/backend/policypack.go`, add to `PublishOperation`:

```go
	// Binaries maps platform ("linux-amd64", ...) to the pack-relative path of a
	// pre-built analyzer binary to publish for that platform. Empty means discover
	// by convention (bin/pulumi-analyzer-<name>-<os>-<arch>).
	Binaries map[string]string

	// SourceOnly skips binary discovery and publishes only the source archive.
	SourceOnly bool
```

- [ ] **Step 2: Write the failing tests**

`pkg/backend/httpstate/policypack_binary_test.go` (with 2026 copyright header):

```go
package httpstate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestPack(t *testing.T, binaries map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PulumiPolicy.yaml"),
		[]byte("runtime: nodejs\n"), 0o600))
	for _, rel := range binaries {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte("bin"), 0o755))
	}
	return dir
}

func TestValidateBinaryMatrixRequiresLinuxAmd64(t *testing.T) {
	t.Parallel()

	binaries := map[string]string{"darwin-arm64": filepath.Join("bin", "b")}
	dir := writeTestPack(t, binaries)
	err := validateBinaryMatrix(dir, binaries)
	require.ErrorContains(t, err, workspace.PlatformLinuxAmd64)
}

func TestValidateBinaryMatrixRequiresHostPlatform(t *testing.T) {
	t.Parallel()

	if workspace.CurrentPlatform() == workspace.PlatformLinuxAmd64 {
		t.Skip("host platform is linux-amd64; the mandatory-platform check subsumes this")
	}
	binaries := map[string]string{workspace.PlatformLinuxAmd64: filepath.Join("bin", "b")}
	dir := writeTestPack(t, binaries)
	err := validateBinaryMatrix(dir, binaries)
	require.ErrorContains(t, err, workspace.CurrentPlatform())
}

func TestValidateBinaryMatrixMissingFile(t *testing.T) {
	t.Parallel()

	binaries := map[string]string{
		workspace.PlatformLinuxAmd64: filepath.Join("bin", "b"),
		workspace.CurrentPlatform():  filepath.Join("bin", "host"),
	}
	dir := writeTestPack(t, binaries)
	require.NoError(t, os.Remove(filepath.Join(dir, "bin", "b")))
	err := validateBinaryMatrix(dir, binaries)
	require.ErrorContains(t, err, filepath.Join("bin", "b"))
}

func tarEntries(t *testing.T, tgz []byte) map[string]int64 {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(tgz))
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	entries := map[string]int64{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if hdr.Typeflag == tar.TypeReg {
			entries[filepath.ToSlash(hdr.Name)] = hdr.Mode
		}
	}
	return entries
}

func TestBuildBinaryArtifact(t *testing.T) {
	t.Parallel()

	rel := filepath.Join("bin", "pulumi-analyzer-mypack-linux-amd64")
	dir := writeTestPack(t, map[string]string{"linux-amd64": rel})

	tgz, err := buildBinaryArtifact(dir, rel, "mypack", "linux-amd64")
	require.NoError(t, err)

	entries := tarEntries(t, tgz)
	assert.Contains(t, entries, "package/PulumiPolicy.yaml")
	assert.Contains(t, entries, "package/pulumi-analyzer-mypack")
	assert.Len(t, entries, 2)
}

func TestBuildBinaryArtifactWindowsSuffix(t *testing.T) {
	t.Parallel()

	rel := filepath.Join("bin", "pulumi-analyzer-mypack-windows-amd64.exe")
	dir := writeTestPack(t, map[string]string{"windows-amd64": rel})

	tgz, err := buildBinaryArtifact(dir, rel, "mypack", "windows-amd64")
	require.NoError(t, err)

	entries := tarEntries(t, tgz)
	assert.Contains(t, entries, "package/pulumi-analyzer-mypack.exe")
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd pkg && go test ./backend/httpstate/ -run 'TestValidateBinaryMatrix|TestBuildBinaryArtifact' -count=1`
Expected: FAIL — `undefined: validateBinaryMatrix`, `undefined: buildBinaryArtifact` (compile error).

- [ ] **Step 4: Implement `policypack_binary.go`**

Create `pkg/backend/httpstate/policypack_binary.go` (with 2026 copyright header). Adapted from `git show dbiwer/executable-policy-packs:pkg/backend/httpstate/policypack_executable.go` — the deltas: no manifest `binaries` map (the map arrives as a parameter), the artifact binary is renamed to the canonical `pulumi-analyzer-<name>`, conformance boots a *staged artifact dir* rather than the author's pack dir, and upload goes through the dual-publish client method:

```go
package httpstate

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// validateBinaryMatrix enforces the publish-time platform requirements for a policy
// pack publishing binaries: linux-amd64 is mandatory (server-side policy evaluation
// runs there), every declared binary must exist, and the publishing host's platform
// must be present so conformance checks can boot it.
func validateBinaryMatrix(packDir string, binaries map[string]string) error {
	if _, ok := binaries[workspace.PlatformLinuxAmd64]; !ok {
		return fmt.Errorf(
			"policy packs published with binaries must include a %s binary: "+
				"server-side policy evaluation runs on %s",
			workspace.PlatformLinuxAmd64, workspace.PlatformLinuxAmd64)
	}
	for _, platform := range slices.Sorted(maps.Keys(binaries)) {
		rel := binaries[platform]
		if _, err := os.Stat(filepath.Join(packDir, rel)); err != nil {
			return fmt.Errorf("the binary for %s was not found at %s: %w", platform, rel, err)
		}
	}
	hostPlatform := workspace.CurrentPlatform()
	if _, ok := binaries[hostPlatform]; !ok {
		return fmt.Errorf(
			"cannot publish from %s: no %s binary was built, "+
				"which is required to run publish-time conformance checks",
			hostPlatform, hostPlatform)
	}
	return nil
}

func canonicalBinaryName(analyzerName, platform string) string {
	name := "pulumi-analyzer-" + analyzerName
	if strings.HasPrefix(platform, "windows-") {
		name += ".exe"
	}
	return name
}

// stageBinaryArtifact lays out one platform's artifact in a temp directory: the pack
// manifest plus the binary under its canonical name. The caller removes the directory.
func stageBinaryArtifact(packDir, binaryRelPath, analyzerName, platform string) (string, error) {
	stage, err := os.MkdirTemp("", "pulumi-policy-artifact-")
	if err != nil {
		return "", err
	}
	if err := copyFile(
		filepath.Join(packDir, "PulumiPolicy.yaml"),
		filepath.Join(stage, "PulumiPolicy.yaml"), 0o644); err != nil {
		os.RemoveAll(stage)
		return "", err
	}
	if err := copyFile(
		filepath.Join(packDir, binaryRelPath),
		filepath.Join(stage, canonicalBinaryName(analyzerName, platform)), 0o755); err != nil {
		os.RemoveAll(stage)
		return "", err
	}
	return stage, nil
}

// buildBinaryArtifact builds the published artifact for one platform: a gzipped
// tarball of the pack manifest and the canonically named binary, nested under the
// standard "package" directory.
func buildBinaryArtifact(packDir, binaryRelPath, analyzerName, platform string) ([]byte, error) {
	stage, err := stageBinaryArtifact(packDir, binaryRelPath, analyzerName, platform)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
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

// publishWithBinaries publishes a pack that built per-platform binaries: it boots the
// host platform's staged artifact for conformance, builds every platform's artifact,
// and dual-uploads the source archive plus the binaries.
func (pack *cloudPolicyPack) publishWithBinaries(
	ctx context.Context, op backend.PublishOperation, binaries map[string]string,
) error {
	packDir, err := filepath.Abs(op.PlugCtx.Pwd)
	if err != nil {
		return err
	}
	if err := validateBinaryMatrix(packDir, binaries); err != nil {
		return err
	}

	fmt.Println("Running conformance checks against the host platform binary")

	// Boot the exact shape consumers will install: manifest + canonical binary in an
	// otherwise empty directory. Booting the author's pack dir instead would let
	// build residue (node_modules, package.json) mask a non-self-contained binary.
	hostPlatform := workspace.CurrentPlatform()
	stage, err := stageBinaryArtifact(packDir, binaries[hostPlatform], "conformance", hostPlatform)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	analyzer, err := op.PlugCtx.Host.PolicyAnalyzer(op.PlugCtx, tokens.QName(stage), stage, nil /*opts*/)
	if err != nil {
		return fmt.Errorf("conformance check failed: the %s binary did not boot: %w", hostPlatform, err)
	}

	analyzerInfo, err := analyzer.GetAnalyzerInfo(ctx)
	if err != nil {
		return fmt.Errorf("conformance check failed: GetAnalyzerInfo: %w", err)
	}

	if _, err := analyzer.Analyze(ctx, plugin.AnalyzerResource{
		URN: resource.NewURN(
			tokens.QName("conformance"), tokens.PackageName("conformance"),
			tokens.Type(""), tokens.Type("pulumi:pulumi:Stack"), "conformance"),
		Type: tokens.Type("pulumi:pulumi:Stack"),
		Name: "conformance",
	}); err != nil {
		return fmt.Errorf("conformance check failed: synthetic Analyze call: %w", err)
	}

	pack.ref.name = tokens.QName(analyzerInfo.Name)
	pack.ref.versionTag = analyzerInfo.Version

	fmt.Println("Building per-platform artifacts")

	archives := make(map[string][]byte, len(binaries))
	for platform, rel := range binaries {
		tarball, err := buildBinaryArtifact(packDir, rel, analyzerInfo.Name, platform)
		if err != nil {
			return fmt.Errorf("building artifact for %s: %w", platform, err)
		}
		archives[platform] = tarball
	}

	fmt.Println("Compressing policy pack")

	sourceTarball, err := buildSourceTarball(ctx, op)
	if err != nil {
		return err
	}

	fmt.Println("Uploading policy pack to Pulumi service")

	publishedVersion, err := pack.cl.PublishPolicyPackWithPlatforms(
		ctx, pack.ref.orgName, op.PolicyPack.Runtime.Name(), analyzerInfo,
		sourceTarball, archives, op.Metadata)
	if err != nil {
		return err
	}

	fmt.Printf("\nPermalink: %s/%s\n", pack.ref.CloudConsoleURL(), publishedVersion)
	return nil
}
```

- [ ] **Step 5: Wire the dispatch and extract `buildSourceTarball`**

In `pkg/backend/httpstate/policypack.go`, modify `cloudPolicyPack.Publish`:

5a. At the top of `Publish`, add:

```go
	binaries := op.Binaries
	if len(binaries) == 0 && !op.SourceOnly {
		var err error
		binaries, err = workspace.DiscoverPolicyBinaries(op.PlugCtx.Pwd)
		if err != nil {
			return err
		}
	}
	if len(binaries) > 0 {
		return pack.publishWithBinaries(ctx, op, binaries)
	}
```

5b. Extract the existing tarball block (the `var packTarball []byte` through the `else { … archive.TGZ … }` block, ~lines 427-444) into a package-level helper in `policypack.go`, and call it from both `Publish` and `publishWithBinaries`:

```go
func buildSourceTarball(ctx context.Context, op backend.PublishOperation) ([]byte, error) {
	// TODO[pulumi/pulumi#1334]: move to the language plugins so we don't have to hard code here.
	if strings.EqualFold(op.PolicyPack.Runtime.Name(), "nodejs") {
		packTarball, err := npm.Pack(ctx, npm.AutoPackageManager, op.PlugCtx.Pwd, os.Stderr)
		if err != nil {
			return nil, fmt.Errorf("could not publish policies because of error running npm pack: %w", err)
		}
		return packTarball, nil
	}
	// npm pack puts all the files in a "package" subdirectory inside the .tgz it produces, so we'll do
	// the same for other runtimes. That way, after unpacking, we can look for the PulumiPolicy.yaml inside the
	// package directory to determine the runtime of the policy pack.
	packTarball, err := archive.TGZ(op.PlugCtx.Pwd, "package", true /*useDefaultExcludes*/)
	if err != nil {
		return nil, fmt.Errorf("could not publish policies because of error creating the .tgz: %w", err)
	}
	return packTarball, nil
}
```

In `Publish`, replace the extracted block with `packTarball, err := buildSourceTarball(ctx, op)` / error check, keeping the surrounding flow (`runtime := op.PolicyPack.Runtime.Name()` stays, as `PublishPolicyPack` still takes it).

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd pkg && go test ./backend/httpstate/... -run 'TestValidateBinaryMatrix|TestBuildBinaryArtifact' -count=1 && go build ./...`
Expected: PASS, and the module builds.

- [ ] **Step 7: Format and commit**

```bash
mise exec -- make format
git add pkg/backend/policypack.go pkg/backend/httpstate/policypack_binary.go pkg/backend/httpstate/policypack_binary_test.go pkg/backend/httpstate/policypack.go
git commit -m "Publish policy pack binaries with a conformance gate and dual upload

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 6: CLI flags on `pulumi policy publish`

**Files:**
- Modify: `pkg/cmd/pulumi/policy/policy_publish.go:43-168`
- Test: `pkg/cmd/pulumi/policy/policy_publish_test.go` (check if it exists; if not, create with just this test)

**Decision update:** binary publishing is opt-in via `--binary` only — there is no
`--source-only` flag and no publish-time convention discovery. No `--binary` flags →
legacy source publish, always. `resolveBinaries` collapsed into a direct call to
`workspace.ParsePolicyBinaryOverrides` since it no longer has any branching logic of
its own.

**Interfaces:**
- Consumes: `workspace.ParsePolicyBinaryOverrides` (Task 1), `backend.PublishOperation.Binaries` (Task 5)

- [ ] **Step 1: Write the failing test**

Add to `pkg/cmd/pulumi/policy/policy_publish_test.go` (create with package decl `package policy`, 2026 header, if absent):

```go
func TestPolicyPublishFlagValidation(t *testing.T) {
	t.Parallel()

	_, err := workspace.ParsePolicyBinaryOverrides([]string{"bogus"})
	require.ErrorContains(t, err, "expected <os>-<arch>=<path>")

	binaries, err := workspace.ParsePolicyBinaryOverrides(
		[]string{"linux-amd64=bin/a", "darwin-arm64=bin/b"})
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"linux-amd64":  filepath.Join("bin", "a"),
		"darwin-arm64": filepath.Join("bin", "b"),
	}, binaries)
}
```

Imports: `"path/filepath"`, `"testing"`, `"github.com/stretchr/testify/require"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd pkg && go test ./cmd/pulumi/policy/ -run TestPolicyPublishFlagValidation -count=1`
Expected: FAIL — `undefined: workspace` (compile error) until the import is added.

- [ ] **Step 3: Implement**

In `pkg/cmd/pulumi/policy/policy_publish.go`:

3a. The struct carries only the raw flag values — no `sourceOnly` field:

```go
type policyPublishCmd struct {
	getwd func() (string, error)

	binaryFlags []string
}
```

3b. In `newPolicyPublishCmd`, register the flag after the `constrictor.AttachArguments` call:

```go
	cmd.Flags().StringArrayVar(&policyPublishCmd.binaryFlags, "binary", nil,
		"Pre-built analyzer binary to publish for a platform, as <os>-<arch>=<path>, "+
			"where <path> is relative to the policy pack directory (repeatable)")
```

3c. In `Run`, before constructing `backend.PublishOperation`, parse the flags directly:

```go
	binaries, err := workspace.ParsePolicyBinaryOverrides(cmd.binaryFlags)
	if err != nil {
		return err
	}
```

and add `Binaries: binaries,` to the `backend.PublishOperation{…}` literal.

The `workspace` import already exists in the file.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd pkg && go test ./cmd/pulumi/policy/ -count=1`
Expected: PASS (whole package, to catch any broken existing tests).

- [ ] **Step 5: Format and commit**

```bash
mise exec -- make format
git add pkg/cmd/pulumi/policy/policy_publish.go pkg/cmd/pulumi/policy/policy_publish_test.go
git commit -m "Add --binary flag to pulumi policy publish

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 7: Platform-aware download and toolchain-free install

**Files:**
- Modify: `pkg/backend/httpstate/policypack.go` (`Download` ~line 105, `installRequiredPolicy` ~line 499)
- Test: `pkg/backend/httpstate/policypack_binary_test.go` (append)

**Interfaces:**
- Consumes: `RequiredPolicy.PackLocations` (Task 2), `workspace.PolicyPackBinary` / `workspace.CurrentPlatform` (Task 1)

- [ ] **Step 1: Write the failing tests**

Append to `pkg/backend/httpstate/policypack_binary_test.go`:

```go
func TestPackLocationSelection(t *testing.T) {
	t.Parallel()

	platform := workspace.CurrentPlatform()

	t.Run("binary for this platform", func(t *testing.T) {
		t.Parallel()
		rp := cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name:          "p",
			PackLocation:  "source-key",
			PackLocations: map[string]string{platform: "bin-key"},
		}}
		loc, err := rp.packLocation()
		require.NoError(t, err)
		assert.Equal(t, "bin-key", loc)
	})

	t.Run("platform gap falls back to source", func(t *testing.T) {
		t.Parallel()
		other := "linux-amd64"
		if platform == other {
			other = "darwin-arm64"
		}
		rp := cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name:          "p",
			PackLocation:  "source-key",
			PackLocations: map[string]string{other: "bin-key"},
		}}
		loc, err := rp.packLocation()
		require.NoError(t, err)
		assert.Equal(t, "source-key", loc)
	})

	t.Run("legacy pack", func(t *testing.T) {
		t.Parallel()
		rp := cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name:         "p",
			PackLocation: "source-key",
		}}
		loc, err := rp.packLocation()
		require.NoError(t, err)
		assert.Equal(t, "source-key", loc)
	})

	t.Run("binary only, platform missing", func(t *testing.T) {
		t.Parallel()
		other := "linux-amd64"
		if platform == other {
			other = "darwin-arm64"
		}
		rp := cloudRequiredPolicy{RequiredPolicy: apitype.RequiredPolicy{
			Name:          "p",
			PackLocations: map[string]string{other: "bin-key"},
		}}
		_, err := rp.packLocation()
		require.ErrorContains(t, err, platform)
		require.ErrorContains(t, err, other)
	})
}
```

Add imports to the test file as needed: `"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd pkg && go test ./backend/httpstate/ -run TestPackLocationSelection -count=1`
Expected: FAIL — `undefined: packLocation` (compile error; `rp.packLocation` does not exist).

- [ ] **Step 3: Implement selection**

In `pkg/backend/httpstate/policypack.go`, add after `policyPath()` (~line 89):

```go
// packLocation picks the artifact to download: this platform's binary when the pack
// published one, otherwise the source archive. A pack that published binaries but
// not for this platform falls back to source with a warning — never silently.
func (rp *cloudRequiredPolicy) packLocation() (string, error) {
	if len(rp.PackLocations) == 0 {
		return rp.PackLocation, nil
	}
	platform := workspace.CurrentPlatform()
	if loc, ok := rp.PackLocations[platform]; ok {
		return loc, nil
	}
	if rp.PackLocation != "" {
		fmt.Fprintf(os.Stderr,
			"warning: policy pack %q has no binary for %s (available: %s); "+
				"falling back to the source archive, which requires the pack's language toolchain\n",
			rp.RequiredPolicy.Name, platform,
			strings.Join(slices.Sorted(maps.Keys(rp.PackLocations)), ", "))
		return rp.PackLocation, nil
	}
	return "", fmt.Errorf(
		"policy pack %q does not provide an artifact for %s; it supports: %s. "+
			"The pack must be republished with a %s binary to run on this machine",
		rp.RequiredPolicy.Name, platform,
		strings.Join(slices.Sorted(maps.Keys(rp.PackLocations)), ", "), platform)
}
```

And change `Download` to use it:

```go
	location, err := rp.packLocation()
	if err != nil {
		return nil, 0, err
	}
	tarball, size, err := rp.client.DownloadPolicyPack(ctx, location)
```

`maps`, `slices`, `strings`, `os`, `fmt` are already imported in this file.

- [ ] **Step 4: Implement the install skip**

In `installRequiredPolicy` (same file), immediately after the `workspace.LoadPolicyPack` succeeds (~line 547), insert:

```go
	if bin, ok := workspace.PolicyPackBinary(finalDir); ok {
		if goruntime.GOOS != "windows" {
			if err := os.Chmod(bin, 0o755); err != nil {
				return fmt.Errorf("marking policy pack binary executable: %w", err)
			}
		}
		return nil
	}
```

Add the import `goruntime "runtime"`.

- [ ] **Step 5: Write and run the install-skip test**

Append to `pkg/backend/httpstate/policypack_binary_test.go`:

```go
func TestInstallRequiredPolicySkipsDependenciesForBinaryPack(t *testing.T) {
	t.Parallel()

	rel := filepath.Join("bin", "pulumi-analyzer-mypack-linux-amd64")
	packDir := writeTestPack(t, map[string]string{"linux-amd64": rel})
	tgz, err := buildBinaryArtifact(packDir, rel, "mypack", "linux-amd64")
	require.NoError(t, err)

	finalDir := filepath.Join(t.TempDir(), "installed")
	// ctx is never used past the binary short-circuit: a nil-host context proves no
	// language runtime was resolved.
	err = installRequiredPolicy(nil, finalDir, io.NopCloser(bytes.NewReader(tgz)), io.Discard, io.Discard)
	require.NoError(t, err)

	bin, ok := workspace.PolicyPackBinary(finalDir)
	require.True(t, ok)
	assert.Equal(t, filepath.Join(finalDir, "pulumi-analyzer-mypack"), bin)
	info, err := os.Stat(bin)
	require.NoError(t, err)
	if goruntime.GOOS != "windows" {
		assert.NotZero(t, info.Mode()&0o111)
	}
}
```

Add import `goruntime "runtime"` to the test file. If `installRequiredPolicy` dereferences `ctx` before the short-circuit (it does not on master — first use is `ctx.Host.LanguageRuntime` after the python block), the nil-context call is safe; if that changes, construct a test context instead.

Run: `cd pkg && go test ./backend/httpstate/ -run 'TestPackLocationSelection|TestInstallRequiredPolicy' -count=1`
Expected: PASS

- [ ] **Step 6: Format and commit**

```bash
mise exec -- make format
git add pkg/backend/httpstate/policypack.go pkg/backend/httpstate/policypack_binary_test.go
git commit -m "Install the platform binary artifact for policy packs that publish one

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 8: Changelog, lint, full verification

**Files:**
- Create: `changelog/pending/20260714--cli--binary-policy-packs.yaml`

- [ ] **Step 1: Add the changelog entry**

```yaml
component: cli
kind: feat
body: "`pulumi policy publish` can now publish pre-built per-platform analyzer binaries alongside the source archive, letting consumers run policy packs without the pack's language toolchain"
time: 2026-07-14T00:00:00.000000+00:00
custom:
    PR: ""
```

(The PR number is filled in once the PR exists, per repo convention.)

- [ ] **Step 2: Run the full verification gauntlet**

```bash
mise exec -- make format
mise exec -- make lint
mise exec -- make tidy
```

Expected: no diffs from format, lint passes, tidy reports clean (only `sdk` and `pkg` modules were touched; `pkg` already depends on `sdk` via replace).

```bash
cd sdk && go test ./go/common/workspace/... ./go/common/apitype/... ./go/common/resource/plugin/... -count=1
cd ../pkg && go test ./backend/httpstate/... ./cmd/pulumi/policy/... ./host/... -count=1
```

Expected: PASS. If `make lint` or `make tidy` change files, include them in the commit.

- [ ] **Step 3: Run the fast test suite**

Run: `mise exec -- make test_fast`
Expected: PASS. This is the repo-mandated gate for `.go` changes. If pre-existing failures occur, verify they also fail on `master` before dismissing (`git stash` is forbidden — use the worktree's clean master checkout or `git worktree` if needed; do NOT use bare `git stash`).

- [ ] **Step 4: Commit**

```bash
git add changelog/pending/20260714--cli--binary-policy-packs.yaml
git commit -m "Add changelog entry for binary policy pack publishing

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

## Out of scope (tracked, not planned here)

- **Service repo:** `policy_pack_artifacts` table, per-platform upload URIs, platform-aware `RequiredPolicy` resolution, evaluator/TF-check exec path. All CLI changes in this plan are inert until the service returns the new fields.
- **Official pack repos:** `bun build --compile` CI matrices producing `bin/pulumi-analyzer-<name>-<os>-<arch>`.
- **pulumi-policy PR #452** (`version` in `PolicyPackArgs`): required before bun-compiled TS packs boot without `package.json`; independent of this repo.
