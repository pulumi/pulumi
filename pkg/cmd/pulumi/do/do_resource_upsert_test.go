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

package do

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/blang/semver"
	"github.com/gofrs/uuid"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	sdkDisplay "github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	deployproviders "github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// installMockUpsertBackend wires a MockBackend + MockStack whose snapshot exposes the given
// snapshot, plus a MockContext with a project/stack selected. Returns the workspace + login
// manager callers pass to NewDoCmd.
func installMockUpsertBackend(t *testing.T, snapshot *deploy.Snapshot) (pkgWorkspace.Context, cmdBackend.LoginManager) {
	t.Helper()
	t.Setenv("PULUMI_STACK", "myorg/proj/dev")

	proj := &workspace.Project{Name: tokens.PackageName("proj")}
	mws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) {
			return proj, t.TempDir(), nil
		},
		NewF: func(string) (pkgWorkspace.W, error) {
			return &pkgWorkspace.MockW{
				SettingsF: func() *pkgWorkspace.Settings {
					return &pkgWorkspace.Settings{Stack: "myorg/proj/dev"}
				},
			}, nil
		},
	}
	stackRef := &backend.MockStackReference{
		StringV: "myorg/proj/dev", NameV: tokens.MustParseStackName("dev"),
		FullyQualifiedNameV: "myorg/proj/dev",
	}
	mockStack := &backend.MockStack{
		RefF: func() backend.StackReference { return stackRef },
		SnapshotF: func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
			return snapshot, nil
		},
	}
	mockBackend := &backend.MockBackend{
		ParseStackReferenceF: func(_ string) (backend.StackReference, error) { return stackRef, nil },
		GetStackF: func(_ context.Context, _ backend.StackReference) (backend.Stack, error) {
			return mockStack, nil
		},
	}
	mlm := &cmdBackend.MockLoginManager{
		CurrentF: func(
			context.Context, pkgWorkspace.Context, diag.Sink,
			string, *workspace.Project, bool,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
		LoginF: func(context.Context, pkgWorkspace.Context, diag.Sink,
			string, *workspace.Project, bool,
			bool, colors.Colorization,
		) (backend.Backend, error) {
			return mockBackend, nil
		},
	}
	return mws, mlm
}

func snapshotWithReferencedResource(t *testing.T, referencedURN resource.URN) *deploy.Snapshot {
	t.Helper()

	providerURN := resource.URN("urn:pulumi:dev::proj::pulumi:providers:azure::default_1_2_3")
	providerID := resource.ID("provider-id")
	providerRef, err := sdkproviders.NewReference(providerURN, providerID)
	require.NoError(t, err)

	version := semver.MustParse("1.2.3")
	providerInputs := resource.PropertyMap{}
	deployproviders.SetProviderVersion(providerInputs, &version)
	deployproviders.SetProviderURL(providerInputs, "https://example.com/azure")

	return &deploy.Snapshot{
		Resources: []*pkgresource.State{
			{
				Type:   "pulumi:providers:azure",
				URN:    providerURN,
				Custom: true,
				ID:     providerID,
				Inputs: providerInputs,
			},
			{
				Type:     referencedURN.Type(),
				URN:      referencedURN,
				Custom:   true,
				ID:       "resource-id",
				Provider: providerRef.String(),
			},
		},
	}
}

// TestDoCmdResourceUpsertConstructsSnippet checks the CLI-to-UpdateOperation path for
// `pulumi do <token> upsert <name>`: the input file's raw contents become the snippet's Code, the
// snippet carries a fresh UUID (no existing snippet in the snapshot to match against), and the
// resource token / package descriptor are threaded through. The real backend/engine wiring lives
// behind the runStatefulUpdate hook; the stub here captures what the CLI passed.
//
//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdResourceUpsertConstructsSnippet(t *testing.T) {
	mws, mlm := installMockUpsertBackend(t, &deploy.Snapshot{})

	var got StatefulUpdateRequest
	var called bool
	stub := func(_ context.Context, _ *pflag.FlagSet, req StatefulUpdateRequest,
	) (*StatefulUpdateResult, error) {
		called = true
		got = req
		return &StatefulUpdateResult{SnippetUUIDs: []string{req.Snippets[len(req.Snippets)-1].UUID}}, nil
	}
	loader := func(_ context.Context, _ *plugin.Context, _, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		return &testProvider{spec: doResourceSpec(false)}, nil
	}
	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin, stub)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	pcl := `name = "example"
size = 2
`
	inputFile := writeHCLFile(t, "upsert.pcl", pcl)
	cmd.SetArgs([]string{
		"azure:index:myResource", "upsert", "myres", "--yes",
		"--input", "pcl", "--input-file", inputFile,
	})
	require.NoError(t, cmd.Execute())

	require.True(t, called, "runStatefulUpdate should have been invoked")
	assert.Equal(t, "myres", got.Snippets[len(got.Snippets)-1].Name)
	assert.Equal(t, "azure:index:myResource", got.Snippets[len(got.Snippets)-1].Type)
	assert.Equal(t, pcl, got.Snippets[len(got.Snippets)-1].Code, "snippet Code should be the raw input file")
	assert.Equal(t, "azure", got.Snippets[len(got.Snippets)-1].Descriptor.Name)
	require.NotEmpty(t, got.Snippets[len(got.Snippets)-1].UUID, "snippet should carry a generated UUID")
	require.NotNil(t, got.Stack, "runStatefulUpdate should receive the loaded stack")
	assert.False(t, got.DryRun)
	assert.True(t, got.Yes)

	assert.Contains(t, stdout.String(), got.Snippets[len(got.Snippets)-1].UUID)
	assert.Contains(t, stdout.String(), "Created myres")
	_ = stderr
}

