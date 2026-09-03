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

package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// fakeEnvStore is an in-memory stand-in for a single named ESC environment, modelled as a revision graph
// rather than as a single head: creating a revision from a parent appends to history without moving
// `latest`, exactly as the service does.
//
// The mock's latest-moving update function field is deliberately left nil. MockEnvironmentsBackend panics
// on an unset function field, so any test that reaches that route fails loudly.
type fakeEnvStore struct {
	t *testing.T

	// definitions maps a revision number to the definition stored at it, and parents maps it to the
	// revision it was created from.
	definitions map[int]string
	parents     map[int]int

	// revision is the revision `latest` points at. Creating a revision never moves it.
	revision int

	// exists reports whether the environment has been created.
	exists bool

	gets            int
	creates         int
	lastParent      int
	lastVersionRead string
	revisionCalls   map[string]int
}

func newFakeEnvStore(t *testing.T, yaml string) *fakeEnvStore {
	return &fakeEnvStore{
		t:             t,
		definitions:   map[int]string{1: yaml},
		parents:       map[int]int{1: 0},
		revision:      1,
		exists:        true,
		revisionCalls: map[string]int{},
	}
}

// nextRevision is the number the next created revision will get.
func (s *fakeEnvStore) nextRevision() int {
	n := s.revision
	for k := range s.definitions {
		if k > n {
			n = k
		}
	}
	return n + 1
}

// head is the definition at the most recently created revision, which is what a stack whose pointer was
// just rewritten reads.
func (s *fakeEnvStore) head() string {
	return s.definitions[s.nextRevision()-1]
}

func (s *fakeEnvStore) backend() *backend.MockEnvironmentsBackend {
	return &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{
			NameF: func() string { return "test" },
		},
		GetEnvironmentDefinitionF: func(
			_ context.Context, org, envProject, envName, version string,
		) ([]byte, string, int, error) {
			assert.Equal(s.t, "test-org", org)
			assert.Equal(s.t, "payments", envProject)
			assert.Equal(s.t, "dev", envName)
			if !s.exists {
				return nil, "", 0, fmt.Errorf("%w: payments/dev", backend.ErrEnvironmentNotFound)
			}
			s.gets++
			s.lastVersionRead = version

			revision := s.revision
			if version != "" {
				n, err := strconv.Atoi(version)
				if err != nil {
					return nil, "", 0, fmt.Errorf(
						"%w: payments/dev@%s", backend.ErrEnvironmentNotFound, version)
				}
				revision = n
			}
			def, ok := s.definitions[revision]
			if !ok {
				return nil, "", 0, fmt.Errorf(
					"%w: payments/dev@%d", backend.ErrEnvironmentNotFound, revision)
			}
			return []byte(def), fmt.Sprintf("etag-%d", revision), revision, nil
		},
		CreateEnvironmentRevisionFromParentF: func(
			_ context.Context, org, envProject, envName string, yaml []byte, parent int,
		) (apitype.EnvironmentDiagnostics, int, error) {
			s.creates++
			s.lastParent = parent
			if _, ok := s.definitions[parent]; !ok {
				return nil, 0, fmt.Errorf("%w: payments/dev@%d", backend.ErrEnvironmentNotFound, parent)
			}
			revision := s.nextRevision()
			s.definitions[revision] = string(yaml)
			s.parents[revision] = parent
			// `latest` deliberately does not move.
			return nil, revision, nil
		},
		GetEnvironmentRevisionF: func(
			_ context.Context, org, envProject, envName, version string,
		) (int, error) {
			ref := envProject + "/" + envName
			s.revisionCalls[ref]++
			switch ref {
			case "payments/dev":
				if version == "" {
					return s.revision, nil
				}
				if n, err := strconv.Atoi(version); err == nil {
					return n, nil
				}
				return 0, fmt.Errorf("no such version %v", version)
			case "payments/base":
				return 2, nil
			}
			return 0, fmt.Errorf("no such environment %v", ref)
		},
	}
}

func mainEnvStack(be backend.Backend) *backend.MockStack {
	return &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				NameV:               tokens.MustParseStackName("testStack"),
				FullyQualifiedNameV: "test-org/testProject/testStack",
			}
		},
		ConfigLocationF: func() backend.StackConfigLocation { return backend.StackConfigLocation{} },
		OrgNameF:        func() string { return "test-org" },
		BackendF:        func() backend.Backend { return be },
	}
}

func mainEnvLoginManager(be backend.Backend) cmdBackend.LoginManager {
	return &cmdBackend.MockLoginManager{
		CurrentF: func(
			context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool,
		) (backend.Backend, error) {
			return be, nil
		},
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool,
			colors.Colorization,
		) (backend.Backend, error) {
			return be, nil
		},
	}
}

func loadStackFile(t *testing.T, project *workspace.Project, text string) *workspace.ProjectStack {
	t.Helper()
	ps, err := workspace.LoadProjectStackBytes(
		cmdutil.Diag(), project, []byte(text), "Pulumi.testStack.yaml", encoding.YAML)
	require.NoError(t, err)
	return ps
}

func newMainEnvSetCmd(ps *workspace.ProjectStack, stdout *bytes.Buffer) *configSetCmd {
	return &configSetCmd{
		Stdout: stdout,
		LoadProjectStack: func(
			context.Context, diag.Sink, *workspace.Project, backend.Stack, string,
		) (*workspace.ProjectStack, error) {
			return ps, nil
		},
	}
}

//
// Precedence (DoD 3).
//

func TestEffectiveStackEnv(t *testing.T) {
	t.Parallel()

	project := &workspace.Project{Name: "testProject"}
	localStack := mainEnvStack(nil)

	t.Run("neither field", func(t *testing.T) {
		t.Parallel()

		ps := loadStackFile(t, project, "config:\n  testProject:foo: bar\n")
		envDef, mainEnv, warnings := effectiveStackEnv(localStack, ps)
		assert.Nil(t, envDef)
		assert.Nil(t, mainEnv)
		assert.Empty(t, warnings)
	})

	t.Run("environment only", func(t *testing.T) {
		t.Parallel()

		ps := loadStackFile(t, project, "environment:\n  - payments/base\n")
		envDef, mainEnv, warnings := effectiveStackEnv(localStack, ps)
		assert.Equal(t, ps.Environment, envDef)
		assert.Nil(t, mainEnv)
		assert.Empty(t, warnings)
	})

	t.Run("mainEnvironment only", func(t *testing.T) {
		t.Parallel()

		ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")
		envDef, mainEnv, warnings := effectiveStackEnv(localStack, ps)
		require.NotNil(t, mainEnv)
		assert.Equal(t, "payments/dev", mainEnv.String())
		assert.JSONEq(t, `{"imports":["payments/dev"]}`, string(envDef.Definition()))
		assert.Empty(t, warnings)
	})

	t.Run("mainEnvironment pinned", func(t *testing.T) {
		t.Parallel()

		ps := loadStackFile(t, project, "mainEnvironment: payments/dev@4\n")
		envDef, mainEnv, _ := effectiveStackEnv(localStack, ps)
		require.NotNil(t, mainEnv)
		assert.JSONEq(t, `{"imports":["payments/dev@4"]}`, string(envDef.Definition()))
	})

	t.Run("mainEnvironment wins over environment", func(t *testing.T) {
		t.Parallel()

		ps := loadStackFile(t, project, "mainEnvironment: payments/dev\nenvironment:\n  - other/env\n")
		envDef, mainEnv, warnings := effectiveStackEnv(localStack, ps)
		require.NotNil(t, mainEnv)
		assert.JSONEq(t, `{"imports":["payments/dev"]}`, string(envDef.Definition()))
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "'environment' is ignored")
	})

	t.Run("remote stack config wins over mainEnvironment", func(t *testing.T) {
		t.Parallel()

		remote := mainEnvStack(nil)
		remote.ConfigLocationF = func() backend.StackConfigLocation {
			return backend.StackConfigLocation{IsRemote: true}
		}

		ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")
		envDef, mainEnv, warnings := effectiveStackEnv(remote, ps)
		assert.Nil(t, envDef)
		assert.Nil(t, mainEnv)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "'mainEnvironment' is ignored")
	})
}

