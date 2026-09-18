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

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	resourceconfig "github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
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

func TestApplySecretConfigMarksEncryptedValueSecure(t *testing.T) {
	t.Parallel()
	cfg := resourceconfig.Map{}
	require.NoError(t, applySecretConfig(t.Context(), cfg, map[string]string{"project:token": "secret"},
		resourceconfig.Base64Crypter))
	value := cfg[resourceconfig.MustMakeKey("project", "token")]
	assert.True(t, value.Secure())
	plaintext, err := value.Value(resourceconfig.Base64Crypter)
	require.NoError(t, err)
	assert.Equal(t, "secret", plaintext)
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

func TestDriver_PreviewManyReturnsNativePlanAndEvents(t *testing.T) {
	t.Parallel()
	requireYAMLHost(t)

	root := t.TempDir()
	backendURL := "file://" + filepath.Join(root, "state")
	producer := writeProject(t, root, "producer", `name: zzz-producer
runtime: yaml
outputs:
  message: from-producer
`)
	consumer := writeProject(t, root, "consumer", `name: aaa-consumer
runtime: yaml

resources:
  producer:
    type: pulumi:pulumi:StackReference
    properties:
      name: organization/zzz-producer/dev
outputs:
  message: ${producer.outputs["message"]}
`)
	results, err := PreviewMany(context.Background(), []Options{
		// Deliberately put the lexically earlier consumer first: the native graph must wait
		// for the co-previewed producer rather than accidentally relying on slice order.
		{BackendURL: backendURL, WorkDir: consumer, Stack: "dev"},
		{BackendURL: backendURL, WorkDir: producer, Stack: "dev"},
	})
	require.NoError(t, err)
	require.Len(t, results, 2)
	consumerURNs, producerURNs := map[string]bool{}, map[string]bool{}
	for i, result := range results {
		require.NotNil(t, result.Plan, "approval preview must retain the engine plan")
		require.NotEmpty(t, result.Plan.ResourcePlans, "member %d must have a nonempty resource plan", i)
		assert.NotEmpty(t, result.Events, "approval preview must retain native engine details")
		urns := consumerURNs
		wantProject := "aaa-consumer"
		if i == 1 {
			urns = producerURNs
			wantProject = "zzz-producer"
		}
		for urn := range result.Plan.ResourcePlans {
			urns[string(urn)] = true
			assert.Equal(t, wantProject, string(urn.Project()),
				"member %d's resources must carry its own project, not the other member's", i)
		}
	}
	// The two members' URN sets must be disjoint -- each ran as its own Deployment.
	for urn := range consumerURNs {
		assert.False(t, producerURNs[urn], "urn %q must not appear in both members' plans", urn)
	}
	// A missing co-preview dependency would fail while evaluating the consumer's output.
	// Success with the consumer first proves the waiter used the producer's native preview.
}

// TestDriver_PreviewManyDistinctDefaultProviderURNs is the direct regression test for the
// "Duplicate resource URN ... pulumi:providers:pulumi::default" failure: three members share the
// stack name "prod" but have distinct projects, and two of them each need their own default
// `pulumi` provider (via a StackReference) -- previously, the shared-engine multistack path
// stamped every member's resources, including that default provider, under whichever member was
// first in the slice, so the second member's registration collided. Each member now runs as its
// own Deployment, so every resource -- including default providers -- must carry that member's
// own project+stack, in either spec order.
func TestDriver_PreviewManyDistinctDefaultProviderURNs(t *testing.T) {
	t.Parallel()
	requireYAMLHost(t)

	root := t.TempDir()
	backendURL := "file://" + filepath.Join(root, "state")

	sharedTarget := writeProject(t, root, "shared-target", `name: shared-target
runtime: yaml
outputs:
  value: shared
`)
	_, err := UpMany(context.Background(), []Options{{BackendURL: backendURL, WorkDir: sharedTarget, Stack: "dev"}})
	require.NoError(t, err)

	// No StackReference -- mirrors the live shape where the URN was wrongly stamped under the
	// member that did not even own the colliding registration.
	foundation := writeProject(t, root, "foundation", `name: foundation
runtime: yaml
outputs:
  ok: "true"
`)
	referrerA := writeProject(t, root, "referrer-a", `name: referrer-a
runtime: yaml
resources:
  ref:
    type: pulumi:pulumi:StackReference
    properties:
      name: organization/shared-target/dev
outputs:
  value: ${ref.outputs["value"]}
`)
	referrerB := writeProject(t, root, "referrer-b", `name: referrer-b
runtime: yaml
resources:
  ref:
    type: pulumi:pulumi:StackReference
    properties:
      name: organization/shared-target/dev
outputs:
  value: ${ref.outputs["value"]}
`)

	orders := [][]string{
		{foundation, referrerA, referrerB},
		{referrerB, referrerA, foundation},
	}
	for _, order := range orders {
		specs := make([]Options, len(order))
		for i, dir := range order {
			specs[i] = Options{BackendURL: backendURL, WorkDir: dir, Stack: "prod"}
		}
		results, err := PreviewMany(context.Background(), specs)
		require.NoError(t, err)
		require.Len(t, results, 3)
		for i, result := range results {
			require.NotNil(t, result.Plan, "member %d must have its own plan", i)
			require.NotEmpty(t, result.Plan.ResourcePlans, "member %d must have a nonempty resource plan", i)
			wantProject := filepath.Base(order[i])
			for urn := range result.Plan.ResourcePlans {
				assert.Equal(t, wantProject, string(urn.Project()),
					"member %d's resources must carry its own project", i)
				assert.Equal(t, "prod", string(urn.Stack()))
			}
		}
	}
}

// TestDriver_PreviewManyOnlyChangedMemberPlansChanges is the existing-state differential test:
// once both members have real state, previewing unchanged programs must plan zero non-Same
// changes for either member, and changing only one member's program must plan changes for that
// member alone.
func TestDriver_PreviewManyOnlyChangedMemberPlansChanges(t *testing.T) {
	t.Parallel()
	requireYAMLHost(t)

	root := t.TempDir()
	backendURL := "file://" + filepath.Join(root, "state")

	// Two referenceable targets so member B's StackReference can point at one, then the
	// other -- a real resource-property change with a real diff, unlike a bare stack output
	// (which the yaml host doesn't diff, since it's not an input of any resource).
	target1 := writeProject(t, root, "target1", `name: target1
runtime: yaml
outputs:
  v: "1"
`)
	target2 := writeProject(t, root, "target2", `name: target2
runtime: yaml
outputs:
  v: "2"
`)
	for _, dir := range []string{target1, target2} {
		_, err := UpMany(context.Background(), []Options{{BackendURL: backendURL, WorkDir: dir, Stack: "dev"}})
		require.NoError(t, err)
	}

	memberA := writeProject(t, root, "member-a", `name: member-a
runtime: yaml
outputs:
  value: a-v1
`)
	memberB := writeProject(t, root, "member-b", `name: member-b
runtime: yaml
resources:
  ref:
    type: pulumi:pulumi:StackReference
    properties:
      name: organization/target1/dev
`)
	specs := []Options{
		{BackendURL: backendURL, WorkDir: memberA, Stack: "dev"},
		{BackendURL: backendURL, WorkDir: memberB, Stack: "dev"},
	}
	_, err := UpMany(context.Background(), specs)
	require.NoError(t, err)

	nonSame := func(changes display.ResourceChanges) int {
		total := 0
		for op, count := range changes {
			if op != deploy.OpSame {
				total += count
			}
		}
		return total
	}

	results, err := PreviewMany(context.Background(), specs)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Zero(t, nonSame(results[0].Changes), "member A expected no changes")
	assert.Zero(t, nonSame(results[1].Changes), "member B expected no changes")

	// Change only member B's program: point its StackReference at the other target.
	require.NoError(t, os.WriteFile(filepath.Join(memberB, "Pulumi.yaml"), []byte(`name: member-b
runtime: yaml
resources:
  ref:
    type: pulumi:pulumi:StackReference
    properties:
      name: organization/target2/dev
`), 0o600))

	results, err = PreviewMany(context.Background(), specs)
	require.NoError(t, err)
	assert.Zero(t, nonSame(results[0].Changes), "member A must plan no changes after only B changed")
	assert.Positive(t, nonSame(results[1].Changes), "member B's changed StackReference target must plan a change")
}

// TestDriver_PreviewManyConsumerSeesProducersProjectedOutput proves the co-deployed output
// waiter is actually wired into the Deployment (the constructor hookup the review found
// missing): the producer has existing committed state with an old output value; its program is
// then changed to a new value; co-previewing both, with the consumer ordered first, must resolve
// the consumer's StackReference to the producer's currently-previewed NEW value, not the
// backend's stale committed one.
func TestDriver_PreviewManyConsumerSeesProducersProjectedOutput(t *testing.T) {
	t.Parallel()
	requireYAMLHost(t)

	root := t.TempDir()
	backendURL := "file://" + filepath.Join(root, "state")

	producer := writeProject(t, root, "zzz-cascade-producer", `name: zzz-cascade-producer
runtime: yaml
outputs:
  message: old-value
  secretMessage:
    fn::secret: old-secret
`)
	consumer := writeProject(t, root, "aaa-cascade-consumer", `name: aaa-cascade-consumer
runtime: yaml
resources:
  ref:
    type: pulumi:pulumi:StackReference
    properties:
      name: organization/zzz-cascade-producer/dev
outputs:
  message: ${ref.outputs["message"]}
  secretMessage: ${ref.outputs["secretMessage"]}
`)

	_, err := UpMany(context.Background(), []Options{{BackendURL: backendURL, WorkDir: producer, Stack: "dev"}})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(producer, "Pulumi.yaml"), []byte(`name: zzz-cascade-producer
runtime: yaml
outputs:
  message: new-value
  secretMessage:
    fn::secret: new-secret
`), 0o600))

	results, err := PreviewMany(context.Background(), []Options{
		{BackendURL: backendURL, WorkDir: consumer, Stack: "dev"},
		{BackendURL: backendURL, WorkDir: producer, Stack: "dev"},
	})
	require.NoError(t, err)
	require.Len(t, results, 2)

	// The secret marking on the producer's projected output must survive the co-deployed
	// StackReference cascade end to end -- through NewDeployment's WithOutputWaiters wiring,
	// WaitForOutputs, and readStackReference -- not just the plaintext value.
	secretMsg, ok := results[0].Outputs.GetOk("secretMessage")
	require.True(t, ok, "consumer's projected secret output must resolve")
	assert.True(t, secretMsg.Secret(), "the producer's secret output must stay marked secret through the cascade")
	assert.Equal(t, "new-secret", secretMsg.AsString())

	msg, ok := results[0].Outputs.GetOk("message")
	require.True(t, ok, "consumer's projected output must resolve")
	assert.Equal(t, "new-value", msg.AsString(),
		"consumer must see the producer's currently-previewed value, not stale backend state")
}

