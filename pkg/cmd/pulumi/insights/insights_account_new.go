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

package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cloud"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// insightsAccountNewClient is the subset of cloud-API operations the `new`
// command needs. Defined inside this package so unit tests can stub it without
// touching the full HTTP client surface.
type insightsAccountNewClient interface {
	CreateInsightsAccount(
		ctx context.Context, org, account string, req apitype.CreateInsightsAccountRequest,
	) error
	GetInsightsAccount(
		ctx context.Context, org, account string,
	) (apitype.InsightsAccount, error)
	// ListESCEnvironments backs the interactive environment picker. Returns
	// a single page; the picker doesn't paginate — if a user has more envs
	// than fit on a page, the "Enter manually" option covers them.
	ListESCEnvironments(
		ctx context.Context, org, continuationToken string,
	) ([]apitype.ESCEnvironment, string, error)
}

// accountNewClientFactory resolves the cloud client and the effective org for
// the call. orgOverride wins when non-empty; otherwise the default org from
// the cloud context is used. A helpful error is returned when the caller is
// not logged in or no org can be determined.
type accountNewClientFactory func(
	ctx context.Context, orgOverride string,
) (insightsAccountNewClient, string, error)

// accountNewArgs holds the resolved flag values for `pulumi insights account
// new`. Required values may be filled in interactively when the corresponding
// flag isn't passed.
type accountNewArgs struct {
	org            string
	provider       string
	environment    string
	scanSchedule   string
	agentPoolID    string
	providerConfig string
}

func newInsightsAccountNewCmd() *cobra.Command {
	return newInsightsAccountNewCmdWith(nil)
}

// newInsightsAccountNewCmdWith builds the `pulumi insights account new`
// command. factory produces the cloud client and resolves the effective org;
// pass nil to use the production factory backed by [cloud.ResolveContext].
func newInsightsAccountNewCmdWith(factory accountNewClientFactory) *cobra.Command {
	args := accountNewArgs{}
	var yes bool
	output := outputflag.OutputFlag[insightsAccountRenderFunc]{
		RenderForTerminal: renderInsightsAccountText,
		RenderJSON:        renderInsightsAccountJSON,
	}

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new Insights account",
		Long: "[EXPERIMENTAL] Create a new Pulumi Insights account.\n\n" +
			"An Insights account represents a cloud provider account (e.g. AWS,\n" +
			"Azure, GCP, OCI, Kubernetes) configured for resource discovery.",
		Example: "  # Walk through prompts to create an account\n" +
			"  pulumi insights account new prod-aws\n\n" +
			"  # Create an AWS Insights account using credentials from an ESC environment\n" +
			"  pulumi insights account new prod-aws \\\n" +
			"      --provider aws --environment infra/prod-aws-creds\n\n" +
			"  # Override the organization and schedule a daily scan\n" +
			"  pulumi insights account new prod-aws --org acme \\\n" +
			"      --provider aws --environment infra/prod-aws-creds \\\n" +
			"      --scan-schedule daily\n\n" +
			"  # Pass provider-specific configuration as inline JSON\n" +
			"  pulumi insights account new prod-aws \\\n" +
			"      --provider aws --environment infra/prod-aws-creds \\\n" +
			"      --provider-config '{\"regions\":[\"us-east-1\",\"us-west-2\"]}'",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			if factory == nil {
				factory = defaultAccountNewClientFactory
			}
			ctx := cmd.Context()
			// Resolve the cloud client and effective org up front so the
			// interactive env picker can list the org's ESC environments.
			c, org, err := factory(ctx, args.org)
			if err != nil {
				return err
			}
			skipPrompts := yes || !cmdutil.Interactive()
			opts := display.Options{Color: cmdutil.GetGlobalColorization()}
			resolved, err := resolveAccountNewArgs(ctx, skipPrompts, c, org, args, opts)
			if err != nil {
				return err
			}
			return runInsightsAccountNew(
				ctx, cmd.OutOrStdout(), c, org, posArgs[0], resolved, output.Get(),
			)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "name"}},
		Required:  1,
	})

	cmd.Flags().StringVar(&args.org, "org", "",
		"Organization that will own the Insights account (defaults to the current default org)")
	cmd.Flags().StringVar(&args.provider, "provider", "",
		"Cloud provider for the account. One of: aws, gcp, azure-native, oci, kubernetes")
	cmd.Flags().StringVar(&args.environment, "environment", "",
		"ESC environment containing provider credentials, "+
			"in the form project/environment with an optional @version suffix")
	cmd.Flags().StringVar(&args.scanSchedule, "scan-schedule", "",
		"Automated scan schedule. One of: none, 12h, daily")
	cmd.Flags().StringVar(&args.agentPoolID, "agent-pool-id", "",
		"ID of the agent pool to run discovery workflows (defaults to the org's default pool)")
	cmd.Flags().StringVar(&args.providerConfig, "provider-config", "",
		"Provider-specific configuration as an inline JSON object (e.g. '{\"regions\":[\"us-east-1\"]}')")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false,
		"Skip prompts; fail if any required value is missing")
	outputflag.Var(cmd.Flags(), &output)

	return cmd
}