//
// Reads (DoD 2 and DoD 8).
//

func TestMainEnvironmentReadResolvesThroughEnvironment(t *testing.T) {
	t.Parallel()

	var openedYAML string
	be := &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{NameF: func() string { return "test" }},
		OpenYAMLEnvironmentF: func(
			_ context.Context, org string, yaml []byte, _ time.Duration, _ map[string]string,
		) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			assert.Equal(t, "test-org", org)
			openedYAML = string(yaml)
			return &esc.Environment{Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(map[string]esc.Value{
					// Defined by the main environment itself.
					"testProject:instanceCount": {
						Value: "6",
						Trace: esc.Trace{Def: esc.Range{Environment: "payments/dev"}},
					},
					// Inherited through the main environment's own `imports:`.
					"testProject:logLevel": {
						Value: "info",
						Trace: esc.Trace{Def: esc.Range{Environment: "payments/base"}},
					},
				}),
			}}, nil, nil
		},
		GetEnvironmentRevisionF: func(_ context.Context, _, envProject, envName, _ string) (int, error) {
			if envProject+"/"+envName == "payments/dev" {
				return 8, nil
			}
			return 0, errors.New("unexpected environment")
		},
	}
	s := mainEnvStack(be)
	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")

	var stderr bytes.Buffer
	cfg, err := getStackConfigurationFromProjectStackWithWriter(
		t.Context(), &stderr, s, project, nil, ps, nil)
	require.NoError(t, err)

	// The main environment is resolved as a single import, so ESC resolves its own imports server-side.
	assert.JSONEq(t, `{"imports":["payments/dev"]}`, openedYAML)
	assert.Equal(t, []string{"payments/dev"}, cfg.EnvironmentImports)

	values, ok := cfg.Environment.Value.(map[string]esc.Value)
	require.True(t, ok)
	assert.Contains(t, values, "testProject:instanceCount")
	assert.Contains(t, values, "testProject:logLevel")

	// DoD 8: the revision the run resolved against is reported.
	assert.Contains(t, stderr.String(), "Config source: payments/dev@8")
}

func TestNonMainEnvironmentReadPrintsNoConfigSource(t *testing.T) {
	t.Parallel()

	be := &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{NameF: func() string { return "test" }},
		OpenYAMLEnvironmentF: func(
			_ context.Context, _ string, _ []byte, _ time.Duration, _ map[string]string,
		) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			return &esc.Environment{Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(map[string]esc.Value{"testProject:a": esc.NewValue("b")}),
			}}, nil, nil
		},
	}
	s := mainEnvStack(be)
	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "environment:\n  - payments/base\n")

	var stderr bytes.Buffer
	_, err := getStackConfigurationFromProjectStackWithWriter(t.Context(), &stderr, s, project, nil, ps, nil)
	require.NoError(t, err)
	assert.NotContains(t, stderr.String(), "Config source:")
}

//
// Writes (DoD 4, 5, 6).
//

func TestMainEnvironmentConfigSet(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "values:\n  pulumiConfig:\n    testProject:existing: keep\n")
	s := mainEnvStack(store.backend())
	project := &workspace.Project{Name: "testProject"}
	const stackYAML = "mainEnvironment: payments/dev\n"
	ps := loadStackFile(t, project, stackYAML)
	configFile := writeStackFile(t, stackYAML)

	var stdout bytes.Buffer
	cmd := newMainEnvSetCmd(ps, &stdout)
	ws := &pkgWorkspace.MockContext{}

	require.NoError(t, cmd.Run(t.Context(), ws, []string{"testProject:instanceCount", "6"}, project, s, configFile))
	require.NoError(t, cmd.Run(t.Context(), ws, []string{"testProject:region", "us-west-2"}, project, s, configFile))

	// Successive writes chain: the second branches from the revision the first created, not from latest.
	assert.Equal(t,
		"Created payments/dev@2 (parent @1).\n"+
			"Pulumi.testStack.yaml now points at @2; latest is still @1.\n"+
			"Created payments/dev@3 (parent @2).\n"+
			"Pulumi.testStack.yaml now points at @3; latest is still @1.\n",
		stdout.String())
	assert.Equal(t, 2, store.creates)
	// `latest` never moved.
	assert.Equal(t, 1, store.revision)

	// Untyped values keep `pulumi config set` semantics: "6" stays a string.
	assert.Contains(t, store.head(), `testProject:instanceCount: "6"`)
	assert.Contains(t, store.head(), "testProject:region: us-west-2")
	// Existing values are preserved by the read-modify-write.
	assert.Contains(t, store.head(), "testProject:existing: keep")

	// The stack file now names the revision that was created.
	saved, err := os.ReadFile(configFile)
	require.NoError(t, err)
	assert.Equal(t, "mainEnvironment: payments/dev@3\n", string(saved))
}

// TestMainEnvironmentConfigSetBranchesFromPinnedParent asserts a write branches from the revision the stack
// file names, not from a separately resolved `latest`, and that the pinned stack is no longer refused.
func TestMainEnvironmentConfigSetBranchesFromPinnedParent(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
	// The environment's history has moved on without this stack: `latest` is 7, the stack is pinned to 5.
	store.definitions[5] = "values:\n  pulumiConfig:\n    testProject:existing: pinned\n"
	store.definitions[7] = "values:\n  pulumiConfig:\n    testProject:existing: newer\n"
	store.revision = 7

	s := mainEnvStack(store.backend())
	project := &workspace.Project{Name: "testProject"}
	const stackYAML = "mainEnvironment: payments/dev@5\n"
	ps := loadStackFile(t, project, stackYAML)
	configFile := writeStackFile(t, stackYAML)

	var stdout bytes.Buffer
	cmd := newMainEnvSetCmd(ps, &stdout)
	require.NoError(t, cmd.Run(
		t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:a", "b"}, project, s, configFile))

	// The read happened at the pin, and the create branched from exactly the revision that read reported.
	assert.Equal(t, "5", store.lastVersionRead)
	assert.Equal(t, 5, store.lastParent)
	assert.Equal(t, 1, store.gets)
	assert.Equal(t, 1, store.creates)

	// The new revision descends from @5's content, not from @7's.
	assert.Contains(t, store.head(), "testProject:existing: pinned")
	assert.Contains(t, store.head(), "testProject:a: b")

	assert.Equal(t,
		"Created payments/dev@8 (parent @5).\n"+
			"Pulumi.testStack.yaml now points at @8; latest is still @7.\n",
		stdout.String())

	saved, err := os.ReadFile(configFile)
	require.NoError(t, err)
	assert.Equal(t, "mainEnvironment: payments/dev@8\n", string(saved))
}

// TestMainEnvironmentConfigSetRewritesPointer asserts the pointer is the only thing a write changes in the
// stack file: every other key, and the file's trivia, survive byte for byte.
func TestMainEnvironmentConfigSetRewritesPointer(t *testing.T) {
	t.Parallel()

	const stackYAML = `# how this stack gets its configuration
mainEnvironment: payments/dev
secretsprovider: passphrase
encryptionsalt: v1:saltysalt:v1:abc:def
config:
  # left alone by a write to the environment
  testProject:legacy: "9"
`

	store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
	s := mainEnvStack(store.backend())
	project := &workspace.Project{Name: "testProject"}
	configFile := writeStackFile(t, stackYAML)

	// Loading from bytes populates the raw representation the trivia-preserving save edits.
	ps := loadStackFile(t, project, stackYAML)

	var stdout, stderr bytes.Buffer
	cmd := newMainEnvSetCmd(ps, &stdout)
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Run(
		t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:a", "b"}, project, s, configFile))

	saved, err := os.ReadFile(configFile)
	require.NoError(t, err)
	assert.Equal(t,
		strings.Replace(stackYAML, "mainEnvironment: payments/dev\n", "mainEnvironment: payments/dev@2\n", 1),
		string(saved))
}

func TestMainEnvironmentConfigSetTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		typ      string
		value    string
		expected string
	}{
		{"", "6", `testProject:test: "6"`},
		{"string", "6", `testProject:test: "6"`},
		{"int", "6", "testProject:test: 6"},
		{"bool", "true", "testProject:test: true"},
		{"float", "1.5", "testProject:test: 1.5"},
	}
	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			t.Parallel()

			store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
			s := mainEnvStack(store.backend())
			project := &workspace.Project{Name: "testProject"}
			const stackYAML = "mainEnvironment: payments/dev\n"
			ps := loadStackFile(t, project, stackYAML)

			var stdout bytes.Buffer
			cmd := newMainEnvSetCmd(ps, &stdout)
			cmd.Type = c.typ
			require.NoError(t, cmd.Run(
				t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:test", c.value},
				project, s, writeStackFile(t, stackYAML)))

			assert.Contains(t, store.head(), c.expected)
		})
	}
}

