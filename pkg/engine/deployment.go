// Copyright 2016-2018, Pulumi Corporation.
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
	"fmt"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const clientRuntimeName = "client"

// ProjectInfoContext returns information about the current project, including its pwd, main, and plugin context.
func ProjectInfoContext(projinfo *Projinfo, host plugin.Host, config plugin.ConfigSource,
	diag, statusDiag diag.Sink, disableProviderPreview bool,
	tracingSpan opentracing.Span) (string, string, *plugin.Context, error) {

	contract.Require(projinfo != nil, "projinfo")

	// If the package contains an override for the main entrypoint, use it.
	pwd, main, err := projinfo.GetPwdMain()
	if err != nil {
		return "", "", nil, err
	}

	// Create a context for plugins.
	ctx, err := plugin.NewContextWithRoot(diag, statusDiag, host, config, pwd, projinfo.Root,
		projinfo.Proj.Runtime.Options(), disableProviderPreview, tracingSpan)
	if err != nil {
		return "", "", nil, err
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
	contract.Require(u != nil, "u")

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

	DOT        bool         // true if we should print the DOT file for this deployment.
	Events     eventEmitter // the channel to write events from the engine to.
	Diag       diag.Sink    // the sink to use for diag'ing.
	StatusDiag diag.Sink    // the sink to use for diag'ing status messages.

	isImport bool            // True if this is an import.
	imports  []deploy.Import // Resources to import, if this is an import.

	// true if we're executing a refresh.
	isRefresh bool

	// true if we should trust the dependency graph reported by the language host. Not all Pulumi-supported languages
	// correctly report their dependencies, in which case this will be false.
	trustDependencies bool
}

// deploymentSourceFunc is a callback that will be used to prepare for, and evaluate, the "new" state for a stack.
type deploymentSourceFunc func(
	client deploy.BackendClient, opts deploymentOptions, proj *workspace.Project, pwd, main string,
	target *deploy.Target, plugctx *plugin.Context, dryRun bool) (deploy.Source, error)

// newDeployment creates a new deployment with the given context and options.
func newDeployment(ctx *Context, info *deploymentContext, opts deploymentOptions, dryRun bool) (*deployment, error) {
	contract.Assert(info != nil)
	contract.Assert(info.Update != nil)
	contract.Assert(opts.SourceFunc != nil)

	// First, load the package metadata and the deployment target in preparation for executing the package's program
	// and creating resources.  This includes fetching its pwd and main overrides.
	proj, target := info.Update.GetProject(), info.Update.GetTarget()
	contract.Assert(proj != nil)
	contract.Assert(target != nil)
	projinfo := &Projinfo{Proj: proj, Root: info.Update.GetRoot()}
	pwd, main, plugctx, err := ProjectInfoContext(projinfo, opts.Host, target,
		opts.Diag, opts.StatusDiag, opts.DisableProviderPreview, info.TracingSpan)
	if err != nil {
		return nil, err
	}

	opts.trustDependencies = proj.TrustResourceDependencies()
	// Now create the state source.  This may issue an error if it can't create the source.  This entails,
	// for example, loading any plugins which will be required to execute a program, among other things.
	source, err := opts.SourceFunc(ctx.BackendClient, opts, proj, pwd, main, target, plugctx, dryRun)
	if err != nil {
		contract.IgnoreClose(plugctx)
		return nil, err
	}

	localPolicyPackPaths := ConvertLocalPolicyPacksToPaths(opts.LocalPolicyPacks)

	var depl *deploy.Deployment
	if !opts.isImport {
		depl, err = deploy.NewDeployment(
			plugctx, target, target.Snapshot, source, localPolicyPackPaths, dryRun, ctx.BackendClient)
	} else {
		_, defaultProviderVersions, pluginErr := installPlugins(proj, pwd, main, target, plugctx,
			false /*returnInstallErrors*/)
		if pluginErr != nil {
			return nil, pluginErr
		}
		for i := range opts.imports {
			imp := &opts.imports[i]
			_, err := tokens.ParseTypeToken(imp.Type.String())
			if err != nil {
				return nil, errors.New(fmt.Sprintf("import type %q is not a valid resource type token. "+
					"Type tokens must be of the format <package>:<module>:<type> - "+
					"refer to the import section of the provider resource documentation.", imp.Type.String()))
			}
			if imp.Provider == "" && imp.Version == nil {
				imp.Version = defaultProviderVersions[imp.Type.Package()]
			}
		}

		depl, err = deploy.NewImportDeployment(plugctx, target, proj.Name, opts.imports, dryRun)
	}

	if err != nil {
		contract.IgnoreClose(plugctx)
		return nil, err
	}
	return &deployment{
		Ctx:        info,
		Plugctx:    plugctx,
		Deployment: depl,
		Options:    opts,
	}, nil
}

type deployment struct {
	Ctx        *deploymentContext // deployment context information.
	Plugctx    *plugin.Context    // the context containing plugins and their state.
	Deployment *deploy.Deployment // the deployment created by this command.
	Options    deploymentOptions  // the options used while deploying.
}

type runActions interface {
	deploy.Events

	Changes() ResourceChanges
	MaybeCorrupt() bool
}

// run executes the deployment. It is primarily responsible for handling cancellation.
func (deployment *deployment) run(cancelCtx *Context, actions runActions, policyPacks map[string]string,
	preview bool) (ResourceChanges, result.Result) {

	// Change into the plugin context's working directory.
	chdir, err := fsutil.Chdir(deployment.Plugctx.Pwd)
	if err != nil {
		return nil, result.FromError(err)
	}
	defer chdir()

	// Create a new context for cancellation and tracing.
	ctx, cancelFunc := context.WithCancel(context.Background())

	// Inject our opentracing span into the context.
	if deployment.Ctx.TracingSpan != nil {
		ctx = opentracing.ContextWithSpan(ctx, deployment.Ctx.TracingSpan)
	}

	// Emit an appropriate prelude event.
	deployment.Options.Events.preludeEvent(preview, deployment.Ctx.Update.GetTarget().Config)

	// Execute the deployment.
	start := time.Now()

	done := make(chan bool)
	var walkResult result.Result
	go func() {
		opts := deploy.Options{
			Events:                    actions,
			Parallel:                  deployment.Options.Parallel,
			Refresh:                   deployment.Options.Refresh,
			RefreshOnly:               deployment.Options.isRefresh,
			RefreshTargets:            deployment.Options.RefreshTargets,
			ReplaceTargets:            deployment.Options.ReplaceTargets,
			DestroyTargets:            deployment.Options.DestroyTargets,
			UpdateTargets:             deployment.Options.UpdateTargets,
			TargetDependents:          deployment.Options.TargetDependents,
			TrustDependencies:         deployment.Options.trustDependencies,
			UseLegacyDiff:             deployment.Options.UseLegacyDiff,
			DisableResourceReferences: deployment.Options.DisableResourceReferences,
		}
		walkResult = deployment.Deployment.Execute(ctx, opts, preview)
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

	// Wait for the deployment to finish executing or for the user to terminate the run.
	var res result.Result
	select {
	case <-cancelCtx.Cancel.Terminated():
		res = result.WrapIfNonNil(cancelCtx.Cancel.TerminateErr())

	case <-done:
		res = walkResult
	}

	duration := time.Since(start)
	changes := actions.Changes()

	// Emit a summary event.
	deployment.Options.Events.summaryEvent(preview, actions.MaybeCorrupt(), duration, changes, policyPacks)

	return changes, res
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