// TestDoCmdResourceUpsertBareProviderReference verifies that supplying --provider (without any
// provider overrides) makes the resource snippet reference the given provider URN via its
// References map and injects an options { provider = provider } block into its code, without
// producing an inline provider snippet.
//
//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdResourceUpsertBareProviderReference(t *testing.T) {
	// Existing provider resource in the snapshot for the URN we point at with --provider.
	providerURN := "urn:pulumi:dev::proj::pulumi:providers:azure::existing"
	mws, mlm := installMockUpsertBackend(t, &deploy.Snapshot{
		Resources: []*pkgresource.State{{
			URN:    resource.URN(providerURN),
			Type:   tokens.Type("pulumi:providers:azure"),
			Inputs: resource.PropertyMap{"region": resource.NewProperty("us-east-1")},
		}},
	})

	var got StatefulUpdateRequest
	stub := func(_ context.Context, _ *pflag.FlagSet, req StatefulUpdateRequest,
	) (*StatefulUpdateResult, error) {
		got = req
		return &StatefulUpdateResult{SnippetUUIDs: []string{req.Snippets[len(req.Snippets)-1].UUID}}, nil
	}
	loader := func(_ context.Context, _ *plugin.Context, _, source string) (plugin.Provider, error) {
		return &testProvider{spec: doResourceSpec(false)}, nil
	}
	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin, stub)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	inputFile := writeHCLFile(t, "upsert.pcl", `name = "example"`+"\n")
	cmd.SetArgs([]string{
		"azure:index:myResource", "upsert", "myres", "--yes",
		"--provider", providerURN,
		"--input", "pcl", "--input-file", inputFile,
	})
	require.NoError(t, cmd.Execute())

	require.Len(t, got.Snippets, 1, "bare --provider should not add a provider snippet")
	resSnippet := got.Snippets[0]
	assert.Equal(t, providerURN, resSnippet.References["provider"])
	assert.Contains(t, resSnippet.Code, "options {")
	assert.Contains(t, resSnippet.Code, "provider = provider")
	_ = stderr
}

// TestDoCmdResourceUpsertMaterializesProviderSnippet verifies that --provider-file (with or
// without --provider) produces an inline provider snippet whose Code carries the overrides, whose
// URN is referenced from the resource snippet, and injects the options block.
//
//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdResourceUpsertMaterializesProviderSnippet(t *testing.T) {
	providerURN := "urn:pulumi:dev::proj::pulumi:providers:azure::existing"
	mws, mlm := installMockUpsertBackend(t, &deploy.Snapshot{
		Resources: []*pkgresource.State{{
			URN:  resource.URN(providerURN),
			Type: tokens.Type("pulumi:providers:azure"),
			Inputs: resource.PropertyMap{
				"__internal": resource.NewProperty(resource.PropertyMap{
					"pluginDownloadURL": resource.NewProperty("https://example.com/plugins"),
				}),
				"region":  resource.NewProperty("eu-west-1"),
				"token":   resource.MakeSecret(resource.NewProperty("base-secret")),
				"version": resource.NewProperty("1.2.3"),
			},
		}},
	})

	var got StatefulUpdateRequest
	stub := func(_ context.Context, _ *pflag.FlagSet, req StatefulUpdateRequest,
	) (*StatefulUpdateResult, error) {
		got = req
		return &StatefulUpdateResult{SnippetUUIDs: []string{req.Snippets[len(req.Snippets)-1].UUID}}, nil
	}
	loader := func(_ context.Context, _ *plugin.Context, _, source string) (plugin.Provider, error) {
		return &testProvider{spec: doResourceSpec(false)}, nil
	}
	// Provider parsing runs through pc.format (yaml by default); route the yaml converter
	// through a stub that echoes the source verbatim so the test doesn't need a real converter.
	loadConverter := func(
		_ *plugin.Context, _ string, _ func(sev diag.Severity, msg string),
	) (plugin.Converter, error) {
		return &plugin.MockConverter{
			ConvertSnippetF: func(_ context.Context, req *plugin.ConvertSnippetRequest) (
				*plugin.ConvertSnippetResponse, error,
			) {
				return &plugin.ConvertSnippetResponse{
					Source:   req.Source,
					Filename: req.Filename,
				}, nil
			},
		}, nil
	}
	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, loadConverter, stub)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	providerFile := writeHCLFile(t, "provider.pcl", `region = "us-east-1"`+"\n")
	inputFile := writeHCLFile(t, "upsert.pcl", `name = "example"`+"\n")
	cmd.SetArgs([]string{
		"azure:index:myResource", "upsert", "myres", "--yes",
		"--provider", providerURN, "--provider-file", providerFile,
		"--input", "pcl", "--input-file", inputFile,
	})
	require.NoError(t, cmd.Execute())

	require.Len(t, got.Snippets, 2, "materialize path emits provider snippet then resource snippet")
	providerSnippet := got.Snippets[0]
	resSnippet := got.Snippets[1]
	assert.Equal(t, "myres-provider", providerSnippet.Name)
	assert.Equal(t, "pulumi:providers:azure", providerSnippet.Type)
	assert.Contains(t, providerSnippet.Code, `region = "us-east-1"`)
	assert.Contains(t, providerSnippet.Code, `secret("base-secret")`)
	assert.NotContains(t, providerSnippet.Code, "__internal")
	assert.NotContains(t, providerSnippet.Code, `version = "1.2.3"`)
	require.NotEmpty(t, providerSnippet.UUID)

	// The resource snippet references the provider snippet by its computed URN.
	wantProviderURN := string(resource.CreateURN(
		"myres-provider", "pulumi:providers:azure", "", "proj", "dev",
	))
	assert.Equal(t, wantProviderURN, resSnippet.References["provider"])
	assert.Contains(t, resSnippet.Code, "options {")
	assert.Contains(t, resSnippet.Code, "provider = provider")
	_ = stderr
}

//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdResourceUpsertConstructsSnippetWithReferences(t *testing.T) {
	referencedURN := resource.URN("urn:pulumi:dev::proj::azure:index:myResource::source")
	mws, mlm := installMockUpsertBackend(t, snapshotWithReferencedResource(t, referencedURN))

	var got StatefulUpdateRequest
	stub := func(_ context.Context, _ *pflag.FlagSet, req StatefulUpdateRequest,
	) (*StatefulUpdateResult, error) {
		got = req
		return &StatefulUpdateResult{SnippetUUIDs: []string{req.Snippets[len(req.Snippets)-1].UUID}}, nil
	}
	loader := func(_ context.Context, _ *plugin.Context, _, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		return &testProvider{spec: doResourceSpec(false)}, nil
	}
	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin, stub)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	inputFile := writeHCLFile(t, "upsert.pcl", `name = source.name`)
	resourcesFile := writeHCLFile(t, "resources.json", `{"source":"`+string(referencedURN)+`"}`)
	cmd.SetArgs([]string{
		"azure:index:myResource", "upsert", "myres", "--yes",
		"--input", "pcl", "--input-file", inputFile, "--resources-file", resourcesFile,
	})
	require.NoError(t, cmd.Execute())

	assert.Equal(t, map[string]string{"source": string(referencedURN)}, got.Snippets[len(got.Snippets)-1].References)
	_ = stdout
	_ = stderr
}