func TestMainEnvironmentConfigSetSecret(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
	s := mainEnvStack(store.backend())
	project := &workspace.Project{Name: "testProject"}
	const stackYAML = "mainEnvironment: payments/dev\n"
	ps := loadStackFile(t, project, stackYAML)
	configFile := writeStackFile(t, stackYAML)

	var stdout, stderr bytes.Buffer
	cmd := newMainEnvSetCmd(ps, &stdout)
	cmd.Stderr = &stderr
	cmd.Secret = true
	// A stack secrets manager is never loaded on this path; reaching for a snapshot would prove it was.
	s.SnapshotF = func(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
		require.FailNow(t, "the stack's secrets manager must not be consulted")
		return nil, nil
	}
	require.NoError(t, cmd.Run(
		t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:token", "hunter2"},
		project, s, configFile))

	// The value is an ESC secret, not a stack-secrets-manager ciphertext.
	assert.Contains(t, store.head(), "fn::secret")
	assert.Contains(t, store.head(), "hunter2")
	// No secrets provider was ever configured for the stack...
	assert.Empty(t, ps.SecretsProvider)
	assert.Empty(t, ps.EncryptionSalt)
	assert.Empty(t, ps.EncryptedKey)
	// ...the only local change is the pointer, so the plaintext never reaches the stack file...
	saved, err := os.ReadFile(configFile)
	require.NoError(t, err)
	assert.Equal(t, "mainEnvironment: payments/dev@2\n", string(saved))
	assert.NotContains(t, string(saved), "hunter2")
	// ...and it never reaches the command's output either.
	assert.NotContains(t, stdout.String(), "hunter2")
	assert.NotContains(t, stderr.String(), "hunter2")
	assert.Equal(t,
		"Created payments/dev@2 (parent @1).\n"+
			"Pulumi.testStack.yaml now points at @2; latest is still @1.\n",
		stdout.String())
}

