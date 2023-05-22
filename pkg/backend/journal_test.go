package backend_test

import (
	"context"
	"encoding/base64"
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
	"sync"
	"testing"

	"github.com/dustin/go-humanize"
	"github.com/pgavlin/fx"
	"github.com/pgavlin/text"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/pkg/v3/util/cancel"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type updateInfo struct {
	project workspace.Project
	target  deploy.Target
}

func (u *updateInfo) GetRoot() string {
	return ""
}

func (u *updateInfo) GetProject() *workspace.Project {
	return &u.project
}

func (u *updateInfo) GetTarget() *deploy.Target {
	return &u.target
}

func update(base *deploy.Snapshot, snapshots engine.SnapshotManager, program func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error) result.Result {
	defer contract.IgnoreClose(snapshots)

	project, runtime, stack := tokens.PackageName("test"), "test", tokens.Name("test")

	cancelCtx, cancelSrc := cancel.NewContext(context.Background())
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

	lang := deploytest.NewLanguageRuntime(program)
	host := deploytest.NewPluginHost(nil, nil, lang)

	_, _, res := engine.Update(
		&updateInfo{
			project: workspace.Project{
				Name:    project,
				Runtime: workspace.NewProjectRuntimeInfo(runtime, nil),
			},
			target: deploy.Target{
				Name:      stack,
				Config:    nil,
				Decrypter: config.NopDecrypter,
				Snapshot:  base,
			},
		},
		&engine.Context{
			Cancel:          cancelCtx,
			Events:          events,
			SnapshotManager: snapshots,
		},
		engine.UpdateOptions{Host: host},
		false,
	)
	return res
}

func testOrBenchmarkSnapshotManager(t testing.TB, newManager func(testing.TB, *deploy.Snapshot) engine.SnapshotManager, program func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error) {
	base := &deploy.Snapshot{
		SecretsManager: b64.NewBase64SecretsManager(),
	}
	snapshots := newManager(t, base)

	res := update(base, snapshots, program)
	require.Nil(t, res)
}

type dynamicStackCase struct {
	seed                 int
	resourceCount        int
	resourcePayloadBytes int
}

func (c dynamicStackCase) getName() string {
	return fmt.Sprintf("%v_x_%v", c.resourceCount, humanize.Bytes(uint64(c.resourcePayloadBytes)))
}

func (c dynamicStackCase) getRun(t testing.TB, newManager func(testing.TB, *deploy.Snapshot) engine.SnapshotManager) func(t testing.TB) {
	return func(t testing.TB) {
		r := rand.New(rand.NewSource(int64(c.seed)))
		testOrBenchmarkSnapshotManager(t, newManager, func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
				Project:     info.Project,
				Stack:       info.Stack,
				Parallel:    info.Parallel,
				DryRun:      info.DryRun,
				MonitorAddr: info.MonitorAddress,
			})
			if err != nil {
				return fmt.Errorf("creating context: %w", err)
			}

			return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
				type Dummy struct {
					pulumi.ResourceState
				}

				for i := 0; i < c.resourceCount; i++ {
					var dummy Dummy
					err := ctx.RegisterComponentResource("examples:dummy:Dummy", fmt.Sprintf("dummy-%d", i), &dummy)
					if err != nil {
						return err
					}
					err = ctx.RegisterResourceOutputs(&dummy, pulumi.Map{
						"deadweight": pulumi.String(c.pseudoRandomString(r, c.resourcePayloadBytes)),
					})
					if err != nil {
						return err
					}
				}
				return nil
			})
		})
	}
}

func (c dynamicStackCase) pseudoRandomString(r *rand.Rand, desiredLength int) string {
	buf := make([]byte, desiredLength)
	r.Read(buf)
	text := base64.StdEncoding.EncodeToString(buf)
	return text[0:desiredLength]
}

