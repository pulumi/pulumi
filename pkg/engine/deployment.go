// Copyright 2016-2021, Pulumi Corporation.
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

package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	interceptors "github.com/pulumi/pulumi/pkg/v3/util/rpcdebug"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const clientRuntimeName = "client"

// ProjectInfoContext returns information about the current project, including its pwd, main, and plugin context.
func ProjectInfoContext(projinfo *Projinfo, host plugin.Host,
	diag, statusDiag diag.Sink,
	debugTraceMutex *sync.Mutex,
	debugging plugin.DebugContext, disableProviderPreview bool,
	tracingSpan opentracing.Span, config map[config.Key]string,
) (string, string, *plugin.Context, error) {
	contract.Requiref(projinfo != nil, "projinfo", "must not be nil")

	// If the package contains an override for the main entrypoint, use it.
	pwd, main, err := projinfo.GetPwdMain()
	if err != nil {
		return "", "", nil, err
	}

	// Create a context for plugins.
	ctx, err := plugin.NewContextWithRoot(context.TODO(), diag, statusDiag, host, pwd, projinfo.Root,
		projinfo.Proj.Runtime.Options(), disableProviderPreview, tracingSpan, projinfo.Proj.Plugins,
		projinfo.Proj.GetPackageSpecs(), config, debugging)
	if err != nil {
		return "", "", nil, err
	}

	if logFile := env.DebugGRPC.Value(); logFile != "" {
		di, err := interceptors.NewDebugInterceptor(interceptors.DebugInterceptorOptions{
			LogFile: logFile,
			Mutex:   debugTraceMutex,
		})
		if err != nil {
			return "", "", nil, err
		}
		ctx.DialOptions = func(metadata interface{}) []grpc.DialOption {
			return di.DialOptions(interceptors.LogOptions{
				Metadata: metadata,
			})
		}
	}

	// If the project wants to connect to an existing language runtime, do so now.
	if projinfo.Proj.Runtime.Name() == clientRuntimeName {
		addressValue, ok := projinfo.Proj.Runtime.Options()["address"]
		if !ok {
			return "", "", nil, errors.New("missing address of language runtime service")
		}
		address, ok := addressValue.(string)
		if !ok {
			return "", "", nil, errors.New("address of language runtime service must be a string")
		}
		host, err := connectToLanguageRuntime(ctx, address)
		if err != nil {
			return "", "", nil, err
		}
		ctx.Host = host
	}

	return pwd, main, ctx, nil
}

// newDeploymentContext creates a context for a subsequent deployment. Callers must call Close on the context after the
// associated deployment completes.
func newDeploymentContext(u UpdateInfo, opName string, parentSpan opentracing.SpanContext) (*deploymentContext, error) {
	// Create a root span for the operation
	opts := []opentracing.StartSpanOption{}
	if opName != "" {
		opts = append(opts, opentracing.Tag{Key: "operation", Value: opName})
	}
	if parentSpan != nil {
		opts = append(opts, opentracing.ChildOf(parentSpan))
	}
	tracingSpan := opentracing.StartSpan("pulumi-plan", opts...)

	return &deploymentContext{
		Update:      u,
		TracingSpan: tracingSpan,
	}, nil
}

type deploymentContext struct {
	Update      UpdateInfo       // The update being processed.
	TracingSpan opentracing.Span // An OpenTracing span to parent deployment operations within.
}

func (ctx *deploymentContext) Close() {
	ctx.TracingSpan.Finish()
}

// deploymentOptions includes a full suite of options for performing a deployment.
type deploymentOptions struct {
	UpdateOptions

	// SourceFunc is a factory that returns an EvalSource to use during deployment.  This is the thing that
	// creates resources to compare against the current checkpoint state (e.g., by evaluating a program, etc).
	SourceFunc deploymentSourceFunc

	// true if we should print the DOT file for this deployment.
	DOT bool
	// the channel to write events from the engine to.
	Events eventEmitter
	// the sink to use for diag'ing.
	Diag diag.Sink
	// the sink to use for diag'ing status messages.
	StatusDiag diag.Sink

	debugTraceMutex *sync.Mutex

	// True if this is an import operation.
	isImport bool
	// Resources to import, if this is an import.
	imports []deploy.Import

	// true if this deployment is (only) a refresh operation. This should not be
	// confused with UpdateOptions.Refresh, which will be true whenever a refresh
	// is happening as part of an operation (e.g. `up --refresh`).
	isRefresh bool

	// true if this deployment is a dry run, such as a preview action or a preview
	// operation preceding e.g. a refresh or destroy.
	DryRun bool
}

