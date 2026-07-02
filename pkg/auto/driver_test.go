// Copyright 2026, Pulumi Corporation.

package auto

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeProject writes a minimal YAML Pulumi project into its own directory and returns the
// directory. YAML keeps the test hermetic: no providers, no SDK install, just the language
// host (which must be on PATH).
func writeProject(t *testing.T, root, name, program string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte(program), 0o600))
	return dir
}

func requireYAMLHost(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("pulumi-language-yaml"); err != nil {
		t.Skip("pulumi-language-yaml not on PATH; skipping in-process driver test")
	}
}

// TestDriver_PreviewUpOutputs proves the in-process driver previews, ups, and reads outputs
// of a stack with no CLI process, and that a second up is a no-op (idempotent replay).
func TestDriver_PreviewUpOutputs(t *testing.T) {
	t.Parallel()
	requireYAMLHost(t)

	root := t.TempDir()
	backendURL := "file://" + filepath.Join(root, "state")
	dir := writeProject(t, root, "single", `name: auto-single
runtime: yaml
outputs:
  message: hello, world
`)

	ctx := context.Background()
	s, err := Select(ctx, Options{BackendURL: backendURL, WorkDir: dir, Stack: "dev"})
	require.NoError(t, err)

	// Preview before the first up sees the stack as a change.
	_, _, err = s.Preview(ctx)
	require.NoError(t, err)

	res, err := s.Up(ctx)
	require.NoError(t, err)
	msg, ok := res.Outputs.GetOk("message")
	require.True(t, ok, "expected a message output")
	assert.Equal(t, "hello, world", msg.AsString())

	// A second up against unchanged source is a no-op: zero non-Same changes.
	res2, err := s.Up(ctx)
	require.NoError(t, err)
	var nonSame int
	for op, n := range res2.Changes {
		if op != "same" {
			nonSame += n
		}
	}
	assert.Equal(t, 0, nonSame, "second up should be a no-op, got changes: %v", res2.Changes)
}

