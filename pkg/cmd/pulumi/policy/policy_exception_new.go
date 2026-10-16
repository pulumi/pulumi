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
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// latestRun is the value of --update and --preview when they're given without one.
const latestRun = "latest"

// policyEventType is the engine event type code of policy violations.
const policyEventType = "9"

type policyExceptionNewArgs struct {
	pack     string
	stacks   []string
	update   string
	preview  string
	policies []string
	targets  []string
	file     string
	reason   string
	note     string
	dryRun   bool
	force    bool
	output   outputflag.OutputFlag[policyExceptionNewRenderFunc]
}

// exceptionRun is the run of a stack whose violations the command read.
type exceptionRun struct {
	Stack string `json:"stack"`
	Kind  string `json:"kind"`
	ID    string `json:"id"`
}

// exceptionViolation is a policy violation that no exception allows yet.
type exceptionViolation struct {
	Stack      string `json:"stack"`
	PolicyPack string `json:"policyPack"`
	Policy     string `json:"policy"`
	Level      string `json:"level"`
	URN        string `json:"urn"`
	Message    string `json:"message,omitempty"`
}

// policyExceptionNewResult is what the command outputs. A dry run fills in everything but the IDs.
type policyExceptionNewResult struct {
	Runs       []exceptionRun                       `json:"runs,omitempty"`
	Violations []exceptionViolation                 `json:"violations,omitempty"`
	Exception  apitype.CreatePolicyExceptionRequest `json:"exception"`
	// Covered lists the exceptions and change requests that already allow the run's other violations.
	Covered []string `json:"covered,omitempty"`
	// ID is the created exception's ID. ChangeRequestID is set instead when the org requires approval.
	ID              string `json:"id,omitempty"`
	ChangeRequestID string `json:"changeRequestId,omitempty"`
	URL             string `json:"url,omitempty"`
	DryRun          bool   `json:"dryRun,omitempty"`
}

type policyExceptionNewRenderFunc func(w io.Writer, result policyExceptionNewResult) error

func newPolicyExceptionNewCmd(factory policyExceptionEnvFactory) *cobra.Command {
	args := policyExceptionNewArgs{output: outputflag.OutputFlag[policyExceptionNewRenderFunc]{
		RenderForTerminal: renderPolicyExceptionNewText,
		RenderJSON: func(w io.Writer, result policyExceptionNewResult) error {
			return writeJSON(w, result)
		},
	}}

	cmd := &cobra.Command{
		Use:     "new [<policy-pack>]",
		Aliases: []string{"create"},
		Short:   "[EXPERIMENTAL] Create a policy exception from a run's violations",
		Long: "[EXPERIMENTAL] Create a policy exception from a run's violations.\n" +
			"\n" +
			"Reads the policy violations of the stack's latest run, update or preview, so you don't have to re-run\n" +
			"it or copy URNs. In a terminal, you pick the violations to except and enter a reason. --policy and\n" +
			"--target select violations without prompting, and --file - reads a whole definition from stdin.\n" +
			"\n" +
			"The exception gets one entry per resource, listing the resource's selected policies. If the org\n" +
			"requires approval to create exceptions, the command requests one in a change request instead.\n" +
			"Violations that an active exception or an open request already allows are skipped, so running the\n" +
			"command twice creates nothing the second time.\n" +
			"\n" +
			"--dry-run --output json prints the run's violations, with their URNs, and the exception the command\n" +
			"would create, so scripts and agents can read the violations from it. Use `pulumi policy issue list`\n" +
			"for the findings of successful updates.\n" +
			"\n" +
			"The command never prompts when --force is set or the terminal isn't interactive. Then a policy pack\n" +
			"and --reason are required, unless the violations are all from one pack and --file has a reason.",
		Example: "  # Pick from the violations of the current stack's latest run\n" +
			"  pulumi policy exception new aws-security\n\n" +
			"  # Except one policy on every resource that violated it\n" +
			"  pulumi policy exception new aws-security --policy required-tags --reason \"Shared buckets (LOG-31)\"\n\n" +
			"  # Two policies on two buckets, across the web-* stacks\n" +
			"  pulumi policy exception new aws-security --stack 'web-*' --reason LOG-31 \\\n" +
			"      --policy required-tags --policy s3-no-public-read \\\n" +
			"      --target urn:pulumi:prod::web::aws:s3/bucket:Bucket::logs \\\n" +
			"      --target urn:pulumi:prod::web::aws:s3/bucket:Bucket::cdn-logs\n\n" +
			"  # Show the violations and the exception, without creating it\n" +
			"  pulumi policy exception new aws-security --dry-run --output json\n\n" +
			"  # Except the mandatory violations, chosen with jq\n" +
			"  pulumi policy exception new aws-security --dry-run --output json \\\n" +
			"    | jq '{reason: \"LOG-31\", match: [.violations[] | select(.level == \"mandatory\")\n" +
			"          | {policies: [.policy], targets: [.urn]}]}' \\\n" +
			"    | pulumi policy exception new aws-security --force --file -",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			if len(posArgs) > 0 {
				args.pack = posArgs[0]
			}
			if args.update != "" && args.preview != "" {
				return cmdCmd.ConfigurationError{Message: "--update and --preview can't be used together"}
			}
			env, err := factory(cmd.Context())
			if err != nil {
				return err
			}
			return runPolicyExceptionNew(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), cmd.InOrStdin(), env, args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "policy-pack", Usage: "[<org>/]<policy-pack>"}},
	})

	flags := cmd.Flags()
	flags.StringArrayVarP(&args.stacks, "stack", "s", nil,
		"The stack whose violations to except; repeatable, and * matches any characters. Defaults to the current stack")
	flags.StringVar(&args.update, "update", "", "Read the violations of this update version (default: the latest)")
	flags.Lookup("update").NoOptDefVal = latestRun
	flags.StringVar(&args.preview, "preview", "", "Read the violations of this preview ID (default: the latest)")
	flags.Lookup("preview").NoOptDefVal = latestRun
	flags.StringArrayVar(&args.policies, "policy", nil, "Only except violations of this policy; repeatable")
	flags.StringArrayVar(&args.targets, "target", nil, "Only except violations on this resource URN; repeatable")
	flags.StringVar(&args.file, "file", "",
		"Read the exception's reason and match entries from this JSON or YAML file, or from stdin with -")
	flags.StringVar(&args.reason, "reason", "", "Why the violations should be allowed")
	flags.StringVar(&args.note, "note", "", "A note for the reviewer, when the org requires approval")
	flags.BoolVar(&args.dryRun, "dry-run", false, "Show the violations and the exception, without creating it")
	flags.BoolVarP(&args.force, "force", "f", false, "Don't prompt or ask for confirmation")
	flags.BoolVar(&args.force, "yes", false, "Alias for --force")
	outputflag.VarP(flags, &args.output)
	return cmd
}