var dynamicCases = []dynamicStackCase{
	dynamicStackCase{seed: 0, resourceCount: 1, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 2, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 4, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 8, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 16, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 32, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 48, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 64, resourcePayloadBytes: 2},
	dynamicStackCase{seed: 0, resourceCount: 1, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 2, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 4, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 8, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 16, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 32, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 48, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 64, resourcePayloadBytes: 8192},
	dynamicStackCase{seed: 0, resourceCount: 1, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 2, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 4, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 8, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 16, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 32, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 48, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 64, resourcePayloadBytes: 32768},
	dynamicStackCase{seed: 0, resourceCount: 2, resourcePayloadBytes: 131072},
	dynamicStackCase{seed: 0, resourceCount: 4, resourcePayloadBytes: 131072},
	dynamicStackCase{seed: 0, resourceCount: 8, resourcePayloadBytes: 131072},
	dynamicStackCase{seed: 0, resourceCount: 16, resourcePayloadBytes: 131072},
	dynamicStackCase{seed: 0, resourceCount: 32, resourcePayloadBytes: 131072},
	dynamicStackCase{seed: 0, resourceCount: 48, resourcePayloadBytes: 131072},
	dynamicStackCase{seed: 0, resourceCount: 64, resourcePayloadBytes: 131072},
	dynamicStackCase{seed: 0, resourceCount: 1, resourcePayloadBytes: 524288},
	dynamicStackCase{seed: 0, resourceCount: 2, resourcePayloadBytes: 524288},
	dynamicStackCase{seed: 0, resourceCount: 4, resourcePayloadBytes: 524288},
	dynamicStackCase{seed: 0, resourceCount: 8, resourcePayloadBytes: 524288},
	dynamicStackCase{seed: 0, resourceCount: 16, resourcePayloadBytes: 524288},
}

type recordedStackCase string

func (c recordedStackCase) getName() string {
	return string(c)
}

func (c recordedStackCase) getRun(t testing.TB, newManager func(testing.TB, *deploy.Snapshot) engine.SnapshotManager) func(t testing.TB) {
	type deployment struct {
		Version    int                   `json:"version"`
		Deployment *apitype.DeploymentV3 `json:"deployment"`
	}

	f, err := os.Open(filepath.Join("testdata", string(c)))
	require.NoError(t, err)
	defer f.Close()

	var d deployment
	err = json.NewDecoder(f).Decode(&d)
	require.NoError(t, err)
	require.Equal(t, 3, d.Version)

	return func(t testing.TB) {
		testOrBenchmarkSnapshotManager(t, newManager, func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			r := registrar{project: info.Project, stack: info.Stack, monitor: monitor.Client()}
			for _, res := range d.Deployment.Resources {
				r.registerResource(context.Background(), res)
			}

			r.resources.Range(func(_, resAny any) bool {
				if _, e := resAny.(*resourceData).wait(); e != nil {
					err = errors.Join(err, fmt.Errorf("registering resource: %w", e))
				}
				return true
			})
			return err
		})
	}
}

var recordedCases = []recordedStackCase{
	recordedStackCase("checkpoints.json"),
	recordedStackCase("parallel.json"),
}

func toStrings[S ~string](s []S) []string {
	return fx.ToSlice(fx.Map(fx.IterSlice(s), func(s S) string { return string(s) }))
}

type resourceData struct {
	c *sync.Cond

	done bool
	urn  string
	err  error
}

func newResourceData() *resourceData {
	m := &sync.Mutex{}
	return &resourceData{c: sync.NewCond(m)}
}

func (r *resourceData) resolve(urn string, err error) {
	r.c.L.Lock()
	defer r.c.L.Unlock()

	r.done, r.urn, r.err = true, urn, err
	r.c.Broadcast()
}

func (r *resourceData) wait() (string, error) {
	r.c.L.Lock()
	for !r.done {
		r.c.Wait()
	}
	defer r.c.L.Unlock()
	return r.urn, r.err
}

type registrar struct {
	stack     string
	project   string
	monitor   pulumirpc.ResourceMonitorClient
	resources sync.Map
}

func (r *registrar) DecryptValue(ctx context.Context, _ string) (string, error) {
	return "null", nil
}

func (r *registrar) BulkDecrypt(ctx context.Context, ciphertexts []string) (map[string]string, error) {
	return config.DefaultBulkDecrypt(ctx, r, ciphertexts)
}

func (r *registrar) mapURN(urn resource.URN) string {
	if !urn.IsValid() {
		return string(urn)
	}

	parentType, typ := tokens.Type(""), urn.QualifiedType()
	if lastDelim := text.LastIndex(typ, resource.URNTypeDelimiter); lastDelim != -1 {
		parentType, typ = typ[:lastDelim], typ[lastDelim+1:]
	}
	return string(resource.NewURN(
		tokens.QName(r.stack),
		tokens.PackageName(r.project),
		parentType,
		typ,
		urn.Name()))
}