// resolveAccountNewArgs prompts for any required values not provided via
// flags. When skipPrompts is true (--yes or non-interactive), missing values
// return an error instead of prompting. The ESC environment prompt asks the
// service for the org's environments and presents them as a picker; if the
// list call fails or comes back empty we silently fall back to free-text
// input so the picker never blocks the user.
func resolveAccountNewArgs(
	ctx context.Context,
	skipPrompts bool,
	c insightsAccountNewClient,
	org string,
	args accountNewArgs,
	opts display.Options,
) (accountNewArgs, error) {
	if strings.TrimSpace(args.provider) == "" {
		if skipPrompts {
			return args, errors.New("--provider is required")
		}
		// The server validates the enum, so we let unknown values through
		// rather than hard-coding the list of providers in two places.
		// Showing the documented options as prompt entries keeps the
		// happy-path obvious.
		args.provider = ui.PromptUserSkippable(
			false, "Cloud provider",
			[]string{"aws", "gcp", "azure-native", "oci", "kubernetes"},
			"aws", opts.Color)
	}
	if strings.TrimSpace(args.environment) == "" {
		if skipPrompts {
			return args, errors.New("--environment is required")
		}
		env, err := promptForESCEnvironment(ctx, c, org, newDisplayPrompter(opts))
		if err != nil {
			return args, err
		}
		args.environment = env
	}
	return args, nil
}

// escEnvManualOption is the label of the picker entry that lets a user
// bail out of the listed choices and type an environment by hand. Exposed
// as a constant so the test for [escEnvironmentChoices] can assert on it.
const escEnvManualOption = "[Enter manually]"

// escEnvironmentChoices builds the picker option list for the org's ESC
// environments. Returns nil when the listing call errors or comes back
// empty — the caller treats that signal as "fall back to free-text input."
// Otherwise it returns the envs formatted as `project/name`, sorted, with
// [escEnvManualOption] appended as an escape hatch.
//
// Pulled out of [promptForESCEnvironment] so the data-shaping logic is
// unit-testable without driving the survey/stdin interactions.
func escEnvironmentChoices(
	ctx context.Context, c insightsAccountNewClient, org string,
) []string {
	envs, _, err := c.ListESCEnvironments(ctx, org, "")
	if err != nil || len(envs) == 0 {
		return nil
	}
	options := make([]string, 0, len(envs)+1)
	for _, e := range envs {
		options = append(options, e.Project+"/"+e.Name)
	}
	sort.Strings(options)
	options = append(options, escEnvManualOption)
	return options
}

// escEnvPrompter is the small interface promptForESCEnvironment uses to
// reach the UI. Splitting it out lets unit tests substitute stubs in place
// of the survey-driven [ui.PromptUser]/[ui.PromptForValue] calls, which
// otherwise block waiting on stdin.
type escEnvPrompter struct {
	// pick shows a picker over options and returns the chosen entry, or
	// "" when the user dismissed the prompt.
	pick func(options []string) string
	// enter prompts the user to type an environment reference, with the
	// usual non-empty validation.
	enter func() (string, error)
}