//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdResourceUpsertConvertsReferences(t *testing.T) {
	referencedURN := resource.URN("urn:pulumi:dev::proj::azure:index:myResource::source")
	mws, mlm := installMockUpsertBackend(t, snapshotWithReferencedResource(t, referencedURN))

	var got StatefulUpdateRequest
	stub := func(_ context.Context, _ *pflag.FlagSet, req StatefulUpdateRequest,
	) (*StatefulUpdateResult, error) {
		got = req
		return &StatefulUpdateResult{SnippetUUIDs: []string{req.Snippets[len(req.Snippets)-1].UUID}}, nil
	}
	loader := func(_ context.Context, _ *plugin.Context, _, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		return &testProvider{spec: doResourceSpec(false)}, nil
	}

	loadConverter := func(_ *plugin.Context, name string, _ func(sev diag.Severity, msg string),
	) (plugin.Converter, error) {
		assert.Equal(t, "yaml", name)
		return &plugin.MockConverter{
			ConvertSnippetF: func(_ context.Context, req *plugin.ConvertSnippetRequest) (
				*plugin.ConvertSnippetResponse, error,
			) {
				assert.Equal(t, "upsert.yaml", filepath.Base(req.Filename))
				assert.Equal(t, "name: ${source-name.name}\n", string(req.Source))
				require.Len(t, req.Resources, 2)
				ref := req.Resources["source-name"]
				assert.Equal(t, "azure:index:myResource", ref.Token)
				require.NotNil(t, ref.Package)
				assert.Equal(t, "azure", ref.Package.Package)
				assert.Equal(t, "1.2.3", ref.Package.Version)
				assert.Equal(t, "https://example.com/azure", ref.Package.DownloadUrl)
				assert.Equal(t, req.Resources["source-name"], req.Resources["source"],
					"auto-assigned identifier should describe the same resource")
				return &plugin.ConvertSnippetResponse{
					Filename: "upsert.pp",
					Source:   []byte("name = sourceName.name\n"),
					ResourceNames: map[string]string{
						"source-name": "sourceName",
					},
				}, nil
			},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, loadConverter, stub)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	inputFile := writeHCLFile(t, "upsert.yaml", "name: ${source-name.name}\n")
	resourcesFile := writeHCLFile(t, "resources.json", `{"source-name":"`+string(referencedURN)+`"}`)
	cmd.SetArgs([]string{
		"azure:index:myResource", "upsert", "myres", "--yes",
		"--input", "yaml", "--input-file", inputFile, "--resources-file", resourcesFile,
	})
	require.NoError(t, cmd.Execute())

	assert.Equal(t, "name = sourceName.name\n", got.Snippets[len(got.Snippets)-1].Code)
	assert.Equal(t, map[string]string{"sourceName": string(referencedURN)}, got.Snippets[len(got.Snippets)-1].References)
	_ = stdout
	_ = stderr
}

func TestResourceReferenceInfosIncludesProviderParameterization(t *testing.T) {
	t.Parallel()

	referencedURN := resource.URN("urn:pulumi:dev::proj::myparam:index:Thing::source")
	providerURN := resource.URN("urn:pulumi:dev::proj::pulumi:providers:myparam::default_1_2_3")
	providerID := resource.ID("provider-id")
	providerRef, err := sdkproviders.NewReference(providerURN, providerID)
	require.NoError(t, err)

	baseVersion := semver.MustParse("2.0.0")
	parameterVersion := semver.MustParse("1.2.3")
	parameterValue := []byte("parameter-value")
	providerInputs := resource.PropertyMap{}
	deployproviders.SetProviderVersion(providerInputs, &baseVersion)
	deployproviders.SetProviderName(providerInputs, "terraform-provider")
	deployproviders.SetProviderParameterization(providerInputs, &workspace.Parameterization{
		Name:    "myparam",
		Version: parameterVersion,
		Value:   parameterValue,
	})
	deployproviders.SetProviderURL(providerInputs, "https://example.com/terraform-provider")

	infos, err := resourceReferenceInfos(map[string]string{
		"source": string(referencedURN),
	}, &deploy.Snapshot{
		Resources: []*pkgresource.State{
			{
				Type:   "pulumi:providers:myparam",
				URN:    providerURN,
				Custom: true,
				ID:     providerID,
				Inputs: providerInputs,
			},
			{
				Type:     referencedURN.Type(),
				URN:      referencedURN,
				Custom:   true,
				ID:       "resource-id",
				Provider: providerRef.String(),
			},
		},
	})
	require.NoError(t, err)

	ref := infos["source"]
	assert.Equal(t, "myparam:index:Thing", ref.Token)
	require.NotNil(t, ref.Package)
	assert.Equal(t, "terraform-provider", ref.Package.Package)
	assert.Equal(t, "2.0.0", ref.Package.Version)
	assert.Equal(t, "https://example.com/terraform-provider", ref.Package.DownloadUrl)
	require.NotNil(t, ref.Package.Parameterization)
	assert.Equal(t, "myparam", ref.Package.Parameterization.Name)
	assert.Equal(t, "1.2.3", ref.Package.Parameterization.Version)
	assert.Equal(t, parameterValue, ref.Package.Parameterization.Value)
}

// TestDoCmdResourceUpsertReusesExistingSnippet asserts that upsert against a stack whose snapshot
// already has a snippet with the same (Name, Type) reuses that snippet's UUID (so the engine
// replaces the snippet in place rather than adding a duplicate).
//
//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdResourceUpsertReusesExistingSnippet(t *testing.T) {
	existing := resource.Snippet{
		UUID: "3fa85f64-5717-4562-b3fc-2c963f66afa6",
		Name: "myres", Type: "azure:index:myResource",
		Code:       `name = "old"`,
		Descriptor: resource.PackageDescriptor{Name: "azure"},
	}
	mws, mlm := installMockUpsertBackend(t, &deploy.Snapshot{Snippets: []resource.Snippet{existing}})

	var got StatefulUpdateRequest
	stub := func(_ context.Context, _ *pflag.FlagSet, req StatefulUpdateRequest,
	) (*StatefulUpdateResult, error) {
		got = req
		return &StatefulUpdateResult{SnippetUUIDs: []string{req.Snippets[len(req.Snippets)-1].UUID}}, nil
	}
	loader := func(_ context.Context, _ *plugin.Context, _, source string) (plugin.Provider, error) {
		return &testProvider{spec: doResourceSpec(false)}, nil
	}
	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin, stub)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	inputFile := writeHCLFile(t, "upsert.pcl", `name = "new"`)
	cmd.SetArgs([]string{
		"azure:index:myResource", "upsert", "myres", "--yes",
		"--input", "pcl", "--input-file", inputFile,
	})
	require.NoError(t, cmd.Execute())

	assert.Equal(t, existing.UUID, got.Snippets[len(got.Snippets)-1].UUID,
		"existing snippet UUID should be reused")
	assert.Equal(t, `name = "new"`, got.Snippets[len(got.Snippets)-1].Code,
		"code should be replaced with the new file contents")
	assert.Contains(t, stdout.String(), "Updated myres")
	_ = stderr
}

