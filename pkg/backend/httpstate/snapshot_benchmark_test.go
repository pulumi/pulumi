// Copyright 2016-2025, Pulumi Corporation.
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

package httpstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/gofrs/uuid"
	"github.com/pgavlin/fx/v2"
	fxm "github.com/pgavlin/fx/v2/maps"
	fxs "github.com/pgavlin/fx/v2/slices"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/pkg/v3/util/cancel"
	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
)

// The snapshotBackendClient allows calls to the builtin pulumi:pulumi provider to work. A more complete implementation
// would keep track of stack and resource outputs and return real data in replies.
type snapshotBackendClient struct{}

func (snapshotBackendClient) GetStackOutputs(
	ctx context.Context,
	name string,
	onDecryptError func(error) error,
) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

func (snapshotBackendClient) GetStackResourceOutputs(
	ctx context.Context,
	stackName string,
) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

// The snapshotBenchProvider implements plugin.Provider by mapping resource URNs to preloaded data. This allows test
// programs to use arbitrary custom resources without the need to implement providers.
type snapshotBenchProvider struct {
	deploytest.Provider

	outputs gsync.Map[resource.URN, resource.PropertyMap]
}

// Implement Parameterize so that we can return legal results for arbitrary parameterizations.
func (p *snapshotBenchProvider) Parameterize(
	ctx context.Context,
	params plugin.ParameterizeRequest,
) (plugin.ParameterizeResponse, error) {
	var name string
	if v, ok := params.Parameters.(*plugin.ParameterizeValue); ok {
		name = v.Name
	} else {
		uuid, err := uuid.NewV4()
		if err != nil {
			return plugin.ParameterizeResponse{}, err
		}
		name = uuid.String()
	}

	return plugin.ParameterizeResponse{
		Name:    name,
		Version: semver.MustParse("1.0.0"),
	}, nil
}

// Implement Configure to allow for multiple calls
func (p *snapshotBenchProvider) Configure(
	ctx context.Context,
	req plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *snapshotBenchProvider) Create(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
	outputs, ok := p.outputs.Load(req.URN)
	if !ok {
		return plugin.CreateResponse{}, fmt.Errorf("unknown resource %v", req.URN)
	}
	return plugin.CreateResponse{
		ID:         "id",
		Properties: outputs,
	}, nil
}

func (p *snapshotBenchProvider) Read(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	outputs, ok := p.outputs.Load(req.URN)
	if !ok {
		return plugin.ReadResponse{}, fmt.Errorf("unknown resource %v", req.URN)
	}
	return plugin.ReadResponse{
		ReadResult: plugin.ReadResult{
			ID:      "id",
			Outputs: outputs,
		},
	}, nil
}

func (p *snapshotBenchProvider) Update(ctx context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
	outputs, ok := p.outputs.Load(req.URN)
	if !ok {
		return plugin.UpdateResponse{}, fmt.Errorf("unknown resource %v", req.URN)
	}
	return plugin.UpdateResponse{
		Properties: outputs,
	}, nil
}

// update drives an update for snapshot manager benchmarking.
func update(
	t testing.TB,
	project string,
	stack string,
	base *deploy.Snapshot,
	snapshots engine.SnapshotManager,
	program func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor, provider *snapshotBenchProvider) error,
) error {
	defer contract.IgnoreClose(snapshots)

	stackName, err := tokens.ParseStackName(stack)
	if err != nil {
		return err
	}

	ctx, cancelF := context.WithCancel(t.Context())
	defer cancelF()

	cancelCtx, cancelSrc := cancel.NewContext(ctx)
	defer cancelSrc.Cancel()

	// Drain events.
	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()

	events := make(chan engine.Event)
	go func() {
		for range events {
		}
		wg.Done()
	}()
	defer close(events)

	provider := &snapshotBenchProvider{}
	lang := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return program(info, monitor, provider)
	})
	host := deploytest.NewPluginHost(nil, nil, lang,
		// Set up the bench provider to match any provider request.
		deploytest.NewProviderLoader("*", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return provider, nil
		}),
	)

	_, _, err = engine.Update(
		engine.UpdateInfo{
			Root: t.TempDir(),
			Project: &workspace.Project{
				Name:    tokens.PackageName(project),
				Runtime: workspace.NewProjectRuntimeInfo("test", nil),
			},
			Target: &deploy.Target{
				Name:      stackName,
				Config:    nil,
				Decrypter: config.NopDecrypter,
				Snapshot:  base,
			},
		},
		&engine.Context{
			Cancel:          cancelCtx,
			Events:          events,
			SnapshotManager: snapshots,
			PluginManager:   framework.NopPluginManager{},
			BackendClient:   snapshotBackendClient{},
		},
		engine.UpdateOptions{Host: host},
		false,
	)
	require.NoError(t, err)

	return err
}

