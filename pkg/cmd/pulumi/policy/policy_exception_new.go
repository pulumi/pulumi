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
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// latestRun is the value of --update and --preview when they're passed without one.
const latestRun = "latest"

// policyEventType is the engine event type code of policy violations.
const policyEventType = "9"

// changeRequestApplied is the status of a change request whose change was made.
const changeRequestApplied = "applied"

type policyExceptionNewArgs struct {
	policyPack   string
	stacks       []string
	update       string
	preview      string
	policies     []string
	targets      []string
	file         string
	reason       string
	dryRun       bool
	draft        bool
	yes          bool
	outputFormat outputflag.OutputFlag[policyExceptionNewRenderFunc]
}

// policyExceptionStackResolver resolves a stack name, or the current stack when the name is empty.
type policyExceptionStackResolver func(ctx context.Context, name string) (client.StackIdentifier, error)

// policyExceptionRun is the run of a stack whose violations the command read.
type policyExceptionRun struct {
	Stack string `json:"stack"`
	Kind  string `json:"kind"`
	ID    string `json:"id"`
}

// policyExceptionViolation is a policy violation that no exception allows yet.
type policyExceptionViolation struct {
	Stack      string `json:"stack"`
	PolicyPack string `json:"policyPack"`
	Policy     string `json:"policy"`
	Level      string `json:"level"`
	URN        string `json:"urn"`
}

// policyExceptionNewResult is what the command outputs. A dry run fills in everything but the ID.
type policyExceptionNewResult struct {
	OrganizationName string                     `json:"organizationName"`
	Runs             []policyExceptionRun       `json:"runs"`
	Violations       []policyExceptionViolation `json:"violations"`
	// Covered lists the exceptions that already allow the runs' other violations.
	Covered []string `json:"covered,omitempty"`
	// Exception is the exception to create. It's empty when there was nothing to except.
	Exception apitype.CreatePolicyExceptionRequest `json:"exception,omitzero"`
	// ID is the created exception's ID. It's empty for a dry run, when there was nothing to except, or while the
	// exception awaits approval.
	ID string `json:"id,omitempty"`
	// ChangeRequestID is the change request that requested the exception, when the organization requires approval.
	ChangeRequestID string `json:"changeRequestId,omitempty"`
	// ChangeRequestStatus is the change request's status after it was submitted; `applied` once the exception exists.
	ChangeRequestStatus string `json:"changeRequestStatus,omitempty"`
	// ChangeRequestURL is the console page of the change request.
	ChangeRequestURL string `json:"changeRequestUrl,omitempty"`
	DryRun           bool   `json:"dryRun,omitempty"`
}

// policyExceptionNewRenderFunc writes the result to stdout, and status messages to stderr.
type policyExceptionNewRenderFunc func(stdout, stderr io.Writer, result policyExceptionNewResult) error

func defaultPolicyExceptionNewOutputFormat() outputflag.OutputFlag[policyExceptionNewRenderFunc] {
	return outputflag.OutputFlag[policyExceptionNewRenderFunc]{
		RenderForTerminal: renderPolicyExceptionNewText,
		RenderJSON: func(stdout, _ io.Writer, result policyExceptionNewResult) error {
			return ui.FprintJSON(stdout, result)
		},
	}
}