// deploymentSourceFunc is a callback that will be used to prepare for, and evaluate, the "new" state for a stack.
type deploymentSourceFunc func(
	ctx context.Context,
	opts *deploymentOptions, proj *workspace.Project, pwd, main, projectRoot string,
	target *deploy.Target, plugctx *plugin.Context) (deploy.Source, error)

type deploymentBuilder struct {
	opts         *deploymentOptions // the options to use for the deployment.
	debugContext plugin.DebugContext
	cancelCtx    context.Context                                     // the context to use for cancellation.
	cancelFunc   context.CancelFunc                                  // the function to call to cancel the deployment.
	plugctxs     map[resource.AbsoluteStackReference]*plugin.Context // the plugin contexts for each stack reference.
	sources      []deploy.Source                                     // the sources that have been created for this deployment.
}

func newDeploymentBuilder(
	opts *deploymentOptions,
) (*deploymentBuilder, error) {
	contract.Assertf(!opts.isImport, "deploymentBuilder cannot be used for imports")
	contract.Assertf(opts.SourceFunc != nil, "a source factory must be provided")

	debugContext := newDebugContext(opts.Events, opts.AttachDebugger)

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	return &deploymentBuilder{
		opts:         opts,
		debugContext: debugContext,
		cancelCtx:    cancelCtx,
		cancelFunc:   cancelFunc,
	}, nil
}

func (b *deploymentBuilder) enqueue(
	ctx *Context,
	info *deploymentContext,
) (*deploymentBuilder, error) {
	contract.Assertf(info != nil, "a deployment context must be provided")

	// First, load the package metadata and the deployment target in preparation for executing the package's program
	// and creating resources.  This includes fetching its pwd and main overrides.
	proj, target := info.Update.Project, info.Update.Target
	contract.Assertf(proj != nil, "update project cannot be nil")
	contract.Assertf(target != nil, "update target cannot be nil")
	projinfo := &Projinfo{Proj: proj, Root: info.Update.Root}

	// Decrypt the configuration.
	config, err := target.Config.Decrypt(target.Decrypter)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config: %w", err)
	}

	// Create a context for plugins.
	pwd, main, plugctx, err := ProjectInfoContext(projinfo, b.opts.Host,
		b.opts.Diag, b.opts.StatusDiag,
		b.opts.debugTraceMutex,
		b.debugContext, b.opts.DisableProviderPreview, info.TracingSpan, config)
	if err != nil {
		return nil, err
	}

	// Keep the plugin context open until the context is terminated, to allow for graceful provider cancellation.
	plugctx = plugctx.WithCancelChannel(ctx.Cancel.Terminated())

	// Set up a goroutine that will signal cancellation to the source if the caller context
	// is cancelled.
	go func() {
		<-ctx.Cancel.Canceled()
		logging.V(7).Infof("engine.newDeployment(...): received cancellation signal")
		b.cancelFunc()
	}()

	// Now create the state source.  This may issue an error if it can't create the source.  This entails,
	// for example, loading any plugins which will be required to execute a program, among other things.
	source, err := b.opts.SourceFunc(
		b.cancelCtx, b.opts, proj, pwd, main, projinfo.Root, target, plugctx)
	if err != nil {
		contract.IgnoreClose(plugctx)
		return nil, err
	}

	b.plugctxs[target.StackReference()] = plugctx
	b.sources = append(b.sources, source)
	return b, nil
}

