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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
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
	var stackName string
	var file string
	var diffDisplay bool
	var jsonDisplay bool
	var policyPackPaths []string
	var policyPackConfigs []string

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze existing resource state against policy packs",
		Long: "Analyze existing resource state against one or more local policy packs.\n" +
			"\n" +
			"This command runs policy analysis against existing resource state without\n" +
			"executing the Pulumi program or making provider calls. Pass --stack to analyze\n" +
			"a stack (defaulting to the current stack), or --file to analyze a state file\n" +
			"(as produced by `pulumi stack export`) without requiring a stack or backend login.\n" +
			"Secrets in the file that can't be decrypted offline are redacted to the\n" +
			"string `[secret]` for analysis, regardless of their original type.\n" +
			"\n" +
			"If any remediation policy fires, the change is reported but the state is not\n" +
			"modified. Exits with a non-zero status if any mandatory violations are found.",
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
					reg := cmdCmd.NewDefaultRegistry(ctx, lm, ws, nil, cmdutil.Diag(), env.Global())
					pluginHost, err := pkghost.New(context.WithoutCancel(ctx), cmdutil.Diag(), cmdutil.Diag(),
						nil, pkgWorkspace.EnsureLanguageInstalled,
						schema.NewLoaderServerFromContext, convert.NewMapperServerFromContext,
						packageworkspace.NewResolverServer(reg))
					if err != nil {
						return nil, nil, fmt.Errorf("creating plugin host: %w", err)
					}
					pctx, err := plugin.NewContext(ctx, cmdutil.Diag(), cmdutil.Diag(),
						pluginHost, nil, cwd, nil, true, nil)
					if err != nil {
						contract.IgnoreClose(pluginHost)
						return nil, nil, fmt.Errorf("creating plugin context: %w", err)
					}
					analyzers, err := engine.LoadLocalPolicyPackAnalyzers(ctx, pctx, packs, nil)
					if err != nil {
						contract.IgnoreClose(pctx)
						contract.IgnoreClose(pluginHost)
						return nil, nil, err
					}
					return analyzers, func() { contract.IgnoreClose(pctx); contract.IgnoreClose(pluginHost) }, nil
				}
			}

			// Resolve the snapshot to analyze, either from a state file or a live stack.
			// In file mode there is no backend stack; the display layer only reads the
			// reference's Name() and Project(), so a synthetic reference suffices.
			var snap *deploy.Snapshot
			var stackRef backend.StackReference
			var sourceDesc string
			if file != "" {
				f, err := os.Open(file)
				if err != nil {
					return fmt.Errorf("could not open file: %w", err)
				}
				defer contract.IgnoreClose(f)

				// Decode into a json.RawMessage-backed UntypedDeployment so unrecognized
				// fields round-trip, then deserialize into a typed snapshot.
				var deployment apitype.UntypedDeployment
				if err := json.NewDecoder(f).Decode(&deployment); err != nil {
					return fmt.Errorf("reading deployment from %s: %w", file, err)
				}

				snap, err = stack.DeserializeUntypedDeployment(ctx, &deployment, secretsProvider)
				if err != nil {
					// Secrets couldn't be decrypted offline (e.g. a service-backed export whose
					// secrets manager needs a login this invocation lacks). Retry with secrets
					// redacted so analysis still runs; policies then see "[secret]". A non-secrets
					// failure recurs identically below, so we surface the original err.
					var redactErr error
					snap, redactErr = stack.DeserializeUntypedDeployment(ctx, &deployment, blindingSecretsProvider{})
					if redactErr != nil {
						return fmt.Errorf("deserializing deployment from %s: %w", file, err)
					}
					fmt.Fprintf(cmd.ErrOrStderr(),
						"warning: could not decrypt secrets in %s; analyzing with secret values redacted\n", file)
				}

				// AnalyzeSnapshot dereferences each URN (URN.Name asserts the urn:pulumi:
				// prefix), so reject a structurally invalid URN here instead of panicking.
				for _, r := range snap.Resources {
					if !r.URN.IsValid() {
						return fmt.Errorf("state file %s contains a resource with an invalid URN %q", file, r.URN)
					}
				}

				stackRef = stackReferenceForFile(snap)
				sourceDesc = "File " + file
			} else {
				displayOpts := display.Options{Color: cmdutil.GetGlobalColorization()}
				s, err := cmdStack.RequireStack(ctx, cmdutil.Diag(), ws, lm, stackName, cmdStack.LoadOnly, displayOpts, "")
				if err != nil {
					return err
				}

				snap, err = s.Snapshot(ctx, secretsProvider)
				if err != nil {
					return fmt.Errorf("loading stack snapshot: %w", err)
				}

				stackRef = s.Ref()
				sourceDesc = fmt.Sprintf("Stack %s", stackRef)
			}
			if len(snap.Resources) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s has no resources; nothing to analyze.\n", sourceDesc)
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
			events, finish, err := newAnalyzeEvents(
				cmd.Context(),
				cmd.OutOrStdout(), cmd.ErrOrStderr(), cmdutil.GetGlobalColorization(),
				diffDisplay, jsonDisplay, stackRef, analyzers)
			if err != nil {
				return fmt.Errorf("configuring analysis display: %w", err)
			}
			hasMandatory, err := deploy.AnalyzeSnapshot(ctx, snap, analyzers, events)
			result := apitype.OperationResultFromError(err)
			if err == nil && hasMandatory {
				result = apitype.OperationResultFailed
			}
			finish(result)
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

	cmd.Flags().StringVarP(&stackName, "stack", "s", "",
		"The name of the stack to analyze. Defaults to the current stack")
	cmd.Flags().StringVar(&file, "file", "",
		"Analyze a state file (as produced by `pulumi stack export`) instead of a live stack")
	cmd.Flags().BoolVar(&diffDisplay, "diff", false,
		"Display policy diagnostics as a rich diff instead of grouped progress output")
	cmd.Flags().BoolVarP(&jsonDisplay, "json", "j", false,
		"Serialize policy analysis events as JSON")
	cmd.Flags().StringArrayVar(&policyPackPaths, "policy-pack", []string{},
		"Path to a policy pack to run during analysis")
	cmd.Flags().StringArrayVar(&policyPackConfigs, "policy-pack-config", []string{},
		"Path to a JSON config file for the corresponding --policy-pack")

	cmd.MarkFlagsMutuallyExclusive("json", "diff")
	cmd.MarkFlagsMutuallyExclusive("file", "stack")

	return cmd
}