func newPolicyExceptionNewCmd() *cobra.Command {
	var args policyExceptionNewArgs
	args.outputFormat = defaultPolicyExceptionNewOutputFormat()

	cmd := &cobra.Command{
		Use:     "new [<policy-pack>]",
		Aliases: []string{"create"},
		Short:   "[EXPERIMENTAL] Create a policy exception from a run's violations",
		Long: "[EXPERIMENTAL] Create a policy exception from a run's violations.\n" +
			"\n" +
			"Reads the policy violations of the stack's latest run, update or preview, so you don't have to\n" +
			"re-run it or copy URNs. In a terminal, you pick the violations to allow and enter a reason.\n" +
			"--policy and --target select violations without prompting, and --file reads a whole exception\n" +
			"instead.\n" +
			"\n" +
			"The exception belongs to the Policy Pack named as <policy-pack> or <org>/<policy-pack>; the\n" +
			"organization defaults to the stack's. It gets one entry per resource, listing the resource's\n" +
			"selected policies.\n" +
			"Violations that an active exception already allows are skipped, so running the command twice\n" +
			"creates nothing the second time.\n" +
			"\n" +
			"When the organization requires approval for policy exceptions, or --draft is passed, the\n" +
			"command creates a change request for the exception instead and submits it, with the reason as\n" +
			"its description. The exception is created once the change request is approved.\n" +
			"\n" +
			"stdout gets only the created exception's ID, or the result with --output json; status messages\n" +
			"go to stderr. --dry-run --output json prints the run's violations, with their URNs, and the\n" +
			"exception the command would create, without creating it.\n" +
			"\n" +
			"The command doesn't prompt when --yes is passed or the terminal isn't interactive. Then --reason\n" +
			"is required, unless --file has one, and so is the Policy Pack when the violations are from\n" +
			"several.",
		Example: "  # Pick from the violations of the current stack's latest run\n" +
			"  pulumi policy exception new aws-security\n\n" +
			"  # Except one policy on every resource that violated it\n" +
			"  pulumi policy exception new aws-security --policy required-tags --reason \"Shared buckets (LOG-31)\"\n\n" +
			"  # Read the violations of update 12 instead of the latest run\n" +
			"  pulumi policy exception new aws-security --update=12 --dry-run\n\n" +
			"  # Show the violations and the exception, without creating it\n" +
			"  pulumi policy exception new aws-security --dry-run --output json | jq '.violations'\n\n" +
			"  # Create an exception from a file, or from stdin with -\n" +
			"  pulumi policy exception new aws-security --file exception.yaml\n\n" +
			"  # Request the exception in a change request, even if no approval is required\n" +
			"  pulumi policy exception new aws-security --policy required-tags --reason \"LOG-31\" --draft\n\n" +
			"  # See where violations are recorded, or the findings of a stack\n" +
			"  pulumi stack history events latest --event-type 9 --output json\n" +
			"  pulumi policy issue list",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			if len(posArgs) > 0 {
				args.policyPack = posArgs[0]
			}
			args.yes = args.yes || env.SkipConfirmations.Value()
			return runPolicyExceptionNew(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), cmd.InOrStdin(),
				defaultPolicyExceptionClientFactory, resolvePolicyExceptionStack, cmdutil.Interactive(), args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "policy-pack"},
		},
	})

	flags := cmd.Flags()
	flags.StringArrayVarP(&args.stacks, "stack", "s", nil,
		"The stack whose violations to read; may be repeated. Defaults to the current stack")
	flags.StringVar(&args.update, "update", "",
		"Read the violations of an update, as --update=<version> (default: the latest)")
	flags.Lookup("update").NoOptDefVal = latestRun
	flags.StringVar(&args.preview, "preview", "",
		"Read the violations of a preview, as --preview=<id> (default: the latest)")
	flags.Lookup("preview").NoOptDefVal = latestRun
	flags.StringArrayVar(&args.policies, "policy", nil, "Only except violations of this policy; may be repeated")
	flags.StringArrayVar(&args.targets, "target", nil, "Only except violations on this resource URN; may be repeated")
	flags.StringVar(&args.file, "file", "",
		"Read the exception's reason and match entries from this JSON or YAML file, or from stdin with -")
	flags.StringVar(&args.reason, "reason", "", "Why it's safe to allow the violations")
	flags.BoolVar(&args.dryRun, "dry-run", false, "Show the violations and the exception, without creating it")
	flags.BoolVar(&args.draft, "draft", false,
		"Request the exception in a change request, even if the organization doesn't require approval")
	flags.BoolVarP(&args.yes, "yes", "y", false, "Skip prompts and confirmation")
	outputflag.VarP(flags, &args.outputFormat)

	return cmd
}