// newDisplayPrompter wires the picker / free-text prompt up to the live UI
// for production callers. The display options come from the cobra RunE so
// the prompts inherit the user's global color settings.
func newDisplayPrompter(opts display.Options) escEnvPrompter {
	return escEnvPrompter{
		pick: func(options []string) string {
			return ui.PromptUser("ESC environment", options, options[0], opts.Color)
		},
		enter: func() (string, error) {
			return ui.PromptForValue(
				false,
				"ESC environment (project/environment[@version])",
				"", false,
				nonEmptyValidator("an ESC environment reference"),
				opts,
			)
		},
	}
}

// promptForESCEnvironment offers a picker over the org's ESC environments
// (the same set surfaced by `pulumi esc ls`). The list call is best-effort:
// if it fails or returns no environments, the prompt degrades to free-text
// input so an offline-ish or empty-org user can still proceed. The picker
// always offers an "Enter manually" choice so users can specify a
// `@version` suffix or any env the server doesn't yet list.
func promptForESCEnvironment(
	ctx context.Context, c insightsAccountNewClient, org string, p escEnvPrompter,
) (string, error) {
	options := escEnvironmentChoices(ctx, c, org)
	if len(options) == 0 {
		return p.enter()
	}
	choice := p.pick(options)
	if choice == "" || choice == escEnvManualOption {
		return p.enter()
	}
	return choice, nil
}

// nonEmptyValidator returns a validator function suitable for
// [ui.PromptForValue] that rejects empty strings with a helpful message.
func nonEmptyValidator(what string) func(string) error {
	return func(v string) error {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%s is required", what)
		}
		return nil
	}
}

// runInsightsAccountNew creates the account and then reads it back so the
// caller can render the structure the new convention asks for. ctx and w are
// decoupled from cobra so the function is straightforward to drive from
// tests; c and org are pre-resolved by the caller (the cobra RunE) so the
// env picker in resolveAccountNewArgs can share them.
func runInsightsAccountNew(
	ctx context.Context,
	w io.Writer,
	c insightsAccountNewClient,
	org, name string,
	args accountNewArgs,
	render insightsAccountRenderFunc,
) error {
	// Post-resolve sanity: required values must be set. These would already
	// have been caught by resolveAccountNewArgs in the cobra path, but Run
	// can be called directly from tests so we defend ourselves.
	if strings.TrimSpace(args.provider) == "" {
		return errors.New("--provider is required")
	}
	if strings.TrimSpace(args.environment) == "" {
		return errors.New("--environment is required")
	}

	// Validate --provider-config eagerly so a malformed value surfaces
	// locally rather than as a generic server-side 400. The server contract
	// is "object optional," so non-object JSON is rejected here.
	providerConfig, err := parseProviderConfig(args.providerConfig)
	if err != nil {
		return err
	}

	req := apitype.CreateInsightsAccountRequest{
		Provider:       args.provider,
		Environment:    args.environment,
		ScanSchedule:   args.scanSchedule,
		AgentPoolID:    args.agentPoolID,
		ProviderConfig: providerConfig,
	}
	if err := c.CreateInsightsAccount(ctx, org, name, req); err != nil {
		return fmt.Errorf("creating insights account: %w", err)
	}

	// The CreateAccount endpoint returns 204 No Content. The CLI convention
	// for `new` commands is "respond with the edited structure," so we read
	// the account back. If the read fails the account still exists — make
	// that visible in the error so the user knows the create succeeded.
	account, err := c.GetInsightsAccount(ctx, org, name)
	if err != nil {
		return fmt.Errorf(
			"insights account %q was created in organization %q, "+
				"but reading it back failed: %w\n"+
				"use `pulumi insights account list` to verify",
			name, org, err)
	}

	return render(w, account)
}

// parseProviderConfig validates that --provider-config is a JSON object (or
// empty). The server's contract is "object optional," so non-object JSON
// (numbers, strings, arrays) is rejected here rather than relying on the
// server's error message, which is generic.
func parseProviderConfig(raw string) (json.RawMessage, error) {
	if raw == "" {
		return nil, nil
	}
	var probe map[string]any
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		return nil, fmt.Errorf("invalid --provider-config: must be a JSON object: %w", err)
	}
	return json.RawMessage(raw), nil
}