// TestMainEnvironmentConfigSetOmitsUnknownLatest asserts a failed `latest` lookup costs the command only
// the clause that reports it: the revision was created and the pointer written, so this is not an error.
func TestMainEnvironmentConfigSetOmitsUnknownLatest(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
	be := store.backend()
	be.GetEnvironmentRevisionF = func(
		context.Context, string, string, string, string,
	) (int, error) {
		return 0, errors.New("service unavailable")
	}
	s := mainEnvStack(be)
	project := &workspace.Project{Name: "testProject"}
	const stackYAML = "mainEnvironment: payments/dev\n"
	ps := loadStackFile(t, project, stackYAML)
	configFile := writeStackFile(t, stackYAML)

	var stdout bytes.Buffer
	cmd := newMainEnvSetCmd(ps, &stdout)
	require.NoError(t, cmd.Run(
		t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:a", "b"}, project, s, configFile))

	assert.Equal(t,
		"Created payments/dev@2 (parent @1).\nPulumi.testStack.yaml now points at @2.\n",
		stdout.String())
	saved, err := os.ReadFile(configFile)
	require.NoError(t, err)
	assert.Equal(t, "mainEnvironment: payments/dev@2\n", string(saved))
}

// TestMainEnvironmentConfigSetDiagnostics asserts a rejected definition is reported and leaves the stack
// file exactly as it was: there is no revision to point at.
func TestMainEnvironmentConfigSetDiagnostics(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
	be := store.backend()
	be.CreateEnvironmentRevisionFromParentF = func(
		context.Context, string, string, string, []byte, int,
	) (apitype.EnvironmentDiagnostics, int, error) {
		return apitype.EnvironmentDiagnostics{{Summary: "boom"}}, 0, nil
	}
	s := mainEnvStack(be)
	project := &workspace.Project{Name: "testProject"}
	const stackYAML = "mainEnvironment: payments/dev\n"
	ps := loadStackFile(t, project, stackYAML)
	configFile := writeStackFile(t, stackYAML)

	var stdout bytes.Buffer
	cmd := newMainEnvSetCmd(ps, &stdout)
	err := cmd.Run(
		t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:a", "b"}, project, s, configFile)

	require.ErrorContains(t, err, "creating a revision of environment payments/dev: too many errors")
	assert.Contains(t, stdout.String(), "boom")
	assert.Empty(t, ps.MainEnvironment.Version)
	saved, readErr := os.ReadFile(configFile)
	require.NoError(t, readErr)
	assert.Equal(t, stackYAML, string(saved))
}

// TestMainEnvironmentConfigSetConflict asserts a rejected parent fails with the existing re-run guidance and
// leaves the pointer alone.
func TestMainEnvironmentConfigSetConflict(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
	be := store.backend()
	be.CreateEnvironmentRevisionFromParentF = func(
		context.Context, string, string, string, []byte, int,
	) (apitype.EnvironmentDiagnostics, int, error) {
		return nil, 0, fmt.Errorf("%w: payments/dev", backend.ErrEnvironmentConflict)
	}
	s := mainEnvStack(be)
	project := &workspace.Project{Name: "testProject"}
	const stackYAML = "mainEnvironment: payments/dev\n"
	ps := loadStackFile(t, project, stackYAML)
	configFile := writeStackFile(t, stackYAML)

	var stdout bytes.Buffer
	cmd := newMainEnvSetCmd(ps, &stdout)
	err := cmd.Run(
		t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:a", "b"}, project, s, configFile)

	require.ErrorContains(t, err, "environment payments/dev changed since it was read")
	require.ErrorContains(t, err, "re-run the command")
	assert.Empty(t, ps.MainEnvironment.Version)
	saved, readErr := os.ReadFile(configFile)
	require.NoError(t, readErr)
	assert.Equal(t, stackYAML, string(saved))
}

// TestMainEnvironmentConfigSetRouteUnavailable asserts a 404 from the create route -- a closed rollout gate,
// an organization that is not enabled, or a service that predates the route -- fails with a distinct,
// actionable message rather than falling back to a write that would move `latest`.
func TestMainEnvironmentConfigSetRouteUnavailable(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
	be := store.backend()
	be.CreateEnvironmentRevisionFromParentF = func(
		context.Context, string, string, string, []byte, int,
	) (apitype.EnvironmentDiagnostics, int, error) {
		return nil, 0, fmt.Errorf("%w: payments/dev", backend.ErrEnvironmentNotFound)
	}
	s := mainEnvStack(be)
	project := &workspace.Project{Name: "testProject"}
	const stackYAML = "mainEnvironment: payments/dev\n"
	ps := loadStackFile(t, project, stackYAML)
	configFile := writeStackFile(t, stackYAML)

	var stdout bytes.Buffer
	cmd := newMainEnvSetCmd(ps, &stdout)
	err := cmd.Run(
		t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:a", "b"}, project, s, configFile)

	require.ErrorContains(t, err, "creating a revision of environment payments/dev")
	require.ErrorContains(t, err, "revision branching is not available for organization \"test-org\"")
	require.ErrorContains(t, err, "or the environment does not exist")
	// The stack file is untouched, and nothing fell back to a latest-moving write: the store leaves that
	// function field nil, so any such call would have panicked.
	saved, readErr := os.ReadFile(configFile)
	require.NoError(t, readErr)
	assert.Equal(t, stackYAML, string(saved))
}

// TestMainEnvironmentWriteSaveFails asserts that when the revision is created but the stack file cannot be
// saved, the error names the revision and the exact line that adopts it by hand. The revision is real and
// cannot be rolled back, so it must not be silently orphaned.
func TestMainEnvironmentWriteSaveFails(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
	s := mainEnvStack(store.backend())
	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")
	mainEnv := activeMainEnvironment(io.Discard, s, ps)
	require.NotNil(t, mainEnv)

	w, err := newMainEnvWriter(s, ps, mainEnv, "Pulumi.testStack.yaml")
	require.NoError(t, err)
	w.save = func(context.Context, backend.Stack, *workspace.ProjectStack, string) error {
		return errors.New("read-only file system")
	}

	var stdout bytes.Buffer
	node, err := ConfigValueNode("b", "", false)
	require.NoError(t, err)
	_, err = w.setKey(t.Context(), &stdout, config.MustMakeKey("testProject", "a"), node)

	require.ErrorContains(t, err, "created environment revision payments/dev@2")
	require.ErrorContains(t, err, "read-only file system")
	require.ErrorContains(t, err, "set 'mainEnvironment: payments/dev@2' in Pulumi.testStack.yaml by hand")
	assert.Equal(t, 1, store.creates)
}

func TestMainEnvironmentConfigRemove(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t,
		"values:\n  pulumiConfig:\n    testProject:a: one\n    testProject:b: two\n")
	s := mainEnvStack(store.backend())
	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")

	mainEnv := activeMainEnvironment(io.Discard, s, ps)
	require.NotNil(t, mainEnv)
	w, err := newMainEnvWriter(s, ps, mainEnv, writeStackFile(t, "mainEnvironment: payments/dev\n"))
	require.NoError(t, err)

	var stdout bytes.Buffer
	res, removed, err := w.removeKey(t.Context(), &stdout, config.MustMakeKey("testProject", "a"))
	require.NoError(t, err)
	assert.True(t, removed)
	assert.Equal(t, writeResult{Revision: 2, Parent: 1, Latest: 1}, res)
	assert.NotContains(t, store.head(), "testProject:a")
	assert.Contains(t, store.head(), "testProject:b: two")

	// Removing a key that isn't there creates no revision.
	res, removed, err = w.removeKey(t.Context(), &stdout, config.MustMakeKey("testProject", "missing"))
	require.NoError(t, err)
	assert.False(t, removed)
	assert.Zero(t, res.Revision)
	assert.Equal(t, 1, store.creates)
}

func TestMainEnvironmentConfigRemoveCommandPath(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t,
		"values:\n  pulumiConfig:\n    testProject:a: one\n    testProject:b: two\n")
	s := mainEnvStack(store.backend())
	project := &workspace.Project{Name: "testProject"}
	const stackYAML = "mainEnvironment: payments/dev\n"
	ps := loadStackFile(t, project, stackYAML)
	configFile := writeStackFile(t, stackYAML)
	mainEnv := activeMainEnvironment(io.Discard, s, ps)
	require.NotNil(t, mainEnv)

	var out bytes.Buffer
	require.NoError(t, removeFromMainEnvironment(
		t.Context(), &out, s, ps, mainEnv, config.MustMakeKey("testProject", "a"), false, configFile))
	assert.Equal(t,
		"Created payments/dev@2 (parent @1).\n"+
			"Pulumi.testStack.yaml now points at @2; latest is still @1.\n",
		out.String())
	assert.NotContains(t, store.head(), "testProject:a")

	saved, err := os.ReadFile(configFile)
	require.NoError(t, err)
	assert.Equal(t, "mainEnvironment: payments/dev@2\n", string(saved))

	// Removing a key that is not set reports so, creates no revision, and leaves the pointer alone.
	out.Reset()
	require.NoError(t, removeFromMainEnvironment(
		t.Context(), &out, s, ps, mainEnv, config.MustMakeKey("testProject", "missing"), false, configFile))
	assert.Equal(t, "Configuration key 'testProject:missing' is not set in payments/dev\n", out.String())
	assert.Equal(t, 1, store.creates)
	saved, err = os.ReadFile(configFile)
	require.NoError(t, err)
	assert.Equal(t, "mainEnvironment: payments/dev@2\n", string(saved))

	// `--path` removals are not supported yet.
	err = removeFromMainEnvironment(
		t.Context(), &out, s, ps, mainEnv, config.MustMakeKey("testProject", "b.c"), true, configFile)
	require.ErrorContains(t, err, "'pulumi config rm --path' is not supported yet")
	assert.Equal(t, 1, store.creates)
}

// TestNonMainEnvironmentConfigSetUnaffected asserts a stack with no `mainEnvironment` is untouched by any of
// this: it writes its own config file, and no environment method is ever reached.
func TestNonMainEnvironmentConfigSetUnaffected(t *testing.T) {
	t.Parallel()

	// Every environment function field is nil, so reaching any of them panics.
	be := &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{NameF: func() string { return "test" }},
	}
	s := mainEnvStack(be)
	project := &workspace.Project{Name: "testProject"}
	const stackYAML = "config:\n  testProject:existing: keep\n"
	ps := loadStackFile(t, project, stackYAML)
	configFile := writeStackFile(t, stackYAML)

	var stdout bytes.Buffer
	cmd := newMainEnvSetCmd(ps, &stdout)
	require.NoError(t, cmd.Run(
		t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:a", "b"}, project, s, configFile))

	saved, err := os.ReadFile(configFile)
	require.NoError(t, err)
	assert.Contains(t, string(saved), "testProject:a: b")
	assert.Contains(t, string(saved), "testProject:existing: keep")
	assert.NotContains(t, string(saved), "mainEnvironment")
	assert.Empty(t, stdout.String())
}

// TestMainEnvironmentWriteRequiresEnvironmentsBackend asserts a backend that cannot host named environments
// still fails with the pre-existing refusal.
func TestMainEnvironmentWriteRequiresEnvironmentsBackend(t *testing.T) {
	t.Parallel()

	s := mainEnvStack(&backend.MockBackend{NameF: func() string { return "diy" }})
	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")
	mainEnv := activeMainEnvironment(io.Discard, s, ps)
	require.NotNil(t, mainEnv)

	_, err := newMainEnvWriter(s, ps, mainEnv, "")
	require.Error(t, err)
	assert.Equal(t, errBackendNoEnvironments(s.Backend()).Error(), err.Error())
}

func TestMainEnvironmentWriteRefusals(t *testing.T) {
	t.Parallel()

	project := &workspace.Project{Name: "testProject"}

	// A tag pin is refused: rewriting `@stable` to `@8` would silently un-tag the stack, which is a
	// product decision about moving tags rather than something a write should settle. A numeric pin is
	// not refused -- it is the parent to branch from, covered by
	// TestMainEnvironmentConfigSetBranchesFromPinnedParent.
	t.Run("tag pin", func(t *testing.T) {
		t.Parallel()

		store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
		s := mainEnvStack(store.backend())
		ps := loadStackFile(t, project, "mainEnvironment: payments/dev@stable\n")

		var stdout bytes.Buffer
		cmd := newMainEnvSetCmd(ps, &stdout)
		err := cmd.Run(
			t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:a", "b"},
			project, s, filepath.Join(t.TempDir(), "Pulumi.testStack.yaml"))
		require.ErrorContains(t, err, "pins it to the tag \"stable\"")
		require.ErrorContains(t, err, "pin a revision number instead")
		assert.Equal(t, 0, store.creates)
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		store := newFakeEnvStore(t, "")
		store.exists = false
		s := mainEnvStack(store.backend())
		ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")

		var stdout bytes.Buffer
		cmd := newMainEnvSetCmd(ps, &stdout)
		err := cmd.Run(
			t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:a", "b"},
			project, s, filepath.Join(t.TempDir(), "Pulumi.testStack.yaml"))
		require.ErrorContains(t, err, "environment payments/dev does not exist")
		assert.Equal(t, 0, store.creates)
	})

	t.Run("--path", func(t *testing.T) {
		t.Parallel()

		store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
		s := mainEnvStack(store.backend())
		ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")

		var stdout bytes.Buffer
		cmd := newMainEnvSetCmd(ps, &stdout)
		cmd.Path = true
		err := cmd.Run(
			t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:a.b", "c"},
			project, s, filepath.Join(t.TempDir(), "Pulumi.testStack.yaml"))
		require.ErrorContains(t, err, "'pulumi config set --path' is not supported yet")
		assert.Equal(t, 0, store.creates)
	})

	t.Run("unsupported subcommands", func(t *testing.T) {
		t.Parallel()

		mainEnv := &workspace.MainEnvironment{Project: "payments", Name: "dev"}
		for _, name := range []string{"copy", "set-all", "rm-all", "refresh"} {
			err := errMainEnvUnsupported(name, mainEnv)
			assert.ErrorContains(t, err, "'pulumi config "+name+"' is not supported yet")
			assert.ErrorContains(t, err, "mainEnvironment: payments/dev")
		}
	})
}

//
// Attribution (DoD 7).
//

func prepareMainEnvListConfig(
	t *testing.T,
	stackFile string,
	env *esc.Environment,
	store *fakeEnvStore,
) (backend.Stack, *workspace.Project, *workspace.ProjectStack, cmdStack.SecretsManagerLoader) {
	t.Helper()

	secretsManager, _, _, _, _ := getCountingBase64SecretsManager(t.Context(), t, false)
	snapshot := &deploy.Snapshot{SecretsManager: stack.NewBatchingCachingSecretsManager(secretsManager)}

	be := store.backend()
	be.CheckYAMLEnvironmentF = func(
		context.Context, string, []byte,
	) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
		return env, apitype.EnvironmentDiagnostics{}, nil
	}
	be.OpenYAMLEnvironmentF = func(
		context.Context, string, []byte, time.Duration, map[string]string,
	) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
		return env, apitype.EnvironmentDiagnostics{}, nil
	}

	s := mainEnvStack(be)
	s.SnapshotF = func(context.Context, secrets.Provider) (*deploy.Snapshot, error) { return snapshot, nil }

	project := &workspace.Project{Name: "testProject"}
	return s, project, loadStackFile(t, project, stackFile), cmdStack.SecretsManagerLoader{FallbackToState: true}
}

func mainEnvAttributionEnvironment() *esc.Environment {
	return &esc.Environment{Properties: map[string]esc.Value{
		"pulumiConfig": esc.NewValue(map[string]esc.Value{
			"testProject:instanceCount": {
				Value: "6",
				Trace: esc.Trace{Def: esc.Range{Environment: "payments/dev", Begin: esc.Pos{Line: 4, Column: 7}}},
			},
			"testProject:logLevel": {
				Value: "info",
				Trace: esc.Trace{Def: esc.Range{Environment: "payments/base"}},
			},
		}),
	}}
}

func TestMainEnvironmentListConfigSourceColumn(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	store.revision = 8
	s, project, ps, ssml := prepareMainEnvListConfig(
		t, "mainEnvironment: payments/dev\n", mainEnvAttributionEnvironment(), store)

	var stdout bytes.Buffer
	require.NoError(t, listConfig(
		t.Context(), ssml, &stdout, project, s, ps, false, false, true, ""))

	expected := strings.TrimSpace(`
KEY                        VALUE  SOURCE
testProject:instanceCount  6      payments/dev@8
testProject:logLevel       info   payments/base@2 (imported)
`)
	assert.Equal(t, expected, strings.TrimSpace(stdout.String()))

	// One revision lookup per distinct environment, not one per value.
	assert.Equal(t, 1, store.revisionCalls["payments/dev"])
	assert.Equal(t, 1, store.revisionCalls["payments/base"])
}

func TestMainEnvironmentListConfigUnmigratedValues(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	store.revision = 8
	s, project, ps, ssml := prepareMainEnvListConfig(
		t,
		"mainEnvironment: payments/dev\nconfig:\n  testProject:instanceCount: \"9\"\n",
		mainEnvAttributionEnvironment(),
		store)

	var stdout bytes.Buffer
	require.NoError(t, listConfig(
		t.Context(), ssml, &stdout, project, s, ps, false, false, true, ""))

	out := stdout.String()
	// The stack file's value wins, and is attributed as unmigrated.
	assert.Contains(t, out, "testProject:instanceCount  9      Pulumi.testStack.yaml (unmigrated)")
	assert.Contains(t, out, "1 configuration value(s) still set in Pulumi.testStack.yaml override payments/dev")
}

func TestMainEnvironmentListConfigJSON(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	store.revision = 8
	s, project, ps, ssml := prepareMainEnvListConfig(
		t, "mainEnvironment: payments/dev\n", mainEnvAttributionEnvironment(), store)

	var stdout bytes.Buffer
	require.NoError(t, listConfig(
		t.Context(), ssml, &stdout, project, s, ps, false, true, true, ""))
	assert.Contains(t, stdout.String(), `"source": "payments/dev@8"`)
	assert.Contains(t, stdout.String(), `"source": "payments/base@2 (imported)"`)
}

// TestEnvironmentStackListConfigUnchanged is the explicit regression test for DoD 11: an
// `environment:`-only stack gains neither a SOURCE column nor a `source` JSON field.
func TestEnvironmentStackListConfigUnchanged(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	s, project, ps, ssml := prepareMainEnvListConfig(
		t, "environment:\n  - payments/dev\n", mainEnvAttributionEnvironment(), store)

	var stdout bytes.Buffer
	require.NoError(t, listConfig(
		t.Context(), ssml, &stdout, project, s, ps, false, false, true, ""))
	expected := strings.TrimSpace(`
KEY                        VALUE
testProject:instanceCount  6
testProject:logLevel       info
`)
	assert.Equal(t, expected, strings.TrimSpace(stdout.String()))
	assert.NotContains(t, stdout.String(), "Config source:")

	stdout.Reset()
	require.NoError(t, listConfig(
		t.Context(), ssml, &stdout, project, s, ps, false, true, true, ""))
	assert.NotContains(t, stdout.String(), `"source"`)
	assert.Empty(t, store.revisionCalls)
}

func TestShowSource(t *testing.T) {
	t.Parallel()

	mainEnv := &workspace.MainEnvironment{Project: "payments", Name: "dev"}
	base := esc.Value{
		Value: "debug",
		Trace: esc.Trace{Def: esc.Range{Environment: "payments/base"}},
	}
	pulumiEnv := esc.NewValue(map[string]esc.Value{
		"testProject:logLevel": {
			Value: "info",
			Trace: esc.Trace{
				Def:  esc.Range{Environment: "payments/dev", Begin: esc.Pos{Line: 4, Column: 7}},
				Base: &base,
			},
		},
	})
	key := config.MustMakeKey("testProject", "logLevel")
	index := &sourceIndex{sources: map[config.Key]string{key: "payments/dev@8"}}

	var out bytes.Buffer
	showSource(&out, index, mainEnv, key, pulumiEnv)
	assert.Equal(t, "Source: payments/dev@8\n"+
		"  defined at payments/dev:4:7\n"+
		"  overrides payments/base\n", out.String())
}

// writeStackFile writes a stack file to a temporary directory and returns its path.
func writeStackFile(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Pulumi.testStack.yaml")
	require.NoError(t, os.WriteFile(path, []byte(text), 0o600))
	return path
}

func TestGetConfigShowSource(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	store.revision = 8
	s, project, _, ssml := prepareMainEnvListConfig(
		t, "mainEnvironment: payments/dev\n", mainEnvAttributionEnvironment(), store)
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) { return project, "", nil },
	}

	var out bytes.Buffer
	require.NoError(t, getConfig(
		t.Context(), &out, cmdutil.Diag(), ssml, ws, s,
		config.MustMakeKey("testProject", "logLevel"), false, false, true, true,
		writeStackFile(t, "mainEnvironment: payments/dev\n")))

	assert.Equal(t, "info\nSource: payments/base@2 (imported)\n", out.String())
}

