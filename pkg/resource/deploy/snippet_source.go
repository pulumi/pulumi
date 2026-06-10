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

package deploy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pclruntime "github.com/pulumi/pulumi/pkg/v3/pcl/runtime"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/zclconf/go-cty/cty"
)

type snippet struct {
	snippet    *resource.Snippet
	loader     schema.ReferenceLoader
	rootDir    string
	workingDir string
	// urn is the URN this snippet will register, pre-computed from the project, stack, type, and name so the
	// broker can mark it Expected and so this source can Reject it on failure (unblocking any cascade waiters).
	urn resource.URN
	// broker is the per-update URN broker; nil if the caller does not need cross-source URN coordination. Used by
	// the snippet to wait on (and resolve traversals of) the resources named in Snippet.References.
	broker *URNBroker
}

// NewSnippetSource creates a Source that registers a single PCL resource snippet. broker, if non-nil, is the
// per-update URN broker the snippet will consult to wait for resources named in s.References. urn is the URN
// this snippet is expected to register; passed in by the caller so the source can Reject it on failure and
// unblock any cascade waiters.
func NewSnippetSource(s resource.Snippet,
	loader schema.ReferenceLoader,
	rootDir, workingDir string,
	urn resource.URN,
	broker *URNBroker,
) func(string) *promise.Promise[struct{}] {
	src := &snippet{
		snippet: &s, loader: loader,
		rootDir: rootDir, workingDir: workingDir,
		urn: urn, broker: broker,
	}
	return src.run
}

