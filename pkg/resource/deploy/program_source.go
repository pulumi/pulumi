// Copyright 2016, Pulumi Corporation.
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
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

// NewProgramSource creates a new Source that can be used to evaluate a program and extract resources from it.
func NewProgramSource(
	plugctx *plugin.Context,
	runinfo *EvalRunInfo,
	opts EvalSourceOptions,
	panicErrs chan<- error,
) func(string) *promise.Promise[struct{}] {
	src := &programSource{
		plugctx:   plugctx,
		runinfo:   runinfo,
		opts:      opts,
		panicErrs: panicErrs,
	}
	return src.run
}

type programSource struct {
	plugctx *plugin.Context   // the plugin context.
	runinfo *EvalRunInfo      // the directives to use when running the program.
	opts    EvalSourceOptions // options for the evaluation source.
	// channel for reporting panics from goroutines
	panicErrs chan<- error
}

func (src *programSource) run(resourceMonitorTarget string) *promise.Promise[struct{}] {
	// Decrypt the configuration.
	config, err := src.runinfo.Target.Config.Decrypt(src.runinfo.Target.Decrypter)
	if err != nil {
		return promise.Errorf[struct{}]("failed to decrypt config: %w", err)
	}

	// Keep track of any config keys that have secure values.
	configSecretKeys := src.runinfo.Target.Config.SecureKeys()

	// Also start up a schema loader for the language runtime to use to fetch schema information.
	loaderRegistration := schema.LoaderRegistration(
		schema.NewLoaderServer(schema.NewPluginLoader(src.plugctx.Host)))
	loaderServer, err := plugin.NewServer(src.plugctx, loaderRegistration)
	if err != nil {
		return promise.Errorf[struct{}]("failed to start loader server: %w", err)
	}

	// Now invoke Run in a goroutine.  All subsequent resource creation events will come in over the gRPC channel,
	// and we will pump them through the channel.  If the Run call ultimately fails, we need to propagate the error.
	return src.forkRun(config, configSecretKeys, resourceMonitorTarget, loaderServer)
}

// forkRun performs the evaluation from a distinct goroutine. This function blocks until it's our turn to go.
func (src *programSource) forkRun(
	config map[config.Key]string,
	configSecretKeys []config.Key,
	resourceMonitorTarget string,
	loaderServer *plugin.GrpcServer,
) *promise.Promise[struct{}] {
	cts := &promise.CompletionSource[struct{}]{}
	// Fire up the goroutine to make the RPC invocation against the language runtime.  As this executes, calls
	// to queue things up in the resource channel will occur, and we will serve them concurrently.
	go PanicRecovery(src.panicErrs, func() {
		// Next, launch the language plugin.
		run := func() error {
			defer contract.IgnoreClose(loaderServer)

			rt := src.runinfo.Proj.Runtime.Name()

			langhost, err := src.plugctx.Host.LanguageRuntime(rt)
			if err != nil {
				return fmt.Errorf("failed to launch language host %s: %w", rt, err)
			}
			contract.Assertf(langhost != nil, "expected non-nil language host %s", rt)

			rtopts := src.runinfo.Proj.Runtime.Options()
			programInfo := plugin.NewProgramInfo(
				/* rootDirectory */ src.runinfo.ProjectRoot,
				/* programDirectory */ src.runinfo.Pwd,
				/* entryPoint */ src.runinfo.Program,
				/* options */ rtopts)

			// Now run the actual program.
			progerr, bail, err := langhost.Run(plugin.RunInfo{
				MonitorAddress:   resourceMonitorTarget,
				Stack:            src.runinfo.Target.Name.String(),
				Project:          string(src.runinfo.Proj.Name),
				Pwd:              src.runinfo.Pwd,
				Args:             src.runinfo.Args,
				Config:           config,
				ConfigSecretKeys: configSecretKeys,
				DryRun:           src.opts.DryRun,
				Parallel:         src.opts.Parallel,
				Organization:     string(src.runinfo.Target.Organization),
				Info:             programInfo,
				LoaderAddress:    loaderServer.Addr(),
				AttachDebugger:   src.plugctx.Host.AttachDebugger(plugin.DebugSpec{Type: plugin.DebugTypeProgram}),
			})

			// Check if we were asked to Bail.  This a special random constant used for that
			// purpose.
			if err == nil && bail {
				return result.BailErrorf("run bailed")
			}

			if err == nil && progerr != "" {
				// If the program had an unhandled error; propagate it to the caller.
				err = fmt.Errorf("an unhandled error occurred: %v", progerr)
			}
			return err
		}

		// Communicate the error, if it exists, or nil if the program exited cleanly.
		err := run()
		if err != nil {
			cts.Reject(err)
		} else {
			cts.Fulfill(struct{}{})
		}
	})
	return cts.Promise()
}