func TestShowSourceRequiresMainEnvironment(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	s, project, _, ssml := prepareMainEnvListConfig(
		t, "environment:\n  - payments/dev\n", mainEnvAttributionEnvironment(), store)
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) { return project, "", nil },
	}

	var out bytes.Buffer
	err := getConfig(
		t.Context(), &out, cmdutil.Diag(), ssml, ws, s,
		config.MustMakeKey("testProject", "logLevel"), false, false, true, true,
		writeStackFile(t, "environment:\n  - payments/dev\n"))
	require.ErrorContains(t, err, "--show-source requires the stack to set 'mainEnvironment'")
}

// TestMainEnvironmentUnsupportedSubcommands is the command-level half of DoD 12: subcommands that have no
// environment write path refuse rather than silently writing the local `config:` block.
func TestMainEnvironmentUnsupportedSubcommands(t *testing.T) {
	t.Parallel()

	const stackFile = "mainEnvironment: payments/dev\n"

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) {
			return &workspace.Project{Name: "testProject"}, "", nil
		},
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{Current: "https://api.pulumi.com"}, nil
		},
	}

	t.Run("set-all", func(t *testing.T) {
		t.Parallel()

		store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
		s := mainEnvStack(store.backend())
		be := &backend.MockBackend{
			GetStackF: func(context.Context, backend.StackReference) (backend.Stack, error) { return s, nil },
		}

		configPath := writeStackFile(t, stackFile)
		stackName := "testStack"
		cmd := newConfigSetAllCmd(ws, &stackName, mainEnvLoginManager(be), &mockEncrypterFactory{}, &configPath)
		cmd.SetContext(t.Context())
		require.NoError(t, cmd.PersistentFlags().Set("plaintext", "testProject:key=value"))

		err := cmd.RunE(cmd, []string{})
		require.ErrorContains(t, err, "'pulumi config set-all' is not supported yet")

		// Neither the environment nor the stack file was written.
		assert.Equal(t, 0, store.creates)
		data, readErr := os.ReadFile(configPath)
		require.NoError(t, readErr)
		assert.Equal(t, stackFile, string(data))
	})

	t.Run("refresh", func(t *testing.T) {
		t.Parallel()

		store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
		envBackend := store.backend()
		envBackend.GetLatestConfigurationF = func(
			context.Context, backend.Stack,
		) (backend.LatestConfiguration, error) {
			return backend.LatestConfiguration{
				Config: config.Map{config.MustMakeKey("testProject", "key1"): config.NewValue("value1")},
			}, nil
		}
		s := mainEnvStack(envBackend)
		be := &backend.MockBackend{
			GetStackF: func(context.Context, backend.StackReference) (backend.Stack, error) { return s, nil },
			ParseStackReferenceF: func(name string) (backend.StackReference, error) {
				return &backend.MockStackReference{
					NameV:               tokens.MustParseStackName(name),
					FullyQualifiedNameV: tokens.QName("test-org/testProject/" + name),
				}, nil
			},
		}

		configPath := writeStackFile(t, stackFile)
		stackName := "testStack"
		cmd := newConfigRefreshCmd(ws, &stackName, mainEnvLoginManager(be), &configPath)
		cmd.SetContext(t.Context())
		require.NoError(t, cmd.PersistentFlags().Set("force", "true"))

		err := cmd.RunE(cmd, []string{})
		require.ErrorContains(t, err, "'pulumi config refresh' is not supported yet")

		assert.Equal(t, 0, store.creates)
		data, readErr := os.ReadFile(configPath)
		require.NoError(t, readErr)
		assert.Equal(t, stackFile, string(data))
	})
}