func resolvePolicyExceptionStack(ctx context.Context, name string) (client.StackIdentifier, error) {
	_, id, err := cmdStack.RequireCloudStack(
		ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, name)
	return id, err
}

func runPolicyExceptionNew(
	ctx context.Context, stdout, stderr io.Writer, stdin io.Reader,
	factory policyExceptionClientFactory, resolveStack policyExceptionStackResolver,
	interactive bool, args policyExceptionNewArgs,
) error {
	interactive = interactive && !args.yes && !args.dryRun
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}
	org, pack, err := parsePolicyPackArg(args.policyPack)
	if err != nil {
		return err
	}
	if args.update != "" && args.preview != "" {
		return cmdCmd.ConfigurationError{Message: "--update and --preview can't be used together"}
	}
	render := func(result policyExceptionNewResult) error {
		return args.outputFormat.Get()(stdout, stderr, result)
	}

	result := policyExceptionNewResult{Runs: []policyExceptionRun{}, Violations: []policyExceptionViolation{}}
	var c policyExceptionClients
	if args.file != "" {
		if pack == "" {
			return cmdCmd.ConfigurationError{Message: "a Policy Pack is required with --file"}
		}
		if len(args.stacks) > 0 || args.update != "" || args.preview != "" ||
			len(args.policies) > 0 || len(args.targets) > 0 {
			return cmdCmd.ConfigurationError{
				Message: "--file can't be used with --stack, --update, --preview, --policy or --target",
			}
		}
		if result.Exception.PolicyException, err = readPolicyExceptionFile(stdin, args.file); err != nil {
			return err
		}
		if c, org, err = factory(ctx, org); err != nil {
			return err
		}
	} else {
		names := args.stacks
		if len(names) == 0 {
			names = []string{""}
		}
		var stacks []client.StackIdentifier
		for _, name := range names {
			stack, err := resolveStack(ctx, name)
			if err != nil {
				return err
			}
			stacks = append(stacks, stack)
		}
		if c, org, err = factory(ctx, cmp.Or(org, stacks[0].Owner)); err != nil {
			return err
		}
		if err := readPolicyExceptionViolations(ctx, c, org, stacks, pack, args, &result); err != nil {
			return err
		}
		if len(result.Violations) == 0 {
			result.OrganizationName = org
			return render(result)
		}
		if interactive {
			if result.Violations, err = pickPolicyExceptionViolations(result.Violations, len(stacks) > 1, opts); err != nil {
				return err
			}
		}
		if pack, err = choosePolicyExceptionPack(pack, result.Violations, interactive, args.dryRun, opts); err != nil {
			return err
		}
		if pack != "" {
			result.Violations = slices.DeleteFunc(result.Violations, func(v policyExceptionViolation) bool {
				return v.PolicyPack != pack
			})
			result.Exception.Match = groupPolicyExceptionMatch(result.Violations)
		}
	}
	result.OrganizationName = org
	result.Exception.PolicyPack = pack
	result.Exception.Reason = cmp.Or(args.reason, result.Exception.Reason)

	if args.dryRun {
		result.DryRun = true
		return render(result)
	}

	if result.Exception.Reason == "" {
		if !interactive {
			return cmdCmd.ConfigurationError{Message: "--reason is required when not running interactively"}
		}
		reason, err := ui.PromptForValue(false, "reason for the exception", "", false, func(v string) error {
			if v == "" {
				return errors.New("a reason is required")
			}
			return nil
		}, opts)
		if err != nil {
			return backenderr.CancelledError{Operation: "creating the policy exception"}
		}
		result.Exception.Reason = reason
	}

	if interactive {
		fmt.Fprintf(stderr, "\nPolicy Pack %s:\n", pack)
		if err := fprintPolicyExceptionConfig(stderr, "EXC-?", result.Exception.PolicyException); err != nil {
			return err
		}
		answer, err := ui.PromptUserErr("Create this policy exception?", []string{"yes", "no"}, "yes", opts.Color)
		if err != nil || answer != "yes" {
			return backenderr.CancelledError{Operation: "creating the policy exception"}
		}
	}

	if err := createPolicyException(ctx, c, org, args.draft, &result); err != nil {
		return err
	}
	return render(result)
}