// insightsAccountRenderFunc is the renderer signature shared by `new` and
// (when it lands) `get`. Both produce a single account in the same shape, so
// the renderers belong with the first command that needs them.
type insightsAccountRenderFunc func(w io.Writer, account apitype.InsightsAccount) error

// renderInsightsAccountJSON writes the account as indented JSON. Indentation
// matches the rest of the cli/cloud commands so jq-style scripting feels
// consistent.
func renderInsightsAccountJSON(w io.Writer, account apitype.InsightsAccount) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(account)
}

// renderInsightsAccountText writes a human-readable key/value view of the
// account to w. The layout intentionally mirrors `stack schedule get` for
// visual consistency across the cli/cloud command family.
func renderInsightsAccountText(w io.Writer, a apitype.InsightsAccount) error {
	line := func(label, value string) {
		fmt.Fprintf(w, "%-22s %s\n", label+":", value)
	}
	provider := a.Provider
	if a.ProviderVersion != "" {
		provider = fmt.Sprintf("%s (v%s)", a.Provider, a.ProviderVersion)
	}
	scanSchedule := "off"
	if a.ScheduledScanEnabled {
		scanSchedule = "enabled"
	}
	agentPool := a.AgentPoolID
	if agentPool == "" {
		agentPool = "(default)"
	}
	owner := a.OwnedBy.Name
	if a.OwnedBy.GitHubLogin != "" {
		owner = fmt.Sprintf("%s (%s)", a.OwnedBy.Name, a.OwnedBy.GitHubLogin)
	}

	line("ID", a.ID)
	line("Name", a.Name)
	line("Owner", owner)
	line("Provider", provider)
	if a.ProviderEnvRef != "" {
		line("Environment", a.ProviderEnvRef)
	}
	line("Scheduled scan", scanSchedule)
	line("Agent pool", agentPool)

	// ProviderConfig is schemaless from the CLI's perspective — its shape
	// depends on which provider Insights is configured for, so we render
	// it as indented JSON rather than guessing at known fields. We only
	// print it when non-empty so an account without any provider-specific
	// config doesn't show an empty section.
	if len(a.ProviderConfig) > 0 {
		// Pretty-print if it's valid JSON; otherwise fall back to the raw
		// bytes so the user still sees what the service returned.
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, a.ProviderConfig, "", "  "); err == nil {
			fmt.Fprintf(w, "Provider config:\n%s\n", pretty.String())
		} else {
			line("Provider config", string(a.ProviderConfig))
		}
	}

	// ScanStatus is absent before the first scan completes; only render it
	// when the server set one. We surface the most useful fields (status,
	// finished, resource count) without dumping the entire job tree.
	if a.ScanStatus != nil {
		line("Scan status", a.ScanStatus.Status)
		if a.ScanStatus.FinishedAt != nil && !a.ScanStatus.FinishedAt.IsZero() {
			line("Last scan finished", a.ScanStatus.FinishedAt.UTC().Format(time.RFC3339))
		}
		if a.ScanStatus.ResourceCount > 0 {
			line("Resources discovered", strconv.FormatInt(a.ScanStatus.ResourceCount, 10))
		}
	}
	return nil
}

// defaultAccountNewClientFactory is the production wiring for
// accountNewClientFactory. It parallels defaultClientFactory in
// insights_resource_get.go and defaultAccountListClientFactory in
// insights_account_list.go; the duplication is small enough not to justify a
// shared helper yet, but a fourth command should consolidate.
func defaultAccountNewClientFactory(
	ctx context.Context, orgOverride string,
) (insightsAccountNewClient, string, error) {
	resolved, err := cloud.ResolveContext(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("resolving cloud context: %w", err)
	}
	if !resolved.LoggedIn {
		return nil, "", errors.New("not logged in to Pulumi Cloud; run `pulumi login` first")
	}

	org := orgOverride
	if org == "" {
		org = resolved.OrgName
	}
	if org == "" {
		return nil, "", errors.New(
			"no organization available; pass --org or set a default with `pulumi org set-default`")
	}

	return resolved.Client, org, nil
}