func (b *deploymentBuilder) build() (*deployment, error) {
	localPolicyPackPaths := ConvertLocalPolicyPacksToPaths(b.opts.LocalPolicyPacks)

	deplOpts := &deploy.Options{
		ParallelDiff:              b.opts.ParallelDiff,
		DryRun:                    b.opts.DryRun,
		Parallel:                  b.opts.Parallel,
		Refresh:                   b.opts.Refresh,
		RefreshOnly:               b.opts.isRefresh,
		RefreshProgram:            b.opts.RefreshProgram,
		DestroyProgram:            b.opts.DestroyProgram,
		ReplaceTargets:            b.opts.ReplaceTargets,
		Targets:                   b.opts.Targets,
		Excludes:                  b.opts.Excludes,
		TargetDependents:          b.opts.TargetDependents,
		ExcludeDependents:         b.opts.ExcludeDependents,
		UseLegacyDiff:             b.opts.UseLegacyDiff,
		UseLegacyRefreshDiff:      b.opts.UseLegacyRefreshDiff,
		DisableResourceReferences: b.opts.DisableResourceReferences,
		DisableOutputValues:       b.opts.DisableOutputValues,
		GeneratePlan:              b.opts.GeneratePlan,
		ContinueOnError:           b.opts.ContinueOnError,
		Autonamer:                 b.opts.Autonamer,
	}

	plugctxs := plugin.NewContexts(b.plugctxs)

	depl, err := deploy.NewDeployment(
		b.opts.Diag,
		b.opts.debugTraceMutex,
		plugctxs,
		deplOpts, actions, target.Snapshot, opts.Plan, source,
		localPolicyPackPaths, ctx.BackendClient)
	if err != nil {
		contract.IgnoreClose(plugctxs)
		return nil, err
	}

	return &deployment{
		Ctx:        info,
		Plugctx:    plugctx,
		Deployment: depl,
		Actions:    actions,
		Options:    opts,
	}, nil
}