// TestDoCmdResourceUpsertEndToEnd drives `pulumi do <token> upsert` through the real deployment
// engine (via the lifecycletest framework) so we can assert that a fresh snippet ends up in the
// snapshot and the engine creates the underlying resource. The runStatefulUpdate stub bridges the
// CLI's constructed StatefulUpdateRequest into lt.TestOp(engine.Update).RunStep with a stub
// provider — same pattern the engine's own PCL snippet tests use.
//
//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdResourceUpsertEndToEnd(t *testing.T) {
	// The lifecycletest framework hardcodes UpdateInfo.Root to "/", which is not an absolute path
	// on Windows and trips plugin.NewProgramInfo's IsAbs check. The rest of pkg/engine/lifecycletest
	// is skipped on Windows for the same class of issues (see TODO[pulumi/pulumi#19675]).
	if runtime.GOOS == "windows" {
		t.Skip("lifecycletest framework is not supported on windows (pulumi/pulumi#19675)")
	}

	// Schema matches doResourceSpec(false); the loader serves it back on GetSchema so PCL binding
	// inside the engine's snippet runner resolves `azure:index:myResource` and validates propA/name.
	const azureSchemaJSON = `{
  "version": "0.0.1",
  "name": "azure",
  "resources": {
    "azure:index:myResource": {
      "inputProperties": {
        "name": {"type": "string"},
        "size": {"type": "integer"},
        "enabled": {"type": "boolean"}
      },
      "requiredInputs": ["name"]
    }
  }
}`

	var createdInputs resource.PropertyMap
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("azure", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				GetSchemaF: func(_ context.Context, _ plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					return plugin.GetSchemaResponse{Schema: []byte(azureSchemaJSON)}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					createdInputs = req.Properties
					return plugin.CreateResponse{
						ID:         "res-1",
						Properties: req.Properties,
					}, nil
				},
			}, nil
		}),
	}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			SkipDisplayTests: true,
			T:                t,
			HostF:            deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...),
		},
	}

	// Establish an empty baseline snapshot so subsequent updates have a target to build on.
	baseSnap, err := lt.TestOp(engine.Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0",
	)
	require.NoError(t, err)

	var finalSnap *deploy.Snapshot
	stub := func(_ context.Context, _ *pflag.FlagSet, req StatefulUpdateRequest,
	) (*StatefulUpdateResult, error) {
		snap := baseSnap
		snap.Snippets = append([]resource.Snippet{}, req.Snippets...)
		var err error
		finalSnap, err = lt.TestOp(engine.Update).RunStep(
			p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1",
		)
		if err != nil {
			return nil, err
		}
		return &StatefulUpdateResult{SnippetUUIDs: []string{req.Snippets[len(req.Snippets)-1].UUID}}, nil
	}

	mws, mlm := installMockUpsertBackend(t, &deploy.Snapshot{})
	loader := func(_ context.Context, _ *plugin.Context, _, source string) (plugin.Provider, error) {
		return &testProvider{spec: doResourceSpec(false)}, nil
	}
	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin, stub)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	inputFile := writeHCLFile(t, "upsert.pcl", `name = "myres"
size = 3
`)
	cmd.SetArgs([]string{
		"azure:index:myResource", "upsert", "myres", "--yes",
		"--input", "pcl", "--input-file", inputFile,
	})
	require.NoError(t, cmd.Execute())

	require.NotNil(t, finalSnap, "engine update should have run")
	require.Len(t, finalSnap.Snippets, 1, "snippet should be persisted in the snapshot")
	assert.Equal(t, "myres", finalSnap.Snippets[0].Name)
	assert.Equal(t, "azure:index:myResource", finalSnap.Snippets[0].Type)

	// The engine registers a default provider for the package plus the snippet's resource; the
	// order is provider first, resource second (see TestPclSnippet in pcl_test.go).
	require.Len(t, finalSnap.Resources, 2)
	assert.Equal(t, tokens.Type("pulumi:providers:azure"), finalSnap.Resources[0].Type)
	assert.Equal(t, tokens.Type("azure:index:myResource"), finalSnap.Resources[1].Type)
	assert.Equal(t, "myres", finalSnap.Resources[1].URN.Name())

	require.NotNil(t, createdInputs, "provider.Create should have been called")
	assert.Equal(t, "myres", createdInputs["name"].StringValue())
	assert.Equal(t, 3.0, createdInputs["size"].NumberValue())
	_ = stderr
}

