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