func (s *snippet) run(resourceMonitorTarget string) *promise.Promise[struct{}] {
	cts := &promise.CompletionSource[struct{}]{}

	// fail rejects the snippet's outer promise AND its broker entry so that any other snippet waiting on this
	// URN unblocks promptly instead of hanging. Use fail in place of cts.Reject for any failure path that means
	// the snippet's RegisterResource will not run.
	fail := func(err error) {
		if s.broker != nil && s.urn != "" {
			s.broker.Reject(s.urn, err)
		}
		cts.Reject(err)
	}

	go func() {
		// Build the schema.PackageDescriptor from the structurally-attached Snippet.Descriptor. The binder and
		// interpreter both consult this to load the right (possibly parameterized) package schema.
		var parameterization *schema.ParameterizationDescriptor
		if s.snippet.Descriptor.Parameterization != nil {
			parameterization = &schema.ParameterizationDescriptor{
				Name:    s.snippet.Descriptor.Parameterization.Name,
				Version: s.snippet.Descriptor.Parameterization.Version,
				Value:   s.snippet.Descriptor.Parameterization.Value,
			}
		}
		descriptor := &schema.PackageDescriptor{
			Name:             s.snippet.Descriptor.Name,
			Version:          s.snippet.Descriptor.Version,
			DownloadURL:      s.snippet.Descriptor.DownloadURL,
			Parameterization: parameterization,
		}

		// Determine the package name the binder will key the descriptor by. For unparameterized snippets this is
		// the descriptor's Name; for parameterized snippets it's the parameterization's Name (the alias visible
		// to PCL code). DecomposeToken on the snippet's Type gives us that key directly.
		pkgName, _, _, diags := pcl.DecomposeToken(s.snippet.Type, hcl.Range{})
		if diags.HasErrors() {
			fail(fmt.Errorf("invalid snippet type %q: %s", s.snippet.Type, diags.Error()))
			return
		}
		packageDescriptors := map[string]*schema.PackageDescriptor{pkgName: descriptor}

		// Synthesize a one-resource program from the snippet's body. The first label is quoted so arbitrary
		// snippet names (kebab-case, etc.) survive HCL parsing; the binder treats both quoted and bare labels
		// the same. The interpreter then drives this synthesized program through its normal registerResource
		// path, picking up dependency tracking, range (count/foreach), and the full options set for free.
		sourceCode := fmt.Sprintf("resource %q %q {\n%s\n}\n", s.snippet.Name, s.snippet.Type, s.snippet.Code)
		parser := hclsyntax.NewParser()
		if err := parser.ParseFile(strings.NewReader(sourceCode), "<snippet>"); err != nil {
			fail(fmt.Errorf("parse snippet: %w", err))
			return
		}
		if parser.Diagnostics.HasErrors() {
			fail(fmt.Errorf("parse snippet: %v", parser.Diagnostics))
			return
		}

		// Pre-declare scope variables for References so the binder typechecks `producer.value` and friends
		// without seeing a `resource` block for them. The runtime values are injected on the eval context
		// further down once the broker has resolved each URN.
		bindOpts := []pcl.BindOption{
			pcl.Loader(s.loader),
			pcl.PackageDescriptors(packageDescriptors),
		}
		if len(s.snippet.References) > 0 {
			extras := make(map[string]*model.Variable, len(s.snippet.References))
			for name := range s.snippet.References {
				extras[name] = &model.Variable{Name: name, VariableType: model.DynamicType}
			}
			bindOpts = append(bindOpts, pcl.ExtraScopeVariables(extras))
		}
		program, diags, err := pcl.BindProgram(parser.Files, bindOpts...)
		if err != nil {
			fail(fmt.Errorf("bind snippet: %w", err))
			return
		}
		if diags.HasErrors() {
			fail(fmt.Errorf("bind snippet: %v", diags))
			return
		}

		// Wait on the broker for every Reference. Once they resolve, shape each entry as a Pulumi resource value
		// (object with urn + id + outputs, wrapped in an Output so traversals work) so the interpreter sees it
		// the same way it would see a sibling resource declared in the same program.
		scopeVars := make(map[string]cty.Value, len(s.snippet.References))
		if len(s.snippet.References) > 0 {
			if s.broker == nil {
				fail(errors.New("snippet has References but no URNBroker is configured"))
				return
			}
			refURNs := make(map[string]resource.URN, len(s.snippet.References))
			for name, raw := range s.snippet.References {
				urn, err := resource.ParseURN(raw)
				if err != nil {
					fail(fmt.Errorf("invalid URN for reference %q: %w", name, err))
					return
				}
				refURNs[name] = urn
			}
			grp, gctx := errgroup.WithContext(context.TODO())
			resolved := make(map[string]URNRegistration, len(refURNs))
			var mu sync.Mutex
			for name, urn := range refURNs {
				grp.Go(func() error {
					reg, err := s.broker.Get(urn).Result(gctx)
					if err != nil {
						return fmt.Errorf("waiting for reference %q (%s): %w", name, urn, err)
					}
					mu.Lock()
					resolved[name] = reg
					mu.Unlock()
					return nil
				})
			}
			if err := grp.Wait(); err != nil {
				fail(err)
				return
			}
			for name, reg := range resolved {
				urn := refURNs[name]
				obj := make(resource.PropertyMap, len(reg.Outputs)+2)
				for k, v := range reg.Outputs {
					obj[k] = v
				}
				obj["urn"] = resource.NewProperty(string(urn))
				obj["id"] = resource.NewProperty(string(reg.ID))
				wrapped := resource.NewProperty(resource.Output{
					Element:      resource.NewProperty(obj),
					Dependencies: []resource.URN{urn},
					Known:        true,
				})
				ctyVal, err := pclruntime.PropertyValueToCty(context.TODO(), nil, wrapped)
				if err != nil {
					fail(fmt.Errorf("converting outputs for reference %q: %w", name, err))
					return
				}
				scopeVars[name] = ctyVal
			}
		}

		// Dial the resource monitor and ask it for the deployment context the interpreter needs (project,
		// stack, organization). The interpreter wires this into the eval context so `pulumi.project` etc.
		// resolve to the same values the main program sees.
		conn, err := grpc.NewClient(
			resourceMonitorTarget,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			rpcutil.GrpcChannelOptions(),
		)
		if err != nil {
			fail(fmt.Errorf("connect to resource monitor: %w", err))
			return
		}
		defer contract.IgnoreClose(conn)
		monitor := pulumirpc.NewResourceMonitorClient(conn)

		infoResp, err := monitor.GetDeploymentInfo(context.TODO(), &emptypb.Empty{})
		if err != nil {
			fail(fmt.Errorf("get deployment info: %w", err))
			return
		}

		interp := pclruntime.NewInterpreter(program, pclruntime.RunInfo{
			Project:            infoResp.Project,
			Stack:              infoResp.Stack,
			Organization:       infoResp.Organization,
			RootDirectory:      s.rootDir,
			WorkingDir:         s.workingDir,
			PackageDescriptors: packageDescriptors,
		})
		if err := interp.RunEmbedded(context.TODO(), monitor, s.loader, scopeVars); err != nil {
			fail(fmt.Errorf("execute snippet: %w", err))
			return
		}
		cts.Fulfill(struct{}{})
	}()
	return cts.Promise()
}