// TestDoCmdResourceUpsertStateless drives `upsert` in stateless mode: the given ID is read from
// the provider, and the resource is either fully updated (existing) or created (missing).
func TestDoCmdResourceUpsertStateless(t *testing.T) {
	t.Parallel()

	t.Run("fully updates an existing resource", func(t *testing.T) {
		t.Parallel()
		var calls []string
		cmd, stdout, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					calls = append(calls, "read")
					assert.Equal(t, resource.ID("res-1"), req.ID)
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID: "res-1",
							Inputs: resource.PropertyMap{
								"name":    resource.NewProperty("old"),
								"size":    resource.NewProperty(1.0),
								"enabled": resource.NewProperty(true),
							},
							Outputs: resource.PropertyMap{
								"name":    resource.NewProperty("old"),
								"size":    resource.NewProperty(1.0),
								"enabled": resource.NewProperty(true),
							},
						},
					}, nil
				},
				CheckF: func(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
					calls = append(calls, "check")
					assert.Equal(t, "old", req.Olds["name"].StringValue())
					assert.Equal(t, "new", req.News["name"].StringValue())
					assert.Equal(t, 2.0, req.News["size"].NumberValue())
					_, hasEnabled := req.News["enabled"]
					assert.False(t, hasEnabled, "inputs should be fully replaced, not merged")
					return plugin.CheckResponse{Properties: req.News}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
					calls = append(calls, "diff")
					return plugin.DiffResponse{
						Changes:     plugin.DiffSome,
						ChangedKeys: []resource.PropertyKey{"name", "size", "enabled"},
					}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					calls = append(calls, "update")
					assert.Equal(t, "new", req.NewInputs["name"].StringValue())
					assert.Equal(t, 2.0, req.NewInputs["size"].NumberValue())
					_, hasEnabled := req.NewInputs["enabled"]
					assert.False(t, hasEnabled, "inputs should be fully replaced, not merged")
					return plugin.UpdateResponse{
						Properties: resource.PropertyMap{
							"name": resource.NewProperty("new"),
							"size": resource.NewProperty(2.0),
						},
					}, nil
				},
			},
		})

		inputFile := writeHCLFile(t, "upsert.pcl", `
name = "new"
size = 2
`)
		cmd.SetArgs([]string{
			"--stateless", "azure:index:myResource", "upsert", "res-1", "--yes",
			"--input", "pcl", "--input-file", inputFile, "--output", "json",
		})
		require.NoError(t, cmd.Execute())
		assert.Equal(t, []string{"read", "check", "diff", "update"}, calls)
		assert.JSONEq(t, `{"id":"res-1","name":"new","size":2}`, stdout.String())
	})

	t.Run("creates a missing resource", func(t *testing.T) {
		t.Parallel()
		var calls []string
		cmd, stdout, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					calls = append(calls, "read")
					assert.Equal(t, resource.ID("res-1"), req.ID)
					return plugin.ReadResponse{}, nil
				},
				CheckF: func(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
					calls = append(calls, "check")
					assert.Empty(t, req.Olds)
					assert.Equal(t, "new", req.News["name"].StringValue())
					return plugin.CheckResponse{Properties: req.News}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					calls = append(calls, "create")
					assert.Equal(t, "new", req.Properties["name"].StringValue())
					return plugin.CreateResponse{
						ID: "res-2",
						Properties: resource.PropertyMap{
							"name": resource.NewProperty("new"),
							"size": resource.NewProperty(2.0),
						},
					}, nil
				},
			},
		})

		inputFile := writeHCLFile(t, "upsert.pcl", `
name = "new"
size = 2
`)
		cmd.SetArgs([]string{
			"--stateless", "azure:index:myResource", "upsert", "res-1", "--yes",
			"--input", "pcl", "--input-file", inputFile, "--output", "json",
		})
		require.NoError(t, cmd.Execute())
		assert.Equal(t, []string{"read", "check", "create"}, calls)
		assert.JSONEq(t, `{"id":"res-2","name":"new","size":2}`, stdout.String())
	})

	t.Run("creates a missing resource read back as emptied state", func(t *testing.T) {
		t.Parallel()
		var calls []string
		cmd, stdout, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					calls = append(calls, "read")
					return plugin.ReadResponse{ReadResult: plugin.ReadResult{
						Inputs:  resource.PropertyMap{},
						Outputs: resource.PropertyMap{},
					}}, nil
				},
				CheckF: func(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
					calls = append(calls, "check")
					return plugin.CheckResponse{Properties: req.News}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					calls = append(calls, "create")
					return plugin.CreateResponse{
						ID: "res-2",
						Properties: resource.PropertyMap{
							"name": resource.NewProperty("new"),
						},
					}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					require.Fail(t, "Update should not be called for a missing resource")
					return plugin.UpdateResponse{}, nil
				},
			},
		})

		inputFile := writeHCLFile(t, "upsert.pcl", `name = "new"`)
		cmd.SetArgs([]string{
			"--stateless", "azure:index:myResource", "upsert", "res-1", "--yes",
			"--input", "pcl", "--input-file", inputFile, "--output", "json",
		})
		require.NoError(t, cmd.Execute())
		assert.Equal(t, []string{"read", "check", "create"}, calls)
		assert.JSONEq(t, `{"id":"res-2","name":"new"}`, stdout.String())
	})
}

// TestDoCmdResourceStatefulCreateConstructsSnippet mirrors the upsert-constructs-snippet test but
// for stateful `create`: the CLI takes a name arg, mints a fresh UUID, hands the snippet to
// runStatefulUpdate, and the completion line uses the "Created" verb.
//
//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdResourceStatefulCreateConstructsSnippet(t *testing.T) {
	mws, mlm := installMockUpsertBackend(t, &deploy.Snapshot{})

	var got StatefulUpdateRequest
	var called bool
	stub := func(_ context.Context, _ *pflag.FlagSet, req StatefulUpdateRequest,
	) (*StatefulUpdateResult, error) {
		called = true
		got = req
		return &StatefulUpdateResult{SnippetUUIDs: []string{req.Snippets[len(req.Snippets)-1].UUID}}, nil
	}
	loader := func(_ context.Context, _ *plugin.Context, _, source string) (plugin.Provider, error) {
		return &testProvider{spec: doResourceSpec(false)}, nil
	}
	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin, stub)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	inputFile := writeHCLFile(t, "create.pcl", `name = "example"`)
	cmd.SetArgs([]string{
		"azure:index:myResource", "create", "myres", "--yes",
		"--input", "pcl", "--input-file", inputFile,
	})
	require.NoError(t, cmd.Execute())

	require.True(t, called, "runStatefulUpdate should have been invoked")
	assert.Equal(t, "myres", got.Snippets[len(got.Snippets)-1].Name)
	assert.Equal(t, "azure:index:myResource", got.Snippets[len(got.Snippets)-1].Type)
	assert.Contains(t, got.Snippets[len(got.Snippets)-1].Code, `name = "example"`)
	require.NotEmpty(t, got.Snippets[len(got.Snippets)-1].UUID)
	assert.Contains(t, stdout.String(), "Created myres")
	_ = stderr
}