func runPolicyExceptionNew(
	ctx context.Context, stdout, stderr io.Writer, stdin io.Reader,
	env policyExceptionEnv, args policyExceptionNewArgs,
) error {
	interactive := env.interactive && !args.force
	opts := display.Options{Color: cmdutil.GetGlobalColorization(), Stdout: stderr}
	org, pack, err := parsePolicyPackArg(args.pack)
	if err != nil {
		return err
	}

	var result policyExceptionNewResult
	if args.file != "" {
		if pack == "" {
			return cmdCmd.ConfigurationError{Message: "a policy pack is required with --file"}
		}
		def, err := readExceptionDefinition(stdin, args.file)
		if err != nil {
			return err
		}
		if org == "" && len(args.stacks) > 0 {
			stacks, err := env.stacks(ctx, args.stacks)
			if err != nil {
				return err
			}
			org = stacks[0].Owner
		}
		org = cmp.Or(org, env.org)
		result.Exception = apitype.CreatePolicyExceptionRequest{Reason: def.Reason, Match: def.Match}
	} else {
		stacks, err := env.stacks(ctx, args.stacks)
		if err != nil {
			return err
		}
		org = cmp.Or(org, stacks[0].Owner)
		if err := readViolations(ctx, env.client, stacks, args, pack, &result); err != nil {
			return err
		}
		if err := dropCovered(ctx, env.client, org, stacks, &result); err != nil {
			return err
		}
		if len(result.Violations) == 0 {
			if len(result.Covered) > 0 {
				fmt.Fprintf(stderr, "Nothing to except: the violations are already allowed by %s\n",
					strings.Join(result.Covered, ", "))
			} else {
				fmt.Fprintln(stderr, "Nothing to except: the run has no matching violations")
			}
			return args.output.Get()(stdout, result)
		}
		if interactive && !args.dryRun {
			if result.Violations, err = pickViolations(result.Violations, len(stacks) > 1, opts); err != nil {
				return err
			}
		}
		if pack, err = choosePack(pack, result.Violations, interactive && !args.dryRun, args.dryRun, opts); err != nil {
			return err
		}
		if pack != "" {
			result.Violations = slices.DeleteFunc(result.Violations, func(v exceptionViolation) bool {
				return v.PolicyPack != pack
			})
			result.Exception.Match = groupByURN(result.Violations)
		}
	}
	result.Exception.PolicyPack = pack
	result.Exception.Reason = cmp.Or(args.reason, result.Exception.Reason)

	if args.dryRun {
		result.DryRun = true
		return args.output.Get()(stdout, result)
	}

	if result.Exception.Reason == "" {
		if !interactive {
			return cmdCmd.ConfigurationError{Message: "--reason is required when not running interactively"}
		}
		if result.Exception.Reason, err = promptText("Reason", true, opts); err != nil {
			return err
		}
	}

	if interactive {
		def := exceptionDefinition{Reason: result.Exception.Reason, Match: result.Exception.Match}
		fmt.Fprintf(stderr, "\nPolicy pack %s:\n%s\n", pack, configSnippet("EXC-?", def))
		answer, err := ui.PromptUserErr("Create this policy exception?", []string{"yes", "no"}, "yes", opts.Color)
		if err != nil || answer != "yes" {
			return backenderr.CancelledError{Operation: "creating the policy exception"}
		}
	}

	created, err := env.client.CreatePolicyException(ctx, org, result.Exception)
	if err == nil {
		result.ID, result.URL = created.ID, exceptionConsoleURL(env, org, created.ID)
		fmt.Fprintf(stderr, "Created %s\n  %s\n", created.ID, result.URL)
		return args.output.Get()(stdout, result)
	}
	if !requiresApproval(err) {
		return fmt.Errorf("creating the policy exception: %w", err)
	}

	note := args.note
	if note == "" && interactive {
		if note, err = promptText("This org requires approval. Note for the reviewer (optional)", false, opts); err != nil {
			return err
		}
	}
	draft, err := env.client.CreatePolicyExceptionDraft(ctx, org, result.Exception)
	if err != nil {
		return fmt.Errorf("requesting the policy exception: %w", err)
	}
	description := cmp.Or(note, result.Exception.Reason)
	if err := env.client.SubmitChangeRequest(ctx, org, draft.ChangeRequestID, description); err != nil {
		return fmt.Errorf("submitting change request %s: %w", draft.ChangeRequestID, err)
	}
	result.ChangeRequestID, result.URL = draft.ChangeRequestID, changeRequestConsoleURL(env, org, draft.ChangeRequestID)
	fmt.Fprintf(stderr, "Requested the policy exception in change request %s; it's created once approved\n  %s\n",
		draft.ChangeRequestID, result.URL)
	return args.output.Get()(stdout, result)
}