// testOrBenchmarkSnapshotManager tests or benchmarks a snapshot manager by running the given program with the given
// snapshot manager. The program will start from an empty statefile.
func testOrBenchmarkSnapshotManager(
	t testing.TB,
	project string,
	stack string,
	newManager func(testing.TB, *deploy.Snapshot) engine.SnapshotManager,
	program func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor, provider *snapshotBenchProvider) error,
) {
	base := &deploy.Snapshot{
		SecretsManager: b64.NewBase64SecretsManager(),
	}
	snapshots := newManager(t, base)

	err := update(t, project, stack, base, snapshots, program)
	require.NoError(t, err)
}

// getRun returns a function that can be invoked with a testing.TB in order to run a single benchmark.
//
// The returned function will run an update that creates the configured number of independent resources with the
// configured amount of random deadweight in each resource state.
func (c dynamicStackCase) getRun(
	t testing.TB,
	newManager func(testing.TB, *deploy.Snapshot) engine.SnapshotManager,
) func(t testing.TB) {
	return func(t testing.TB) {
		r := rand.New(rand.NewSource(int64(c.seed))) //nolint:gosec
		testOrBenchmarkSnapshotManager(
			t,
			"test",
			"test",
			newManager,
			func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor, provider *snapshotBenchProvider) error {
				cancelCtx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ctx, err := pulumi.NewContext(cancelCtx, pulumi.RunInfo{
					Project:     info.Project,
					Stack:       info.Stack,
					Parallel:    info.Parallel,
					DryRun:      info.DryRun,
					MonitorAddr: info.MonitorAddress,
				})
				if err != nil {
					return fmt.Errorf("creating context: %w", err)
				}
				defer contract.IgnoreClose(ctx)

				return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
					type Dummy struct {
						pulumi.CustomResourceState
					}

					for i := 0; i < c.resourceCount; i++ { //nolint:staticcheck
						name := fmt.Sprintf("dummy-%d", i)
						urn := resource.NewURN("test", "test", "", "test:dummy:Dummy", name)

						deadweight := c.pseudoRandomString(r, c.resourcePayloadBytes)
						provider.outputs.Store(urn, resource.PropertyMap{
							"deadweight": resource.NewProperty(deadweight),
						})

						var dummy Dummy
						return ctx.RegisterResource("test:dummy:Dummy", name, pulumi.Map{}, &dummy)
					}
					return nil
				})
			})
	}
}

// recordedReplayCases lists a set of test/benchmark cases, each of which is represented by a state file.
//
// The contents of this slice are determined by the comma-separated list of paths in the PULUMI_TEST_SNAPSHOT_REPLAY
// environment variable.
var recordedReplayCases = []recordedReplayCase{}

func init() {
	for _, c := range strings.Split(os.Getenv("PULUMI_TEST_SNAPSHOT_REPLAY"), ",") {
		if c != "" {
			recordedReplayCases = append(recordedReplayCases, recordedReplayCase(c))
		}
	}
}

type recordedReplayCase string

func (c recordedReplayCase) getName() string {
	return filepath.Base(string(c))
}