//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdResourceStatefulCreateConstructsSnippetWithReferences(t *testing.T) {
	referencedURN := resource.URN("urn:pulumi:dev::proj::azure:index:myResource::source")
	mws, mlm := installMockUpsertBackend(t, snapshotWithReferencedResource(t, referencedURN))

	var got StatefulUpdateRequest
	stub := func(_ context.Context, _ *pflag.FlagSet, req StatefulUpdateRequest,
	) (*StatefulUpdateResult, error) {
		got = req
		return &StatefulUpdateResult{SnippetUUIDs: []string{req.Snippets[len(req.Snippets)-1].UUID}}, nil
	}
	loader := func(_ context.Context, _ *plugin.Context, _, source string) (plugin.Provider, error) {
		assert.Equal(t, "azure", source)
		return &testProvider{spec: doResourceSpec(false)}, nil
	}
	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin, stub)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	inputFile := writeHCLFile(t, "create.pcl", `name = source.name`)
	resourcesFile := writeHCLFile(t, "resources.json", `{"source":"`+string(referencedURN)+`"}`)
	cmd.SetArgs([]string{
		"azure:index:myResource", "create", "myres", "--yes",
		"--input", "pcl", "--input-file", inputFile, "--resources-file", resourcesFile,
	})
	require.NoError(t, cmd.Execute())

	assert.Equal(t, map[string]string{"source": string(referencedURN)}, got.Snippets[len(got.Snippets)-1].References)
}

func TestReadResourceReferences(t *testing.T) {
	t.Parallel()

	referencedURN := "urn:pulumi:dev::proj::azure:index:myResource::source"
	resourcesFile := writeHCLFile(t, "resources.json", `{"source":"`+referencedURN+`"}`)

	refs, err := readResourceReferences(resourcesFile)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"source": referencedURN}, refs)
}

func TestReadResourceReferencesRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contents string
		wantErr  string
	}{
		{
			name:     "invalid urn",
			contents: `{"source":"not-a-urn"}`,
			wantErr:  `invalid URN for "source"`,
		},
		{
			name:     "not object",
			contents: `[]`,
			wantErr:  "parse resources file",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resourcesFile := writeHCLFile(t, "resources.json", tt.contents)
			_, err := readResourceReferences(resourcesFile)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// TestDoCmdResourceStatefulCreateRejectsExisting checks the invariant that distinguishes create
// from upsert: if a snippet with the same (Name, Type) already lives in the snapshot, create
// refuses rather than replacing it in place.
//
//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdResourceStatefulCreateRejectsExisting(t *testing.T) {
	existing := resource.Snippet{
		UUID: "3fa85f64-5717-4562-b3fc-2c963f66afa6",
		Name: "myres", Type: "azure:index:myResource",
		Code:       `name = "old"`,
		Descriptor: resource.PackageDescriptor{Name: "azure"},
	}
	mws, mlm := installMockUpsertBackend(t, &deploy.Snapshot{Snippets: []resource.Snippet{existing}})

	stub := func(_ context.Context, _ *pflag.FlagSet, _ StatefulUpdateRequest,
	) (*StatefulUpdateResult, error) {
		require.Fail(t, "runStatefulUpdate should not be called when the snippet already exists")
		return nil, nil
	}
	loader := func(_ context.Context, _ *plugin.Context, _, source string) (plugin.Provider, error) {
		return &testProvider{spec: doResourceSpec(false)}, nil
	}
	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin, stub)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	inputFile := writeHCLFile(t, "create.pcl", `name = "new"`)
	cmd.SetArgs([]string{
		"azure:index:myResource", "create", "myres", "--yes",
		"--input", "pcl", "--input-file", inputFile,
	})
	err := cmd.Execute()
	require.ErrorContains(t, err, "already exists")
	require.ErrorContains(t, err, "upsert")
	_ = stdout
	_ = stderr
}

// TestDoCmdResourceStatefulDeleteConstructsSnippetDelete checks that stateful `delete` resolves
// the existing snippet by (Name, Type), targets that snippet UUID, and marks the request as a
// snippet deletion.
//
//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdResourceStatefulDeleteConstructsSnippetDelete(t *testing.T) {
	existing := resource.Snippet{
		UUID: "3fa85f64-5717-4562-b3fc-2c963f66afa6",
		Name: "myres", Type: "azure:index:myResource",
		Code:       `name = "old"`,
		Descriptor: resource.PackageDescriptor{Name: "azure"},
	}
	mws, mlm := installMockUpsertBackend(t, &deploy.Snapshot{Snippets: []resource.Snippet{existing}})

	var got StatefulUpdateRequest
	var called bool
	stub := func(_ context.Context, _ *pflag.FlagSet, req StatefulUpdateRequest,
	) (*StatefulUpdateResult, error) {
		called = true
		got = req
		return &StatefulUpdateResult{SnippetUUIDs: []string{req.Snippets[len(req.Snippets)-1].UUID}}, nil
	}
	loader := func(_ context.Context, _ *plugin.Context, _, source string) (plugin.Provider, error) {
		return &testProvider{spec: doResourceSpec(false)}, nil
	}
	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin, stub)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	cmd.SetArgs([]string{"azure:index:myResource", "delete", "myres", "--yes"})
	require.NoError(t, cmd.Execute())

	require.True(t, called, "runStatefulUpdate should have been invoked")
	assert.True(t, got.Delete)
	assert.Equal(t, existing.UUID, got.Snippets[len(got.Snippets)-1].UUID)
	assert.Equal(t, "myres", got.Snippets[len(got.Snippets)-1].Name)
	assert.Equal(t, "azure:index:myResource", got.Snippets[len(got.Snippets)-1].Type)
	assert.Contains(t, stdout.String(), "Deleted myres")
	assert.Contains(t, stdout.String(), existing.UUID)
	_ = stderr
}

// TestDoCmdResourceStatefulDeleteRejectsMissing checks that delete refuses to run a backend update
// when the named snippet does not exist in the stack snapshot.
//
//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdResourceStatefulDeleteRejectsMissing(t *testing.T) {
	mws, mlm := installMockUpsertBackend(t, &deploy.Snapshot{})

	stub := func(_ context.Context, _ *pflag.FlagSet, _ StatefulUpdateRequest,
	) (*StatefulUpdateResult, error) {
		require.Fail(t, "runStatefulUpdate should not be called when the snippet is missing")
		return nil, nil
	}
	loader := func(_ context.Context, _ *plugin.Context, _, source string) (plugin.Provider, error) {
		return &testProvider{spec: doResourceSpec(false)}, nil
	}
	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin, stub)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	cmd.SetArgs([]string{"azure:index:myResource", "delete", "myres", "--yes"})
	err := cmd.Execute()
	require.ErrorContains(t, err, "does not exist")
	require.ErrorContains(t, err, "myres")
	_ = stdout
	_ = stderr
}