func requiresApproval(err error) bool {
	apiErr, ok := errors.AsType[*apitype.ErrorResponse](err)
	return ok && apiErr.Code == http.StatusConflict && strings.Contains(apiErr.Message, "requires approval")
}

// readExceptionDefinition reads `{reason, match}`, or a version 2 config file with one exception, as JSON or YAML.
func readExceptionDefinition(stdin io.Reader, file string) (exceptionDefinition, error) {
	var data []byte
	var err error
	if file == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(file)
	}
	if err != nil {
		return exceptionDefinition{}, fmt.Errorf("reading the exception: %w", err)
	}

	var doc struct {
		exceptionDefinition `yaml:",inline"`
		Exceptions          map[string]exceptionDefinition `yaml:"exceptions"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return exceptionDefinition{}, cmdCmd.ConfigurationError{Message: fmt.Sprintf("invalid exception: %v", err)}
	}
	def := doc.exceptionDefinition
	if len(doc.Exceptions) > 1 {
		return exceptionDefinition{}, cmdCmd.ConfigurationError{Message: "the file has more than one exception"}
	}
	for _, e := range doc.Exceptions {
		def = e
	}
	if len(def.Match) == 0 {
		return exceptionDefinition{}, cmdCmd.ConfigurationError{Message: "the exception has no match entries"}
	}
	for i, m := range def.Match {
		if len(m.Policies) == 0 || len(m.Targets) == 0 || slices.Contains(m.Policies, "") {
			return exceptionDefinition{}, cmdCmd.ConfigurationError{
				Message: fmt.Sprintf("match entry %d needs at least one policy and one target", i+1),
			}
		}
		for _, t := range m.Targets {
			if _, err := resource.ParseURN(t); err != nil {
				return exceptionDefinition{}, cmdCmd.ConfigurationError{
					Message: fmt.Sprintf("match entry %d: invalid target %q: %v", i+1, t, err),
				}
			}
		}
	}
	return def, nil
}

// runOf picks the run whose violations to read: the one --update or --preview names, or the stack's latest. The
// latest previews are those since the latest update, so a preview, if there is one, is newer than any update.
func runOf(
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

// readViolations reads the violations of each stack's run that pass the pack, --policy and --target filters, and
// that no exception allowed in the run.
func readViolations(
	ctx context.Context, c policyExceptionClient, stacks []client.StackIdentifier,
	args policyExceptionNewArgs, pack string, result *policyExceptionNewResult,
) error {
	type key struct{ pack, policy, urn string }
	seen := map[key]bool{}
	for _, stack := range stacks {
		run, err := runOf(ctx, c, stack, args)
		if err != nil {
			return err
		}
		result.Runs = append(result.Runs, exceptionRun{Stack: stack.String(), Kind: string(run.UpdateKind), ID: run.UpdateID})

		opts := client.GetUpdateEngineEventsOptions{EventTypes: []string{policyEventType}}
		for {
			resp, err := c.GetUpdateEngineEvents(ctx, run, opts)
			if err != nil {
				return fmt.Errorf("reading the policy events of %s %s %s: %w", stack, run.UpdateKind, run.UpdateID, err)
			}
			for _, ev := range resp.Events {
				p := ev.PolicyEvent
				if p == nil || p.ResourceURN == "" {
					continue
				}
				for _, e := range p.Exceptions {
					if !slices.Contains(result.Covered, e.ID) {
						result.Covered = append(result.Covered, e.ID)
					}
				}
				k := key{p.PolicyPackName, p.PolicyName, p.ResourceURN}
				if len(p.Exceptions) > 0 || seen[k] ||
					(pack != "" && p.PolicyPackName != pack) ||
					(len(args.policies) > 0 && !slices.Contains(args.policies, p.PolicyName)) ||
					(len(args.targets) > 0 && !slices.Contains(args.targets, p.ResourceURN)) {
					continue
				}
				seen[k] = true
				result.Violations = append(result.Violations, exceptionViolation{
					Stack:      stack.String(),
					PolicyPack: p.PolicyPackName,
					Policy:     p.PolicyName,
					Level:      p.EnforcementLevel,
					URN:        p.ResourceURN,
					Message:    strings.TrimSpace(p.Message),
				})
			}
			if resp.ContinuationToken == nil || len(resp.Events) == 0 {
				break
			}
			opts.ContinuationToken = resp.ContinuationToken
		}
	}
	// Mandatory violations first.
	rank := func(v exceptionViolation) int {
		if v.Level == "mandatory" {
			return 0
		}
		return 1
	}
	slices.SortStableFunc(result.Violations, func(a, b exceptionViolation) int { return cmp.Compare(rank(a), rank(b)) })
	return nil
}

// dropCovered drops the violations that an active exception or an open request already allows.
func dropCovered(
	ctx context.Context, c policyExceptionClient, org string, stacks []client.StackIdentifier,
	result *policyExceptionNewResult,
) error {
	type covering struct {
		id   string
		pack string
		def  []apitype.PolicyExceptionMatch
	}
	var coverings []covering
	for _, stack := range stacks {
		active, err := c.ListPolicyExceptions(ctx, org, client.ListPolicyExceptionsQuery{
			Stack:  stack.Project + "/" + stack.Stack.String(),
			Status: []string{"active"},
		})
		if err != nil {
			return fmt.Errorf("listing the policy exceptions of %s: %w", stack, err)
		}
		for _, e := range active {
			coverings = append(coverings, covering{e.ID, e.PolicyPack, e.Match})
		}
	}
	requests, err := c.ListChangeRequests(ctx, org, "policy_exceptions")
	if err != nil {
		return fmt.Errorf("listing the open policy exception requests: %w", err)
	}
	for _, cr := range requests {
		if cr.Status != "pending" && cr.Status != "ready" {
			continue
		}
		draft, err := c.GetPolicyExceptionDraft(ctx, org, cr.ID)
		if err != nil {
			return fmt.Errorf("reading change request %s: %w", cr.ID, err)
		}
		coverings = append(coverings, covering{"change request " + cr.ID, draft.PolicyPack, draft.Match})
	}

	result.Violations = slices.DeleteFunc(result.Violations, func(v exceptionViolation) bool {
		for _, cov := range coverings {
			if cov.pack == v.PolicyPack && matches(cov.def, v.Policy, v.URN) {
				if !slices.Contains(result.Covered, cov.id) {
					result.Covered = append(result.Covered, cov.id)
				}
				return true
			}
		}
		return false
	})
	return nil
}

func matches(match []apitype.PolicyExceptionMatch, policy, urn string) bool {
	return slices.ContainsFunc(match, func(m apitype.PolicyExceptionMatch) bool {
		return slices.Contains(m.Policies, policy) && slices.Contains(m.Targets, urn)
	})
}

// groupByURN makes one match entry per resource, listing its policies in the order they were seen.
func groupByURN(violations []exceptionViolation) []apitype.PolicyExceptionMatch {
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

func violationLabel(v exceptionViolation, withStack bool) string {
	urn := resource.URN(v.URN)
	label := fmt.Sprintf("%s/%s  %s %s  (%s)", v.PolicyPack, v.Policy, urn.Type(), urn.Name(), v.Level)
	if withStack {
		label = v.Stack + ": " + label
	}
	return label
}

// pickViolations lets the user select the violations to except. Mandatory violations start selected.
func pickViolations(
	violations []exceptionViolation, withStack bool, opts display.Options,
) ([]exceptionViolation, error) {
	labels := make([]string, len(violations))
	var preselected []string
	for i, v := range violations {
		labels[i] = violationLabel(v, withStack)
		if slices.Contains(labels[:i], labels[i]) {
			labels[i] += fmt.Sprintf(" [%d]", i+1)
		}
		if v.Level == "mandatory" {
			preselected = append(preselected, labels[i])
		}
	}
	picked := ui.PromptUserMulti("Violations to except", labels, preselected, opts.Color)
	if len(picked) == 0 {
		return nil, backenderr.CancelledError{Operation: "creating the policy exception"}
	}
	var selected []exceptionViolation
	for i, v := range violations {
		if slices.Contains(picked, labels[i]) {
			selected = append(selected, v)
		}
	}
	return selected, nil
}

// choosePack returns the pack of the exception: the named one, the violations' only pack, or one the user picks.
// When the violations span several packs and it can't prompt, it fails, or returns "" if ambiguous is allowed.
func choosePack(
	pack string, violations []exceptionViolation, interactive bool, ambiguous bool, opts display.Options,
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
	if len(packs) == 1 {
		return packs[0], nil
	}
	if ambiguous {
		return "", nil
	}
	if !interactive {
		return "", cmdCmd.ConfigurationError{Message: fmt.Sprintf(
			"the violations are from several policy packs (%s); name one as the first argument", strings.Join(packs, ", "))}
	}
	pack, err := ui.PromptUserErr("An exception belongs to one policy pack. Which one?", packs, packs[0], opts.Color)
	if err != nil {
		return "", backenderr.CancelledError{Operation: "creating the policy exception"}
	}
	return pack, nil
}

func promptText(label string, required bool, opts display.Options) (string, error) {
	var validate func(string) error
	if required {
		validate = func(v string) error {
			if v == "" {
				return errors.New("a value is required")
			}
			return nil
		}
	}
	value, err := ui.PromptForValue(false, label, "", false, validate, opts)
	if err != nil {
		return "", backenderr.CancelledError{Operation: "creating the policy exception"}
	}
	return value, nil
}

func renderPolicyExceptionNewText(w io.Writer, r policyExceptionNewResult) error {
	if !r.DryRun {
		if id := cmp.Or(r.ID, r.ChangeRequestID); id != "" {
			_, err := fmt.Fprintln(w, id)
			return err
		}
		return nil
	}
	for _, run := range r.Runs {
		fmt.Fprintf(w, "%s: %s %s\n", run.Stack, run.Kind, run.ID)
	}
	fmt.Fprintln(w, "\nViolations to except:")
	for _, v := range r.Violations {
		fmt.Fprintf(w, "  %s\n    %s\n", violationLabel(v, len(r.Runs) > 1), v.URN)
	}
	if len(r.Covered) > 0 {
		fmt.Fprintf(w, "\nAlready allowed by: %s\n", strings.Join(r.Covered, ", "))
	}
	if r.Exception.PolicyPack == "" {
		_, err := fmt.Fprintln(w, "\nThe violations are from several policy packs; name one to see its exception.")
		return err
	}
	def := exceptionDefinition{Reason: r.Exception.Reason, Match: r.Exception.Match}
	_, err := fmt.Fprintf(w, "\nPolicy pack %s:\n%s", r.Exception.PolicyPack, configSnippet("EXC-?", def))
	return err
}