// getRun returns a function that can be invoked with a testing.TB in order to run a single benchmark.
//
// The returned function will run an update that registers the resources present in its state file in appropriate
// dependency order.
func (c recordedReplayCase) getRun(
	t testing.TB,
	newManager func(testing.TB, *deploy.Snapshot) engine.SnapshotManager,
) func(t testing.TB) {
	type deployment struct {
		Version    int                   `json:"version"`
		Deployment *apitype.DeploymentV3 `json:"deployment"`
	}

	f, err := os.Open(string(c))
	require.NoError(t, err)
	defer f.Close()

	var d deployment
	err = json.NewDecoder(f).Decode(&d)
	require.NoError(t, err)
	require.Equal(t, 3, d.Version)

	project, stack := "test", "test"
	if len(d.Deployment.Resources) != 0 {
		urn := d.Deployment.Resources[0].URN
		project, stack = string(urn.Project()), string(urn.Stack())
	}

	return func(t testing.TB) {
		testOrBenchmarkSnapshotManager(
			t,
			project,
			stack,
			newManager,
			func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor, provider *snapshotBenchProvider) error {
				r := registrar{project: info.Project, stack: info.Stack, monitor: monitor.Client()}

				// First create resourceData nodes for all of our resources.
				for _, res := range d.Deployment.Resources {
					r.declareResource(t.Context(), res)
				}
				// Next kick off registration for each resource. The registration of a single resource R will wait on
				// any resources that R depends upon.
				for _, res := range d.Deployment.Resources {
					r.registerResource(context.Background(), res, provider)
				}

				r.resources.Range(func(_ resource.URN, res *resourceData) bool {
					if _, _, e := res.wait(); e != nil {
						err = errors.Join(err, fmt.Errorf("registering resource: %w", e))
					}
					return true
				})
				return err
			},
		)
	}
}

func toStrings[S ~string](s []S) []string {
	return slice.Map(s, func(s S) string { return string(s) })
}

// A resourceData node serves as a promise for a resource's URN, ID, and registration error.
type resourceData struct {
	c *sync.Cond

	done bool
	urn  string
	id   string
	err  error
}

func newResourceData() *resourceData {
	m := &sync.Mutex{}
	return &resourceData{c: sync.NewCond(m)}
}

func (r *resourceData) resolve(urn, id string, err error) {
	r.c.L.Lock()
	defer r.c.L.Unlock()

	r.done, r.urn, r.id, r.err = true, urn, id, err
	r.c.Broadcast()
}

func (r *resourceData) wait() (urn, id string, _ error) {
	r.c.L.Lock()
	for !r.done {
		r.c.Wait()
	}
	defer r.c.L.Unlock()
	return r.urn, r.id, r.err
}

// The registrar is responsible for registring resources from a state file and ensuring that registration happens in
// dependency order.
type registrar struct {
	stack     string
	project   string
	monitor   pulumirpc.ResourceMonitorClient
	resources gsync.Map[resource.URN, *resourceData]
}

func (r *registrar) DecryptValue(ctx context.Context, v string) (string, error) {
	return "null", nil
}

func (r *registrar) BatchDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	return config.DefaultBatchDecrypt(ctx, r, ciphertexts)
}

func (r *registrar) mapResourceWithID(resURN resource.URN) (urn, id string, err error) {
	if resURN == "" {
		return "", "", nil
	}

	res, ok := r.resources.Load(resURN)
	if !ok {
		return "", "", fmt.Errorf("missing resource %v", urn)
	}
	return res.wait()
}

func (r *registrar) mapResource(resURN resource.URN) (string, error) {
	urn, _, err := r.mapResourceWithID(resURN)
	return urn, err
}

func (r *registrar) mapResources(urns []resource.URN) ([]string, error) {
	return fxs.TryCollect(fxs.MapUnpack(urns, r.mapResource))
}

func (r *registrar) mapPropertyDependencies(
	deps map[resource.PropertyKey][]resource.URN,
) (map[string]*pulumirpc.RegisterResourceRequest_PropertyDependencies, error) {
	return fxm.TryCollect(fxm.Map(deps, func(k resource.PropertyKey, deps []resource.URN) (
		kvp fx.Pair[string, *pulumirpc.RegisterResourceRequest_PropertyDependencies],
		_ error,
	) {
		urns, err := r.mapResources(deps)
		if err != nil {
			return kvp, err
		}
		return fx.Pack(string(k), &pulumirpc.RegisterResourceRequest_PropertyDependencies{Urns: urns}), nil
	}))
}

func (r *registrar) declareResource(ctx context.Context, res apitype.ResourceV3) {
	if res.Delete || providers.IsDefaultProvider(res.URN) {
		return
	}
	out := newResourceData()
	r.resources.Store(res.URN, out)
}