// createPolicyException creates the result's exception. With --draft, or when the organization requires approval
// (the create returns a 409 saying so), the exception is requested in a change request instead, which is submitted
// with the reason as its description. A change request that needs no approvals is applied on submission, creating
// the exception.
func createPolicyException(
	ctx context.Context, c policyExceptionClients, org string, draft bool, result *policyExceptionNewResult,
) error {
	if !draft {
		created, err := c.CreatePolicyException(ctx, org, result.Exception)
		if err == nil {
			result.ID = created.ID
			return nil
		}
		// An organization without the feature also returns a 409, which the message tells apart.
		apiErr, ok := errors.AsType[*apitype.ErrorResponse](err)
		if !ok || apiErr.Code != http.StatusConflict || !strings.Contains(apiErr.Message, "requires approval") {
			return err
		}
	}

	created, err := c.CreatePolicyExceptionDraft(ctx, org, result.Exception)
	if err != nil {
		return err
	}
	id := created.ChangeRequestID
	result.ChangeRequestID = id
	if u := client.CloudConsoleURL(c.URL(), org, "approvals"); u != "" {
		result.ChangeRequestURL = u + "?requestId=" + url.QueryEscape(id)
	}
	// A change request's description is limited to 1000 characters, half of a reason's.
	reason := []rune(result.Exception.Reason)
	description := string(reason[:min(len(reason), 1000)])
	if err := c.changeRequests.SubmitChangeRequest(ctx, org, id, &description); err != nil {
		return fmt.Errorf("submitting change request %s: %w", id, err)
	}
	cr, err := c.changeRequests.GetChangeRequest(ctx, org, id)
	if err != nil {
		return fmt.Errorf("getting change request %s: %w", id, err)
	}
	result.ChangeRequestStatus = cr.Status
	if cr.Status != changeRequestApplied {
		return nil
	}

	// The change request doesn't name the exception it created, so find it among the newest of its Policy Pack.
	listOpts := client.ListPolicyExceptionsOptions{PolicyPack: result.Exception.PolicyPack}
	resp, err := c.ListPolicyExceptions(ctx, org, listOpts)
	if err != nil {
		return err
	}
	if i := slices.IndexFunc(resp.Exceptions, func(e apitype.PolicyExceptionInfo) bool {
		return e.ChangeRequestID == id
	}); i >= 0 {
		result.ID = resp.Exceptions[i].ID
	}
	return nil
}

// parsePolicyPackArg splits `[<org>/]<policy-pack>`. The org is empty when the argument doesn't name one.
func parsePolicyPackArg(s string) (string, string, error) {
	org, pack, found := strings.Cut(s, "/")
	if !found {
		return "", s, nil
	}
	if org == "" || pack == "" || strings.Contains(pack, "/") {
		return "", "", cmdCmd.ConfigurationError{
			Message: fmt.Sprintf("invalid Policy Pack %q: expected [<org>/]<policy-pack>", s),
		}
	}
	return org, pack, nil
}

