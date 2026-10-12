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

package operations

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"time"

	"github.com/spf13/pflag"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/autonaming"
	cmdConfig "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/config"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/metadata"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// UpdateArgs holds the inputs that `pulumi up`, `pulumi preview`, and the Neo pulumi tool
// share when they build a backend.UpdateOperation. Field names mirror the CLI flags.
type UpdateArgs struct {
	Proj *workspace.Project
	Root string

	ConfigFile           string
	EnvOverrides         []string
	SkipConfigValidation bool

	Message   string
	ExecKind  string
	ExecAgent string
	// UpdatePlan records in the update metadata that a plan file is in use.
	UpdatePlan bool
	// Flags, when non-nil, has its set flags recorded in the update metadata.
	Flags *pflag.FlagSet
	// LanguageRuntimeMetadata, when non-nil, is merged into the update metadata.
	LanguageRuntimeMetadata *promise.Promise[map[string]string]

	// Refresh is the raw --refresh value. Empty defers to the project's `options.refresh`.
	Refresh               string
	RefreshProgram        bool
	Targets               []string
	Excludes              []string
	Replaces              []string
	TargetReplaces        []string
	TargetDependents      bool
	ExcludeDependents     bool
	PolicyPackPaths       []string
	PolicyPackConfigPaths []string
	Parallel              int32
	Debug                 bool
	ShowSecrets           bool
	// GeneratePlan forces plan generation. PULUMI_EXPERIMENTAL also turns it on.
	GeneratePlan         bool
	Strict               bool
	ContinueOnError      bool
	IgnoreProtect        bool
	AttachDebugger       []string
	SkipPluginPreInstall bool

	CoherenceWindow string
	Scopes          backend.CancellationScopeSource
}

// DefaultParallel is the degree of parallelism used when --parallel is unset.
func DefaultParallel() int32 { return defaultParallel() }

// NewUpdateOperation loads and validates the stack configuration for s, then builds the
// backend.UpdateOperation that `pulumi up`, `pulumi preview`, and the Neo pulumi tool run.
// opts carries the display and confirmation settings. opts.Engine is replaced with options
// derived from args, the project, and the environment.
func NewUpdateOperation(
	ctx context.Context,
	ssml cmdStack.SecretsManagerLoader,
	s backend.Stack,
	opts backend.UpdateOptions,
	args UpdateArgs,
) (backend.UpdateOperation, error) {
	cfg, sm, err := cmdConfig.GetStackConfiguration(
		ctx, cmdutil.Diag(), ssml, s, args.Proj, args.ConfigFile, args.EnvOverrides)
	if err != nil {
		return backend.UpdateOperation{}, fmt.Errorf("getting stack configuration: %w", err)
	}

	m, err := metadata.GetUpdateMetadata(
		args.Message, args.Root, args.ExecKind, args.ExecAgent, args.UpdatePlan, cfg, args.Flags)
	if err != nil {
		return backend.UpdateOperation{}, fmt.Errorf("gathering environment metadata: %w", err)
	}
	cmdutil.SetStringSpanAttributes(ctx, m.Environment)

	decrypter := sm.Decrypter()
	encrypter := sm.Encrypter()
	stackName := s.Ref().Name().String()
	if args.SkipConfigValidation {
		// Still apply project config defaults onto the stack config, but skip validation.
		if err := pkgWorkspace.ApplyProjectConfig(
			ctx, stackName, args.Proj, cfg.Environment, cfg.Config, encrypter, decrypter,
		); err != nil {
			return backend.UpdateOperation{}, fmt.Errorf("applying stack config: %w", err)
		}
	} else if err := pkgWorkspace.ValidateStackConfigAndApplyProjectConfig(
		ctx, stackName, args.Proj, cfg.Environment, cfg.Config, encrypter, decrypter,
	); err != nil {
		return backend.UpdateOperation{}, fmt.Errorf("validating stack config: %w", err)
	}

	targetURNs := append(append([]string{}, args.Targets...), args.TargetReplaces...)
	replaceURNs := append(append([]string{}, args.Replaces...), args.TargetReplaces...)

	refresh, err := getRefreshOption(args.Proj, args.Refresh)
	if err != nil {
		return backend.UpdateOperation{}, err
	}

	autonamer, err := autonaming.ParseAutonamingConfig(autonamingStackContext(args.Proj, s), cfg.Config, decrypter)
	if err != nil {
		return backend.UpdateOperation{}, fmt.Errorf("getting autonaming config: %w", err)
	}

	opts.Engine = engine.UpdateOptions{
		ParallelDiff:              env.ParallelDiff.Value(),
		LocalPolicyPacks:          engine.MakeLocalPolicyPacks(args.PolicyPackPaths, args.PolicyPackConfigPaths),
		Parallel:                  args.Parallel,
		Debug:                     args.Debug,
		Refresh:                   refresh,
		RefreshProgram:            args.RefreshProgram,
		ReplaceTargets:            deploy.NewUrnTargets(replaceURNs),
		UseLegacyDiff:             env.EnableLegacyDiff.Value(),
		UseLegacyRefreshDiff:      env.EnableLegacyRefreshDiff.Value(),
		DisableProviderPreview:    env.DisableProviderPreview.Value(),
		DisableResourceReferences: env.DisableResourceReferences.Value(),
		DisableOutputValues:       env.DisableOutputValues.Value(),
		ShowSecrets:               args.ShowSecrets,
		Targets:                   deploy.NewUrnTargets(targetURNs),
		Excludes:                  deploy.NewUrnTargets(args.Excludes),
		TargetDependents:          args.TargetDependents,
		ExcludeDependents:         args.ExcludeDependents,
		GeneratePlan:              env.Experimental.Value() || args.GeneratePlan,
		Experimental:              env.Experimental.Value(),
		Strict:                    args.Strict,
		ContinueOnError:           args.ContinueOnError,
		IgnoreProtect:             args.IgnoreProtect,
		AttachDebugger:            args.AttachDebugger,
		Autonamer:                 autonamer,
		SkipPluginPreInstall:      args.SkipPluginPreInstall,
	}

	if args.LanguageRuntimeMetadata != nil {
		start := time.Now()
		runtimeMetadata, err := args.LanguageRuntimeMetadata.Result(ctx)
		slog.InfoContext(ctx, "Waiting for language runtime metadata", "duration", time.Since(start))
		if err != nil {
			slog.InfoContext(ctx, "Could not retrieve language runtime metadata", "err", err)
		} else {
			maps.Copy(m.Environment, runtimeMetadata)
		}
	}

	return backend.UpdateOperation{
		Proj:               args.Proj,
		Root:               args.Root,
		M:                  m,
		Opts:               opts,
		StackConfiguration: cfg,
		SecretsManager:     sm,
		SecretsProvider:    secrets.DefaultProvider,
		Scopes:             args.Scopes,
		CoherenceWindow:    args.CoherenceWindow,
	}, nil
}