// newDeployment creates a new deployment with the given context and options.
func newDeployment(
	ctx *Context,
	info *deploymentContext,
	opts *deploymentOptions,
) (*deployment, error) {
	contract.Assertf(info != nil, "a deployment context must be provided")
	contract.Assertf(info.Update != nil, "update info cannot be nil")
	contract.Assertf(opts.SourceFunc != nil, "a source factory must be provided")

	// Create an appropriate set of event listeners.
	var actions runActions
	if opts.DryRun {
		actions = newPreviewActions(opts)
	} else {
		actions = newUpdateActions(ctx, opts)
	}

	// First, load the package metadata and the deployment target in preparation for executing the package's program
	// and creating resources.  This includes fetching its pwd and main overrides.
	proj, target := info.Update.Project, info.Update.Target
	contract.Assertf(proj != nil, "update project cannot be nil")
	contract.Assertf(target != nil, "update target cannot be nil")
	projinfo := &Projinfo{Proj: proj, Root: info.Update.Root}

	// Decrypt the configuration.
	config, err := target.Config.Decrypt(target.Decrypter)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config: %w", err)
	}

	// Create a context for plugins.
	debugContext := newDebugContext(opts.Events, opts.AttachDebugger)
	pwd, main, plugctx, err := ProjectInfoContext(projinfo, opts.Host,
		opts.Diag, opts.StatusDiag, opts.debugTraceMutex, debugContext, opts.DisableProviderPreview, info.TracingSpan, config)
	if err != nil {
		return nil, err
	}

	// Keep the plugin context open until the context is terminated, to allow for graceful provider cancellation.
	plugctx = plugctx.WithCancelChannel(ctx.Cancel.Terminated())
	plugctxs := plugin.NewContexts(map[resource.StackReference]*plugin.Context{
		target.StackReference(): plugctx,
	})

	// Set up a goroutine that will signal cancellation to the source if the caller context
	// is cancelled.
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	go func() {
		<-ctx.Cancel.Canceled()
		logging.V(7).Infof("engine.newDeployment(...): received cancellation signal")
		cancelFunc()
	}()

	// Now create the state source.  This may issue an error if it can't create the source.  This entails,
	// for example, loading any plugins which will be required to execute a program, among other things.
	source, err := opts.SourceFunc(
		cancelCtx, opts, proj, pwd, main, projinfo.Root, target, plugctx)
	if err != nil {
		contract.IgnoreClose(plugctx)
		return nil, err
	}

	localPolicyPackPaths := ConvertLocalPolicyPacksToPaths(opts.LocalPolicyPacks)

	deplOpts := &deploy.Options{
		ParallelDiff:              opts.ParallelDiff,
		DryRun:                    opts.DryRun,
		Parallel:                  opts.Parallel,
		Refresh:                   opts.Refresh,
		RefreshOnly:               opts.isRefresh,
		RefreshProgram:            opts.RefreshProgram,
		DestroyProgram:            opts.DestroyProgram,
		ReplaceTargets:            opts.ReplaceTargets,
		Targets:                   opts.Targets,
		Excludes:                  opts.Excludes,
		TargetDependents:          opts.TargetDependents,
		ExcludeDependents:         opts.ExcludeDependents,
		UseLegacyDiff:             opts.UseLegacyDiff,
		UseLegacyRefreshDiff:      opts.UseLegacyRefreshDiff,
		DisableResourceReferences: opts.DisableResourceReferences,
		DisableOutputValues:       opts.DisableOutputValues,
		GeneratePlan:              opts.GeneratePlan,
		ContinueOnError:           opts.ContinueOnError,
		Autonamer:                 opts.Autonamer,
	}

	var depl *deploy.Deployment
	if !opts.isImport {
		depl, err = deploy.NewDeployment(
			opts.Diag, opts.debugTraceMutex, plugctxs,
			deplOpts, actions, target.Snapshot, opts.Plan, source,
			localPolicyPackPaths, ctx.BackendClient)
	} else {
		_, defaultProviderInfo, pluginErr := installPlugins(
			cancelCtx,
			proj,
			pwd,
			main,
			target,
			opts,
			plugctx,
			false, /*returnInstallErrors*/
		)
		if pluginErr != nil {
			return nil, pluginErr
		}
		for i := range opts.imports {
			imp := &opts.imports[i]
			if imp.Component {
				if imp.ID != "" {
					return nil, fmt.Errorf("import %s cannot specify an ID as it's a component", imp.Name)
				}
			}

			if !imp.Component || imp.Remote {
				_, err := tokens.ParseTypeToken(imp.Type.String())
				if err != nil {
					return nil, fmt.Errorf("import type %q is not a valid resource type token. "+
						"Type tokens must be of the format <package>:<module>:<type> - "+
						"refer to the import section of the provider resource documentation.", imp.Type.String())
				}
			}

			if imp.Provider == "" && (!imp.Component || imp.Remote) {
				if imp.Version == nil {
					dpi := defaultProviderInfo[imp.Type.Package()]
					imp.Version = dpi.Version
					if dpi.Parameterization != nil {
						imp.Version = &dpi.Parameterization.Version
					}
				}
				if imp.PluginDownloadURL == "" {
					imp.PluginDownloadURL = defaultProviderInfo[imp.Type.Package()].PluginDownloadURL
				}
				if imp.PluginChecksums == nil {
					imp.PluginChecksums = defaultProviderInfo[imp.Type.Package()].Checksums
				}
				if imp.Parameterization == nil {
					dpi := defaultProviderInfo[imp.Type.Package()]

					if dpi.Parameterization != nil {
						if dpi.Version == nil {
							return nil, fmt.Errorf("no version specified for provider %q", dpi.Name)
						}

						imp.Parameterization = &deploy.Parameterization{
							PluginName:    tokens.Package(dpi.Name),
							PluginVersion: *dpi.Version,
							Value:         dpi.Parameterization.Value,
						}
					}
				}
			}
		}

		depl, err = deploy.NewImportDeployment(
			opts.debugTraceMutex,
			plugctx, deplOpts, actions, target, proj.Name, opts.imports)
	}

	if err != nil {
		contract.IgnoreClose(plugctx)
		return nil, err
	}
	return &deployment{
		Ctx:        info,
		Plugctx:    plugctx,
		Deployment: depl,
		Actions:    actions,
		Options:    opts,
	}, nil
}