// readPolicyExceptionFile reads `{reason, match}`, or a version 2 config file with one exception, as JSON or YAML.
func readPolicyExceptionFile(stdin io.Reader, file string) (apitype.PolicyException, error) {
	var data []byte
	var err error
	if file == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(file)
	}
	if err != nil {
		return apitype.PolicyException{}, fmt.Errorf("reading the exception: %w", err)
	}

	var doc struct {
		apitype.PolicyException `yaml:",inline"`
		Exceptions              map[string]apitype.PolicyException `yaml:"exceptions"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return apitype.PolicyException{}, cmdCmd.ConfigurationError{Message: fmt.Sprintf("invalid exception: %v", err)}
	}
	if len(doc.Exceptions) > 1 {
		return apitype.PolicyException{}, cmdCmd.ConfigurationError{Message: "the file has more than one exception"}
	}
	e := doc.PolicyException
	for _, v := range doc.Exceptions {
		e = v
	}
	if len(e.Match) == 0 {
		return apitype.PolicyException{}, cmdCmd.ConfigurationError{Message: "the exception has no match entries"}
	}
	for i, m := range e.Match {
		if len(m.Policies) == 0 || len(m.Targets) == 0 || slices.Contains(m.Policies, "") {
			return apitype.PolicyException{}, cmdCmd.ConfigurationError{
				Message: fmt.Sprintf("match entry %d needs at least one policy and one target", i+1),
			}
		}
		for _, t := range m.Targets {
			if _, err := resource.ParseURN(t); err != nil {
				return apitype.PolicyException{}, cmdCmd.ConfigurationError{
					Message: fmt.Sprintf("match entry %d: invalid target %q: %v", i+1, t, err),
				}
			}
		}
	}
	return e, nil
}

// policyExceptionRunOf picks the run whose violations to read: the one --update or --preview names, or the stack's
// latest. The latest previews are those since the latest update, so a preview, if there is one, is the newer run.
func policyExceptionRunOf(
	ctx context.Context, c policyExceptionClient, stack client.StackIdentifier, args policyExceptionNewArgs,
) (client.UpdateIdentifier, error) {
	run := client.UpdateIdentifier{StackIdentifier: stack, UpdateKind: apitype.UpdateUpdate}
	switch {
	case args.preview != "" && args.preview != latestRun:
		run.UpdateKind, run.UpdateID = apitype.PreviewUpdate, args.preview
		return run, nil
	case args.update != "" && args.update != latestRun:
		run.UpdateID = args.update
		return run, nil
	}

	if args.update == "" {
		previews, err := c.GetLatestStackPreviews(ctx, stack)
		if err != nil {
			return run, fmt.Errorf("getting the latest preview of %s: %w", stack, err)
		}
		if len(previews) > 0 {
			run.UpdateKind, run.UpdateID = apitype.PreviewUpdate, previews[0].UpdateID
			return run, nil
		}
		if args.preview != "" {
			return run, fmt.Errorf("%s has no previews since its latest update", stack)
		}
	}
	updates, err := c.GetStackUpdates(ctx, stack, 1, 1)
	if err != nil {
		return run, fmt.Errorf("getting the latest update of %s: %w", stack, err)
	}
	if len(updates) == 0 {
		return run, fmt.Errorf("%s has no updates or previews; run `pulumi preview` first", stack)
	}
	run.UpdateID = strconv.Itoa(updates[0].Version)
	return run, nil
}

// readPolicyExceptionViolations reads the violations of each stack's run that pass the Policy Pack, --policy and
// --target filters, and that no exception allowed in the run or allows now. Mandatory violations come first.
func readPolicyExceptionViolations(
	ctx context.Context, c policyExceptionClient, org string, stacks []client.StackIdentifier,
	pack string, args policyExceptionNewArgs, result *policyExceptionNewResult,
) error {
	addCovered := func(id string) {
		if !slices.Contains(result.Covered, id) {
			result.Covered = append(result.Covered, id)
		}
	}

	for _, stack := range stacks {
		run, err := policyExceptionRunOf(ctx, c, stack, args)
		if err != nil {
			return err
		}
		result.Runs = append(result.Runs, policyExceptionRun{
			Stack: stack.String(), Kind: string(run.UpdateKind), ID: run.UpdateID,
		})

		// The run's events don't know about exceptions created since, so those are checked separately.
		var active []apitype.PolicyExceptionInfo
		listOpts := client.ListPolicyExceptionsOptions{Stack: stack.Project + "/" + stack.Stack.String()}
		for {
			resp, err := c.ListPolicyExceptions(ctx, org, listOpts)
			if err != nil {
				return err
			}
			active = append(active, resp.Exceptions...)
			if resp.ContinuationToken == "" {
				break
			}
			listOpts.ContinuationToken = resp.ContinuationToken
		}

		eventOpts := client.GetUpdateEngineEventsOptions{EventTypes: []string{policyEventType}}
		for {
			resp, err := c.GetUpdateEngineEvents(ctx, run, eventOpts)
			if err != nil {
				return fmt.Errorf("reading the policy events of %s %s %s: %w", stack, run.UpdateKind, run.UpdateID, err)
			}
			for _, ev := range resp.Events {
				p := ev.PolicyEvent
				if p == nil || p.ResourceURN == "" ||
					(pack != "" && p.PolicyPackName != pack) ||
					(len(args.policies) > 0 && !slices.Contains(args.policies, p.PolicyName)) ||
					(len(args.targets) > 0 && !slices.Contains(args.targets, p.ResourceURN)) {
					continue
				}
				if len(p.Exceptions) > 0 {
					for _, e := range p.Exceptions {
						addCovered(e.ID)
					}
					continue
				}
				if i := slices.IndexFunc(active, func(e apitype.PolicyExceptionInfo) bool {
					return e.PolicyPack == p.PolicyPackName && slices.ContainsFunc(e.Match,
						func(m apitype.PolicyExceptionMatch) bool {
							return slices.Contains(m.Policies, p.PolicyName) && slices.Contains(m.Targets, p.ResourceURN)
						})
				}); i >= 0 {
					addCovered(active[i].ID)
					continue
				}
				v := policyExceptionViolation{
					Stack:      stack.String(),
					PolicyPack: p.PolicyPackName,
					Policy:     p.PolicyName,
					Level:      p.EnforcementLevel,
					URN:        p.ResourceURN,
				}
				if !slices.Contains(result.Violations, v) {
					result.Violations = append(result.Violations, v)
				}
			}
			if resp.ContinuationToken == nil || len(resp.Events) == 0 {
				break
			}
			eventOpts.ContinuationToken = resp.ContinuationToken
		}
	}

	rank := func(v policyExceptionViolation) int {
		if v.Level == "mandatory" {
			return 0
		}
		return 1
	}
	slices.SortStableFunc(result.Violations, func(a, b policyExceptionViolation) int {
		return cmp.Compare(rank(a), rank(b))
	})
	slices.Sort(result.Covered)
	return nil
}

// groupPolicyExceptionMatch makes one match entry per resource, listing its policies in the order they were seen.
func groupPolicyExceptionMatch(violations []policyExceptionViolation) []apitype.PolicyExceptionMatch {
	var match []apitype.PolicyExceptionMatch
	index := map[string]int{}
	for _, v := range violations {
		i, ok := index[v.URN]
		if !ok {
			i = len(match)
			index[v.URN] = i
			match = append(match, apitype.PolicyExceptionMatch{Targets: []string{v.URN}})
		}
		if !slices.Contains(match[i].Policies, v.Policy) {
			match[i].Policies = append(match[i].Policies, v.Policy)
		}
	}
	return match
}

func policyExceptionViolationLabel(v policyExceptionViolation, withStack bool) string {
	urn := resource.URN(v.URN)
	label := fmt.Sprintf("%s/%s  %s %s  (%s)", v.PolicyPack, v.Policy, urn.Type(), urn.Name(), v.Level)
	if withStack {
		label = v.Stack + ": " + label
	}
	return label
}

// pickPolicyExceptionViolations lets the user select the violations to allow. Mandatory violations start selected.
func pickPolicyExceptionViolations(
	violations []policyExceptionViolation, withStack bool, opts display.Options,
) ([]policyExceptionViolation, error) {
	labels := make([]string, len(violations))
	var preselected []string
	for i, v := range violations {
		labels[i] = policyExceptionViolationLabel(v, withStack)
		// Violations differ by their full URN, which the label shortens, so keep labels unique for the prompt.
		if slices.Contains(labels[:i], labels[i]) {
			labels[i] += fmt.Sprintf(" [%d]", i+1)
		}
		if v.Level == "mandatory" {
			preselected = append(preselected, labels[i])
		}
	}
	picked := ui.PromptUserMulti("Violations to allow", labels, preselected, opts.Color)
	if len(picked) == 0 {
		return nil, backenderr.CancelledError{Operation: "creating the policy exception"}
	}
	var selected []policyExceptionViolation
	for i, v := range violations {
		if slices.Contains(picked, labels[i]) {
			selected = append(selected, v)
		}
	}
	return selected, nil
}

// choosePolicyExceptionPack returns the exception's Policy Pack: the named one, the violations' only one, or one the
// user picks. When the violations are from several and it can't prompt, a dry run gets "" and anything else fails.
func choosePolicyExceptionPack(
	pack string, violations []policyExceptionViolation, interactive, dryRun bool, opts display.Options,
) (string, error) {
	if pack != "" {
		return pack, nil
	}
	var packs []string
	for _, v := range violations {
		if !slices.Contains(packs, v.PolicyPack) {
			packs = append(packs, v.PolicyPack)
		}
	}
	switch {
	case len(packs) == 1:
		return packs[0], nil
	case dryRun:
		return "", nil
	case !interactive:
		return "", cmdCmd.ConfigurationError{Message: fmt.Sprintf(
			"the violations are from several Policy Packs (%s); name one as the first argument",
			strings.Join(packs, ", "))}
	}
	pack, err := ui.PromptUserErr("An exception belongs to one Policy Pack. Which one?", packs, packs[0], opts.Color)
	if err != nil {
		return "", backenderr.CancelledError{Operation: "creating the policy exception"}
	}
	return pack, nil
}

func renderPolicyExceptionNewText(stdout, stderr io.Writer, r policyExceptionNewResult) error {
	if len(r.Violations) == 0 && len(r.Exception.Match) == 0 {
		if len(r.Covered) > 0 {
			fmt.Fprintf(stderr, "No violations to except; they're already allowed by %s.\n", strings.Join(r.Covered, ", "))
		} else {
			fmt.Fprintln(stderr, "No violations to except.")
		}
		return nil
	}
	if !r.DryRun {
		if r.ChangeRequestID != "" {
			fmt.Fprintf(stderr, "Change request created: %s\n", r.ChangeRequestID)
			if r.ChangeRequestURL != "" {
				fmt.Fprintf(stderr, "Change request URL: %s\n", r.ChangeRequestURL)
			}
			fmt.Fprintln(stderr, "Change request submitted")
		}
		switch {
		case r.ID != "":
			fmt.Fprintf(stderr, "Created policy exception %s in organization %s.\n", r.ID, r.OrganizationName)
			fmt.Fprintln(stdout, r.ID)
		case r.ChangeRequestStatus == changeRequestApplied:
			fmt.Fprintf(stderr, "Created the policy exception in organization %s.\n", r.OrganizationName)
		default:
			fmt.Fprintln(stderr, "The policy exception is created once the change request is approved.")
		}
		return nil
	}

	for _, run := range r.Runs {
		fmt.Fprintf(stdout, "%s: %s %s\n", run.Stack, run.Kind, run.ID)
	}
	if len(r.Violations) > 0 {
		fmt.Fprintln(stdout, "\nViolations to except:")
		for _, v := range r.Violations {
			fmt.Fprintf(stdout, "  %s\n    %s\n", policyExceptionViolationLabel(v, len(r.Runs) > 1), v.URN)
		}
	}
	if len(r.Covered) > 0 {
		fmt.Fprintf(stdout, "\nAlready allowed by %s.\n", strings.Join(r.Covered, ", "))
	}
	if r.Exception.PolicyPack == "" {
		fmt.Fprintln(stdout, "\nThe violations are from several Policy Packs; name one to see its exception.")
		return nil
	}
	fmt.Fprintf(stdout, "\nPolicy Pack %s:\n", r.Exception.PolicyPack)
	return fprintPolicyExceptionConfig(stdout, "EXC-?", r.Exception.PolicyException)
}