func (r *registrar) mapURNs(urns []resource.URN) []string {
	return fx.ToSlice(fx.Map(fx.IterSlice(urns), r.mapURN))
}

func (r *registrar) mapResource(urn resource.URN) (string, error) {
	if resAny, ok := r.resources.Load(urn); ok {
		res := resAny.(*resourceData)
		urn, err := res.wait()
		if err != nil {
			return "", err
		}
		return urn, nil
	}
	return r.mapURN(urn), nil
}

func (r *registrar) mapResources(urns []resource.URN) ([]string, error) {
	return fx.TrySlice(fx.Map(fx.IterSlice(urns), func(urn resource.URN) fx.Result[string] {
		mapped, err := r.mapResource(urn)
		if err != nil {
			return fx.Err[string](err)
		}
		return fx.OK(mapped)
	}))
}

func (r *registrar) mapPropertyDependencies(deps map[resource.PropertyKey][]resource.URN) (map[string]*pulumirpc.RegisterResourceRequest_PropertyDependencies, error) {
	return fx.TryMap(fx.Map(fx.IterMap(deps), func(kvp fx.Pair[resource.PropertyKey, []resource.URN]) fx.Result[fx.Pair[string, *pulumirpc.RegisterResourceRequest_PropertyDependencies]] {
		urns, err := r.mapResources(kvp.Snd)
		if err != nil {
			return fx.Err[fx.Pair[string, *pulumirpc.RegisterResourceRequest_PropertyDependencies]](err)
		}
		return fx.OK(fx.NewPair(string(kvp.Fst), &pulumirpc.RegisterResourceRequest_PropertyDependencies{Urns: urns}))
	}))
}

func (r *registrar) mapProperties(p resource.PropertyMap) resource.PropertyMap {
	m := make(resource.PropertyMap, len(p))
	for k, v := range p {
		m[k] = r.mapProperty(v)
	}
	return m
}

func (r *registrar) mapProperty(p resource.PropertyValue) resource.PropertyValue {
	switch {
	case p.IsArray():
		return resource.NewArrayProperty(fx.ToSlice(fx.Map(fx.IterSlice(p.ArrayValue()), r.mapProperty)))
	case p.IsObject():
		return resource.NewObjectProperty(r.mapProperties(p.ObjectValue()))
	case p.IsResourceReference():
		ref := p.ResourceReferenceValue()
		ref.ID, ref.URN = resource.NewNullProperty(), resource.URN(r.mapURN(ref.URN))
		return resource.NewResourceReferenceProperty(ref)
	default:
		return p
	}
}

func (r *registrar) mapInputs(res *apitype.ResourceV3) (resource.PropertyMap, error) {
	inputs, err := stack.DeserializeProperties(res.Inputs, r, config.NopEncrypter)
	if err != nil {
		return nil, fmt.Errorf("deserializing: %w", err)
	}
	inputs = r.mapProperties(inputs)

	if res.Custom {
		custom := map[string]any{}
		if res.CustomTimeouts != nil {
			timeouts := map[string]any{}
			if res.CustomTimeouts.Create != 0 {
				timeouts["create"] = res.CustomTimeouts.Create
			}
			if res.CustomTimeouts.Delete != 0 {
				timeouts["delete"] = res.CustomTimeouts.Delete
			}
			if res.CustomTimeouts.Update != 0 {
				timeouts["update"] = res.CustomTimeouts.Update
			}
			custom["customTimeouts"] = timeouts
		}
		if res.Delete {
			custom["delete"] = true
		}
		if res.External {
			custom["external"] = true
		}
		if res.ImportID != "" {
			custom["importID"] = res.ImportID
		}
		if res.Provider != "" {
			ref, err := providers.ParseReference(res.Provider)
			if err != nil {
				return nil, fmt.Errorf("parsing provider: %w", err)
			}
			mapped, err := r.mapResource(ref.URN())
			if err != nil {
				return nil, fmt.Errorf("mapping provider: %w", err)
			}
			custom["provider"] = mapped + "::" + ref.ID().String()
		}
		if res.RetainOnDelete {
			custom["retainOnDelete"] = true
		}
		inputs["__custom"] = resource.NewObjectProperty(resource.NewPropertyMapFromMap(custom))
	}

	return inputs, nil
}