// TestMainEnvironmentCopySourceRefused covers the half of DoD 12 that `config copy` reaches through its
// *source* stack: a migrated stack's values live in ESC, not in its `config:` block, so copying from one
// would quietly write an empty (or stale) destination.
func TestMainEnvironmentCopySourceRefused(t *testing.T) { //nolint:paralleltest // t.Chdir forbids t.Parallel
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "Pulumi.yaml"), []byte("name: testProject\nruntime: nodejs\n"), 0o600))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "Pulumi.source.yaml"), []byte("mainEnvironment: payments/dev\n"), 0o600))
	const destFile = "config:\n  testProject:existing: keep\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.dest.yaml"), []byte(destFile), 0o600))

	store := newFakeEnvStore(t, "values:\n  pulumiConfig:\n    testProject:a: one\n")
	source := mainEnvStack(store.backend())
	source.RefF = func() backend.StackReference {
		return &backend.MockStackReference{
			NameV:               tokens.MustParseStackName("source"),
			FullyQualifiedNameV: "test-org/testProject/source",
		}
	}
	dest := mainEnvStack(store.backend())
	dest.RefF = func() backend.StackReference {
		return &backend.MockStackReference{
			NameV:               tokens.MustParseStackName("dest"),
			FullyQualifiedNameV: "test-org/testProject/dest",
		}
	}

	be := &backend.MockBackend{
		GetStackF: func(_ context.Context, ref backend.StackReference) (backend.Stack, error) {
			if ref.Name().String() == "dest" {
				return dest, nil
			}
			return source, nil
		},
		ParseStackReferenceF: func(name string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				NameV:               tokens.MustParseStackName(name),
				FullyQualifiedNameV: tokens.QName("test-org/testProject/" + name),
			}, nil
		},
	}
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) {
			return &workspace.Project{Name: "testProject"}, filepath.Join(dir, "Pulumi.yaml"), nil
		},
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{Current: "https://api.pulumi.com"}, nil
		},
	}

	stackName, configFile := "source", ""
	cmd := newConfigCopyCmd(ws, &stackName, mainEnvLoginManager(be), &configFile)
	cmd.SetContext(t.Context())
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	require.NoError(t, cmd.PersistentFlags().Set("dest", "dest"))

	err := cmd.RunE(cmd, []string{})
	require.ErrorContains(t, err, "'pulumi config copy' is not supported yet")

	// The destination stack file was not touched.
	data, readErr := os.ReadFile(filepath.Join(dir, "Pulumi.dest.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, destFile, string(data))
}

// TestGetConfigShowSourceJSON asserts `--json --show-source` stays parseable: attribution goes inside the
// JSON object rather than being printed alongside it.
func TestGetConfigShowSourceJSON(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	store.revision = 8
	s, project, _, ssml := prepareMainEnvListConfig(
		t, "mainEnvironment: payments/dev\n", mainEnvAttributionEnvironment(), store)
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) { return project, "", nil },
	}

	var out bytes.Buffer
	require.NoError(t, getConfig(
		t.Context(), &out, cmdutil.Diag(), ssml, ws, s,
		config.MustMakeKey("testProject", "instanceCount"), false, true, true, true,
		writeStackFile(t, "mainEnvironment: payments/dev\n")))

	var value configValueJSON
	require.NoError(t, json.Unmarshal(out.Bytes(), &value))
	require.NotNil(t, value.Value)
	assert.Equal(t, "6", *value.Value)
	assert.Equal(t, "payments/dev@8", value.Source)
}

