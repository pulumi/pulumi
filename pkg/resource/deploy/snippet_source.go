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

//go:build !js || !wasm

package deploy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

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
)

type snippet struct {
	ctx            context.Context
	snippet        *resource.Snippet
	loader         schema.ReferenceLoader
	rootDir        string
	workingDir     string
	observer       *RegistrationObserver
	observerSource *RegistrationObserverSource
}

// NewSnippetSource creates a Source that registers a single PCL resource snippet.
func NewSnippetSource(ctx context.Context,
	s resource.Snippet,
	loader schema.ReferenceLoader,
	rootDir, workingDir string,
	observer *RegistrationObserver,
) func(string) *promise.Promise[struct{}] {
	contract.Requiref(observer != nil, "observer", "must not be nil")

	src := &snippet{
		ctx: ctx, snippet: &s, loader: loader,
		rootDir: rootDir, workingDir: workingDir,
		observer:       observer,
		observerSource: observer.NewSource(),
	}
	return src.run
}

func (s *snippet) run(resourceMonitorTarget string) *promise.Promise[struct{}] {
	cts := &promise.CompletionSource[struct{}]{}

	fail := func(err error) {
		cts.Reject(err)
	}

	go func() {
		defer s.observerSource.Done()
		if err := s.ctx.Err(); err != nil {
			fail(err)
			return
		}
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

		// Parse the snippet as a resource body. Use the snippet's name in the filename so diagnostics
		// distinguish between multiple snippets in the same update.
		filename := fmt.Sprintf("<snippet:%s>", s.snippet.Name)
		parser := hclsyntax.NewParser()
		if err := parser.ParseFile(strings.NewReader(s.snippet.Code), filename); err != nil {
			fail(fmt.Errorf("parse snippet: %w", err))
			return
		}
		if parser.Diagnostics.HasErrors() {
			fail(fmt.Errorf("parse snippet: %v", parser.Diagnostics))
			return
		}

		// Pre-declare scope variables for References so the binder typechecks `producer.value` and friends
		// without seeing a `resource` block for them. The runtime values are injected on the eval context
		// further down once the registration observer has resolved each URN.
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
		program, diags, err := pcl.BindResourceProgram(parser.Files[0], s.snippet.Name, s.snippet.Type, bindOpts...)
		if err != nil {
			fail(fmt.Errorf("bind snippet: %w", err))
			return
		}
		if diags.HasErrors() {
			fail(fmt.Errorf("bind snippet: %v", diags))
			return
		}

		// Wait on the observer for every Reference. Once they resolve, shape each entry as a Pulumi resource
		// value (object with urn + id + outputs, wrapped in an Output so traversals work) so the interpreter
		// sees it the same way it would see a sibling resource declared in the same program.
		scopeVars := make(map[string]resource.PropertyValue, len(s.snippet.References))
		if len(s.snippet.References) > 0 {
			refURNs := make(map[string]resource.URN, len(s.snippet.References))
			for name, raw := range s.snippet.References {
				urn, err := resource.ParseURN(raw)
				if err != nil {
					fail(fmt.Errorf("invalid URN for reference %q: %w", name, err))
					return
				}
				refURNs[name] = urn
			}
			for name, urn := range refURNs {
				reg, err := s.observerSource.Wait(urn).Result(s.ctx)
				if err != nil {
					fail(fmt.Errorf("waiting for reference %q (%s): %w", name, urn, err))
					return
				}
				scopeVars[name] = snippetReferenceValue(urn, reg)
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

		infoResp, err := monitor.GetDeploymentInfo(s.ctx, &emptypb.Empty{})
		if err != nil {
			fail(fmt.Errorf("get deployment info: %w", err))
			return
		}

		interp := pclruntime.NewInterpreter(
			program, snippetRunInfo(infoResp, s.rootDir, s.workingDir, packageDescriptors))
		if err := interp.RunEmbedded(s.ctx, monitor, s.loader, scopeVars); err != nil {
			fail(fmt.Errorf("execute snippet: %w", err))
			return
		}
		cts.Fulfill(struct{}{})
	}()
	return cts.Promise()
}

func snippetReferenceValue(urn resource.URN, reg URNRegistration) resource.PropertyValue {
	obj := make(resource.PropertyMap, len(reg.Outputs)+4)
	for k, v := range reg.Outputs {
		obj[k] = v
	}
	obj["urn"] = resource.NewProperty(string(urn))
	obj["id"] = resource.NewProperty(string(reg.ID))
	obj["__name"] = resource.NewProperty(urn.Name())
	obj["__type"] = resource.NewProperty(string(urn.Type()))
	return resource.NewProperty(resource.Output{
		Element:      resource.NewProperty(obj),
		Dependencies: []resource.URN{urn},
		Known:        true,
	})
}

func snippetRunInfo(
	info *pulumirpc.DeploymentInfo,
	rootDir, workingDir string,
	packageDescriptors map[string]*schema.PackageDescriptor,
) pclruntime.RunInfo {
	return pclruntime.RunInfo{
		Project:            info.GetProject(),
		Stack:              info.GetStack(),
		Organization:       info.GetOrganization(),
		RootDirectory:      rootDir,
		WorkingDir:         workingDir,
		Config:             info.GetConfig(),
		ConfigSecrets:      info.GetConfigSecretKeys(),
		DryRun:             info.GetDryRun(),
		Parallel:           info.GetParallel(),
		PackageDescriptors: packageDescriptors,
	}
}

// NewMuxSource creates a source that runs main alongside zero or more secondary sources, returning a single
// promise that resolves only after every source has finished. Errors from individual sources are joined with
// [errors.Join].
//
// ctx scopes how long the multiplexer is willing to wait. If it is cancelled before every source has resolved,
// the returned promise rejects with the joined errors so far (including ctx.Err()) and stops waiting. The
// individual source goroutines must also observe cancellation through a captured context or their own channels.
//
// observer is optional. When non-nil, source liveness is tracked so unresolved references are rejected
// once every remaining source is blocked waiting for another source.
func NewMuxSource(
	ctx context.Context,
	observer *RegistrationObserver,
	main func(string) *promise.Promise[struct{}],
	sources ...func(string) *promise.Promise[struct{}],
) func(string) *promise.Promise[struct{}] {
	src := &muxSource{
		ctx:     ctx,
		main:    main,
		sources: sources,
	}
	if observer != nil {
		src.mainObserverSource = observer.NewSource()
		observer.SourcesReady()
	}
	return src.run
}

type muxSource struct {
	ctx                context.Context //nolint:containedctx // bounded to one engine update; passed by NewMuxSource
	mainObserverSource *RegistrationObserverSource
	main               func(string) *promise.Promise[struct{}]
	sources            []func(string) *promise.Promise[struct{}]
}

func (m *muxSource) run(input string) *promise.Promise[struct{}] {
	cts := &promise.CompletionSource[struct{}]{}

	mainP := m.main(input)
	sourceP := make([]*promise.Promise[struct{}], len(m.sources))
	for i, s := range m.sources {
		sourceP[i] = s(input)
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
			if m.mainObserverSource != nil {
				defer m.mainObserverSource.Done()
			}
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