func (r *registrar) mapOutputs(res *apitype.ResourceV3) (resource.PropertyMap, error) {
	outputs, err := stack.DeserializeProperties(res.Outputs, r, config.NopEncrypter)
	if err != nil {
		return nil, fmt.Errorf("deserializing: %w", err)
	}
	outputs = r.mapProperties(outputs)

	if res.Custom {
		custom := map[string]any{"id": res.ID}
		if len(res.InitErrors) != 0 {
			custom["initErrors"] = res.InitErrors
		}
		outputs["__custom"] = resource.NewObjectProperty(resource.NewPropertyMapFromMap(custom))
	}

	return outputs, nil
}

func (r *registrar) registerResource(ctx context.Context, res apitype.ResourceV3) {
	typ, name := string(res.URN.Type()), string(res.URN.Name())

	out := newResourceData()
	r.resources.Store(res.URN, out)
	go func() {
		out.resolve(func() (string, error) {
			if providers.IsProviderType(res.URN.Type()) {
				typ = "replay:provider:" + string(providers.GetProviderPackage(res.URN.Type()))
			}

			inputs, err := r.mapInputs(&res)
			if err != nil {
				return "", fmt.Errorf("mapping inputs for %v: %w", res.URN, err)
			}

			inputObject, err := plugin.MarshalProperties(inputs, plugin.MarshalOptions{
				KeepSecrets:   true,
				KeepResources: true,
			})
			if err != nil {
				return "", fmt.Errorf("marshaling inputs for %v: %w", res.URN, err)
			}

			outputs, err := r.mapOutputs(&res)
			if err != nil {
				return "", fmt.Errorf("mapping outputs for %v: %w", res.URN, err)
			}

			outputObject, err := plugin.MarshalProperties(outputs, plugin.MarshalOptions{
				KeepSecrets:   true,
				KeepResources: true,
			})
			if err != nil {
				return "", fmt.Errorf("marshaling outputs for %v: %w", res.URN, err)
			}

			deletedWith := r.mapURN(res.DeletedWith)

			parent, err := r.mapResource(res.Parent)
			if err != nil {
				return "", fmt.Errorf("mapping parent for %v: %w", res.URN, err)
			}

			deps, err := r.mapResources(res.Dependencies)
			if err != nil {
				return "", fmt.Errorf("mapping deps for %v: %w", res.URN, err)
			}

			propertyDeps, err := r.mapPropertyDependencies(res.PropertyDependencies)
			if err != nil {
				return "", fmt.Errorf("mapping property deps for %v: %w", res.URN, err)
			}

			resp, err := r.monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
				Type:                    typ,
				Name:                    name,
				Parent:                  parent,
				Custom:                  false,
				Object:                  inputObject,
				Protect:                 res.Protect,
				Dependencies:            deps,
				PropertyDependencies:    propertyDeps,
				AcceptSecrets:           true,
				AdditionalSecretOutputs: toStrings(res.AdditionalSecretOutputs),
				AliasURNs:               r.mapURNs(res.Aliases),
				SupportsPartialValues:   true,
				Remote:                  false,
				AcceptResources:         true,
				DeletedWith:             deletedWith,
			})
			if err != nil {
				return "", err
			}

			_, err = r.monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
				Urn:     resp.Urn,
				Outputs: outputObject,
			})
			return resp.Urn, err
		}())
	}()
}

type benchmarkServer struct {
	t          testing.TB
	p          any
	totalCalls int
	totalBytes int64
}

func newServerPersister(t testing.TB) *benchmarkServer {
	s := &benchmarkServer{t: t}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	p, err := httpstate.NewMockPersister(srv)
	require.NoError(t, err)
	s.p = p
	return s
}

func (s *benchmarkServer) reset() {
	s.totalCalls, s.totalBytes = 0, 0
}

func (s *benchmarkServer) persist(req *http.Request) error {
	n, err := io.Copy(io.Discard, req.Body)
	assert.NoError(s.t, err)
	s.totalCalls, s.totalBytes = s.totalCalls+1, s.totalBytes+n
	return nil
}

func (s *benchmarkServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	_, base := path.Split(req.URL.Path)
	switch base {
	case "capabilities":
		resp := apitype.CapabilitiesResponse{Capabilities: []apitype.APICapabilityConfig{{
			Capability:    apitype.DeltaCheckpointUploads,
			Configuration: json.RawMessage(`{"checkpointCutoffSizeBytes":0}`),
		}}}
		err := json.NewEncoder(w).Encode(resp)
		assert.NoError(s.t, err)
	case "checkpointverbatim", "checkpointdelta", "checkpoint", "rebase", "journal":
		s.persist(req)
		_, err := w.Write([]byte("{}"))
		assert.NoError(s.t, err)
	default:
		s.t.Errorf("unsupported path %q", req.URL.Path)
	}
}