// TestMainEnvironmentAttributionPinnedImport asserts a value inherited from a pinned import is attributed to
// the pinned version rather than to the import's latest revision.
func TestMainEnvironmentAttributionPinnedImport(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "")
	store.revision = 8
	env := &esc.Environment{Properties: map[string]esc.Value{
		"pulumiConfig": esc.NewValue(map[string]esc.Value{
			"testProject:logLevel": {
				Value: "info",
				Trace: esc.Trace{Def: esc.Range{Environment: "payments/base@4"}},
			},
		}),
	}}
	s, project, ps, ssml := prepareMainEnvListConfig(t, "mainEnvironment: payments/dev\n", env, store)

	var requestedVersion string
	be := s.Backend().(*backend.MockEnvironmentsBackend)
	inner := be.GetEnvironmentRevisionF
	be.GetEnvironmentRevisionF = func(
		ctx context.Context, org, envProject, envName, version string,
	) (int, error) {
		if envProject+"/"+envName == "payments/base" {
			requestedVersion = version
			return 4, nil
		}
		return inner(ctx, org, envProject, envName, version)
	}

	var stdout bytes.Buffer
	require.NoError(t, listConfig(
		t.Context(), ssml, &stdout, project, s, ps, false, false, true, ""))

	assert.Equal(t, "4", requestedVersion)
	assert.Contains(t, stdout.String(), "payments/base@4 (imported)")
}

// TestMainEnvironmentWriteWarnsAboutShadowingLocalValue covers the transitional state where a key is set
// both in the stack file and in the environment: the stack file wins on reads, so a write that did not say
// so would report a value the very next read would not return.
func TestMainEnvironmentWriteWarnsAboutShadowingLocalValue(t *testing.T) {
	t.Parallel()

	const stackFile = "mainEnvironment: payments/dev\nconfig:\n  testProject:a: local\n"
	project := &workspace.Project{Name: "testProject"}

	t.Run("set", func(t *testing.T) {
		t.Parallel()

		store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
		s := mainEnvStack(store.backend())
		ps := loadStackFile(t, project, stackFile)

		var stdout bytes.Buffer
		c := newMainEnvSetCmd(ps, &stdout)
		require.NoError(t, c.Run(
			t.Context(), nil, []string{"testProject:a", "env"}, project, s, writeStackFile(t, stackFile)))

		assert.Contains(t, stdout.String(), "Created payments/dev@2 (parent @1).")
		assert.Contains(t, stdout.String(),
			"warning: 'testProject:a' is also set in Pulumi.testStack.yaml, which shadows the value just written")
	})

	t.Run("rm", func(t *testing.T) {
		t.Parallel()

		store := newFakeEnvStore(t, "values:\n  pulumiConfig:\n    testProject:a: env\n")
		s := mainEnvStack(store.backend())
		ps := loadStackFile(t, project, stackFile)
		mainEnv := activeMainEnvironment(io.Discard, s, ps)
		require.NotNil(t, mainEnv)

		var out bytes.Buffer
		require.NoError(t, removeFromMainEnvironment(
			t.Context(), &out, s, ps, mainEnv, config.MustMakeKey("testProject", "a"), false,
			writeStackFile(t, stackFile)))
		assert.Contains(t, out.String(), "Created payments/dev@2 (parent @1).")
		assert.Contains(t, out.String(),
			"warning: 'testProject:a' is still set in Pulumi.testStack.yaml, which takes precedence over payments/dev")
	})
}

// TestMainEnvironmentRemoteConfigWarnsOnWritePath asserts that a stack configured both ways is told its
// `mainEnvironment` is ignored, instead of hitting the pre-existing remote-config refusal with no
// explanation.
func TestMainEnvironmentRemoteConfigWarnsOnWritePath(t *testing.T) {
	t.Parallel()

	store := newFakeEnvStore(t, "values:\n  pulumiConfig: {}\n")
	s := mainEnvStack(store.backend())
	escEnv := "payments/remote"
	s.ConfigLocationF = func() backend.StackConfigLocation {
		return backend.StackConfigLocation{IsRemote: true, EscEnv: &escEnv}
	}
	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "mainEnvironment: payments/dev\n")

	var stdout, stderr bytes.Buffer
	c := newMainEnvSetCmd(ps, &stdout)
	c.Stderr = &stderr
	err := c.Run(t.Context(), nil, []string{"testProject:a", "b"}, project, s, "")

	require.ErrorContains(t, err, "config set not supported for remote stack config")
	assert.Contains(t, stderr.String(),
		"'mainEnvironment' is ignored because this stack's configuration is stored remotely")
	assert.Equal(t, 0, store.creates)
}

// TestMainEnvironmentConfigEnvSubcommands asserts `pulumi config env ls/add/rm` do not silently operate on
// the `environment:` list that a main environment supersedes.
func TestMainEnvironmentConfigEnvSubcommands(t *testing.T) {
	t.Parallel()

	const stackYAML = "mainEnvironment: payments/dev\nenvironment:\n  - payments/legacy\n"
	env := &esc.Environment{Properties: map[string]esc.Value{
		"pulumiConfig": esc.NewValue(map[string]esc.Value{}),
	}}

	newCmd := func(stdout io.Writer) *configEnvCmd {
		return newConfigEnvCmdForTest(
			strings.NewReader(""), stdout, "name: test\nruntime: yaml", stackYAML, env, nil, nil)
	}

	t.Run("ls", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var listed []string
		require.NoError(t, newCmd(&stdout).listStackEnvironments(
			t.Context(), func(_ io.Writer, imports []string) error {
				listed = imports
				return nil
			}))
		assert.Equal(t, []string{"payments/dev"}, listed)
	})

	t.Run("add", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		err := newCmd(&stdout).editStackEnvironment(
			t.Context(), "add", false, true, func(*workspace.ProjectStack) error { return nil })
		require.ErrorContains(t, err, "'pulumi config env add' is not supported yet")
	})

	t.Run("rm", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		err := newCmd(&stdout).editStackEnvironment(
			t.Context(), "rm", false, true, func(*workspace.ProjectStack) error { return nil })
		require.ErrorContains(t, err, "'pulumi config env rm' is not supported yet")
	})
}

//
// Stack birth, end to end on the mock backend (DoD 10).
//

// birthEnvUniverse is an in-memory organization's worth of named environments: it can create them, read
// them back at a version, branch a revision off a named parent, and resolve one into the values ESC would
// return.
type birthEnvUniverse struct {
	t *testing.T
	// definitions maps `<project>/<env>` to that environment's revisions, keyed by revision number.
	definitions map[string]map[int]string
	etags       map[string]string
	// latest maps an environment to the revision its `latest` tag points at, which branching never moves.
	latest map[string]int
	// head maps an environment to its most recently created revision.
	head map[string]int
}

func newBirthEnvUniverse(t *testing.T) *birthEnvUniverse {
	return &birthEnvUniverse{
		t:           t,
		definitions: map[string]map[int]string{},
		etags:       map[string]string{},
		latest:      map[string]int{},
		head:        map[string]int{},
	}
}