// TestDoCmdResourceCreateStatelessTakesNoNameArg pins the diverging UX between the two `create`
// variants: stateless `create` uses the resource type's short name (no positional arg), stateful
// `create` requires an explicit `<name>`.
func TestDoCmdResourceCreateStatelessTakesNoNameArg(t *testing.T) {
	t.Parallel()

	cmd, stdout, _ := newDoResourceCommand(t, &testProvider{spec: doResourceSpec(false)})
	cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "create", "myres"})
	err := cmd.Execute()
	require.Error(t, err, "stateless create should reject a positional name arg")
	_ = stdout
}

// TestDoCmdResourceUpsertMergesInputFlags verifies that --input-* flags are merged into the
// snippet body at the PCL AST layer: the flag value replaces (or is added alongside) attributes
// from --input-file so what the engine persists matches what the user typed on the command line.
//
//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdResourceUpsertMergesInputFlags(t *testing.T) {
	mws, mlm := installMockUpsertBackend(t, &deploy.Snapshot{})
	var got StatefulUpdateRequest
	stub := func(_ context.Context, _ *pflag.FlagSet, req StatefulUpdateRequest,
	) (*StatefulUpdateResult, error) {
		got = req
		return &StatefulUpdateResult{SnippetUUIDs: []string{req.Snippets[len(req.Snippets)-1].UUID}}, nil
	}
	loader := func(_ context.Context, _ *plugin.Context, _, source string) (plugin.Provider, error) {
		return &testProvider{spec: doResourceSpec(false)}, nil
	}
	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin, stub)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	inputFile := writeHCLFile(t, "upsert.pcl", `name = "example"`)
	cmd.SetArgs([]string{
		"azure:index:myResource", "upsert", "myres", "--yes",
		"--input", "pcl", "--input-file", inputFile,
		"--size", "3",
	})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, got.Snippets[len(got.Snippets)-1].Code, `name = "example"`)
	assert.Contains(t, got.Snippets[len(got.Snippets)-1].Code, "size = 3")
	_ = stdout
	_ = stderr
}

// TestDoCmdResourceUpsertAllowsInputFlagsWithoutFile verifies that --input-file is optional when
// the resource inputs can be supplied entirely through --input-* flags.
//
//nolint:paralleltest // installMockUpsertBackend calls t.Setenv.
func TestDoCmdResourceUpsertAllowsInputFlagsWithoutFile(t *testing.T) {
	mws, mlm := installMockUpsertBackend(t, &deploy.Snapshot{})
	var got StatefulUpdateRequest
	stub := func(_ context.Context, _ *pflag.FlagSet, req StatefulUpdateRequest,
	) (*StatefulUpdateResult, error) {
		got = req
		return &StatefulUpdateResult{SnippetUUIDs: []string{req.Snippets[len(req.Snippets)-1].UUID}}, nil
	}
	loader := func(_ context.Context, _ *plugin.Context, _, source string) (plugin.Provider, error) {
		return &testProvider{spec: doResourceSpec(false)}, nil
	}
	var stdout, stderr bytes.Buffer
	cmd := NewDoCmd(mlm, mws, loader, testHost, panicLoadConverterPlugin, stub)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	cmd.SetArgs([]string{
		"azure:index:myResource", "upsert", "myres", "--yes",
		"--input", "pcl",
		"--name", "example",
		"--size", "3",
	})
	require.NoError(t, cmd.Execute())

	assert.Equal(t, `name = "example"
size = 3
`, got.Snippets[len(got.Snippets)-1].Code)
	_ = stdout
	_ = stderr
}

//nolint:paralleltest // Uses t.Chdir
func TestDefaultRunStatefulUpdateDryRunUsesPreview(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(root+"/Pulumi.yaml", []byte("name: proj\nruntime: yaml\n"), 0o600))
	t.Chdir(root)

	stackRef := &backend.MockStackReference{
		StringV:             "myorg/proj/dev",
		NameV:               tokens.MustParseStackName("dev"),
		FullyQualifiedNameV: "myorg/proj/dev",
	}
	var previewCalled bool
	mockBackend := &backend.MockBackend{
		PreviewF: func(_ context.Context, _ backend.Stack, op backend.UpdateOperation,
		) (*deploy.Plan, sdkDisplay.ResourceChanges, error) {
			previewCalled = true
			require.Len(t, op.Opts.Engine.TargetSnippets, 1)
			assert.Equal(t, "3fa85f64-5717-4562-b3fc-2c963f66afa6", op.Opts.Engine.TargetSnippets[0])
			require.Len(t, op.Opts.Engine.Snippets, 1)
			return nil, nil, nil
		},
		UpdateF: func(context.Context, backend.Stack, backend.UpdateOperation) (sdkDisplay.ResourceChanges, error) {
			require.Fail(t, "dry-run upsert must not call UpdateStack")
			return nil, nil
		},
	}
	mockStack := &backend.MockStack{
		RefF:            func() backend.StackReference { return stackRef },
		ConfigLocationF: func() backend.StackConfigLocation { return backend.StackConfigLocation{IsRemote: true} },
		LoadRemoteF: func(context.Context, *workspace.Project) (*workspace.ProjectStack, error) {
			return &workspace.ProjectStack{}, nil
		},
		DefaultSecretManagerF: func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
			return b64.NewBase64SecretsManager(), nil
		},
		SnapshotF: func(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
			return &deploy.Snapshot{SecretsManager: b64.NewBase64SecretsManager()}, nil
		},
		BackendF: func() backend.Backend { return mockBackend },
	}
	var stdout, stderr bytes.Buffer

	result, err := DefaultRunStatefulUpdate(
		t.Context(), pflag.NewFlagSet("test", pflag.ContinueOnError), StatefulUpdateRequest{
			Snippets: []resource.Snippet{{
				UUID:       "3fa85f64-5717-4562-b3fc-2c963f66afa6",
				Name:       "myres",
				Type:       "azure:index:myResource",
				Code:       `name = "myres"`,
				Descriptor: resource.PackageDescriptor{Name: "azure"},
			}},
			Stack:  mockStack,
			DryRun: true,
			Proj:   &workspace.Project{Name: tokens.PackageName("proj")},
			Root:   root,
			Sink:   diag.DefaultSink(&stdout, &stderr, diag.FormatOptions{Color: colors.Never}),
		},
	)
	require.NoError(t, err)
	require.True(t, previewCalled, "dry-run upsert should call PreviewStack")
	require.Equal(t, []string{"3fa85f64-5717-4562-b3fc-2c963f66afa6"}, result.SnippetUUIDs)
}

