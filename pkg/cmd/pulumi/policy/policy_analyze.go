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

package policy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

func newPolicyAnalyzeCmd(
	ws pkgWorkspace.Context,
	lm cmdBackend.LoginManager,
	secretsProvider secrets.Provider,
	// loadAnalyzers loads and configures local policy pack analyzers.
	// Nil uses the real plugin host.
	loadAnalyzers func(ctx context.Context, packs []engine.LocalPolicyPack) ([]plugin.Analyzer, func(), error),
) *cobra.Command {
	var stack string
	var policyPackPaths []string
	var policyPackConfigs []string

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze a stack's current state against policy packs",
		Long: "Analyze a stack's current state against one or more local policy packs.\n" +
			"\n" +
			"This command runs policy analysis against the stack's existing resource state\n" +
			"without executing the Pulumi program or making provider calls.\n" +
			"\n" +
			"If any remediation policy fires, the change is reported but the stack state\n" +
			"is not modified. Exits with a non-zero status if any mandatory violations are found.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if len(policyPackPaths) == 0 {
				return errors.New("at least one --policy-pack path must be specified")
			}
			if len(policyPackConfigs) > 0 && len(policyPackConfigs) != len(policyPackPaths) {
				return errors.New(
					"the number of --policy-pack-config paths must match the number of --policy-pack paths")
			}

			// Default: loadAnalyzers.
			if loadAnalyzers == nil {
				loadAnalyzers = func(ctx context.Context, packs []engine.LocalPolicyPack) ([]plugin.Analyzer, func(), error) {
					cwd, err := os.Getwd()
					if err != nil {
						return nil, nil, fmt.Errorf("getting working directory: %w", err)
					}
					pctx, err := plugin.NewContext(ctx, cmdutil.Diag(), cmdutil.Diag(),
						nil, nil, cwd, nil, true, nil, schema.NewLoaderServerFromHost)
					if err != nil {
						return nil, nil, fmt.Errorf("creating plugin context: %w", err)
					}
					analyzers, err := engine.LoadLocalPolicyPackAnalyzers(ctx, pctx, packs, nil)
					if err != nil {
						contract.IgnoreClose(pctx)
						return nil, nil, err
					}
					return analyzers, func() { contract.IgnoreClose(pctx) }, nil
				}
			}

			// Get the stack.
			displayOpts := display.Options{Color: cmdutil.GetGlobalColorization()}
			s, err := cmdStack.RequireStack(ctx, cmdutil.Diag(), ws, lm, stack, cmdStack.LoadOnly, displayOpts)
			if err != nil {
				return err
			}

			// Load the current stack snapshot.
			snap, err := s.Snapshot(ctx, secretsProvider)
			if err != nil {
				return fmt.Errorf("loading stack snapshot: %w", err)
			}
			if snap == nil || len(snap.Resources) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "Stack %s has no resources; nothing to analyze.\n", s.Ref())
				return nil
			}

			// Load the policy pack analyzers.
			packs := engine.MakeLocalPolicyPacks(policyPackPaths, policyPackConfigs)
			analyzers, cleanup, err := loadAnalyzers(ctx, packs)
			if err != nil {
				return fmt.Errorf("loading policy packs: %w", err)
			}
			defer cleanup()

			// Run analysis against the snapshot.
			events := &analyzeEvents{out: cmd.OutOrStdout()}
			hasMandatory, err := deploy.AnalyzeSnapshot(ctx, snap, analyzers, events)
			if err != nil {
				return fmt.Errorf("running policy analysis: %w", err)
			}

			if hasMandatory {
				return errors.New("one or more mandatory policy violations were found")
			}
			return nil
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to analyze. Defaults to the current stack")
	cmd.Flags().StringArrayVar(&policyPackPaths, "policy-pack", []string{},
		"Path to a policy pack to run during analysis")
	cmd.Flags().StringArrayVar(&policyPackConfigs, "policy-pack-config", []string{},
		"Path to a JSON config file for the corresponding --policy-pack")

	return cmd
}

// analyzeEvents implements deploy.PolicyEvents and writes human-readable output.
type analyzeEvents struct {
	outLock sync.Mutex
	out     io.Writer
}

func (e *analyzeEvents) OnPolicyViolation(urn resource.URN, d plugin.AnalyzeDiagnostic) {
	var level string
	switch d.EnforcementLevel { //nolint:exhaustive // default case handles other levels
	case apitype.Mandatory:
		level = colors.Always.Colorize(colors.BrightRed + "mandatory" + colors.Reset)
	case apitype.Advisory:
		level = colors.Always.Colorize(colors.Yellow + "advisory" + colors.Reset)
	default:
		level = string(d.EnforcementLevel)
	}

	e.outLock.Lock()
	defer e.outLock.Unlock()

	fmt.Fprintf(e.out, "\n  [%s] %s (%s v%s)\n",
		level, d.PolicyName, d.PolicyPackName, d.PolicyPackVersion)
	if urn != "" {
		fmt.Fprintf(e.out, "  %s\n", urn)
	}
	if d.Description != "" {
		fmt.Fprintf(e.out, "  Description: %s\n", d.Description)
	}
	fmt.Fprintf(e.out, "  %s\n", d.Message)
}

func (e *analyzeEvents) OnPolicyRemediation(
	urn resource.URN,
	rem plugin.Remediation,
	before resource.PropertyMap,
	after resource.PropertyMap,
) {
	e.outLock.Lock()
	defer e.outLock.Unlock()

	fmt.Fprintf(e.out, "\n  [remediation] %s (%s v%s) would change %s\n",
		rem.PolicyName, rem.PolicyPackName, rem.PolicyPackVersion, urn)
	if rem.Description != "" {
		fmt.Fprintf(e.out, "  Description: %s\n", rem.Description)
	}
	for k, newVal := range after {
		if oldVal, ok := before[k]; ok {
			if !oldVal.DeepEquals(newVal) {
				fmt.Fprintf(e.out, "  ~ %s: %v => %v\n", k, oldVal, newVal)
			}
		} else {
			fmt.Fprintf(e.out, "  + %s: %v\n", k, newVal)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			fmt.Fprintf(e.out, "  - %s\n", k)
		}
	}
}

func (e *analyzeEvents) OnPolicyAnalyzeSummary(_ plugin.PolicySummary)      {}
func (e *analyzeEvents) OnPolicyRemediateSummary(_ plugin.PolicySummary)    {}
func (e *analyzeEvents) OnPolicyAnalyzeStackSummary(_ plugin.PolicySummary) {}

// Ensure analyzeEvents satisfies the interface at compile time.
var _ deploy.PolicyEvents = (*analyzeEvents)(nil)