// TestDriver_StackSettingsConfig proves Select loads the stack's own Pulumi.<stack>.yaml:
// its config resolves in the program (the per-stack region/size settings real stacks carry),
// project-level defaults apply beneath it, and the driver's Config overlay wins over both --
// the CLI's -c semantics. A secure: value without a caller-supplied secrets manager is a
// loud error, not a silent misread.
func TestDriver_StackSettingsConfig(t *testing.T) {
	t.Parallel()
	requireYAMLHost(t)

	root := t.TempDir()
	backendURL := "file://" + filepath.Join(root, "state")
	dir := writeProject(t, root, "settings", `name: auto-settings
runtime: yaml
config:
  region:
    type: string
  tier:
    type: string
    default: free
outputs:
  echoRegion: ${region}
  echoTier: ${tier}
`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.staging.yaml"), []byte(`config:
  auto-settings:region: us-test-1
`), 0o600))

	ctx := context.Background()
	s, err := Select(ctx, Options{BackendURL: backendURL, WorkDir: dir, Stack: "staging"})
	require.NoError(t, err)
	res, err := s.Up(ctx)
	require.NoError(t, err)
	region, ok := res.Outputs.GetOk("echoRegion")
	require.True(t, ok)
	assert.Equal(t, "us-test-1", region.AsString(), "stack settings config must resolve")
	tier, ok := res.Outputs.GetOk("echoTier")
	require.True(t, ok)
	assert.Equal(t, "free", tier.AsString(), "project defaults apply beneath the stack file")

	// The driver's own Config overlay wins over the stack file (CLI -c semantics).
	s2, err := Select(ctx, Options{
		BackendURL: backendURL, WorkDir: dir, Stack: "staging",
		Config: map[string]string{"auto-settings:region": "eu-override-1"},
	})
	require.NoError(t, err)
	res, err = s2.Up(ctx)
	require.NoError(t, err)
	region, ok = res.Outputs.GetOk("echoRegion")
	require.True(t, ok)
	assert.Equal(t, "eu-override-1", region.AsString(), "driver overlay wins over the stack file")

	// A secure: value the driver cannot decrypt is a loud, early error.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.sec.yaml"), []byte(`config:
  auto-settings:region:
    secure: AAABAJ7fZ9mQ==
`), 0o600))
	_, err = Select(ctx, Options{BackendURL: backendURL, WorkDir: dir, Stack: "sec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secure", "the error must name the secure-config gap")
}

// TestDriver_PreviewProjectsOutputs proves Preview returns the stack's projected outputs --
// what its outputs would be if the plan were applied -- before anything is created. A known
// (static) output is projected as known, which is what lets a delivery rollout's cascaded
// preview thread one stack's result into the next stack's previewed inputs.
func TestDriver_PreviewProjectsOutputs(t *testing.T) {
	t.Parallel()
	requireYAMLHost(t)

	root := t.TempDir()
	backendURL := "file://" + filepath.Join(root, "state")
	dir := writeProject(t, root, "projected", `name: auto-projected
runtime: yaml
outputs:
  message: hello, preview
`)

	ctx := context.Background()
	s, err := Select(ctx, Options{BackendURL: backendURL, WorkDir: dir, Stack: "dev"})
	require.NoError(t, err)

	res, _, err := s.Preview(ctx)
	require.NoError(t, err)
	msg, ok := res.Outputs.GetOk("message")
	require.True(t, ok, "preview should project the static stack output")
	assert.Equal(t, "hello, preview", msg.AsString(), "a known output is projected as known in preview")
}

// TestDriver_DefaultsToCurrentBackend proves Select resolves the ambient backend when no
// BackendURL is given -- the same one the CLI would use -- so a caller need not restate the
// backend it is already logged into. PULUMI_BACKEND_URL stands in for the current login.
func TestDriver_DefaultsToCurrentBackend(t *testing.T) {
	requireYAMLHost(t)

	root := t.TempDir()
	backendURL := "file://" + filepath.Join(root, "state")
	t.Setenv("PULUMI_BACKEND_URL", backendURL)

	dir := writeProject(t, root, "ambient", `name: auto-ambient
runtime: yaml
outputs:
  message: from the ambient backend
`)

	ctx := context.Background()
	s, err := Select(ctx, Options{WorkDir: dir, Stack: "dev"}) // no BackendURL
	require.NoError(t, err)
	res, err := s.Up(ctx)
	require.NoError(t, err)
	msg, ok := res.Outputs.GetOk("message")
	require.True(t, ok, "expected a message output")
	assert.Equal(t, "from the ambient backend", msg.AsString())
}

// TestDriver_CrossStackReference proves that a stack driven in-process resolves another
// stack's outputs through a StackReference -- the capability Pulumi Delivery's Stage relies
// on -- with both stacks living on the same file:// backend.
func TestDriver_CrossStackReference(t *testing.T) {
	t.Parallel()
	requireYAMLHost(t)

	root := t.TempDir()
	backendURL := "file://" + filepath.Join(root, "state")

	netDir := writeProject(t, root, "networking", `name: networking
runtime: yaml
outputs:
  vpcId: vpc-abc123
`)
	appDir := writeProject(t, root, "app", `name: app
runtime: yaml
resources:
  net:
    type: pulumi:pulumi:StackReference
    properties:
      name: organization/networking/dev
outputs:
  networkVpc: ${net.outputs["vpcId"]}
`)

	ctx := context.Background()

	netStack, err := Select(ctx, Options{BackendURL: backendURL, WorkDir: netDir, Stack: "dev"})
	require.NoError(t, err)
	_, err = netStack.Up(ctx)
	require.NoError(t, err)

	appStack, err := Select(ctx, Options{BackendURL: backendURL, WorkDir: appDir, Stack: "dev"})
	require.NoError(t, err)
	res, err := appStack.Up(ctx)
	require.NoError(t, err)

	vpc, ok := res.Outputs.GetOk("networkVpc")
	require.True(t, ok, "expected a networkVpc output")
	assert.Equal(t, "vpc-abc123", vpc.AsString(),
		"app should read networking's vpcId through the StackReference")
}