//nolint:paralleltest // Uses t.Chdir
func TestDefaultRunStatefulUpdateYesAutoApprovesUpdate(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(root+"/Pulumi.yaml", []byte("name: proj\nruntime: yaml\n"), 0o600))
	t.Chdir(root)

	stackRef := &backend.MockStackReference{
		StringV:             "myorg/proj/dev",
		NameV:               tokens.MustParseStackName("dev"),
		FullyQualifiedNameV: "myorg/proj/dev",
	}
	var updateCalled bool
	mockBackend := &backend.MockBackend{
		PreviewF: func(
			context.Context, backend.Stack, backend.UpdateOperation,
		) (*deploy.Plan, sdkDisplay.ResourceChanges, error) {
			require.Fail(t, "non-dry-run upsert should use UpdateStack and let the backend run its normal preview")
			return nil, nil, nil
		},
		UpdateF: func(_ context.Context, _ backend.Stack, op backend.UpdateOperation) (sdkDisplay.ResourceChanges, error) {
			updateCalled = true
			assert.True(t, op.Opts.AutoApprove, "--yes should auto-approve the backend preview prompt")
			require.Len(t, op.Opts.Engine.TargetSnippets, 1)
			assert.Equal(t, "3fa85f64-5717-4562-b3fc-2c963f66afa6", op.Opts.Engine.TargetSnippets[0])
			return nil, nil
		},
	}
	mockStack := &backend.MockStack{
		RefF:            func() backend.StackReference { return stackRef },
		ConfigLocationF: func() backend.StackConfigLocation { return backend.StackConfigLocation{IsRemote: true} },
		LoadRemoteF: func(context.Context, *workspace.Project) (*workspace.ProjectStack, error) {
			return &workspace.ProjectStack{}, nil
		},
		DefaultSecretManagerF: func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
			return b64.NewBase64SecretsManager(), nil
		},
		SnapshotF: func(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
			return &deploy.Snapshot{SecretsManager: b64.NewBase64SecretsManager()}, nil
		},
		BackendF: func() backend.Backend { return mockBackend },
	}
	var stdout, stderr bytes.Buffer

	result, err := DefaultRunStatefulUpdate(
		t.Context(), pflag.NewFlagSet("test", pflag.ContinueOnError), StatefulUpdateRequest{
			Snippets: []resource.Snippet{{
				UUID:       "3fa85f64-5717-4562-b3fc-2c963f66afa6",
				Name:       "myres",
				Type:       "azure:index:myResource",
				Code:       `name = "myres"`,
				Descriptor: resource.PackageDescriptor{Name: "azure"},
			}},
			Stack: mockStack,
			Yes:   true,
			Proj:  &workspace.Project{Name: tokens.PackageName("proj")},
			Root:  root,
			Sink:  diag.DefaultSink(&stdout, &stderr, diag.FormatOptions{Color: colors.Never}),
		},
	)
	require.NoError(t, err)
	require.True(t, updateCalled, "non-dry-run upsert should call UpdateStack")
	require.Equal(t, []string{"3fa85f64-5717-4562-b3fc-2c963f66afa6"}, result.SnippetUUIDs)
}

//nolint:paralleltest // Uses t.Chdir
func TestDefaultRunStatefulUpdateDeletePassesNilSnippet(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(root+"/Pulumi.yaml", []byte("name: proj\nruntime: yaml\n"), 0o600))
	t.Chdir(root)

	stackRef := &backend.MockStackReference{
		StringV:             "myorg/proj/dev",
		NameV:               tokens.MustParseStackName("dev"),
		FullyQualifiedNameV: "myorg/proj/dev",
	}
	var updateCalled bool
	mockBackend := &backend.MockBackend{
		UpdateF: func(_ context.Context, _ backend.Stack, op backend.UpdateOperation) (sdkDisplay.ResourceChanges, error) {
			updateCalled = true
			require.Len(t, op.Opts.Engine.TargetSnippets, 1)
			assert.Equal(t, "3fa85f64-5717-4562-b3fc-2c963f66afa6", op.Opts.Engine.TargetSnippets[0])
			require.Len(t, op.Opts.Engine.Snippets, 1)
			snippetID := uuid.Must(uuid.FromString("3fa85f64-5717-4562-b3fc-2c963f66afa6"))
			assert.Nil(t, op.Opts.Engine.Snippets[snippetID])
			return nil, nil
		},
	}
	mockStack := &backend.MockStack{
		RefF:            func() backend.StackReference { return stackRef },
		ConfigLocationF: func() backend.StackConfigLocation { return backend.StackConfigLocation{IsRemote: true} },
		LoadRemoteF: func(context.Context, *workspace.Project) (*workspace.ProjectStack, error) {
			return &workspace.ProjectStack{}, nil
		},
		DefaultSecretManagerF: func(context.Context, *workspace.ProjectStack) (secrets.Manager, error) {
			return b64.NewBase64SecretsManager(), nil
		},
		SnapshotF: func(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
			return &deploy.Snapshot{SecretsManager: b64.NewBase64SecretsManager()}, nil
		},
		BackendF: func() backend.Backend { return mockBackend },
	}
	var stdout, stderr bytes.Buffer

	result, err := DefaultRunStatefulUpdate(
		t.Context(), pflag.NewFlagSet("test", pflag.ContinueOnError), StatefulUpdateRequest{
			Snippets: []resource.Snippet{{
				UUID: "3fa85f64-5717-4562-b3fc-2c963f66afa6",
				Name: "myres",
				Type: "azure:index:myResource",
			}},
			Stack:  mockStack,
			Yes:    true,
			Delete: true,
			Proj:   &workspace.Project{Name: tokens.PackageName("proj")},
			Root:   root,
			Sink:   diag.DefaultSink(&stdout, &stderr, diag.FormatOptions{Color: colors.Never}),
		},
	)
	require.NoError(t, err)
	require.True(t, updateCalled, "stateful delete should call UpdateStack")
	require.Equal(t, []string{"3fa85f64-5717-4562-b3fc-2c963f66afa6"}, result.SnippetUUIDs)
}