func (r *registrar) registerResource(ctx context.Context, res apitype.ResourceV3, provider *snapshotBenchProvider) {
	if res.Delete || providers.IsDefaultProvider(res.URN) {
		return
	}

	out, ok := r.resources.Load(res.URN)
	if !ok {
		return
	}
	typ, name := string(res.URN.Type()), res.URN.Name()
	go func() {
		out.resolve(func() (resURN, resID string, _ error) {
			inputs, err := stack.DeserializeProperties(res.Inputs, r)
			if err != nil {
				return "", "", fmt.Errorf("deserializing inputs for %v: %w", res.URN, err)
			}
			inputObject, err := plugin.MarshalProperties(inputs, plugin.MarshalOptions{
				KeepSecrets:   true,
				KeepResources: true,
			})
			if err != nil {
				return "", "", fmt.Errorf("marshaling inputs for %v: %w", res.URN, err)
			}

			outputs, err := stack.DeserializeProperties(res.Outputs, r)
			if err != nil {
				return "", "", fmt.Errorf("deserializing outputs for %v: %w", res.URN, err)
			}

			parent, err := r.mapResource(res.Parent)
			if err != nil {
				return "", "", fmt.Errorf("mapping parent for %v: %w", res.URN, err)
			}

			deps, err := r.mapResources(res.Dependencies)
			if err != nil {
				return "", "", fmt.Errorf("mapping deps for %v: %w", res.URN, err)
			}

			propertyDeps, err := r.mapPropertyDependencies(res.PropertyDependencies)
			if err != nil {
				return "", "", fmt.Errorf("mapping property deps for %v: %w", res.URN, err)
			}

			var providerRef string
			if res.Custom {
				provider.outputs.Store(res.URN, outputs)

				if res.Provider != "" {
					ref, err := providers.ParseReference(res.Provider)
					if err != nil {
						return "", "", fmt.Errorf("parsing provider for %v: %w", res.URN, err)
					}
					if !providers.IsDefaultProvider(ref.URN()) {
						mappedURN, mappedID, err := r.mapResourceWithID(ref.URN())
						if err != nil {
							return "", "", fmt.Errorf("mapping provider for %v: %w", res.URN, err)
						}
						ref, err = providers.NewReference(resource.URN(mappedURN), resource.ID(mappedID))
						if err != nil {
							return "", "", fmt.Errorf("mapping provider for %v: %w", res.URN, err)
						}
						providerRef = ref.String()
					}
				}
			}

			var urn, id string
			if res.External {
				resp, err := r.monitor.ReadResource(ctx, &pulumirpc.ReadResourceRequest{
					Id:           res.ID.String(),
					Type:         typ,
					Name:         name,
					Parent:       parent,
					Properties:   inputObject,
					Dependencies: deps,
					Provider:     providerRef,
				})
				if err != nil {
					return "", "", err
				}
				urn, id = resp.Urn, res.ID.String()
			} else {
				resp, err := r.monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
					Type:                    typ,
					Name:                    name,
					Parent:                  parent,
					Custom:                  res.Custom,
					ImportId:                string(res.ImportID),
					Object:                  inputObject,
					Protect:                 &res.Protect,
					Dependencies:            deps,
					PropertyDependencies:    propertyDeps,
					AcceptSecrets:           true,
					AdditionalSecretOutputs: toStrings(res.AdditionalSecretOutputs),
					AliasURNs:               toStrings(res.Aliases),
					SupportsPartialValues:   true,
					Remote:                  false,
					AcceptResources:         true,
					DeletedWith:             string(res.DeletedWith),
					Provider:                providerRef,
				})
				if err != nil {
					return "", "", err
				}
				urn, id = resp.Urn, resp.Id
			}

			if !res.Custom {
				outputObject, oerr := plugin.MarshalProperties(outputs, plugin.MarshalOptions{
					KeepSecrets:   true,
					KeepResources: true,
				})
				if oerr != nil {
					return "", "", fmt.Errorf("marshaling outputs for %v: %w", res.URN, oerr)
				}

				_, err = r.monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
					Urn:     urn,
					Outputs: outputObject,
				})
			}
			return urn, id, err
		}())
	}()
}