// TestDriver_PreviewManyFailingMemberFailsWholeBatch proves that a member which fails during
// preview fails the entire PreviewMany call, rather than the other member's provisional success
// masking it.
func TestDriver_PreviewManyFailingMemberFailsWholeBatch(t *testing.T) {
	t.Parallel()
	requireYAMLHost(t)

	root := t.TempDir()
	backendURL := "file://" + filepath.Join(root, "state")

	failing := writeProject(t, root, "failing-member", `name: failing-member
runtime: yaml
resources:
  ref:
    type: pulumi:pulumi:StackReference
    properties:
      name: organization/does-not-exist/dev
outputs:
  never: ${ref.outputs["nope"]}
`)
	okMember := writeProject(t, root, "ok-member", `name: ok-member
runtime: yaml
outputs:
  fine: "true"
`)

	_, err := PreviewMany(context.Background(), []Options{
		{BackendURL: backendURL, WorkDir: okMember, Stack: "dev"},
		{BackendURL: backendURL, WorkDir: failing, Stack: "dev"},
	})
	require.Error(t, err)
}

// TestDriver_PreviewManyCycleFailsWithError proves that two co-deployed members whose
// StackReferences point at each other terminate with an explicit circular-dependency error
// instead of hanging forever.
func TestDriver_PreviewManyCycleFailsWithError(t *testing.T) {
	t.Parallel()
	requireYAMLHost(t)

	root := t.TempDir()
	backendURL := "file://" + filepath.Join(root, "state")

	a := writeProject(t, root, "cycle-a", `name: cycle-a
runtime: yaml
resources:
  ref:
    type: pulumi:pulumi:StackReference
    properties:
      name: organization/cycle-b/dev
outputs:
  value: ${ref.outputs["value"]}
`)
	b := writeProject(t, root, "cycle-b", `name: cycle-b
runtime: yaml
resources:
  ref:
    type: pulumi:pulumi:StackReference
    properties:
      name: organization/cycle-a/dev
outputs:
  value: ${ref.outputs["value"]}
`)

	_, err := PreviewMany(context.Background(), []Options{
		{BackendURL: backendURL, WorkDir: a, Stack: "dev"},
		{BackendURL: backendURL, WorkDir: b, Stack: "dev"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency detected")
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