// stackReferenceForFile builds a synthetic stack reference for --file mode (no backend stack).
// The display reads only Name()/Project(): a file whose resources share one stack/project is
// labeled with it; a heterogeneous selection (common for an Insights export spanning projects or
// stacks) gets a neutral "multiple" so the label doesn't imply a single origin; a file with no
// analyzable URN falls back to "unknown".
func stackReferenceForFile(snap *deploy.Snapshot) backend.StackReference {
	ref := &backend.MockStackReference{
		StringV:  "<unknown>",
		NameV:    tokens.MustParseStackName("unknown"),
		ProjectV: "unknown",
	}

	var stackName tokens.StackName
	var project tokens.Name
	found, homogeneous := false, true
	for _, r := range snap.Resources {
		if !r.URN.IsValid() {
			continue
		}
		sn, err := tokens.ParseStackName(string(r.URN.Stack()))
		if err != nil {
			continue
		}
		proj := tokens.Name(r.URN.Project())
		if !found {
			stackName, project, found = sn, proj, true
		} else if sn != stackName || proj != project {
			homogeneous = false
			break
		}
	}

	switch {
	case found && homogeneous:
		ref.NameV = stackName
		ref.ProjectV = project
		ref.StringV = stackName.String()
	case found:
		ref.NameV = tokens.MustParseStackName("multiple")
		ref.ProjectV = "multiple"
		ref.StringV = "<multiple>"
	}
	return ref
}

// blindingSecretsProvider builds secrets managers that redact every secret to
// config.BlindingCrypter's "[secret]" sentinel instead of decrypting it. --file mode falls
// back to it when the real provider can't decrypt a state file offline (e.g. a service-backed
// export with no local login), so analysis runs instead of failing.
type blindingSecretsProvider struct{}

func (blindingSecretsProvider) OfType(
	_ context.Context, ty string, state json.RawMessage,
) (secrets.Manager, error) {
	return blindingSecretsManager{ty: ty, state: state}, nil
}

type blindingSecretsManager struct {
	ty    string
	state json.RawMessage
}

func (m blindingSecretsManager) Type() string                { return m.ty }
func (m blindingSecretsManager) State() json.RawMessage      { return m.state }
func (m blindingSecretsManager) Encrypter() config.Encrypter { return config.BlindingCrypter }
func (m blindingSecretsManager) Decrypter() config.Decrypter { return redactingDecrypter{} }

type redactingDecrypter struct{}

// DecryptValue returns config.BlindingCrypter's "[secret]" sentinel, JSON-encoded:
// deployment deserialization unmarshals each decrypted plaintext as a JSON value, so the
// bare sentinel (not valid JSON) can't be returned directly.
func (redactingDecrypter) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	redacted, err := config.BlindingCrypter.DecryptValue(ctx, ciphertext)
	if err != nil {
		return "", err
	}
	plaintext, err := json.Marshal(redacted)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (d redactingDecrypter) BatchDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	return config.DefaultBatchDecrypt(ctx, d, ciphertexts)
}

// analyzeEvents implements deploy.PolicyEvents and writes human-readable output.
type analyzeEvents struct {
	outLock sync.Mutex
	out     io.Writer
	opts    display.Options
	seen    map[resource.URN]engine.StepEventMetadata
	events  chan<- engine.Event
	json    *json.Encoder
	seq     int
}