// newMockPersister creates a new cloudSnapshotPersister that uses a mock token source and targets the given httptest
// server.
func newMockPersister(t testing.TB, server *httptest.Server) *cloudSnapshotPersister {
	newMockTokenSource := func() tokenSourceCapability {
		return tokenSourceFn(func() (string, error) {
			return "token", nil
		})
	}

	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	backendGeneric, err := New(t.Context(), sink, server.URL, &workspace.Project{}, false)
	require.NoError(t, err)

	backend := backendGeneric.(*cloudBackend)
	stackID := client.StackIdentifier{Owner: "owner", Project: "project", Stack: tokens.MustParseStackName("stack")}
	return backend.newSnapshotPersister(context.Background(), client.UpdateIdentifier{
		StackIdentifier: stackID,
		UpdateKind:      "update",
		UpdateID:        "update",
	}, newMockTokenSource())
}

type benchmarkServer struct {
	t          testing.TB
	p          *cloudSnapshotPersister
	totalCalls int
	totalBytes int64
}

// newServerPersister creates a new benchmarkServer that implements both SnapshotPersister and JournalPersister. The
// benchmarkServer is responsible for tracking the total number of calls made and bytes sent to persistence endpoints.
func newServerPersister(t testing.TB) *benchmarkServer {
	s := &benchmarkServer{t: t}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	s.p = newMockPersister(t, srv)
	return s
}

// reset prepares the sever for use in a new benchmark iteration.
func (s *benchmarkServer) reset() {
	s.totalCalls, s.totalBytes = 0, 0
}

// persist measures the number of bytes present in the request body and increments the call count.
func (s *benchmarkServer) persist(req *http.Request) {
	n, err := io.Copy(io.Discard, req.Body)
	require.NoError(s.t, err)
	s.totalCalls, s.totalBytes = s.totalCalls+1, s.totalBytes+n
}

func (s *benchmarkServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	_, base := path.Split(req.URL.Path)
	switch base {
	case "capabilities":
		bytes, err := json.Marshal(apitype.DeltaCheckpointUploadsConfigV2{})
		require.NoError(s.t, err)
		resp := apitype.CapabilitiesResponse{Capabilities: []apitype.APICapabilityConfig{{
			Version:       2,
			Capability:    apitype.DeltaCheckpointUploadsV2,
			Configuration: json.RawMessage(bytes),
		}}}
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(s.t, err)
	case "checkpointverbatim", "checkpointdelta", "checkpoint", "journalentries":
		s.persist(req)
		_, err := w.Write([]byte("{}"))
		require.NoError(s.t, err)
	default:
		s.t.Errorf("unsupported path %q", req.URL.Path)
	}
}

func (s *benchmarkServer) Save(deployment *apitype.DeploymentV3, version int, features []string) error {
	return s.p.Save(deployment, version, features)
}

func (s *benchmarkServer) Append(ctx context.Context, entry apitype.JournalEntry) error {
	return s.p.backend.client.SaveJournalEntry(ctx, s.p.update, entry, s.p.tokenSource)
}

func BenchmarkSnapshot(b *testing.B) {
	useJournal := os.Getenv("PULUMI_TEST_SNAPSHOT") == "journal"

	p := newServerPersister(b)
	getManager := func(t testing.TB, base *deploy.Snapshot) engine.SnapshotManager {
		return backend.NewSnapshotManager(p, base.SecretsManager, base)
	}
	if useJournal {
		getManager = func(t testing.TB, base *deploy.Snapshot) engine.SnapshotManager {
			j, err := backend.NewJournaler(t.Context(), p, base.SecretsManager, base)
			require.NoError(b, err)
			m, err := engine.NewJournalSnapshotManager(j, base, base.SecretsManager)
			require.NoError(b, err)
			return m
		}
	}

	b.Run("Dynamic", func(b *testing.B) {
		for _, c := range dynamicCases {
			b.Run(c.getName(), func(b *testing.B) {
				run := c.getRun(b, getManager)
				for i := 0; i < b.N; i++ {
					p.reset()
					run(b)
					b.ReportMetric(float64(p.totalCalls), "calls/op")
					b.ReportMetric(float64(p.totalBytes), "bytes_sent/op")
				}
			})
		}
	})
	if len(recordedReplayCases) != 0 {
		b.Run("Recorded", func(b *testing.B) {
			for _, c := range recordedReplayCases {
				b.Run(c.getName(), func(b *testing.B) {
					run := c.getRun(b, getManager)
					for i := 0; i < b.N; i++ {
						p.reset()
						run(b)
						b.ReportMetric(float64(p.totalCalls), "calls/op")
						b.ReportMetric(float64(p.totalBytes), "bytes_sent/op")
					}
				})
			}
		})
	}
}