// definitionAt returns an environment's definition at a version -- a revision number, or "" for latest.
func (u *birthEnvUniverse) definitionAt(ref, version string) (string, int, bool) {
	revision := u.latest[ref]
	if version != "" {
		n, err := strconv.Atoi(version)
		if err != nil {
			return "", 0, false
		}
		revision = n
	}
	def, ok := u.definitions[ref][revision]
	return def, revision, ok
}

// resolve flattens an environment and the environments it imports into the values ESC would hand back,
// attributing each value to the environment that defines it. A reference may carry an `@version` pin, in
// which case that revision is what is read.
func (u *birthEnvUniverse) resolve(reference string) map[string]esc.Value {
	ref, version, _ := strings.Cut(reference, "@")
	def, _, ok := u.definitionAt(ref, version)
	require.True(u.t, ok, "no such environment %v", reference)

	var doc struct {
		Imports []string `yaml:"imports"`
		Values  struct {
			PulumiConfig map[string]string `yaml:"pulumiConfig"`
		} `yaml:"values"`
	}
	require.NoError(u.t, yaml.Unmarshal([]byte(def), &doc))

	values := map[string]esc.Value{}
	for _, imported := range doc.Imports {
		maps.Copy(values, u.resolve(imported))
	}
	for k, v := range doc.Values.PulumiConfig {
		values[k] = esc.Value{Value: v, Trace: esc.Trace{Def: esc.Range{Environment: reference}}}
	}
	return values
}

func (u *birthEnvUniverse) backend() *backend.MockEnvironmentsBackend {
	return &backend.MockEnvironmentsBackend{
		MockBackend: backend.MockBackend{NameF: func() string { return "test" }},
		CreateEnvironmentF: func(
			_ context.Context, _, envProject, envName string, yaml []byte,
		) (apitype.EnvironmentDiagnostics, error) {
			ref := envProject + "/" + envName
			u.definitions[ref] = map[int]string{1: string(yaml)}
			u.etags[ref], u.latest[ref], u.head[ref] = "etag-1", 1, 1
			return nil, nil
		},
		GetEnvironmentDefinitionF: func(
			_ context.Context, _, envProject, envName, version string,
		) ([]byte, string, int, error) {
			ref := envProject + "/" + envName
			def, revision, ok := u.definitionAt(ref, version)
			if !ok {
				return nil, "", 0, fmt.Errorf("%w: %v", backend.ErrEnvironmentNotFound, ref)
			}
			return []byte(def), u.etags[ref], revision, nil
		},
		CreateEnvironmentRevisionFromParentF: func(
			_ context.Context, _, envProject, envName string, yaml []byte, parent int,
		) (apitype.EnvironmentDiagnostics, int, error) {
			ref := envProject + "/" + envName
			if _, ok := u.definitions[ref][parent]; !ok {
				return nil, 0, fmt.Errorf("%w: %v@%d", backend.ErrEnvironmentNotFound, ref, parent)
			}
			revision := u.head[ref] + 1
			u.definitions[ref][revision] = string(yaml)
			u.head[ref] = revision
			// `latest` deliberately does not move.
			return nil, revision, nil
		},
		GetEnvironmentRevisionF: func(_ context.Context, _, envProject, envName, version string) (int, error) {
			ref := envProject + "/" + envName
			if version == "" {
				return u.latest[ref], nil
			}
			n, err := strconv.Atoi(version)
			if err != nil {
				return 0, fmt.Errorf("no such version %v", version)
			}
			return n, nil
		},
		OpenYAMLEnvironmentF: func(
			_ context.Context, _ string, yaml []byte, _ time.Duration, _ map[string]string,
		) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			// The stack resolves a synthesized environment that imports exactly the reference its
			// `mainEnvironment` names, pin included, so read the imports rather than assuming one.
			return &esc.Environment{Properties: map[string]esc.Value{
				"pulumiConfig": esc.NewValue(u.resolveSynthesized(yaml)),
			}}, nil, nil
		},
	}
}

// resolveSynthesized flattens the synthesized environment a `mainEnvironment` stack resolves through.
func (u *birthEnvUniverse) resolveSynthesized(definition []byte) map[string]esc.Value {
	var doc struct {
		Imports []string `yaml:"imports"`
	}
	require.NoError(u.t, yaml.Unmarshal(definition, &doc))

	values := map[string]esc.Value{}
	for _, imported := range doc.Imports {
		maps.Copy(values, u.resolve(imported))
	}
	return values
}

// TestStackBirthEndToEnd walks the whole loop the PoC demos: the environments a stack is born with feed
// `pulumi config`'s SOURCE column, and `pulumi config set` produces the next revision, with no manual
// environment setup in between.
func TestStackBirthEndToEnd(t *testing.T) {
	t.Parallel()

	u := newBirthEnvUniverse(t)
	be := u.backend()
	s := mainEnvStack(be)

	regionNode, err := ConfigValueNode("us-west-2", "", false)
	require.NoError(t, err)

	// 1. Birth: `pulumi stack init --esc-config` creates both environments and records the main one.
	var birthOut bytes.Buffer
	env, err := cmdStack.CreateStackEnvironments(t.Context(), s, cmdStack.StackEnvironmentOptions{
		EnvProject: "testProject",
		EnvName:    "testStack",
		Values:     map[string]yaml.Node{"aws:region": regionNode},
		Stdout:     &birthOut,
	})
	require.NoError(t, err)
	require.False(t, env.StackEnvironmentReused)
	mainEnv := env.MainEnvironment
	assert.Equal(t, "testProject/testStack", mainEnv.String())
	assert.Contains(t, birthOut.String(), "Creating environment 'test-org/testProject/base'...")
	assert.Contains(t, birthOut.String(),
		"Creating environment 'test-org/testProject/testStack'... (imports testProject/base)")

	project := &workspace.Project{Name: "testProject"}
	ps := loadStackFile(t, project, "mainEnvironment: "+mainEnv.String()+"\n")

	secretsManager, _, _, _, _ := getCountingBase64SecretsManager(t.Context(), t, false)
	s.SnapshotF = func(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
		return &deploy.Snapshot{SecretsManager: stack.NewBatchingCachingSecretsManager(secretsManager)}, nil
	}
	ssml := cmdStack.SecretsManagerLoader{FallbackToState: true}

	// 2. `pulumi config` attributes the value to the freshly created environment at revision 1.
	var listOut bytes.Buffer
	require.NoError(t, listConfig(t.Context(), ssml, &listOut, project, s, ps, false, false, true, ""))
	assert.Contains(t, listOut.String(), "aws:region  us-west-2  testProject/testStack@1")

	// 3. `pulumi config set` on the newborn stack branches the next revision off the one it read, leaves
	// `latest` where it was, and repoints the stack file, with no manual setup.
	configFile := writeStackFile(t, "mainEnvironment: "+mainEnv.String()+"\n")
	var setOut bytes.Buffer
	setCmd := newMainEnvSetCmd(ps, &setOut)
	require.NoError(t, setCmd.Run(
		t.Context(), &pkgWorkspace.MockContext{}, []string{"testProject:instanceCount", "6"},
		project, s, configFile))
	assert.Equal(t,
		"Created testProject/testStack@2 (parent @1).\n"+
			"Pulumi.testStack.yaml now points at @2; latest is still @1.\n",
		setOut.String())

	saved, err := os.ReadFile(configFile)
	require.NoError(t, err)
	assert.Equal(t, "mainEnvironment: testProject/testStack@2\n", string(saved))

	listOut.Reset()
	require.NoError(t, listConfig(t.Context(), ssml, &listOut, project, s, ps, false, false, true, ""))
	assert.Contains(t, listOut.String(), "aws:region                 us-west-2  testProject/testStack@2")
	assert.Contains(t, listOut.String(), "testProject:instanceCount  6          testProject/testStack@2")
}