// NewMuxSource creates a source that runs main alongside zero or more secondary sources, returning a single
// promise that resolves only after every source has finished. Errors from individual sources are joined with
// [errors.Join].
//
// ctx scopes how long the multiplexer is willing to wait. If it is cancelled before every source has resolved,
// the returned promise rejects with the joined errors so far (including ctx.Err()) and stops waiting. The
// individual source goroutines are not interrupted directly; they are expected to react to cancellation through
// their own channels (typically the resource monitor's shutdown signal).
//
// broker is optional. When non-nil, a watchdog goroutine waits for main to resolve and then calls
// [URNBroker.RejectUnresolved] on the broker — at that point the program won't register anything more, so any
// pending URN that no still-running source pre-declared (via [URNBroker.MarkExpected]) is dead and waiting
// snippets are unblocked with a clear error.
func NewMuxSource(
	ctx context.Context,
	broker *URNBroker,
	main func(string) *promise.Promise[struct{}],
	sources ...func(string) *promise.Promise[struct{}],
) func(string) *promise.Promise[struct{}] {
	src := &muxSource{
		ctx:     ctx,
		broker:  broker,
		main:    main,
		sources: sources,
	}
	return src.run
}

type muxSource struct {
	ctx     context.Context //nolint:containedctx // bounded to one engine update; passed by NewMuxSource
	broker  *URNBroker
	main    func(string) *promise.Promise[struct{}]
	sources []func(string) *promise.Promise[struct{}]
}

func (m *muxSource) run(input string) *promise.Promise[struct{}] {
	cts := &promise.CompletionSource[struct{}]{}

	mainP := m.main(input)
	sourceP := make([]*promise.Promise[struct{}], len(m.sources))
	for i, s := range m.sources {
		sourceP[i] = s(input)
	}

	// Watchdog: when the main program finishes (success or failure) it will not register any more resources,
	// so any URN still pending on the broker that no still-running source pre-declared via MarkExpected is
	// dead. Rejecting them wakes up any snippets blocked on those references with a useful error instead of
	// letting them hang forever.
	if m.broker != nil {
		go func() {
			_, _ = mainP.Result(m.ctx)
			m.broker.RejectUnresolved(errors.New("no source registered this URN"))
		}()
	}

	go func() {
		// Block until every promise resolves or our ctx is cancelled. Each waiter is independent so a slow
		// source doesn't delay error reporting for fast ones, but we still wait for all of them to settle
		// before fulfilling so callers don't observe a partial completion.
		errs := make([]error, len(sourceP)+1)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := mainP.Result(m.ctx); err != nil {
				errs[0] = err
			}
		}()
		for i, p := range sourceP {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := p.Result(m.ctx); err != nil {
					errs[i+1] = err
				}
			}()
		}
		wg.Wait()

		var nonNil []error
		for _, e := range errs {
			if e != nil {
				nonNil = append(nonNil, e)
			}
		}
		switch len(nonNil) {
		case 0:
			cts.Fulfill(struct{}{})
		case 1:
			cts.Reject(nonNil[0])
		default:
			cts.Reject(errors.Join(nonNil...))
		}
	}()

	return cts.Promise()
}