func (s *benchmarkServer) Save(snapshot *deploy.Snapshot) error {
	return s.p.(backend.SnapshotPersister).Save(snapshot)
}

func (s *benchmarkServer) SecretsManager() secrets.Manager {
	return s.p.(backend.SnapshotPersister).SecretsManager()
}

func (s *benchmarkServer) Rebase(base *apitype.DeploymentV3) error {
	return s.p.(backend.JournalPersister).Rebase(base)
}

func (s *benchmarkServer) Append(entry apitype.JournalEntry) error {
	return s.p.(backend.JournalPersister).Append(entry)
}

type benchmarkPersister struct {
	totalCalls int
	totalBytes int
}

func (p *benchmarkPersister) reset() {
	p.totalCalls, p.totalBytes = 0, 0
}

func (p *benchmarkPersister) persist(v interface{}) error {
	bytes, err := json.Marshal(v)
	if err != nil {
		return err
	}
	p.totalCalls, p.totalBytes = p.totalCalls+1, p.totalBytes+len(bytes)
	return nil
}

func (p *benchmarkPersister) Save(snapshot *deploy.Snapshot) error {
	deployment, err := stack.SerializeDeployment(snapshot, nil, false)
	if err != nil {
		return err
	}
	return p.persist(deployment)
}

func (p *benchmarkPersister) SecretsManager() secrets.Manager {
	return b64.NewBase64SecretsManager()
}

func (p *benchmarkPersister) Rebase(base *apitype.DeploymentV3) error {
	return p.persist(base)
}

func (p *benchmarkPersister) Append(entry apitype.JournalEntry) error {
	return p.persist(entry)
}

func BenchmarkSnapshotPatcher(b *testing.B) {
	for _, c := range dynamicCases {
		b.Run(c.getName(), func(b *testing.B) {
			//p := &benchmarkPersister{}
			p := newServerPersister(b)
			run := c.getRun(b, func(t testing.TB, base *deploy.Snapshot) engine.SnapshotManager {
				return backend.NewSnapshotManager(p, base)
			})
			for i := 0; i < b.N; i++ {
				p.reset()
				run(b)
				b.ReportMetric(float64(p.totalCalls), "calls")
				b.ReportMetric(float64(p.totalBytes), "bytes")
			}
		})
	}
	for _, c := range recordedCases {
		b.Run(c.getName(), func(b *testing.B) {
			//p := &benchmarkPersister{}
			p := newServerPersister(b)
			run := c.getRun(b, func(t testing.TB, base *deploy.Snapshot) engine.SnapshotManager {
				return backend.NewSnapshotManager(p, base)
			})
			for i := 0; i < b.N; i++ {
				p.reset()
				run(b)
				b.ReportMetric(float64(p.totalCalls), "calls")
				b.ReportMetric(float64(p.totalBytes), "bytes")
			}
		})
	}
}

func BenchmarkSnapshotJournal(b *testing.B) {
	for _, c := range dynamicCases {
		b.Run(c.getName(), func(b *testing.B) {
			//p := &benchmarkPersister{}
			p := newServerPersister(b)
			run := c.getRun(b, func(t testing.TB, base *deploy.Snapshot) engine.SnapshotManager {
				j, err := backend.NewJournal(p, base, nil)
				require.NoError(t, err)
				return j
			})
			for i := 0; i < b.N; i++ {
				p.reset()
				run(b)
				b.ReportMetric(float64(p.totalCalls), "calls")
				b.ReportMetric(float64(p.totalBytes), "bytes")
			}
		})
	}
	for _, c := range recordedCases {
		b.Run(c.getName(), func(b *testing.B) {
			//p := &benchmarkPersister{}
			p := newServerPersister(b)
			run := c.getRun(b, func(t testing.TB, base *deploy.Snapshot) engine.SnapshotManager {
				j, err := backend.NewJournal(p, base, nil)
				require.NoError(t, err)
				return j
			})
			for i := 0; i < b.N; i++ {
				p.reset()
				run(b)
				b.ReportMetric(float64(p.totalCalls), "calls")
				b.ReportMetric(float64(p.totalBytes), "bytes")
			}
		})
	}
}