func newAnalyzeEvents(
	ctx context.Context,
	out io.Writer,
	errOut io.Writer,
	colorization colors.Colorization,
	diffDisplay bool,
	jsonDisplay bool,
	stackRef backend.StackReference,
	analyzers []plugin.Analyzer,
) (*analyzeEvents, func(apitype.OperationResult), error) {
	if jsonDisplay {
		policyPacks, err := policyPackVersions(ctx, analyzers)
		if err != nil {
			return nil, nil, err
		}
		events := &analyzeEvents{
			out:  out,
			json: json.NewEncoder(out),
		}
		return events, func(result apitype.OperationResult) {
			events.outLock.Lock()
			defer events.outLock.Unlock()
			events.writeEvent(engine.NewEvent(engine.SummaryEventPayload{
				IsPreview:       false,
				MaybeCorrupt:    false,
				Duration:        0 * time.Second,
				ResourceChanges: nil,
				PolicyPacks:     policyPacks,
				Result:          result,
			}))
		}, nil
	}

	if diffDisplay {
		return &analyzeEvents{
			out: out,
			opts: display.Options{
				Color: colorization,
			},
			seen: make(map[resource.URN]engine.StepEventMetadata),
		}, func(apitype.OperationResult) {}, nil
	}

	policyPacks, err := policyPackVersions(ctx, analyzers)
	if err != nil {
		return nil, nil, err
	}

	displayType := display.DisplayProgress
	opts := display.Options{
		Color:                  colorization,
		Type:                   displayType,
		ShowPolicyRemediations: true,
		IsInteractive:          cmdutil.Interactive(),
		JSONDisplay:            jsonDisplay,
		Stdout:                 out,
		Stderr:                 errOut,
	}
	events := make(chan engine.Event)
	done := make(chan bool)
	project, _ := stackRef.Project()
	go display.ShowEvents(
		"analyze", apitype.UpdateUpdate,
		stackRef.Name(), tokens.PackageName(project),
		"", events, done, opts, false)

	return &analyzeEvents{
			events: events,
		}, func(result apitype.OperationResult) {
			events <- engine.NewEvent(engine.SummaryEventPayload{
				IsPreview:       false,
				MaybeCorrupt:    false,
				Duration:        0 * time.Second,
				ResourceChanges: nil,
				PolicyPacks:     policyPacks,
				Result:          result,
			})
			close(events)
			<-done
		}, nil
}

func policyPackVersions(ctx context.Context, analyzers []plugin.Analyzer) (map[string]string, error) {
	policyPacks := map[string]string{}
	for _, analyzer := range analyzers {
		info, err := analyzer.GetAnalyzerInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting analyzer info: %w", err)
		}
		policyPacks[info.Name] = info.Version
	}
	return policyPacks, nil
}

func (e *analyzeEvents) writeEvent(event engine.Event) {
	if e.events != nil {
		e.events <- event
		return
	}

	if e.json != nil {
		apiEvent, err := display.ConvertEngineEvent(event, false /*showSecrets*/)
		if err != nil {
			return
		}
		apiEvent.Sequence = e.seq
		apiEvent.Timestamp = int(time.Now().Unix())
		e.seq++
		_ = e.json.Encode(apiEvent)
		return
	}

	msg := display.RenderDiffEvent(event, 0, e.seen, e.opts)
	if msg == "" {
		return
	}
	fmt.Fprint(e.out, msg)
}

func (e *analyzeEvents) OnPolicyViolation(urn resource.URN, d plugin.AnalyzeDiagnostic) {
	e.outLock.Lock()
	defer e.outLock.Unlock()
	e.writeEvent(engine.NewEvent(engine.PolicyViolationEventPayload{
		ResourceURN:       urn,
		Message:           d.Message,
		PolicyName:        d.PolicyName,
		PolicyPackName:    d.PolicyPackName,
		PolicyPackVersion: d.PolicyPackVersion,
		EnforcementLevel:  d.EnforcementLevel,
		Severity:          d.Severity,
	}))
}

func (e *analyzeEvents) OnPolicyRemediation(
	urn resource.URN,
	rem plugin.Remediation,
	before property.Map,
	after property.Map,
) {
	e.outLock.Lock()
	defer e.outLock.Unlock()
	e.writeEvent(engine.NewEvent(engine.PolicyRemediationEventPayload{
		ResourceURN:       urn,
		PolicyName:        rem.PolicyName,
		PolicyPackName:    rem.PolicyPackName,
		PolicyPackVersion: rem.PolicyPackVersion,
		Before:            before,
		After:             after,
	}))
}

// Summary callbacks are intentionally no-ops: the progress, diff, and JSON
// display renderers all drop PolicyAnalyzeSummary/PolicyRemediateSummary/
// PolicyAnalyzeStackSummary events. The "Policies:" section in the progress
// display is built from per-violation events, not aggregate summaries.
func (e *analyzeEvents) OnPolicyAnalyzeSummary(_ plugin.PolicySummary)      {}
func (e *analyzeEvents) OnPolicyRemediateSummary(_ plugin.PolicySummary)    {}
func (e *analyzeEvents) OnPolicyAnalyzeStackSummary(_ plugin.PolicySummary) {}

// Ensure analyzeEvents satisfies the interface at compile time.
var _ deploy.PolicyEvents = (*analyzeEvents)(nil)