type deployment struct {
	// deployment context information.
	Ctx *deploymentContext
	// the context containing plugins and their state.
	Plugctx *plugin.Context
	// the deployment created by this command.
	Deployment *deploy.Deployment
	// the actions to run during the deployment.
	Actions runActions
	// the options used while deploying.
	Options *deploymentOptions
}

// runActions represents a set of actions to run as part of a deployment,
// including callbacks that will be used to emit events at various points in the
// deployment process.
type runActions interface {
	deploy.Events

	Changes() display.ResourceChanges
	MaybeCorrupt() bool
}

// run executes the deployment. It is primarily responsible for handling cancellation.
func (deployment *deployment) run(cancelCtx *Context) (*deploy.Plan, display.ResourceChanges, error) {
	// Create a new context for cancellation and tracing.
	ctx, cancelFunc := context.WithCancel(context.Background())

	// Inject our opentracing span into the context.
	if deployment.Ctx.TracingSpan != nil {
		ctx = opentracing.ContextWithSpan(ctx, deployment.Ctx.TracingSpan)
	}

	// Emit an appropriate prelude event.
	deployment.Options.Events.preludeEvent(
		deployment.Options.DryRun, deployment.Ctx.Update.Target.Config)

	// Execute the deployment.
	start := time.Now()

	done := make(chan bool)
	var newPlan *deploy.Plan
	var walkError error
	go func() {
		newPlan, walkError = deployment.Deployment.Execute(ctx)
		close(done)
	}()

	// Asynchronously listen for cancellation, and deliver that signal to the deployment.
	go func() {
		select {
		case <-cancelCtx.Cancel.Canceled():
			// Cancel the deployment's execution context, so it begins to shut down.
			cancelFunc()
		case <-done:
			return
		}
	}()

	var err error
	// Wait for the deployment to finish executing or for the user to terminate the run.
	select {
	case <-cancelCtx.Cancel.Terminated():
		err = cancelCtx.Cancel.TerminateErr()

	case <-done:
		err = walkError
	}

	duration := time.Since(start)
	changes := deployment.Actions.Changes()

	// Refresh and Import do not execute Policy Packs.
	policies := map[string]string{}
	if !deployment.Options.isRefresh && !deployment.Options.isImport {
		for _, p := range deployment.Options.RequiredPolicies {
			policies[p.Name()] = p.Version()
		}
		for _, pack := range deployment.Options.LocalPolicyPacks {
			packName := pack.NameForEvents()
			policies[packName] = pack.Version
		}
	}

	// Emit a summary event.
	deployment.Options.Events.summaryEvent(
		deployment.Options.DryRun, deployment.Actions.MaybeCorrupt(), duration, changes, policies)

	return newPlan, changes, err
}

func (deployment *deployment) Close() error {
	return deployment.Plugctx.Close()
}

func assertSeen(seen map[resource.URN]deploy.Step, step deploy.Step) {
	_, has := seen[step.URN()]
	contract.Assertf(has, "URN '%v' had not been marked as seen", step.URN())
}

func isDefaultProviderStep(step deploy.Step) bool {
	return providers.IsDefaultProvider(step.URN())
}

func checkTargets(targetUrns deploy.UrnTargets, excludeUrns deploy.UrnTargets, snap *deploy.Snapshot) error {
	if !targetUrns.IsConstrained() && !excludeUrns.IsConstrained() {
		return nil
	}
	if snap == nil {
		return errors.New("targets specified, but snapshot was nil")
	}
	urns := map[resource.URN]struct{}{}
	for _, res := range snap.Resources {
		urns[res.URN] = struct{}{}
	}
	for _, target := range targetUrns.Literals() {
		if _, ok := urns[target]; !ok {
			return fmt.Errorf("no resource named '%s' found", target)
		}
	}
	for _, target := range excludeUrns.Literals() {
		if _, ok := urns[target]; !ok {
			return fmt.Errorf("no resource named '%s' found", target)
		}
	}
	return nil
}
